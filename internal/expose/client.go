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
	"time"

	"github.com/jaredallard/localizer/internal/kube"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

type Client struct {
	k     kubernetes.Interface
	kconf *rest.Config
	log   logrus.FieldLogger

	podStore        cache.Store
	replicaSetStore cache.Store
}

// NewExposer returns a new client capable of exposing localports to remote locations
func NewExposer(k kubernetes.Interface, kconf *rest.Config, log logrus.FieldLogger) *Client {
	return &Client{
		k,
		kconf,
		log,
		nil,
		nil,
	}
}

// Start warms up the expose cache and enables running Expose()
// among other things.
func (c *Client) Start(ctx context.Context) error {
	podStore, podInformer := cache.NewInformer(
		cache.NewListWatchFromClient(c.k.CoreV1().RESTClient(), "pods", corev1.NamespaceAll, fields.Everything()),
		&corev1.Pod{},
		time.Second*0,
		cache.ResourceEventHandlerFuncs{},
	)

	replicaSetStore, replicaSetInformer := cache.NewInformer(
		cache.NewListWatchFromClient(c.k.AppsV1().RESTClient(), "replicasets", corev1.NamespaceAll, fields.Everything()),
		&appsv1.ReplicaSet{},
		time.Second*0,
		cache.ResourceEventHandlerFuncs{},
	)

	c.podStore = podStore
	c.replicaSetStore = replicaSetStore

	go podInformer.Run(ctx.Done())
	go replicaSetInformer.Run(ctx.Done())

	if ok := cache.WaitForCacheSync(ctx.Done(), podInformer.HasSynced, replicaSetInformer.HasSynced); !ok {
		return fmt.Errorf("failed to populate pod/replicaset cache")
	}

	return nil
}

// getObjectReplicas attempts to get the number of replicas
// that a statefulset, deployment, or X type has in appsv1.
func (c *Client) getObjectReplicas(ctx context.Context, obj *corev1.ObjectReference) (uint, error) {
	// TODO(jaredallard): I imagine this + "s" hack won't work for everything
	req := c.k.AppsV1().RESTClient().Get().Resource(obj.Kind + "s").
		Namespace(obj.Namespace).Name(obj.Name).SubResource("scale")

	c.log.WithField("url", req.URL().String()).Debug("getting replicas")
	res := req.Do(ctx)
	if res.Error() != nil {
		return 0, res.Error()
	}

	o, err := res.Get()
	if err != nil {
		return 0, err
	}

	scale, ok := o.(*autoscalingv1.Scale)
	if !ok {
		return 0, fmt.Errorf("returned type wasn't a scale, got kind: %v", o.GetObjectKind())
	}

	return uint(scale.Spec.Replicas), nil
}

type patchUInt32Value struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value uint   `json:"value"`
}

// scaleObject attempts to scale a given object in appsv1
func (c *Client) scaleObject(ctx context.Context, obj *corev1.ObjectReference, replicas uint) error {
	payload := []patchUInt32Value{{
		Op:    "replace",
		Path:  "/spec/replicas",
		Value: replicas,
	}}
	payloadBytes, _ := json.Marshal(payload)
	// TODO(jaredallard): I imagine this + "s" hack won't work for everything
	req := c.k.AppsV1().RESTClient().Patch(types.JSONPatchType).Resource(obj.Kind + "s").
		Namespace(obj.Namespace).Name(obj.Name).Body(payloadBytes)

	c.log.WithField("url", req.URL().String()).Debug("setting replicas")
	res := req.Do(ctx)
	if res.Error() != nil {
		return res.Error()
	}
	return nil
}

// getPodOwnerRef returns the first apps/v1 ref that a pod has, if it has one
// if no error is returned and nref is nil then it can be assumed that this
// pod has no owner
func (c *Client) getPodOwnerRef(ref *corev1.ObjectReference) (nref *corev1.ObjectReference, err error) {
	// lookup the replicaset's parent deployment, if
	// it has one.
	k := c.getOwnerRefKey(ref)
	obj, exists, err := c.podStore.GetByKey(k)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find pod: %v", k)
	}
	if !exists {
		return nil, fmt.Errorf("failed to find pod: %v", k)
	}

	p, ok := obj.(*corev1.Pod)
	if !ok {
		c.log.WithField("obj", obj).Debug("got invalid object")
		return nil, fmt.Errorf("found invalid pod: %v", k)
	}

	if len(p.OwnerReferences) != 0 {
		for _, r := range p.OwnerReferences {
			// handoff the "real" owner of this object
			// by mimicing the ObjectReference found in
			// targetRef
			nref = &corev1.ObjectReference{
				APIVersion: r.APIVersion,
				Kind:       r.Kind,
				Namespace:  ref.Namespace,
				Name:       r.Name,
				UID:        r.UID,
			}
			break
		}
	}

	return nref, nil
}

