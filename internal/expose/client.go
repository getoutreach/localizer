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
	"reflect"

	"github.com/getoutreach/localizer/internal/kevents"
	"github.com/getoutreach/localizer/internal/kube"
	"github.com/getoutreach/localizer/internal/reflectconversions"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/cache"
)

type Client struct {
	k     kubernetes.Interface
	kconf *rest.Config
	log   logrus.FieldLogger

	podStore cache.Store
	svcStore cache.Store
	rm       meta.RESTMapper
}

// NewExposer returns a new client capable of exposing localports to remote locations
func NewExposer(k kubernetes.Interface, kconf *rest.Config, log logrus.FieldLogger) *Client {
	return &Client{
		k,
		kconf,
		log,
		nil,
		nil,
		nil,
	}
}

// Start warms up the expose cache and enables running Expose()
// among other things.
func (c *Client) Start(ctx context.Context) error {
	podInformer := kevents.GlobalCache.Core().V1().Pods().Informer()
	svcInformer := kevents.GlobalCache.Core().V1().Services().Informer()

	c.podStore = podInformer.GetStore()
	c.svcStore = svcInformer.GetStore()

	groupResources, err := restmapper.GetAPIGroupResources(c.k.Discovery())
	if err != nil {
		return err
	}

	c.rm = restmapper.NewDiscoveryRESTMapper(groupResources)

	for _, obj := range c.podStore.List() {
		p := obj.(*corev1.Pod)

		if p.Labels[ExposedPodLabel] == "true" {
			key, _ := cache.MetaNamespaceKeyFunc(p)
			log := c.log.WithField("pod", key)
			log.Warn("removing abandoned localizer pod")

			err := c.k.CoreV1().Pods(p.Namespace).Delete(ctx, p.Name, metav1.DeleteOptions{})
			if err != nil {
				log.WithError(err).Warn("failed to remove abandoned localizer pod")
			}

			var objects []scaledObjectType
			err = json.Unmarshal([]byte(p.Annotations[ObjectsPodLabel]), &objects)
			if err != nil {
				c.log.WithError(err).Warn("failed to ensure controllers were scaled back up")
				continue
			}

			for _, obj := range objects {
				err := c.scaleObject(ctx, obj, obj.Replicas)
				if err != nil {
					c.log.WithError(err).WithField("object", obj.GetKey()).Warn("failed to restore controller scale")
					continue
				}
			}
		}
	}

	return nil
}

func (c *Client) getMapping(obj interface{}) (*meta.RESTMapping, error) {
	// We have to type switch because TypeMeta isn't being populated as it should
	// be, so we have no way to get information on what type of object we're
	// looking at without doing this.
	kind := ""
	switch obj.(type) {
	case *appsv1.Deployment:
		kind = "deployment"
	case *appsv1.StatefulSet:
		kind = "statefulset"
	}

	// hardcode appsv1 because we're doing that only in scaleObject right now
	gk := schema.GroupKind{Group: "apps", Kind: kind}
	c.log.WithField("group", gk).Debug("looking up group")
	return c.rm.RESTMapping(gk, "v1")
}

type patchUInt32Value struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value uint   `json:"value"`
}

// scaleObject attempts to scale a given object in appsv1
func (c *Client) scaleObject(ctx context.Context, scaledObj scaledObjectType, replicas int) error {
	payload := []patchUInt32Value{{
		Op:    "replace",
		Path:  "/spec/replicas",
		Value: uint(replicas),
	}}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return errors.Wrap(err, "failed to marshal scale patch body")
	}

	// TODO: build client from self link one day
	req := c.k.AppsV1().RESTClient().Patch(types.JSONPatchType).Resource(scaledObj.Resource).
		Namespace(scaledObj.GetNamespace()).Name(scaledObj.GetName()).Body(payloadBytes)

	c.log.WithField("url", req.URL().String()).Debug("setting replicas")
	res := req.Do(ctx)
	if res.Error() != nil {
		return res.Error()
	}
	return nil
}

