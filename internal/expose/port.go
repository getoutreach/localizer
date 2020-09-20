package expose

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/jaredallard/localizer/internal/kube"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const localizerVersion = "v0.1.0"

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

// Start starts forwarding a service
func (p *ServiceForward) Start(ctx context.Context) error {

	// map the service ports into containerPorts, using the
	containerPorts := make([]corev1.ContainerPort, len(p.Ports))
	for i, port := range p.Ports {
		name := port.OriginalTargetPort
		if name == "" {
			name = port.Name
			if name == "" {
				// fallback to a simple port based name
				name = strconv.Itoa(int(port.TargetPort.IntVal))
			}
		}
		cp := corev1.ContainerPort{
			ContainerPort: int32(port.TargetPort.IntVal),
			Name:          name,
			Protocol:      corev1.ProtocolTCP,
		}

		containerPorts[i] = cp
	}

	// create a pod for our new expose service
	po, err := p.c.k.CoreV1().Pods(p.Namespace).Create(ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    p.Namespace,
			GenerateName: "localizer-",
			Labels:       p.Selector,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyOnFailure,
			Containers: []corev1.Container{
				{
					Name:            "default",
					Image:           "jaredallard/localizer:" + localizerVersion,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Ports:           containerPorts,
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
		return errors.Wrap(err, "failed to create pod")
	}

	p.c.log.Infof("created pod %s", po.ObjectMeta.Name)

	// scale down the other resources that powered this service
	for _, o := range p.objects {
		p.c.log.Infof("scaling %s from %d -> 0", o.GetKey(), o.Replicas)
	}

	// wait for the context to be cancelled
	<-ctx.Done()

	// scale down the other resources that powered this service
	for _, o := range p.objects {
		p.c.log.Infof("scaling %s from 0 -> %d", o.GetKey(), o.Replicas)
	}

	// delete the pod we created earlier
	return p.c.k.CoreV1().Pods(p.Namespace).Delete(context.Background(), po.Name, metav1.DeleteOptions{})
}
