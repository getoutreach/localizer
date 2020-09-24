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
	"os/exec"
	"reflect"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/metal-stack/go-ipam"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/txn2/txeh"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubectl/pkg/polymorphichelpers"
	"k8s.io/kubectl/pkg/util/podutils"
)

// Proxier handles creating an maintaining proxies to a remote
// Kubernetes service
type Proxier struct {
	k     kubernetes.Interface
	hosts *txeh.Hosts

	// stores
	podStore  cache.Store
	servStore cache.Store

	rest  rest.Interface
	kconf *rest.Config
	log   logrus.FieldLogger

	// ip address store
	ipam       ipam.Ipamer
	ipamPrefix *ipam.Prefix

	s []Service

	// active{,services,Pods} are mapping indexes for
	// ProxyConnections
	connMutex  sync.Mutex
	ipMutex    sync.Mutex
	serviceIPs map[string]*ipam.IP

	activeServices map[string]*ProxyConnection
	activePods     map[string]*ProxyConnection
}

// NewProxier creates a new proxier instance
func NewProxier(k kubernetes.Interface, kconf *rest.Config, l logrus.FieldLogger) *Proxier {
	hosts, err := txeh.NewHosts(&txeh.HostsConfig{})
	if err != nil {
		l.Fatalf("failed to open hosts file: %v", err)
	}

	ipamInstance := ipam.New()
	prefix, err := ipamInstance.NewPrefix("127.0.0.1/8")
	if err != nil {
		panic(err)
	}

	// ensure that 127.0.0.1 is never allocated
	_, err = ipamInstance.AcquireSpecificIP(prefix.Cidr, "127.0.0.1")
	if err != nil {
		panic(err)
	}

	return &Proxier{
		k:          k,
		hosts:      hosts,
		kconf:      kconf,
		rest:       k.CoreV1().RESTClient(),
		log:        l,
		s:          make([]Service, 0),
		ipam:       ipamInstance,
		ipamPrefix: prefix,

		serviceIPs:     make(map[string]*ipam.IP),
		activePods:     make(map[string]*ProxyConnection),
		activeServices: make(map[string]*ProxyConnection),
	}
}

func (p *Proxier) handleInformerEvent(ctx context.Context, event string, obj interface{}) { //nolint:funlen,gocyclo
	k, _ := cache.MetaNamespaceKeyFunc(obj)

	item := ""
	switch tobj := obj.(type) {
	case *corev1.Pod:
		item = "pod"
	case *corev1.Service:
		item = "service"
	case *ProxyConnection:
		item = "connection"
		k = tobj.Service.GetKey()
	default:
		// skip unknown types
		p.log.WithFields(logrus.Fields{
			"event": event,
			"type":  reflect.TypeOf(obj).String(),
			"key":   k,
		}).Debug("ignored event")
		return
	}

	// we don't currently process add
	if event == "add" {
		// skip unknown types
		p.log.WithFields(logrus.Fields{
			"event": event,
			"type":  reflect.TypeOf(obj).String(),
			"key":   k,
		}).Debug("skipped event")
		return
	}

	p.log.WithFields(logrus.Fields{
		"item":  item,
		"event": event,
		"key":   k,
	}).Debugf("got event")

	switch item {
	case "pod", "connection":
		if item == "connection" {
			// if the connection died, we assume that the pod was lost
			// so, we mimic the pod dead event
			pc := obj.(*ProxyConnection)
			k, _ = cache.MetaNamespaceKeyFunc(&pc.Pod)
			p.log.Warnf("underlying connection died for %s", k)
		}

		// skip pods we don't know about
		p.connMutex.Lock()
		pc := p.activePods[k]
		if pc == nil {
			p.connMutex.Unlock()
			return
		}

		if err := pc.Close(); err != nil {
			p.log.WithField("service", k).WithError(err).Debug("failed to close proxy connection")
		}

		s := pc.Service
		serviceKey := s.GetKey()

		// reset the activePods
		p.activePods[k] = nil
		p.connMutex.Unlock()

		p.log.Warnf("tunnel for %s is being refreshed due to underlying pod being destroyed", serviceKey)
		ticker := backoff.NewTicker(backoff.NewExponentialBackOff())
		for {
			select {
			case <-ticker.C:
				if err := p.createProxy(ctx, &s); err != nil { //nolint:scopelint
					p.log.WithError(err).Warnf("failed to refresh tunnel for %s (trying again)", serviceKey)
					continue
				}
				ticker.Stop()
				p.log.Infof("refreshed tunnel for '%s'", serviceKey)
				return
			case <-ctx.Done():
				return
			}
		}

	case "service":
		p.connMutex.Lock()
		defer p.connMutex.Unlock()

		// ignore services we don't know anything about
		pc := p.activeServices[k]
		if pc == nil {
			return
		}

		// close the underlying port-forward
		if err := pc.Close(); err != nil {
			p.log.WithField("service", k).WithError(err).Debug("failed to close proxy connection")
		}

		// reset the activeServices section for this service
		p.activeServices[k] = nil

		p.log.Warnf("tunnel for %s has been destroyed due to the underlying service being deleted", k)
	}
}

