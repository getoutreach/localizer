package kevents

import (
	"time"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

// GlobalCache is an optional global cache that can be initialized
var GlobalCache informers.SharedInformerFactory

// ConfigureGlobalCache sets up package wide global cache
func ConfigureGlobalCache(k kubernetes.Interface, namespace string) {
	GlobalCache = informers.NewSharedInformerFactoryWithOptions(k, 10*time.Minute, informers.WithNamespace(namespace))
}
