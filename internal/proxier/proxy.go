package proxier

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubectl/pkg/polymorphichelpers"
	"k8s.io/kubectl/pkg/util/podutils"
)

// Proxier handles creating an maintaining proxies to a remote
// Kubernetes service
type Proxier struct {
	k   kubernetes.Interface
	log logrus.FieldLogger

	s []Service

	active map[uint]*ProxyConnection
}

// ProxyConnection tracks a proxy connection
type ProxyConnection struct {
	Service *Service
	Pod     *corev1.Pod
}

// NewProxier creates a new proxier instance
func NewProxier(k kubernetes.Interface, l logrus.FieldLogger) *Proxier {
	return &Proxier{
		k:   k,
		log: l,
		s:   make([]Service, 0),
	}
}

// Add adds a service to our proxier. When Proxy() is called
// this service will be proxied.
func (p *Proxier) Add(s ...Service) error {
	p.s = append(p.s, s...)

	return nil
}

// findPodBySelector finds a pod by a given selector on a runtime.Object
func (p *Proxier) findPodBySelector(o runtime.Object) (*corev1.Pod, error) {
	namespace, selector, err := polymorphichelpers.SelectorsForObject(o)
	if err != nil {
		return nil, fmt.Errorf("cannot attach to %T: %v", o, err)
	}

	sortBy := func(pods []*corev1.Pod) sort.Interface { return sort.Reverse(podutils.ActivePods(pods)) }
	pod, _, err := polymorphichelpers.GetFirstPod(p.k.CoreV1(), namespace, selector.String(), 1*time.Minute, sortBy)
	return pod, err
}

// Proxy starts a proxier. The proxy thread is run in a go-routine
// so it is safe to execute this function and continue.
func (p *Proxier) Proxy(ctx context.Context) error {
	for _, s := range p.s {
		kserv, err := p.k.CoreV1().Services(s.Namespace).Get(ctx, s.Name, v1.GetOptions{})
		if err != nil {
			p.log.Errorf("failed to get service: %v", err)
			continue
		}

		pod, err := p.findPodBySelector(kserv)
		if err != nil {
			p.log.Errorf("failed to find pod for service '%s': %v", kserv.Name, err)
			continue
		}

		if pod.Status.Phase != corev1.PodRunning {
			p.log.Errorf("unable to forward port because pod is not running, found status %v", pod.Status.Phase)
			continue
		}

		for _, port := range s.Ports {
			p.log.Infof("creating port-forward '%s/%s:%d' -> '127.0.0.1:%d'", s.Namespace, pod.Name, port.RemotePort, port.LocalPort)
		}
	}

	return nil
}
