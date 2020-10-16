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
	"strconv"

	"github.com/jaredallard/localizer/internal/kube"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// This is used to support exposed services being "forwarded"
	ExposedAnnotation = "localizer.jaredallard.github.com/exposed"

	ExposedLocalPortAnnotation = "localizer.jaredallard.github.com/exposed-ports"
)

// Service represents a Service running in Kubernetes
// that should be proxied local <-> remote
type Service struct {
	Name      string
	Namespace string
	Ports     []*ServicePort
}

// GetKey returns a cache stable key for a Service
func (s *Service) GetKey() string {
	return fmt.Sprintf("%s/%s", s.Namespace, s.Name)
}

// ServicePort defines a port that is exposed
// by a remote service.
type ServicePort struct {
	RemotePort uint
	LocalPort  uint
}

// CreateServiceFromKubernetesService creates a Service object from a Kubernetes corev1.Service
func CreateServiceFromKubernetesService(ctx context.Context, olog logrus.FieldLogger,
	k kubernetes.Interface, kserv *corev1.Service) (Service, error) { //nolint:funlen,gocyclo
	serv := Service{
		Name:      kserv.Name,
		Namespace: kserv.Namespace,
		Ports:     make([]*ServicePort, 0),
	}
	key := serv.GetKey()
	log := olog.WithField("service", key)

	// TODO: handle
	// In general we don't support non-clusterIP services
	if kserv.Spec.ClusterIP == "None" {
		return Service{}, fmt.Errorf("service had no cluster ip")
	}

	// skip services that have no ports
	if len(kserv.Spec.Ports) == 0 {
		return Service{}, fmt.Errorf("service had no defined ports")
	}

	if len(kserv.Spec.Selector) == 0 {
		log.Debug("skipping service without a selector")
		return Service{}, fmt.Errorf("service had no selector")
	}

	// convert the Kubernetes ports into our own internal data model
	// we also handle overriding localPorts via the RemapAnnotation here.
	servicePorts, exists, err := kube.ResolveServicePorts(ctx, k, kserv)
	if err != nil {
		log.Debugf("failed to process servicePorts for service: %v", err)
		return serv, nil
	} else if !exists {
		log.Debug("service has no endpoints, will not forward")
		return serv, nil
	}

	for _, p := range servicePorts {
		log := log.WithField("port", p.Port)

		// we only support TCP services currently.
		if p.Protocol != corev1.ProtocolTCP {
			log.Debug("skipping non-TCP port")
		}

		localPort := uint(p.Port)

		// if a service only has one port, name is not required.
		// In that case, we just name it the port. This allows users to still
		// override it if needed.
		if p.Name == "" {
			p.Name = strconv.Itoa(int(p.Port))
		}

		remotePort := p.TargetPort.IntValue()

		// if remote port is 0, or it was originally a string, (and unresolvable)
		// or undefined, er assume the localPort is the same
		if remotePort == 0 {
			remotePort = int(localPort)
		}

		serv.Ports = append(serv.Ports, &ServicePort{
			RemotePort: uint(remotePort),
			LocalPort:  localPort,
		})
	}

	return serv, nil
}
