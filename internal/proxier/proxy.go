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
	"time"

	"github.com/jaredallard/localizer/internal/kevents"
	"github.com/jaredallard/localizer/internal/kube"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// Proxier handles creating an maintaining proxies to a remote
// Kubernetes service
type Proxier struct {
	k      kubernetes.Interface
	rest   *rest.Config
	log    logrus.FieldLogger
	worker *worker

	opts *ProxyOpts

	queue             workqueue.RateLimitingInterface
	threadiness       int
	svcInformer       cache.SharedIndexInformer
	endpointsInformer cache.SharedIndexInformer
	pfrequest         chan<- PortForwardRequest
}

type ServiceStatus struct {
	ServiceInfo ServiceInfo

	Endpoint PodInfo

	// Statuses is dependent on the number of tunnels that exist for this
	// connection. Generally this is one, since a service is usually one
	// connection. Currently only one is supported, but in the future
	// certain services will have more than one.
	Statuses []PortForwardStatus

	// Reason is the reason that this service is in this status.
	// This is generally only set for services that are in a
	// non-running state.
	Reason string

	// IP is the IP address of this tunnel
	IP string

	// Ports are the ports this service is exposing
	Ports []string
}

type ProxyOpts struct {
	ClusterDomain string
	IPCidr        string
}

// NewProxier creates a new proxier instance
func NewProxier(ctx context.Context, k kubernetes.Interface, kconf *rest.Config, log logrus.FieldLogger, opts *ProxyOpts) (*Proxier, error) { //nolint:lll
	svcInformer := kevents.GlobalCache.Core().V1().Services().Informer()
	endpointsInformer := kevents.GlobalCache.Core().V1().Endpoints().Informer()

	p := &Proxier{
		k:                 k,
		rest:              kconf,
		log:               log,
		opts:              opts,
		queue:             workqueue.NewRateLimitingQueue(workqueue.DefaultItemBasedRateLimiter()),
		threadiness:       1,
		svcInformer:       svcInformer,
		endpointsInformer: endpointsInformer,
	}

	svcInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				p.queue.Add(key)
			}
		},
		UpdateFunc: func(oldObj, obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				p.queue.Add(key)
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				p.queue.Add(key)
			}
		},
	})

	endpointsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				p.queue.Add(key)
			}
		},
	})
	return p, nil
}

// Start starts the proxier
// TODO: replace raw cluster domain with options struct, maybe also
// move into NewProxier
func (p *Proxier) Start(ctx context.Context) error {
	defer p.queue.ShutDown()

	log := p.log.WithField("component", "proxier")
	portForwarder, pfdoneChan, worker, err := NewPortForwarder(ctx, p.k, p.rest, p.log, p.opts)
	p.pfrequest = portForwarder

	log.Infof("Starting %d proxier worker(s)", p.threadiness)
	for i := 0; i < p.threadiness; i++ {
		go wait.Until(p.runWorker, time.Second, ctx.Done())
	}

	if err != nil {
		return err
	}
	p.worker = worker

	<-ctx.Done()
	log.Info("waiting for port-forward worker to finish")
	<-pfdoneChan
	return nil
}

func (p *Proxier) runWorker() {
	for p.processNextWorkItem() {

	}
}

func (p *Proxier) processNextWorkItem() bool {
	key, quit := p.queue.Get()
	if quit {
		return false
	}
	defer p.queue.Done(key)

	// Invoke the method containing the business logic
	err := p.reconcile(key.(string))

	if err == nil {
		// Forget about the #AddRateLimited history of the key on every successful synchronization.
		// This ensures that future processing of updates for this key is not delayed because of
		// an outdated error history.
		p.queue.Forget(key)
		return true
	}

	if p.queue.NumRequeues(key) < 5 {
		// Re-enqueue the key rate limited. Based on the rate limiter on the
		// queue and the re-enqueue history, the key will be processed later again.
		p.queue.AddRateLimited(key)
		return true
	}

	// Retries exceeded. Forgetting for this reconciliation loop
	p.queue.Forget(key)
	return true
}