func (c *Client) getOwnerRefKey(ref *corev1.ObjectReference) string {
	return fmt.Sprintf("%s/%s", ref.Namespace, ref.Name)
}

// Expose exposed a port, localPort, on the local host, and opens a remote port
// that can be accessed via the remote service at remotePort
func (c *Client) Expose(ctx context.Context, ports []kube.ResolvedServicePort, namespace, serviceName string) (*ServiceForward, error) { //nolint:funlen,gocyclo,lll
	s, err := c.k.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if len(s.Spec.Selector) == 0 {
		return nil, fmt.Errorf("headless services are not supported")
	}

	e, err := c.k.CoreV1().Endpoints(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	objects := make(map[string]scaledObjectType)

	if len(e.Subsets) == 0 {
		// TODO(jaredallard): If no endpoints, then we can just drop it
		return nil, fmt.Errorf("failed to find any endpoints for this service")
	}

	c.log.Debugf("found %d subsets", len(e.Subsets))
	for _, s := range e.Subsets {
		c.log.Debugf("found %d addresses", len(s.Addresses))
		for _, a := range s.Addresses {
			if a.TargetRef == nil {
				c.log.WithFields(logrus.Fields{
					"endpoint": e.ObjectMeta.Name,
					"address":  a,
				}).Debug("skipping a nil targetRef")
				continue
			}

			if a.TargetRef.Kind != "Pod" {
				c.log.WithFields(logrus.Fields{
					"endpoint": e.ObjectMeta.Name,
					"ref":      a.TargetRef,
				}).Debug("skipping a non-pod targetRef")
				continue
			}

			podOwner, err := c.getPodOwnerRef(a.TargetRef)
			if err != nil {
				return nil, errors.Wrap(err, "failed to calculate owner of a pod")
			}

			// this pod has no owner, not much we can do besides warn
			if podOwner == nil {
				k := c.getOwnerRefKey(a.TargetRef)
				c.log.WithField("pod", k).Warn("a pod was found with no controller, this may lead to intermittent issues")
				continue
			}

			a.TargetRef = podOwner

			replicas := uint(0)
			switch a.TargetRef.Kind {
			case "ReplicaSet", "StatefulSet":
				if a.TargetRef.Kind == "ReplicaSet" {
					// lookup the replicaset's parent deployment, if
					// it has one.
					k := c.getOwnerRefKey(a.TargetRef)

					obj, exists, err := c.replicaSetStore.GetByKey(k)
					if err != nil {
						return nil, errors.Wrapf(err, "failed to find replicaset: %v", k)
					}
					if !exists {
						return nil, fmt.Errorf("failed to find replicaset: %v", k)
					}

					rs, ok := obj.(*appsv1.ReplicaSet)
					if !ok {
						c.log.WithField("obj", obj).Debug("got invalid object")
						return nil, fmt.Errorf("found invalid replicaset: %v", k)
					}

					// if there's an owner reference, attempt to handle it
					// TODO(jaredallard): Do we want to limit to just Deployments?
					if len(rs.OwnerReferences) != 0 {
						for _, ref := range rs.OwnerReferences {
							// handoff the "real" owner of this object
							// by mimicing the ObjectReference found in
							// targetRef
							a.TargetRef = &corev1.ObjectReference{
								APIVersion: ref.APIVersion,
								Kind:       ref.Kind,
								Namespace:  a.TargetRef.Namespace,
								Name:       ref.Name,
								UID:        ref.UID,
							}
							break
						}
					}
				}

				var err error
				replicas, err = c.getObjectReplicas(ctx, a.TargetRef)
				if err != nil {
					return nil, errors.Wrap(err, "failed to get replicas for a deployment")
				}
			default:
				c.log.WithFields(logrus.Fields{
					"endpoint":  e.ObjectMeta.Name,
					"targetRef": a.TargetRef,
				}).Debug("skipping unknown targetRef")
				continue
			}

			o := scaledObjectType{
				*a.TargetRef,
				replicas,
			}

			objects[o.GetKey()] = o
		}
	}

	return &ServiceForward{
		c:         c,
		Namespace: namespace,
		Selector:  s.Spec.Selector,
		Ports:     ports,
		objects:   objects,
	}, nil
}
