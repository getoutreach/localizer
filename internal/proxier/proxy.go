// Copyright 2020 Jared Allard
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package proxier

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"k8s.io/kubectl/pkg/polymorphichelpers"
	"k8s.io/kubectl/pkg/util/podutils"
)

// Proxier handles creating an maintaining proxies to a remote
// Kubernetes service
type Proxier struct {
	k kubernetes.Interface

	// stores
	podStore  cache.Store
	servStore cache.Store

	rest  rest.Interface
	kconf *rest.Config
	log   logrus.FieldLogger

	s []Service

	// active{,services,Pods} are mapping indexes for
	// ProxyConnections
	active         map[uint]*ProxyConnection
	activeServices map[string][]*ProxyConnection
	activePods     map[string][]*ProxyConnection
}

// ProxyConnection tracks a proxy connection
type ProxyConnection struct {
	proxier *Proxier
	fw      *portforward.PortForwarder

	LocalPort  uint
	RemotePort uint

	Service Service
	Pod     corev1.Pod

	// Active denotes if this connection is active
	// or not
	Active bool
}

// GetPort returns the port as a string local:remote
func (pc *ProxyConnection) GetPort() string {
	return fmt.Sprintf("%d:%d", pc.LocalPort, pc.RemotePort)
}

// Start starts a proxy connection
func (pc *ProxyConnection) Start(ctx context.Context) error {
	req := pc.proxier.rest.Post().
		Resource("pods").
		Namespace(pc.Pod.Namespace).
		Name(pc.Pod.Name).
		SubResource("portforward")

	transport, upgrader, err := spdy.RoundTripperFor(pc.proxier.kconf)
	if err != nil {
		return err
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", req.URL())

	pc.proxier.log.Debugf("creating port-forward: %s", pc.GetPort())
	fw, err := portforward.New(dialer, []string{pc.GetPort()}, ctx.Done(), nil, ioutil.Discard, os.Stdout)
	if err != nil {
		return err
	}
	pc.fw = fw

	pc.Active = true

	return fw.ForwardPorts()
}

// Close closes the current proxy connection and marks it as
// no longer being active
func (pc *ProxyConnection) Close() error {
	pc.Active = false
	pc.fw.Close()

	// we'll return an error one day
	return nil
}

// NewProxier creates a new proxier instance
func NewProxier(k kubernetes.Interface, kconf *rest.Config, l logrus.FieldLogger) *Proxier {
	return &Proxier{
		k:     k,
		kconf: kconf,
		rest:  k.CoreV1().RESTClient(),
		log:   l,
		s:     make([]Service, 0),

		active:         make(map[uint]*ProxyConnection),
		activePods:     make(map[string][]*ProxyConnection),
		activeServices: make(map[string][]*ProxyConnection),
	}
}

func (p *Proxier) handleInformerEvent(event string, obj interface{}) {
	// we don't currently process add
	if event == "add" {
		return
	}

	item := ""
	switch obj.(type) {
	case *corev1.Pod:
		item = "pod"
	case *corev1.Service:
		item = "service"
	default:
		// skip unknown types
		return
	}

	k, _ := cache.MetaNamespaceKeyFunc(obj)
	p.log.WithField(item, k).Debugf("%s %s", item, event)

	switch item {
	case "pod":
		refreshServices := make([]Service, len(p.activePods[k]))
		refreshPorts := make([]string, len(p.activePods[k]))
		for i, pc := range p.activePods[k] {
			refreshServices[i] = pc.Service
			refreshPorts[i] = pc.GetPort()
			pc.Close()
		}

		// reset the activePods
		p.activePods[k] = nil

		if len(refreshPorts) > 0 {
			p.log.WithField("ports", refreshPorts).
				Warnf("port-forward for %s is being refreshed due to underlying pod being destroyed", k)
		}

		for _, s := range refreshServices {
			b := backoff.NewExponentialBackOff()
			for {
				// TODO: do we want to limit amount of time we wait?
				if err := p.createProxy(context.TODO(), &s); err != nil {
					wait := b.NextBackOff()
					p.log.Warnf("failed to refresh port-forward for %s: %v (trying again in %s)", k, err, wait.String())

					time.Sleep(wait)
					continue
				}

				// if we didn't error, then we exit the loop
				break
			}

			p.log.WithField("ports", refreshPorts).
				Infof("refreshed port-forward(s) for '%s'", k)
		}

	case "service":
		removedPorts := make([]string, len(p.activeServices[k]))
		for i, pc := range p.activeServices[k] {
			removedPorts[i] = pc.GetPort()
			pc.Close()
		}

		// reset the activeServices section for this service
		p.activeServices[k] = nil

		if len(removedPorts) > 0 {
			p.log.WithField("ports", removedPorts).
				Warnf("port-forward for %s has been destroyed due to the underlying service being deleted", k)
		}
	}
}

// Start starts the internal informer
func (p *Proxier) Start(ctx context.Context) error {
	podStore, podInformer := cache.NewInformer(
		cache.NewListWatchFromClient(p.k.CoreV1().RESTClient(), "pods", v1.NamespaceAll, fields.Everything()),
		&corev1.Pod{},
		time.Second*0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				p.handleInformerEvent("add", obj)
			},
			DeleteFunc: func(obj interface{}) {
				p.handleInformerEvent("delete", obj)
			},
		},
	)

	servStore, servInformer := cache.NewInformer(
		cache.NewListWatchFromClient(p.k.CoreV1().RESTClient(), "services", v1.NamespaceAll, fields.Everything()),
		&corev1.Service{},
		time.Second*0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				p.handleInformerEvent("add", obj)
			},
			DeleteFunc: func(obj interface{}) {
				p.handleInformerEvent("delete", obj)
			},
		},
	)

	p.servStore = servStore
	p.podStore = podStore

	// start the informer
	go podInformer.Run(ctx.Done())
	go servInformer.Run(ctx.Done())

	if ok := cache.WaitForCacheSync(ctx.Done(), podInformer.HasSynced, servInformer.HasSynced); !ok {
		return fmt.Errorf("failed to populate informer cache")
	}

	return nil
}

