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

	corev1 "k8s.io/api/core/v1"
)

type EventType string

var (
	EventAdded   EventType = "added"
	EventDeleted EventType = "deleted"
)

type ServiceEvent struct {
	EventType EventType
	Service   *corev1.Service
}

// CreateHandlers creates Kubernetes event handlers for all
// of our types. These then communicate with a port-forward
// worker to create Kubernetes port-forwards.
//nolint:gocritic // We're OK not naming these.
func CreateHandlers(ctx context.Context, requester chan<- PortForwardRequest) (chan<- ServiceEvent, <-chan struct{}) {
	serviceChan := make(chan ServiceEvent)
	doneChan := make(chan struct{})

	go serviceProcessor(ctx, serviceChan, doneChan, requester)

	return serviceChan, doneChan
}

// Services
func serviceProcessor(ctx context.Context, event <-chan ServiceEvent, doneChan chan struct{}, requester chan<- PortForwardRequest) {
	for {
		select {
		case <-ctx.Done():
			close(doneChan)
			return
		case s := <-event:
			info := ServiceInfo{
				Name:      s.Service.Name,
				Namespace: s.Service.Namespace,
			}

			// Skip this service for now.
			if info.Name == "kubernetes" {
				continue
			}

			if s.EventType == EventAdded {
				ports := make([]int, len(s.Service.Spec.Ports))
				for i, p := range s.Service.Spec.Ports {
					ports[i] = int(p.Port)
				}

				requester <- PortForwardRequest{
					CreatePortForwardRequest: &CreatePortForwardRequest{
						Service: info,
						Ports:   ports,
						Hostnames: []string{
							info.Name,
							fmt.Sprintf("%s.%s", info.Name, info.Namespace),
							fmt.Sprintf("%s.%s.svc", info.Name, info.Namespace),
							fmt.Sprintf("%s.%s.svc.cluster", info.Name, info.Namespace),
							fmt.Sprintf("%s.%s.svc.cluster.local", info.Name, info.Namespace),
						},
					},
				}
			} else if s.EventType == EventDeleted {
				requester <- PortForwardRequest{
					DeletePortForwardRequest: &DeletePortForwardRequest{
						Service: info,
					},
				}
			}
		}
	}
}
