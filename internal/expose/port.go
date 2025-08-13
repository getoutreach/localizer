// Copyright 2022 Outreach Corporation. Licensed under the Apache License 2.0.
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

// Description: This file has the package expose.
package expose

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/getoutreach/localizer/internal/kube"
	"github.com/getoutreach/localizer/internal/ssh"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/portforward"
)

const (
	ExposedPodLabel = "localizer.jaredallard.github.com/exposed"
	ObjectsPodLabel = "localizer.jaredallard.github.com/objects"
)

var (
	// ErrUnderlyingTransportDied is triggered when the kubernetes port-forward loses
	// connection. This results in the transport protocol dying as well.
	ErrUnderlyingTransportDied = errors.New("underlying transport died")

	// ErrUnderlyingTransportProtocolDied is triggered when the ssh tunnel loses connection,
	// this can be due to the ssh connection being destroyed or the port-forward being killed
	ErrUnderlyingTransportProtocolDied = errors.New("underlying transport protocol (ssh) died")

	// ErrNotInitialized is used to start the initialization
	// process. It is not an error, despite its name.
	ErrNotInitialized = errors.New("not initialized")

	// ErrUnderlyingTransportPodDestroyed is triggered only when a pod is destroyed,
	// note that this will usually case ErrUnderlyingTransportDied to be triggered.
	ErrUnderlyingTransportPodDestroyed = errors.New("underlying transport pod died")
)

type ServiceForward struct {
	c   *Client
	log logrus.FieldLogger

	ServiceName string
	Namespace   string
	Selector    map[string]string
	Ports       []kube.ResolvedServicePort

	// TODO(jaredallard): support replacing non associated pods?
	objects []scaledObjectType
}

type scaledObjectType struct {
	obj interface{}

	*metav1.PartialObjectMetadata `json:"object"`

	// Replicas is the original scale at the time of scale down
	// for this controller
	Replicas int `json:"replicas"`

	// Resource is the resource type (REST) for this controller
	// for example, a Deployment would be deployments
	Resource string `json:"resource"`
}

// GetKey() returns a unique, predictable key for the given
// scaledObjectType capable of being used for caching
func (s *scaledObjectType) GetKey() string {
	return s.GetSelfLink()
}

func (p *ServiceForward) createServerPortForward(ctx context.Context, po *corev1.Pod, localPort int) (*portforward.PortForwarder, error) {
	return kube.CreatePortForward(ctx, p.c.k.CoreV1().RESTClient(), p.c.kconf, po, "0.0.0.0", []string{fmt.Sprintf("%d:2222", localPort)})
}

func (p *ServiceForward) createServerPod(ctx context.Context) (func(), *corev1.Pod, error) { //nolint:funlen,lll // Why: there are no reusable parts to extract
	// map the service ports into containerPorts, using the
	containerPorts := make([]corev1.ContainerPort, len(p.Ports))
	for i, port := range p.Ports {
		name := port.OriginalTargetPort
		cp := corev1.ContainerPort{
			ContainerPort: port.TargetPort.IntVal,
			Name:          name,
			Protocol:      corev1.ProtocolTCP,
		}

		containerPorts[i] = cp
	}

	b, err := json.MarshalIndent(p.objects, "", "  ")
	if err != nil {
		return func() {}, nil, errors.Wrap(err, "failed to encode object state")
	}

	// add a label for localizer pods
	labels := map[string]string{
		ExposedPodLabel: "true",
	}

	for k, v := range p.Selector {
		labels[k] = v
	}

	podObject := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    p.Namespace,
			GenerateName: fmt.Sprintf("localizer-%s-", p.ServiceName),
			Labels:       labels,
			Annotations: map[string]string{
				ObjectsPodLabel: string(b),
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyOnFailure,
			Containers: []corev1.Container{
				{
					Name:            "default",
					Image:           "linuxserver/openssh-server",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Ports:           containerPorts,
					Env: []corev1.EnvVar{
						{
							Name:  "PASSWORD_ACCESS",
							Value: "true",
						},
						{
							Name:  "USER_PASSWORD",
							Value: "supersecretpassword",
						},
						{
							Name:  "USER_NAME",
							Value: "outreach",
						},
						{
							Name:  "DOCKER_MODS",
							Value: "linuxserver/mods:openssh-server-ssh-tunnel",
						},
					},
					ReadinessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							TCPSocket: &corev1.TCPSocketAction{
								Port: intstr.FromInt(2222),
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
	}
	p.log.Debug(spew.Sdump(podObject))

	po, err := p.c.k.CoreV1().Pods(p.Namespace).Create(ctx, podObject, metav1.CreateOptions{})
	if err != nil {
		return func() {}, nil, errors.Wrap(err, "failed to create pod")
	}

	cleanupFn := func() {
		p.log.Debug("cleaning up pod")
		// cleanup the pod
		if err := p.c.k.CoreV1().Pods(p.Namespace).Delete(context.Background(), po.Name, metav1.DeleteOptions{}); err != nil {
			p.log.WithError(err).Warn("failed to delete pod")
		}
	}

	p.log.Infof("created pod %s", po.ObjectMeta.Name)

	p.log.Info("waiting for remote pod to be ready ...")
	t := time.NewTicker(3 * time.Second)

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
						return cleanupFn, po, nil
					}
				}
			}
		case <-ctx.Done():
			cleanupFn()
			return func() {}, nil, ctx.Err()
		}
	}
}