// Start starts the internal informer
func (p *Proxier) Start(ctx context.Context) error {
	podStore, podInformer := cache.NewInformer(
		cache.NewListWatchFromClient(p.k.CoreV1().RESTClient(), "pods", corev1.NamespaceAll, fields.Everything()),
		&corev1.Pod{},
		time.Second*0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				p.handleInformerEvent(ctx, "add", obj)
			},
			DeleteFunc: func(obj interface{}) {
				p.handleInformerEvent(ctx, "delete", obj)
			},
		},
	)

	servStore, servInformer := cache.NewInformer(
		cache.NewListWatchFromClient(p.k.CoreV1().RESTClient(), "services", corev1.NamespaceAll, fields.Everything()),
		&corev1.Service{},
		time.Second*0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				p.handleInformerEvent(ctx, "add", obj)
			},
			DeleteFunc: func(obj interface{}) {
				p.handleInformerEvent(ctx, "delete", obj)
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
func (p *Proxier) findPodBySelector(o kruntime.Object) (*corev1.Pod, error) {
	namespace, selector, err := polymorphichelpers.SelectorsForObject(o)
	if err != nil {
		return nil, fmt.Errorf("cannot attach to %T: %v", o, err)
	}

	sortBy := func(pods []*corev1.Pod) sort.Interface { return sort.Reverse(podutils.ActivePods(pods)) }
	pod, _, err := polymorphichelpers.GetFirstPod(p.k.CoreV1(), namespace, selector.String(), 1*time.Minute, sortBy)
	return pod, err
}

// allocateIP allocates an ip for a given service key
func (p *Proxier) allocateIP(serviceKey string) (*ipam.IP, error) {
	p.ipMutex.Lock()
	defer p.ipMutex.Unlock()

	ipAddress := p.serviceIPs[serviceKey]
	if ipAddress == nil {
		var err error
		ipAddress, err = p.ipam.AcquireIP(p.ipamPrefix.Cidr)
		if err != nil {
			return nil, err
		}

		// We only need to create alias on darwin, on other platforms
		// lo0 becomes lo and routes the full /8
		if runtime.GOOS == "darwin" {
			args := []string{"lo0", "alias", ipAddress.IP.String(), "up"}
			if err := exec.Command("ifconfig", args...).Run(); err != nil {
				return nil, errors.Wrap(err, "failed to create ip link")
			}
		}

		p.log.WithField("service", serviceKey).Debugf("allocated ip address %s", ipAddress.IP)
		p.serviceIPs[serviceKey] = ipAddress
	}

	return p.serviceIPs[serviceKey], nil
}

// createProxy creates a proxy connection
func (p *Proxier) createProxy(ctx context.Context, s *Service) error { //nolint:funlen,gocyclo
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

	ports := make([]string, len(s.Ports))
	for i, port := range s.Ports {
		ports[i] = fmt.Sprintf("%d:%d", port.LocalPort, port.RemotePort)
	}

	p.log.Infof("creating tunnel for service %s", serviceKey)

	p.connMutex.Lock()
	p.activeServices[serviceKey] = &ProxyConnection{
		proxier: p,
		Ports:   ports,
		Service: *s,
		Pod:     *pod,
	}
	p.activePods[podKey] = p.activeServices[serviceKey]
	p.connMutex.Unlock()

	// start the proxy
	if err := p.activeServices[serviceKey].Start(ctx); err != nil {
		p.log.WithError(err).Errorf(
			"failed to start proxy for %s",
			serviceKey,
		)
	}

	return nil
}

// Proxy starts a proxier.
func (p *Proxier) Proxy(ctx context.Context) error {
	var wg sync.WaitGroup
	for _, s := range p.s {
		wg.Add(1)

		// spawn a goroutine to create the tunnel
		go func(s Service) {
			ticker := backoff.NewTicker(backoff.NewExponentialBackOff())
		createLoop:
			for {
				select {
				case <-ticker.C:
					if err := p.createProxy(ctx, &s); err != nil { //nolint:scopelint
						p.log.WithError(err).Warnf("failed to create tunnel for '%s/%s' (trying again)", s.Namespace, s.Name)
						continue
					}
					ticker.Stop()
					break createLoop
				case <-ctx.Done():
					break createLoop
				}
			}

			wg.Done()
		}(s)
	}

	// wait for all of the proxies to be up
	wg.Wait()

	p.log.Info("all tunnels created successfully")

	// wait for the process to be terminated
	<-ctx.Done()

	p.log.Info("cleaning up ...")

	p.connMutex.Lock()
	for k, pc := range p.activeServices {
		if err := pc.Close(); err != nil {
			p.log.WithField("service", k).WithError(err).Debug("failed to close proxy connection")
		}
		p.activeServices[k] = nil
	}
	p.connMutex.Unlock()

	p.log.Info("cleaned up")

	return nil
}