// Add adds a service to our proxier. When Proxy() is called
// this service will be proxied.
func (p *Proxier) Add(s ...Service) error {
	p.s = append(p.s, s...)

	return nil
}

// findPodBySelector finds a pod by a given selector on a runtime.Object
func (p *Proxier) findPodBySelector(o runtime.Object) (*corev1.Pod, error) {
	namespace, selector, err := polymorphichelpers.SelectorsForObject(o)
	if err != nil {
		return nil, fmt.Errorf("cannot attach to %T: %v", o, err)
	}

	sortBy := func(pods []*corev1.Pod) sort.Interface { return sort.Reverse(podutils.ActivePods(pods)) }
	pod, _, err := polymorphichelpers.GetFirstPod(p.k.CoreV1(), namespace, selector.String(), 1*time.Minute, sortBy)
	return pod, err
}

// createProxy creates a proxy connection
func (p *Proxier) createProxy(ctx context.Context, s *Service) error {
	item, exists, err := p.servStore.GetByKey(fmt.Sprintf("%s/%s", s.Namespace, s.Name))
	if err != nil {
		p.log.Errorf("failed to get service: %v", err)
		return errors.Wrap(err, "failed to get service")
	} else if !exists {
		// TODO(jaredallard): handle this... if it ever happens?
		p.log.Errorf("service wasn't found in cache")
		return fmt.Errorf("failed to find service in cache")
	}

	kserv := item.(*corev1.Service)
	pod, err := p.findPodBySelector(kserv)
	if err != nil {
		p.log.Errorf("failed to find pod for service '%s': %v", kserv.Name, err)
		return fmt.Errorf("failed to find any pods")
	}

	serviceKey, _ := cache.MetaNamespaceKeyFunc(kserv)
	podKey, _ := cache.MetaNamespaceKeyFunc(pod)

	if pod.Status.Phase != corev1.PodRunning {
		return fmt.Errorf("selected pod wasn't running, got status: %v", pod.Status.Phase)
	}

	for _, port := range s.Ports {
		if port.LocalPort <= 1024 {
			return fmt.Errorf("pod requested a privledged port")
		}

		// ap stores the connections
		ap := p.active[port.LocalPort]
		if ap != nil && ap.Active {
			// Check if a different service than us is using that port already
			// if it is, drop a log
			if ap.Service.Name != s.Name && ap.Service.Namespace != s.Namespace {
				p.log.Warnf(
					"skipping port-forward for '%s/%s:%d', '%s/%s' is using that port already",
					s.Namespace, s.Name, port.LocalPort, ap.Service.Namespace, ap.Service.Name,
				)
			}

			// skip ports that are already in use
			continue
		}

		p.log.Infof("creating port-forward '%s/%s:%d' -> '127.0.0.1:%d'", s.Namespace, s.Name, port.RemotePort, port.LocalPort)

		// build the linking tables
		// port -> conn
		p.active[port.LocalPort] = &ProxyConnection{
			p,
			nil,

			port.LocalPort,
			port.RemotePort,
			*s,
			*pod,
			false,
		}
		conn := p.active[port.LocalPort]

		// service -> []Conn
		if p.activeServices[serviceKey] == nil {
			p.activeServices[serviceKey] = make([]*ProxyConnection, 0)
		}
		p.activeServices[serviceKey] = append(p.activeServices[serviceKey], conn)

		// pod -> []Conn
		if p.activePods[podKey] == nil {
			p.activePods[podKey] = make([]*ProxyConnection, 0)
		}
		p.activePods[podKey] = append(p.activePods[podKey], conn)

		// start the proxy
		if err := conn.Start(ctx); err != nil {
			p.log.Errorf(
				"failed to start proxy for '%s/%s:%d' -> ':%d': %v",
				s.Namespace, s.Name, port.RemotePort, port.LocalPort, err,
			)
		}
	}

	return nil
}

// Proxy starts a proxier. The proxy thread is run in a go-routine
// so it is safe to execute this function and continue.
func (p *Proxier) Proxy(ctx context.Context) error {
	for _, s := range p.s {
		// TODO(jaredallard): Before GA we'll need to make
		// sure that there is a cordinated backoff/retry system
		p.createProxy(ctx, &s)
	}

	return nil
}
