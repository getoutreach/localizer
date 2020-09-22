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
	"sort"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/metal-stack/go-ipam"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/txn2/txeh"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
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
	connMutex      sync.Mutex
	serviceIPs     map[string]*ipam.IP
	activeServices map[string][]*ProxyConnection
	activePods     map[string][]*ProxyConnection
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
		activePods:     make(map[string][]*ProxyConnection),
		activeServices: make(map[string][]*ProxyConnection),
	}
}

// serviceAddresses returns all of the valid addresses
// for a given kubernetes service
func serviceAddresses(s *corev1.Service) []string {
	return []string{
		s.Name,
		fmt.Sprintf("%s.%s", s.Name, s.Namespace),
		fmt.Sprintf("%s.%s.svc", s.Name, s.Namespace),
		fmt.Sprintf("%s.%s.svc.cluster", s.Name, s.Namespace),
		fmt.Sprintf("%s.%s.svc.cluster.local", s.Name, s.Namespace),
	}
}

func (p *Proxier) handleInformerEvent(ctx context.Context, event string, obj interface{}) { //nolint:funlen,gocyclo
	k, _ := cache.MetaNamespaceKeyFunc(obj)

	item := ""
	switch obj.(type) {
	case *corev1.Pod:
		item = "pod"
	case *corev1.Service:
		item = "service"
	case *ProxyConnection:
		item = "connection"
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
			p.log.Infof("underlying connection died for %d (-> %s:%d)", pc.LocalPort, k, pc.RemotePort)
		}

		refreshServices := make([]Service, len(p.activePods[k]))
		refreshPorts := make([]string, len(p.activePods[k]))

		p.connMutex.Lock()
		for i, pc := range p.activePods[k] {
			refreshServices[i] = pc.Service
			refreshPorts[i] = pc.GetPort()
			pc.Close()
		}

		// reset the activePods
		p.activePods[k] = nil
		p.connMutex.Unlock()

		if len(refreshPorts) > 0 {
			p.log.WithField("ports", refreshPorts).
				Warnf("port-forward for %s is being refreshed due to underlying pod being destroyed", k)
		}

		for _, s := range refreshServices {
			ticker := backoff.NewTicker(backoff.NewExponentialBackOff())
			for {
				select {
				case <-ticker.C:
					if err := p.createProxy(ctx, &s); err != nil { //nolint:scopelint
						p.log.Warnf("failed to refresh port-forward for %s: %v (trying again)", k, err)
					}
					ticker.Stop()
					p.log.WithField("ports", refreshPorts).
						Infof("refreshed port-forward(s) for '%s'", k)
					return
				case <-ctx.Done():
					return
				}
			}
		}

	case "service":
		removedPorts := make([]string, len(p.activeServices[k]))

		p.connMutex.Lock()
		for i, pc := range p.activeServices[k] {
			removedPorts[i] = pc.GetPort()
			pc.Close()
		}

		// reset the activeServices section for this service
		p.activeServices[k] = nil
		p.connMutex.Unlock()

		p.hosts.RemoveAddresses(serviceAddresses(obj.(*corev1.Service)))
		if err := p.hosts.Save(); err != nil {
			p.log.Warnf("failed to clean hosts file: %v", err)
		}

		if len(removedPorts) > 0 {
			p.log.WithField("ports", removedPorts).
				Warnf("port-forward for %s has been destroyed due to the underlying service being deleted", k)
		}
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

	ipAddress := p.serviceIPs[serviceKey]
	if ipAddress == nil {
		ipAddress, err = p.ipam.AcquireIP(p.ipamPrefix.Cidr)
		if err != nil {
			return errors.Wrap(err, "failed to allocate IP address")
		}

		args := []string{"lo0", "alias", ipAddress.IP.String(), "up"}
		if err := exec.Command("ifconfig", args...).Run(); err != nil {
			return errors.Wrap(err, "failed to create ip link")
		}

		p.serviceIPs[serviceKey] = ipAddress
	}

	for _, port := range s.Ports {
		p.log.Infof("creating port-forward '%s:%d'", serviceKey, port.RemotePort)

		// build the linking tables
		// port -> conn
		p.connMutex.Lock()
		conn := &ProxyConnection{
			p,
			nil,

			ipAddress,
			port.LocalPort,
			port.RemotePort,
			*s,
			*pod,
			false,
		}

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
		p.connMutex.Unlock()

		// start the proxy
		if err := conn.Start(ctx); err != nil {
			p.log.Errorf(
				"failed to start proxy for '%s:%d'",
				serviceKey, port.RemotePort, port.LocalPort, err,
			)
		}
	}

	// only add addresses for services we actually are routing to
	p.log.Debugf("adding hosts file entry for service '%s'", serviceKey)
	p.hosts.AddHosts(ipAddress.IP.String(), serviceAddresses(kserv))
	if err := p.hosts.Save(); err != nil {
		return errors.Wrap(err, "failed to save address to hosts")
	}

	return nil
}

// Proxy starts a proxier.
func (p *Proxier) Proxy(ctx context.Context) error {
createLoop:
	for _, s := range p.s {
		ticker := backoff.NewTicker(backoff.NewExponentialBackOff())
	createIteratorLoop:
		for {
			select {
			case <-ticker.C:
				if err := p.createProxy(ctx, &s); err != nil { //nolint:scopelint
					p.log.Warnf("failed to create port-forward for '%s/%s': %v (trying again)", s.Namespace, s.Name, err)
				}
				ticker.Stop()
				break createIteratorLoop
			case <-ctx.Done():
				break createLoop
			}
		}
	}

	<-ctx.Done()
	p.log.Info("cleaning up ...")

	for k := range p.activeServices {
		namespace, name, err := cache.SplitMetaNamespaceKey(k)
		if err != nil {
			// TODO: handle this
			continue
		}

		// cleanup the DNS entries
		kserv := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
		addrs := serviceAddresses(kserv)

		p.log.WithField("addresses", addrs).Debug("cleaning up hosts entry")
		p.hosts.RemoveHosts(addrs)
		if err := p.hosts.Save(); err != nil {
			p.log.Warnf("failed to clean hosts file: %v", err)
		}
	}

	for _, ip := range p.serviceIPs {
		args := []string{"lo0", "-alias", ip.IP.String(), "up"}
		if err := exec.Command("ifconfig", args...).Run(); err != nil {
			return errors.Wrapf(err, "failed to remove ip alias '%s'", ip.IP.String())
		}
	}

	p.log.Info("cleaned up")

	return nil
}
