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
package kube

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/jaredallard/localizer/internal/kevents"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/transport/spdy"

	// Needed for external authenticators
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"

	"k8s.io/client-go/rest"
)

// GetKubeClient returns a kubernetes client, and the config used by it, based on
// a given context. If no context is provided then the default will be used
func GetKubeClient(contextName string) (*rest.Config, kubernetes.Interface, error) {
	// attempt to use in cluster config first
	config, err := rest.InClusterConfig()
	if err != nil {
		lr := clientcmd.NewDefaultClientConfigLoadingRules()
		apiconfig, err := lr.Load()
		if err != nil {
			return nil, nil, err
		}

		overrides := &clientcmd.ConfigOverrides{}
		if contextName != "" {
			overrides.CurrentContext = contextName
		}

		ccc := clientcmd.NewDefaultClientConfig(*apiconfig, overrides)

		config, err = ccc.ClientConfig()
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to get kubernetes client config")
		}
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create kubernetes client")
	}

	return config, client, nil
}

func CreatePortForward(ctx context.Context, r rest.Interface, rc *rest.Config,
	p *corev1.Pod, ip string, ports []string) (*portforward.PortForwarder, error) {
	req := r.Post().
		Resource("pods").
		Namespace(p.Namespace).
		Name(p.Name).
		SubResource("portforward")

	transport, upgrader, err := spdy.RoundTripperFor(rc)
	if err != nil {
		return nil, err
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", req.URL())

	return portforward.NewOnAddresses(dialer, []string{ip}, ports, ctx.Done(), nil, ioutil.Discard, ioutil.Discard)
}

type ResolvedServicePort struct {
	corev1.ServicePort

	// OriginalTargetPort is set if the ServicePort
	// was modified
	OriginalTargetPort string

	// MappedPort is the locally mapped port that this should have
	// defaults to the targetPort
	MappedPort uint
}

// ResolveServicePorts converts named ports into their true
// format. TargetPort's that have are named become their integer equivalents
func ResolveServicePorts(ctx context.Context, k kubernetes.Interface,
	s *corev1.Service) ([]ResolvedServicePort, bool, error) {
	store := kevents.GlobalCache.GetStore(&corev1.Endpoints{})
	if store == nil {
		return nil, false, fmt.Errorf("endpoints store was empty")
	}

	obj, exists, err := store.GetByKey(s.Namespace + "/" + s.Name)
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to get endpoints")
	}

	e := obj.(*corev1.Endpoints)
	if !exists || len(e.Subsets) == 0 {
		// if there are no endpoints, don't resolve, just return them
		servicePorts := make([]ResolvedServicePort, len(s.Spec.Ports))
		for i, sp := range s.Spec.Ports {
			servicePorts[i] = ResolvedServicePort{
				sp,
				"",
				uint(sp.Port),
			}
		}
		return servicePorts, false, nil
	}

	servicePorts := make([]ResolvedServicePort, len(s.Spec.Ports))
	for i, p := range s.Spec.Ports {
		original := ""
		if p.TargetPort.Type == intstr.String {
			if len(e.Subsets) == 0 {
				continue
			}

			// iterate over the ports to find what
			// the named port references
			// note that the name of the port will be the
			// service's port name, not the targetPort
			for _, np := range e.Subsets[0].Ports {
				if np.Name == p.Name || np.Name == p.TargetPort.String() {
					original = p.TargetPort.String()
					p.TargetPort = intstr.FromInt(int(np.Port))
					break
				}
			}
		}

		servicePorts[i] = ResolvedServicePort{
			p,
			original,
			uint(p.TargetPort.IntValue()),
		}
	}

	return servicePorts, true, nil
}
