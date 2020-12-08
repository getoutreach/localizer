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

	"github.com/jaredallard/localizer/internal/kevents"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Proxier handles creating an maintaining proxies to a remote
// Kubernetes service
type Proxier struct {
	k      kubernetes.Interface
	rest   *rest.Config
	log    logrus.FieldLogger
	worker *worker

	opts *ProxyOpts
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
func NewProxier(ctx context.Context, k kubernetes.Interface, kconf *rest.Config, log logrus.FieldLogger, opts *ProxyOpts) *Proxier {
	return &Proxier{
		k:    k,
		rest: kconf,
		log:  log,
		opts: opts,
	}
}

// Start starts the proxier
// TODO: replace raw cluster domain with options struct, maybe also
// move into NewProxier
func (p *Proxier) Start(ctx context.Context) error {
	log := p.log.WithField("component", "proxier")
	portForwarder, pfdoneChan, worker, err := NewPortForwarder(ctx, p.k, p.rest, p.log, p.opts)
	if err != nil {
		return err
	}
	p.worker = worker

	serviceChan, handlerDoneChan := CreateHandlers(ctx, portForwarder, p.k, p.opts.ClusterDomain)

	err = kevents.WaitForSync(ctx, kevents.GlobalCache.TrackObject("endpoints", &corev1.Endpoints{}))
	if err != nil {
		return errors.Wrap(err, "failed to sync endpoint cache")
	}

	// Handle services being created, send them to the proxier
	err = kevents.GlobalCache.Subscribe(ctx, "services", &corev1.Service{}, func(e kevents.Event) {
		if e.Event == kevents.EventTypeUpdated {
			return
		}

		serviceChan <- ServiceEvent{
			EventType: e.Event,
			Service:   e.NewObject.(*corev1.Service),
		}
	})
	if err != nil {
		return err
	}

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
