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
	"runtime"
	"time"

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

// GetAddresses returns all of the valid addresses
// for a given kubernetes service
func (pc *ProxyConnection) GetAddresses() []string {
	s := pc.Service
	return []string{
		s.Name,
		fmt.Sprintf("%s.%s", s.Name, s.Namespace),
		fmt.Sprintf("%s.%s.svc", s.Name, s.Namespace),
		fmt.Sprintf("%s.%s.svc.cluster", s.Name, s.Namespace),
		fmt.Sprintf("%s.%s.svc.cluster.local", s.Name, s.Namespace),
	}
}

// Start starts a proxy connection
func (pc *ProxyConnection) Start(ctx context.Context) error {
	serviceKey := pc.Service.GetKey()
	ipAddress, err := pc.proxier.allocateIP(serviceKey)
	if err != nil {
		return errors.Wrap(err, "failed to allocate IP")
	}
	pc.IP = ipAddress

	fw, err := kube.CreatePortForward(ctx, pc.proxier.rest, pc.proxier.kconf, &pc.Pod, pc.IP.IP.String(), pc.Ports)
	if err != nil {
		return errors.Wrap(err, "failed to create tunnel")
	}
	pc.fw = fw

	// only add addresses for services we actually are routing to
	pc.proxier.log.Debugf("adding hosts file entry for service '%s'", serviceKey)
	pc.proxier.hosts.AddHosts(pc.IP.IP.String(), pc.GetAddresses())
	if err := pc.proxier.hosts.Save(); err != nil {
		return errors.Wrap(err, "failed to save address to hosts")
	}

	fw.Ready = make(chan struct{})
	go func() {
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

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-fw.Ready:
	case <-time.After(time.Second * 10):
		// if it's been 5 seconds and it's not ready, then we can safely return an error
		return fmt.Errorf("deadline exceeded")
	}

	return nil
}

// Close closes the current proxy connection and marks it as
// no longer being active
func (pc *ProxyConnection) Close() error {
	// if it's nil then it's already been cleaned up
	if pc == nil {
		return nil
	}

	// note: If the parent context was canceled
	// this has already been closed
	if pc.fw != nil {
		pc.fw.Close()
	}

	// cleanup the DNS entries for this ProxyConnection
	pc.proxier.hosts.RemoveAddresses(pc.GetAddresses())
	if err := pc.proxier.hosts.Save(); err != nil {
		return errors.Wrap(err, "failed to remove hosts entry")
	}

	// if we have an ip we should release it
	if pc.IP != nil {
		// If we are on a platform that needs aliases
		// then we need to remove it
		if runtime.GOOS == "darwin" {
			ipStr := pc.IP.IP.String()
			args := []string{"lo0", "-alias", ipStr}
			if err := exec.Command("ifconfig", args...).Run(); err != nil {
				return errors.Wrapf(err, "failed to remove ip alias '%s'", ipStr)
			}
		}

		// release the IP after cleanup, in case it can't be released
		if _, err := pc.proxier.ipam.ReleaseIP(pc.IP); err != nil {
			return errors.Wrap(err, "failed to release IP address")
		}

		pc.proxier.ipMutex.Lock()
		defer pc.proxier.ipMutex.Unlock()
		pc.proxier.serviceIPs[pc.Service.GetKey()] = nil
	}

	return nil
}