func (p *Proxier) reconcile(key string) error {
	o, exists, err := p.svcInformer.GetStore().GetByKey(key)
	if err != nil {
		return err
	}

	if !exists {
		// we don't have the service object anymore, we need to get the namespace/name from the key
		namespace, name, err := cache.SplitMetaNamespaceKey(key)
		if err != nil {
			return err
		}
		p.pfrequest <- PortForwardRequest{
			DeletePortForwardRequest: &DeletePortForwardRequest{
				Service: ServiceInfo{Namespace: namespace, Name: name},
			},
		}
		return nil
	}
	svc := o.(*corev1.Service)

	if svc.DeletionTimestamp != nil {
		p.pfrequest <- PortForwardRequest{
			DeletePortForwardRequest: &DeletePortForwardRequest{
				Service: ServiceInfo{Namespace: svc.Namespace, Name: svc.Name},
			},
		}
		return nil
	}

	existingForward := p.worker.portForwards[key]
	if existingForward == nil {
		//create a new port forward
		p.createPortforward(svc, "")
		return nil
	}

	e, exists, err := p.endpointsInformer.GetStore().GetByKey(key)
	if !exists || err != nil {
		// no endpoints for service nothing we can do atm
		return nil
	}
	endpoints := e.(*corev1.Endpoints)

	switch existingForward.Status {
	case PortForwardStatusWaiting:
		for _, subset := range endpoints.Subsets {
			for _, address := range subset.Addresses {
				if address.TargetRef != nil && address.TargetRef.Kind == PodKind {
					p.createPortforward(svc, "endpoint became available")
				}
			}
		}

	case PortForwardStatusRunning:
		if !isActiveEndpoint(existingForward.Pod.Name, endpoints) {
			p.createPortforward(svc, fmt.Sprintf("endpoints '%s' was removed", existingForward.Pod.Key()))
		}
	case PortForwardStatusRecreating:
		//make exhaustive linter happy
	}

	return nil
}

func (p *Proxier) createPortforward(svc *corev1.Service, recreate string) {
	info := ServiceInfo{Namespace: svc.Namespace, Name: svc.Name}
	// resolve the service ports using endpoints if possible.
	resolvedPorts, _, err := kube.ResolveServicePorts(svc)
	if err != nil {
		return
	}

	ports := make([]string, len(svc.Spec.Ports))
	for i, p := range resolvedPorts {
		ports[i] = fmt.Sprintf("%d:%d", p.Port, p.TargetPort.IntValue())
	}
	req := CreatePortForwardRequest{
		Service: info,
		Ports:   ports,
		Hostnames: []string{
			info.Name,
			fmt.Sprintf("%s.%s", info.Name, info.Namespace),
			fmt.Sprintf("%s.%s.svc", info.Name, info.Namespace),
			fmt.Sprintf("%s.%s.svc.%s", info.Name, info.Namespace, p.opts.ClusterDomain),
		},
	}
	// hack for basic support of stateful sets.
	// grab the first endpoint to build the name. This sucks, but it's
	// needed for Outreach's usecases. Please remove this.
	if obj, exists, err := p.endpointsInformer.GetStore().GetByKey(svc.Namespace + "/" + svc.Name); err == nil && exists {
		endpoints := obj.(*corev1.Endpoints)
		refName := ""
	loop:
		for _, sub := range endpoints.Subsets {
			for _, a := range sub.Addresses {
				if a.TargetRef != nil && a.TargetRef.Kind == PodKind {
					refName = a.TargetRef.Name
					break loop
				}
			}
		}
		if refName != "" {
			name := fmt.Sprintf("%s.%s", refName, info.Name)
			req.Hostnames = append(req.Hostnames,
				fmt.Sprintf("%s.%s", name, info.Namespace),
				fmt.Sprintf("%s.%s.svc", name, info.Namespace),
				fmt.Sprintf("%s.%s.svc.%s", name, info.Namespace, p.opts.ClusterDomain),
			)
		}
	}

	if recreate != "" {
		req.Recreate = true
		req.RecreateReason = recreate
	}

	p.pfrequest <- PortForwardRequest{
		CreatePortForwardRequest: &req,
	}
}

func (p *Proxier) List(ctx context.Context) ([]ServiceStatus, error) {
	if p.worker == nil {
		return nil, fmt.Errorf("proxier not running")
	}

	statuses := make([]ServiceStatus, 0)
	for _, pf := range p.worker.portForwards {
		ip := pf.IP.String()
		if len(pf.IP) == 0 {
			ip = ""
		}

		statuses = append(statuses, ServiceStatus{
			ServiceInfo: pf.Service,
			Endpoint:    pf.Pod,
			Reason:      pf.StatusReason,
			Statuses:    []PortForwardStatus{pf.Status},
			IP:          ip,
			Ports:       pf.Ports,
		})
	}

	return statuses, nil
}

func isActiveEndpoint(podName string, endpoints *corev1.Endpoints) bool {
	for _, subset := range endpoints.Subsets {
		for _, address := range subset.Addresses {
			if address.TargetRef != nil && address.TargetRef.Kind == PodKind && address.TargetRef.Name == podName {
				return true
			}
		}
	}
	return false
}
