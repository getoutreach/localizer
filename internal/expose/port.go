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
package expose

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jaredallard/localizer/internal/kube"
	"github.com/jaredallard/localizer/internal/proxier"
	"github.com/omrikiei/ktunnel/pkg/client"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/portforward"
)

const localizerVersion = "latest"

type ServiceForward struct {
	c *Client

	Namespace string
	Selector  map[string]string
	Ports     []kube.ResolvedServicePort

	// TODO(jaredallard): support replacing non associated pods?
	objects map[string]scaledObjectType
}

type scaledObjectType struct {
	corev1.ObjectReference

	Replicas uint
}

// GetKey() returns a unique, predictable key for the given
// scaledObjectType capable of being used for caching
func (s *scaledObjectType) GetKey() string {
	return strings.ToLower(fmt.Sprintf("%s/%s/%s", s.Kind, s.Namespace, s.Name))
}

func (p *ServiceForward) createServerPortForward(ctx context.Context, po *corev1.Pod) (*portforward.PortForwarder, error) {
	return kube.CreatePortForward(ctx, p.c.k.CoreV1().RESTClient(), p.c.kconf, po, "50:50")
}

func (p *ServiceForward) createServerPodAndTransport(ctx context.Context) (func(), error) { //nolint:funlen,gocyclo
	// map the service ports into containerPorts, using the
	containerPorts := make([]corev1.ContainerPort, len(p.Ports))
	for i, port := range p.Ports {
		portInt := int(port.TargetPort.IntVal)
		name := port.OriginalTargetPort
		if name == "" {
			name = port.Name
			if name == "" {
				// fallback to a simple port based name
				name = strconv.Itoa(portInt)
			}
		}
		cp := corev1.ContainerPort{
			ContainerPort: int32(portInt),
			Name:          name,
			Protocol:      corev1.ProtocolTCP,
		}

		containerPorts[i] = cp
	}

	// create a pod for our new expose service
	exposedPortsJSON, err := json.Marshal(containerPorts)
	if err != nil {
		return func() {}, err
	}

	po, err := p.c.k.CoreV1().Pods(p.Namespace).Create(ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    p.Namespace,
			GenerateName: "localizer-",
			Annotations: map[string]string{
				proxier.ExposedAnnotation:          "true",
				proxier.ExposedLocalPortAnnotation: string(exposedPortsJSON),
			},
			Labels: p.Selector,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyOnFailure,
			Containers: []corev1.Container{
				{
					Name:            "default",
					Image:           "jaredallard/localizer:" + localizerVersion,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Ports:           containerPorts,
					ReadinessProbe: &corev1.Probe{
						Handler: corev1.Handler{
							HTTPGet: &corev1.HTTPGetAction{
								Port: intstr.FromInt(51),
							},
						},
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("100Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("100Mi"),
						},
					},
				},
			},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return func() {}, errors.Wrap(err, "failed to create pod")
	}

	p.c.log.Infof("created pod %s", po.ObjectMeta.Name)

	p.c.log.Info("waiting for remote pod to be ready ...")
	t := time.NewTicker(10 * time.Second)

loop:
	for {
		select {
		case <-t.C:
			// check if the pod is ready
			obj, exists, err := p.c.podStore.Get(po)
			if err != nil || !exists {
				continue
			}

			po := obj.(*corev1.Pod)

			// if the pod is ready, break out of the waiting loop
			for _, cond := range po.Status.Conditions {
				if cond.Type == corev1.PodReady {
					if cond.Status == corev1.ConditionTrue {
						break loop
					}
				}
			}
		case <-ctx.Done():
			return func() {}, ctx.Err()
		}
	}

	p.c.log.Info("pod is ready, creating port-forward(s)")

	fw, err := p.createServerPortForward(ctx, po)
	if err != nil {
		return func() {}, errors.Wrap(err, "failed to create port-forward for underlying transport")
	}

	fw.Ready = make(chan struct{})
	go func() {
		// TODO(jaredallard): reconnect logic that works with context
		if err := fw.ForwardPorts(); err != nil {
			p.c.log.WithError(err).Error("underlying transport died")
		}
	}()

	p.c.log.Debug("waiting for transport to be marked as ready")
	select {
	case <-fw.Ready:
	case <-ctx.Done():
		return func() {}, ctx.Err()
	}

	return func() {
		p.c.log.Debug("cleaning up pod")
		// cleanup the pod
		//nolint:errcheck
		p.c.k.CoreV1().Pods(p.Namespace).Delete(context.Background(), po.Name, metav1.DeleteOptions{})
	}, nil
}

// Start starts forwarding a service
func (p *ServiceForward) Start(ctx context.Context) error {
	ports := make([]string, len(p.Ports))
	for i, port := range p.Ports {
		prt := int(port.TargetPort.IntVal)
		ports[i] = fmt.Sprintf("%d:%d", prt, prt)
	}

	cleanupFn, err := p.createServerPodAndTransport(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create server and/or transport")
	}
	defer cleanupFn()

	// TODO(jaredallard): We likely need reconnect logic here
	host := "127.0.0.1"
	port := 50
	tls := false

	// scale down the other resources that powered this service
	for _, o := range p.objects {
		p.c.log.Infof("scaling %s from %d -> 0", o.GetKey(), o.Replicas)
		if err := p.c.scaleObject(ctx, &o.ObjectReference, uint(0)); err != nil {
			return errors.Wrap(err, "failed to scale down object")
		}
	}
	defer func() {
		// scale back up the resources that powered this service
		for _, o := range p.objects {
			p.c.log.Infof("scaling %s from 0 -> %d", o.GetKey(), o.Replicas)
			if err := p.c.scaleObject(context.Background(), &o.ObjectReference, o.Replicas); err != nil {
				p.c.log.WithError(err).Warn("failed to scale back up object")
			}
		}
	}()

	p.c.log.Debug("creating ktunnel client")
	err = client.RunClient(ctx, &host, &port, "tcp", &tls, nil, nil, ports)
	if err != nil {
		return errors.Wrap(err, "failed to create grpc transport")
	}

	// wait for the context to finish
	<-ctx.Done()

	return nil
}