func (p *ServiceForward) createTransport(ctx context.Context, po *corev1.Pod,
	localPort int, errorChan chan<- error) (int, *portforward.PortForwarder, error) {
	fw, err := p.createServerPortForward(ctx, po, localPort)
	if err != nil {
		return 0, nil, errors.Wrap(err, "failed to create tunnel for underlying transport")
	}

	fw.Ready = make(chan struct{})

	go func() {
		errorChan <- fw.ForwardPorts()
	}()

	p.log.Debug("waiting for transport to be marked as ready")
	select {
	case <-fw.Ready:
	case <-time.After(time.Second * 10):
		return 0, nil, fmt.Errorf("deadline exceeded")
	case <-ctx.Done():
		return 0, nil, ctx.Err()
	}

	// only find the port if we don't already know it
	if localPort == 0 {
		fwPorts, err := fw.GetPorts()
		if err != nil {
			return 0, nil, errors.Wrap(err, "failed to get generated underlying transport port")
		}

		for _, p := range fwPorts {
			if p.Remote == 2222 {
				localPort = int(p.Local)
			}
		}
		if localPort == 0 {
			return 0, nil, fmt.Errorf("failed to determine the generated underlying transport port")
		}
	}

	return localPort, fw, nil
}

// Start starts forwarding a service, this blocks
func (p *ServiceForward) Start(ctx context.Context) error {
	ports := make([]string, len(p.Ports))
	for i, port := range p.Ports {
		prt := int(port.TargetPort.IntVal)
		ports[i] = fmt.Sprintf("%d:%d", port.MappedPort, prt)
		p.log.Debugf("tunneling port %v", ports[i])
	}

	// scale down the other resources that powered this service
	for _, o := range p.objects {
		p.log.Infof("scaling %s from %d -> 0", o.GetKey(), o.Replicas)
		if err := p.c.scaleObject(ctx, o, 0); err != nil {
			return errors.Wrap(err, "failed to scale down object")
		}
	}
	defer func() {
		// scale back up the resources that powered this service
		for _, o := range p.objects {
			p.log.Infof("scaling %s from 0 -> %d", o.GetKey(), o.Replicas)
			if err := p.c.scaleObject(context.Background(), o, o.Replicas); err != nil {
				p.log.WithError(err).Warn("failed to scale back up object")
			}
		}
	}()

	lastErr := ErrNotInitialized
	localPort := 0
	cleanupFn := func() {}

	var po *corev1.Pod
	var fw *portforward.PortForwarder
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				if errors.Is(lastErr, ErrNotInitialized) {
					p.log.Debug("creating tunnel connection")
				} else {
					p.log.WithError(lastErr).Errorf("connection died, recreating tunnel connection")
				}

				if !errors.Is(lastErr, ErrNotInitialized) {
					// we can't really do exponential backoff right now, so do a set time
					select {
					case <-ctx.Done():
						return
					case <-time.After(time.Second * 5):
					}
				} else {
					// reset our err at this point, if we were not initialized
					lastErr = nil
				}

				// clean up the old pod, if it exists
				cleanupFn()

				var err error
				cleanupFn, po, err = p.createServerPod(ctx)
				if err != nil {
					p.log.WithError(err).Debug("failed to create pod")
					lastErr = ErrUnderlyingTransportPodDestroyed
					continue
				}

				errorChan := make(chan error)
				localPort, fw, err = p.createTransport(ctx, po, 0, errorChan)
				if err != nil {
					if fw != nil {
						fw.Close()
					}

					p.log.WithError(err).Debug("failed to recreate transport port-forward")
					lastErr = ErrUnderlyingTransportDied
					continue
				}

				cli := ssh.NewReverseTunnelClient(p.log, "127.0.0.1", localPort, ports)
				go func() {
					errorChan <- cli.Start(ctx, p.ServiceName)
				}()

				// handle errors
				// if we get an error or even a nil err then we should
				// clean up
				select {
				case <-ctx.Done():
				case err := <-errorChan:
					p.log.WithError(err).Debug("transport died")
					lastErr = err
				}

				// cleanup the port-forward if the above died
				if fw != nil {
					fw.Close()
				}
			}
		}
	}()

	// wait for the context to finish
	<-ctx.Done()

	cleanupFn()
	return nil
}
