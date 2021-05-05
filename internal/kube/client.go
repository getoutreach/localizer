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

	"github.com/getoutreach/localizer/internal/kevents"
	"github.com/getoutreach/localizer/internal/reflectconversions"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/conversion"
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

		overrides := &clientcmd.ConfigOverrides{}
		if contextName != "" {
			overrides.CurrentContext = contextName
		}

		config, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(lr, overrides).ClientConfig()
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
func ResolveServicePorts(log logrus.FieldLogger, s *corev1.Service) ([]ResolvedServicePort, error) {
	store := kevents.GlobalCache.Core().V1().Endpoints().Informer().GetStore()

	hasNamedPorts := false
	for _, p := range s.Spec.Ports {
		if p.TargetPort.Type == intstr.String {
			hasNamedPorts = true
			break
		}
	}

	// Don't try to resolve anything if we don't have named ports
	if !hasNamedPorts {
		servicePorts := make([]ResolvedServicePort, len(s.Spec.Ports))
		for i, sp := range s.Spec.Ports {
			servicePorts[i] = ResolvedServicePort{
				sp,
				"",
				uint(sp.Port),
			}
		}
		return servicePorts, nil
	}

	obj, _, err := store.GetByKey(s.Namespace + "/" + s.Name)
	e, ok := obj.(*corev1.Endpoints)
	if !ok || len(e.Subsets) == 0 || err != nil {
		return ResolveServicePortsFromControllers(log, s)
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

	return servicePorts, nil
}

// satisfiesSelector checks if target has all of the k/v pairs in source
func satisfiesSelector(obj interface{}, matches map[string]string) (bool, error) {
	v, err := conversion.EnforcePtr(obj)
	if err != nil {
		return false, err
	}
	err = reflectconversions.EnforceStruct(v)
	if err != nil {
		return false, err
	}

	v = v.FieldByName("Spec")
	if !v.IsValid() {
		return false, fmt.Errorf("struct lacks field Spec")
	}
	err = reflectconversions.EnforceStruct(v)
	if err != nil {
		return false, err
	}

	v = v.FieldByName("Template")
	if !v.IsValid() {
		return false, fmt.Errorf("struct lacks field Template")
	}
	err = reflectconversions.EnforceStruct(v)
	if err != nil {
		return false, err
	}

	v = v.FieldByName("Labels")
	if !v.IsValid() {
		return false, fmt.Errorf("struct lacks struct Labels")
	}

	target, ok := v.Interface().(map[string]string)
	if !ok {
		return false, fmt.Errorf("expected labels to be map[string]string, got %v", v.Type())
	}

	for k, v := range matches {
		if target[k] != v {
			return false, nil
		}
	}

	return true, nil
}

// ResolveServicePortsFromControllers looks up the controllers of a given service
// and uses their containerPort declarations to resolve named endpoints of a service
func ResolveServicePortsFromControllers(log logrus.FieldLogger, s *corev1.Service) ([]ResolvedServicePort, error) { //nolint:funlen
	controllers, err := FindControllersForService(log, s)
	if err != nil {
		return nil, err
	}
	if len(controllers) == 0 {
		return nil, fmt.Errorf("failed to find any controllers, please ensure a deployment or other type exists for this service")
	}

	// select the first controller, we can't really support multiple ports
	// without a lot of complexity that doesn't seem warranted
	controller := controllers[0]

	v, err := conversion.EnforcePtr(controller)
	if err != nil {
		return nil, err
	}
	err = reflectconversions.EnforceStruct(v)
	if err != nil {
		return nil, err
	}

	v = v.FieldByName("Spec")
	if !v.IsValid() {
		return nil, fmt.Errorf("struct lacks field Spec")
	}
	err = reflectconversions.EnforceStruct(v)
	if err != nil {
		return nil, err
	}

	v = v.FieldByName("Template")
	if !v.IsValid() {
		return nil, fmt.Errorf("struct lacks field Template")
	}
	err = reflectconversions.EnforceStruct(v)
	if err != nil {
		return nil, err
	}

	v = v.FieldByName("Spec")
	if !v.IsValid() {
		return nil, fmt.Errorf("struct lacks field Spec")
	}
	err = reflectconversions.EnforceStruct(v)
	if err != nil {
		return nil, err
	}

	v = v.FieldByName("Containers")
	if !v.IsValid() {
		return nil, fmt.Errorf("struct lacks field Spec")
	}

	containers, ok := v.Interface().([]corev1.Container)
	if !ok {
		return nil, fmt.Errorf("expected Containers to be []*corev1.Container, got %v", v.Type())
	}

	ports := make(map[string]int)
	for i := range containers {
		for _, p := range containers[i].Ports {
			if p.Name == "" {
				continue
			}

			ports[p.Name] = int(p.ContainerPort)
		}
	}

	resolvedPorts := make([]ResolvedServicePort, len(s.Spec.Ports))
	for i, p := range s.Spec.Ports {
		original := ""
		if p.TargetPort.Type == intstr.String {
			// iterate over the ports to find what
			// the named port references
			// note that the name of the port will be the
			// service's port name, not the targetPort
			for name, containerPort := range ports {
				if name == p.TargetPort.String() {
					original = p.TargetPort.String()
					p.TargetPort = intstr.FromInt(containerPort)
					break
				}
			}
		}

		resolvedPorts[i] = ResolvedServicePort{
			p,
			original,
			uint(p.TargetPort.IntValue()),
		}
	}

	return resolvedPorts, nil
}

// FindControllersForService returns the controllers for a given service.
// Controllers are deployments/statefulsets that match the service's selector in their
// pod templates.
func FindControllersForService(log logrus.FieldLogger, s *corev1.Service) ([]interface{}, error) {
	// TODO: Search all types? Not sure how to handle this.
	items := []interface{}{}
	items = append(items, kevents.GlobalCache.Apps().V1().StatefulSets().Informer().GetStore().List()...)
	items = append(items, kevents.GlobalCache.Apps().V1().Deployments().Informer().GetStore().List()...)

	log.WithField("len", len(items)).Debug("processing controllers")

	controllers := make([]interface{}, 0)

	for _, obj := range items {
		b, err := satisfiesSelector(obj, s.Spec.Selector)
		if err != nil {
			// TODO: add more context
			log.WithError(err).Warn("failed to consider controller")
			continue
		}

		if b {
			controllers = append(controllers, obj)
		}
	}

	return controllers, nil
}
