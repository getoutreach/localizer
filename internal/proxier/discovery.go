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
	"strconv"
	"strings"

	"github.com/jaredallard/localizer/internal/kube"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const (
	RemapAnnotationPrefix = "localizer.jaredallard.github.com/remap-"

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

// ServicePort defines a port that is exposed
// by a remote service.
type ServicePort struct {
	RemotePort uint
	LocalPort  uint
}

type Discoverer struct {
	k   kubernetes.Interface
	log logrus.FieldLogger
}

// NewClient creates a new discovery client that is
// capable of finding remote services and creating proxies
func NewDiscoverer(k kubernetes.Interface, l logrus.FieldLogger) *Discoverer {
	return &Discoverer{
		k,
		l,
	}
}

// Discover finds services in a Kubernetes cluster and returns ones that
// should be forwarded locally.
func (d *Discoverer) Discover(ctx context.Context) ([]Service, error) { //nolint:funlen,gocyclo
	cont := ""

	s := make([]Service, 0)
	for {
		l, err := d.k.CoreV1().Services("").List(ctx, metav1.ListOptions{Continue: cont})
		if kerrors.IsResourceExpired(err) {
			// we need a consistent list, so we just restart fetching
			d.log.Warn("service list expired, refetching all services ...")
			s = make([]Service, 0)
			cont = ""
			continue
		} else if err != nil {
			return nil, errors.Wrap(err, "failed to retrieve kubernetes services")
		}

		for _, kserv := range l.Items { //nolint:gocritic
			// In general we don't support non-clusterIP services
			if kserv.Spec.ClusterIP == "None" {
				continue
			}

			serv := Service{
				Name:      kserv.Name,
				Namespace: kserv.Namespace,
				Ports:     make([]*ServicePort, 0),
			}

			// skip services that have no ports
			if len(kserv.Spec.Ports) == 0 {
				continue
			}

			remaps := make(map[string]uint)
			for k, v := range kserv.Annotations {
				if !strings.HasPrefix(k, RemapAnnotationPrefix) {
					continue
				}

				// for now, skip invalid ports. We may want to expose
				// this someday in the future
				portOverride, err := strconv.ParseUint(v, 0, 64)
				if err != nil {
					continue
				}

				// TODO(jaredallard): determine if ToLower is really needed here.
				// for ease of use we transform this remap to lowercase here
				// when processing ports we also convert their name to lowercase
				// just in case. Though the spec may enforce this to begin with.
				portName := strings.ToLower(strings.TrimPrefix(k, RemapAnnotationPrefix))
				remaps[portName] = uint(portOverride)
			}

			// convert the Kubernetes ports into our own internal data model
			// we also handle overriding localPorts via the RemapAnnotation here.
			servicePorts, err := kube.ResolveServicePorts(ctx, d.k, &kserv) //nolint:scopelint
			if err != nil {
				k, _ := cache.MetaNamespaceKeyFunc(kserv)
				d.log.Debug("failed to process servicePorts for service %s: %v", k, err)
				continue
			}

			for _, p := range servicePorts {
				// we only support TCP services currently.
				if p.Protocol != corev1.ProtocolTCP {
					continue
				}

				localPort := uint(p.Port)

				// if a service only has one port, name is not required.
				// In that case, we just name it the port. This allows users to still
				// override it if needed.
				if p.Name == "" {
					p.Name = strconv.Itoa(int(p.Port))
				}

				remotePort := p.TargetPort.IntValue()

				override := remaps[strings.ToLower(p.Name)]
				if override != 0 {
					localPort = override
				}

				serv.Ports = append(serv.Ports, &ServicePort{
					RemotePort: uint(remotePort),
					LocalPort:  localPort,
				})
			}

			s = append(s, serv)
		}

		// if we don't have a continue, then we break and return
		if l.Continue == "" {
			break
		}

		cont = l.Continue
	}

	return s, nil
}
