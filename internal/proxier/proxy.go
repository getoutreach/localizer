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
	k    kubernetes.Interface
	rest *rest.Config
	log  logrus.FieldLogger
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
	portForwarder, pfdoneChan, err := NewPortForwarder(ctx, p.k, p.rest, p.log)
	if err != nil {
		return err
	}

	serviceChan, handlerDoneChan := CreateHandlers(ctx, portForwarder)

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

	<-handlerDoneChan
	<-pfdoneChan

	return nil
}
