package kevents

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	corev1 "k8s.io/api/core/v1"
)

// GlobalCache is an optional global cache that can be initialized
var GlobalCache *Cache

type EventHandler func(Event)

type Cache struct {
	k kubernetes.Interface

	stores    map[string]cache.Store
	subs      map[string][]EventHandler
	namespace string
}

type Event struct {
	Event EventType

	// OldObject is only set when an old object applies
	// which is for EventTypeUpdated.
	OldObject interface{}
	NewObject interface{}
}

type EventType string

var (
	EventTypeAdded   EventType = "added"
	EventTypeUpdated EventType = "updated"
	EventTypeDeleted EventType = "delete"
)

// ConfigureGlobalCache sets up package wide global cache
func ConfigureGlobalCache(k kubernetes.Interface, namespace string) {
	GlobalCache = &Cache{k, make(map[string]cache.Store), make(map[string][]EventHandler), namespace}
}

func WaitForSync(ctx context.Context, cont cache.Controller) error {
	go cont.Run(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), cont.HasSynced) {
		return fmt.Errorf("failed to sync cache")
	}

	return nil
}

func (c *Cache) handleEvent(objType string, e EventType, oldObj, newObj interface{}) {
	// If we have no subs, then just skip it
	if _, ok := c.subs[objType]; !ok {
		return
	}

	event := Event{
		Event:     e,
		OldObject: oldObj,
		NewObject: newObj,
	}

	for _, fn := range c.subs[objType] {
		fn(event)
	}
}

func (c *Cache) getObjectType(obj runtime.Object) string {
	return reflect.TypeOf(obj).String()
}

// TrackObject starts tracking a object and adds it to the cache.
// This must be called before any other calls for a specific object.
func (c *Cache) TrackObject(resourceName string, obj runtime.Object) cache.Controller {
	key := c.getObjectType(obj)

	objStore, objInformer := cache.NewInformer(
		cache.NewListWatchFromClient(c.k.CoreV1().RESTClient(), resourceName, c.namespace, fields.Everything()),
		&corev1.Endpoints{},
		time.Second*60,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				c.handleEvent(key, EventTypeAdded, nil, obj)
			},
			UpdateFunc: func(oldObj, obj interface{}) {
				c.handleEvent(key, EventTypeUpdated, oldObj, obj)
			},
			DeleteFunc: func(obj interface{}) {
				c.handleEvent(key, EventTypeDeleted, nil, obj)
			},
		},
	)

	c.stores[key] = objStore
	c.subs[key] = make([]EventHandler, 0)

	return objInformer
}

// GetStore returns the store for a given object. Can be nil
// if Subscribe or TrackObject was never called.
func (c *Cache) GetStore(obj runtime.Object) cache.Store {
	return c.stores[c.getObjectType(obj)]
}

// Subscribe adds an event listener to a given object in the cache
// Event is receive only
func (c *Cache) Subscribe(ctx context.Context, resourceName string, obj runtime.Object, objChan EventHandler) error {
	key := c.getObjectType(obj)

	var inf cache.Controller
	if _, ok := c.subs[key]; !ok {
		inf = c.TrackObject(resourceName, obj)
	}

	c.subs[key] = append(c.subs[key], objChan)

	// start the informer
	if inf != nil {
		go inf.Run(ctx.Done())
	}

	return nil
}