// getReplicasFromObject uses reflect to extract the replicas from a given object
// assumes path is Spec.Replicas
func getReplicasFromObject(obj interface{}) (int, error) {
	v, err := conversion.EnforcePtr(obj)
	if err != nil {
		return 0, err
	}
	err = reflectconversions.EnforceStruct(v)
	if err != nil {
		return 0, err
	}

	specField := v.FieldByName("Spec")
	if !specField.IsValid() {
		return 0, fmt.Errorf("struct lacks struct Spec")
	}
	err = reflectconversions.EnforceStruct(v)
	if err != nil {
		return 0, err
	}

	replicasField := specField.FieldByName("Replicas")
	if !replicasField.IsValid() {
		return 0, fmt.Errorf("struct lacks field replicas")
	}
	replicasV, err := conversion.EnforcePtr(replicasField.Interface())
	if err != nil {
		return 0, err
	}
	if replicasV.Kind() != reflect.Int32 {
		return 0, fmt.Errorf("expected Replicas to be int32, but got %v: %v (%#v)", replicasV.Kind(), replicasV.Type(), replicasV.Interface())
	}

	return int(replicasV.Int()), nil
}

// getServiceControllers finds controllers that create pods for a given service
// and returns them
func (c *Client) getServiceControllers(_ context.Context, namespace, serviceName string) ([]scaledObjectType, error) {
	obj, exists, err := c.svcStore.GetByKey(fmt.Sprintf("%s/%s", namespace, serviceName))
	if err != nil {
		return nil, err
	} else if !exists {
		return nil, fmt.Errorf("failed to find service")
	}

	svc, ok := obj.(*corev1.Service)
	if !ok {
		return nil, fmt.Errorf("failed to find service in cache and reflect to type *corev1.Service")
	}

	c.log.WithField("service", fmt.Sprintf("%s/%s", namespace, serviceName)).Debug("finding controllers")
	objs, err := kube.FindControllersForService(c.log, svc)
	if err != nil {
		return nil, err
	}

	scaledObjects := make([]scaledObjectType, 0)
	for _, obj := range objs {
		metaObj, err := meta.Accessor(obj)
		if err != nil {
			c.log.WithError(err).Warn("failed to handle object")
			continue
		}
		// Don't store the entire object
		mobj := meta.AsPartialObjectMetadata(metaObj)

		mapping, err := c.getMapping(obj)
		if err != nil {
			c.log.WithError(err).Warn("failed to get controller type")
			continue
		}

		scaledObject := scaledObjectType{
			obj,
			mobj,
			0,
			mapping.Resource.Resource,
		}

		replicas, err := getReplicasFromObject(obj)
		if err != nil {
			c.log.WithError(err).Warn("failed to get replicas from object")
			continue
		}
		scaledObject.Replicas = replicas

		scaledObjects = append(scaledObjects, scaledObject)
	}

	return scaledObjects, nil
}

// Expose exposed a port, localPort, on the local host, and opens a remote port
// that can be accessed via the remote service at remotePort
func (c *Client) Expose(ctx context.Context, ports []kube.ResolvedServicePort, namespace, serviceName string) (*ServiceForward, error) {
	s, err := c.k.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if len(s.Spec.Selector) == 0 {
		return nil, fmt.Errorf("headless services are not supported")
	}

	objects, err := c.getServiceControllers(ctx, namespace, serviceName)
	if err != nil {
		// service either had no controllers, or we failed to get them. Either way
		// it's likely not the end of the world
		c.log.WithError(err).Debug("failed to get controllers")
	}

	log := c.log.WithField("service", fmt.Sprintf("%s/%s", namespace, serviceName))

	return &ServiceForward{
		c:           c,
		log:         log,
		ServiceName: serviceName,
		Namespace:   namespace,
		Selector:    s.Spec.Selector,
		Ports:       ports,
		objects:     objects,
	}, nil
}
