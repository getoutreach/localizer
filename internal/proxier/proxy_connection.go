package proxier

import (
	"context"
	"fmt"

	"github.com/jaredallard/localizer/internal/kube"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/portforward"
)

// ProxyConnection tracks a proxy connection
type ProxyConnection struct {
	proxier *Proxier
	fw      *portforward.PortForwarder

	LocalPort  uint
	RemotePort uint

	Service Service
	Pod     corev1.Pod

	// Active denotes if this connection is active
	// or not
	Active bool
}

// GetPort returns the port as a string local:remote
func (pc *ProxyConnection) GetPort() string {
	return fmt.Sprintf("%d:%d", pc.LocalPort, pc.RemotePort)
}

// Start starts a proxy connection
func (pc *ProxyConnection) Start(ctx context.Context) error {
	fw, err := kube.CreatePortForward(ctx, pc.proxier.rest, pc.proxier.kconf, &pc.Pod, pc.GetPort())
	if err != nil {
		return errors.Wrap(err, "failed to create port-forward")
	}
	pc.fw = fw

	pc.Active = true

	go func() {
		// TODO(jaredallard): Figure out a way to better backoff errors here
		if err := fw.ForwardPorts(); err != nil {
			// if this dies, mark the connection as inactive for
			// the connection reaper
			pc.Close()

			pc.proxier.log.WithField("port", pc.GetPort()).Debug("port-forward died")
			pc.proxier.handleInformerEvent("connection-dead", pc)
		}
	}()

	return nil
}

// Close closes the current proxy connection and marks it as
// no longer being active
func (pc *ProxyConnection) Close() error {
	pc.Active = false

	// note: If the parent context was cancelled
	// this has already been closed
	pc.fw.Close()

	// we'll return an error one day
	return nil
}
