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

	"github.com/jaredallard/localizer/internal/kube"
	"github.com/metal-stack/go-ipam"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/portforward"
)

// ProxyConnection tracks a proxy connection
type ProxyConnection struct {
	proxier *Proxier
	fw      *portforward.PortForwarder

	// IP is the dedicated IP for this tunnel
	IP *ipam.IP

	// Ports is an array of local:remote ports
	Ports []string

	// Service is the service that this proxy is connected too
	Service Service

	// Pod is the pod powering this proxy
	Pod corev1.Pod
}

// Start starts a proxy connection
func (pc *ProxyConnection) Start(ctx context.Context) error {
	fw, err := kube.CreatePortForward(ctx, pc.proxier.rest, pc.proxier.kconf, &pc.Pod, pc.IP.IP.String(), pc.Ports)
	if err != nil {
		return errors.Wrap(err, "failed to create tunnel")
	}
	pc.fw = fw

	go func() {
		// TODO(jaredallard): Figure out a way to better backoff errors here
		if err := fw.ForwardPorts(); err != nil {
			// if this dies, mark the connection as inactive for
			// the connection reaper
			pc.Close()
			pc.fw = nil

			k, _ := cache.MetaNamespaceKeyFunc(pc.Service)
			pc.proxier.log.WithError(err).WithFields(logrus.Fields{
				"ports":   pc.Ports,
				"service": k,
			}).Debug("tunnel died")

			// trigger the recreate logic
			pc.proxier.handleInformerEvent(ctx, "connection-dead", pc)
		}
	}()

	return nil
}

// Close closes the current proxy connection and marks it as
// no longer being active
func (pc *ProxyConnection) Close() error {
	// note: If the parent context was canceled
	// this has already been closed
	if pc.fw != nil {
		pc.fw.Close()
	}

	// if we had an ip address, free it
	if pc.IP != nil {
		_, err := pc.proxier.ipam.ReleaseIP(pc.IP)
		if err != nil {
			return errors.Wrap(err, "failed to free IP address")
		}
	}

	// we'll return an error one day
	return nil
}
