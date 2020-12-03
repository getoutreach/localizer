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

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

// Proxier handles creating an maintaining proxies to a remote
// Kubernetes service
type Proxier struct {
	k      kubernetes.Interface
	rest   *rest.Config
	log    logrus.FieldLogger
	worker *worker
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
	Ports []int
}

// NewProxier creates a new proxier instance
func NewProxier(ctx context.Context, k kubernetes.Interface, kconf *rest.Config, log logrus.FieldLogger) *Proxier {
	return &Proxier{
		k:    k,
		rest: kconf,
		log:  log,
	}
}

// Start starts the proxier
func (p *Proxier) Start(ctx context.Context) error {
	log := p.log.WithField("component", "proxier")
	portForwarder, pfdoneChan, worker, err := NewPortForwarder(ctx, p.k, p.rest, p.log)
	if err != nil {
		return err
	}
	p.worker = worker

	serviceChan, handlerDoneChan := CreateHandlers(ctx, portForwarder, p.k)

	_, servInformer := cache.NewInformer(
		cache.NewListWatchFromClient(p.k.CoreV1().RESTClient(), "services", corev1.NamespaceAll, fields.Everything()),
		&corev1.Service{},
		time.Second*0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				p.log.Debug("got service add event")
				serviceChan <- ServiceEvent{
					EventType: EventAdded,
					Service:   obj.(*corev1.Service),
				}
			},
			DeleteFunc: func(obj interface{}) {
				p.log.Debug("got service delete event")
				serviceChan <- ServiceEvent{
					EventType: EventDeleted,
					Service:   obj.(*corev1.Service),
				}
			},
		},
	)

	go servInformer.Run(ctx.Done())

	log.Info("waiting for kubernetes handlers to finish")
	<-handlerDoneChan

	log.Info("waiting for port-forward worker to finish")
	<-pfdoneChan

	return nil
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
