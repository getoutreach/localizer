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

	"github.com/jaredallard/localizer/internal/kube"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
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
func CreateHandlers(ctx context.Context, requester chan<- PortForwardRequest,
	k kubernetes.Interface) (chan<- ServiceEvent, <-chan struct{}) {
	serviceChan := make(chan ServiceEvent)
	doneChan := make(chan struct{})

	go serviceProcessor(ctx, serviceChan, doneChan, requester, k)

	return serviceChan, doneChan
}

// Services
func serviceProcessor(ctx context.Context, event <-chan ServiceEvent,
	doneChan chan struct{}, requester chan<- PortForwardRequest, k kubernetes.Interface) {
	for {
		select {
		case <-ctx.Done():
			close(doneChan)
			return
		case s := <-event:
			info := ServiceInfo{
				Name:      s.Service.Name,
				Namespace: s.Service.Namespace,
				Type:      "",
			}

			// Skip this service for now.
			if info.Name == "kubernetes" {
				continue
			}

			if s.Service.Spec.ExternalName != "" {
				// skip ExternalName services
				continue
			}

			if s.Service.Spec.ClusterIP == "None" {
				info.Type = ServiceTypeStatefulset
			}

			var msg PortForwardRequest
			switch s.EventType {
			case EventAdded:
				// resolve the service ports using endpoints if possible.
				resolvedPorts, _, err := kube.ResolveServicePorts(ctx, k, s.Service)
				if err != nil {
					continue
				}

				ports := make([]string, len(s.Service.Spec.Ports))
				for i, p := range resolvedPorts {
					ports[i] = fmt.Sprintf("%d:%d", p.Port, p.TargetPort.IntValue())
				}

				switch info.Type {
				case ServiceTypeStandard:
					msg = PortForwardRequest{
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
				case ServiceTypeStatefulset:
					// TODO: This doesn't support multiple pods for a service right now
					// eventually we should support that.
					name := fmt.Sprintf("%s.%s", info.Name+"-0", info.Name)
					msg = PortForwardRequest{
						CreatePortForwardRequest: &CreatePortForwardRequest{
							Service: info,
							Ports:   ports,
							Hostnames: []string{
								name,
								fmt.Sprintf("%s.%s", name, info.Namespace),
								fmt.Sprintf("%s.%s.svc", name, info.Namespace),
								fmt.Sprintf("%s.%s.svc.cluster", name, info.Namespace),
								fmt.Sprintf("%s.%s.svc.cluster.local", name, info.Namespace),
							},
						},
					}
				}
			case EventDeleted:
				requester <- PortForwardRequest{
					DeletePortForwardRequest: &DeletePortForwardRequest{
						Service: info,
					},
				}
			}

			// send the message we generatedl, but check if the context has been canceled first
			select {
			case <-ctx.Done():
				return
			default:
				requester <- msg
			}
		}
	}
}
