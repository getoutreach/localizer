// Copyright 2022 Outreach Corporation. All Rights Reserved.
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

// Description: This file has the package server.
package server

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/getoutreach/localizer/internal/expose"
	"github.com/getoutreach/localizer/internal/kube"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/getoutreach/localizer/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func mapPorts(portMap []string, log logrus.FieldLogger, servicePorts []kube.ResolvedServicePort) error {
	for _, portOverride := range portMap {
		spl := strings.Split(portOverride, ":")
		if len(spl) != 2 {
			return fmt.Errorf("invalid port map '%s', expected 'local:remote'", portOverride)
		}

		local, err := strconv.ParseUint(spl[0], 10, 0)
		if err != nil {
			return errors.Wrapf(err, "failed to parse port map '%s'", portOverride)
		}

		rem, err := strconv.ParseUint(spl[1], 10, 0)
		if err != nil {
			return errors.Wrapf(err, "failed to parse port map '%s'", portOverride)
		}

		// TODO(jaredallard): this is slow...
		for i, sp := range servicePorts {
			log.Debugf("checking if we need to map %s, using %d:%d", sp.TargetPort.String(), rem, local)
			// nolint: gosec // Why: ports are never negative
			if uint(servicePorts[i].TargetPort.IntValue()) == uint(rem) {
				log.Debugf("mapping remote port %d -> %d locally", rem, local)
				servicePorts[i].MappedPort = uint(local)
			}
		}
	}

	return nil
}

type newExpose struct {
	ports       []kube.ResolvedServicePort
	namespace   string
	serviceName string
}

type Exposer struct {
	k     kubernetes.Interface
	kconf *rest.Config
	e     *expose.Client
	log   logrus.FieldLogger

	// parentCtx shuts down all exposers when canceled
	parentCtx context.Context

	portForwards map[string]context.CancelFunc
	pfMutex      sync.Mutex

	workerChan chan newExpose
	doneChan   chan struct{}
}

// NewExposer creates a service that can maintain multiple expose instances
func NewExposer(parentCtx context.Context, k kubernetes.Interface, kconf *rest.Config, log logrus.FieldLogger) (*Exposer, error) {
	log = log.WithField("component", "exposer")

	e := expose.NewExposer(k, kconf, log)

	exp := &Exposer{
		e:            e,
		k:            k,
		kconf:        kconf,
		log:          log,
		parentCtx:    parentCtx,
		portForwards: make(map[string]context.CancelFunc),
		workerChan:   make(chan newExpose),
		doneChan:     make(chan struct{}),
	}

	go exp.worker()

	return exp, nil
}

func getKey(namespace, serviceName string) string {
	return fmt.Sprintf("%s/%s", namespace, serviceName)
}

func (e *Exposer) worker() {
	// when this exits we're done
	defer close(e.doneChan)

	wg := sync.WaitGroup{}
	for {
		select {
		case <-e.parentCtx.Done():
			e.log.Info("waiting for exposes to finish")

			// wait for the connections to close
			wg.Wait()
			e.log.Info("expose instances finished")

			return
		case expMsg := <-e.workerChan:
			key := getKey(expMsg.namespace, expMsg.serviceName)

			if e.portForwards[key] != nil {
				// this one is already allocated, skip it
				continue
			}

			exp, err := e.e.Expose(e.parentCtx, expMsg.ports, expMsg.namespace, expMsg.serviceName)
			if err != nil {
				// TODO(jaredallard): send this error back
				e.log.WithError(err).Error("failed to create expose")
				continue
			}

			workerCtx, cancel := context.WithCancel(e.parentCtx)

			// take lock so we can start the expose
			e.pfMutex.Lock()

			// spin up goroutine that'll terminate itself later
			go func(ctx context.Context) {
				err := exp.Start(ctx)
				if err != nil {
					e.log.WithError(err).Error("expose exited with an error")
				}

				// if we exited we need to signify that we're now not taken
				e.pfMutex.Lock()
				defer e.pfMutex.Unlock()

				e.portForwards[key] = nil

				wg.Done()
			}(workerCtx)

			wg.Add(1)
			e.portForwards[key] = cancel
			e.pfMutex.Unlock()
		}
	}
}

func (e *Exposer) Close(namespace, serviceName string) error {
	k := getKey(namespace, serviceName)
	if e.portForwards[k] == nil {
		return fmt.Errorf("service '%s' isn't exposed", k)
	}

	// canceling the context doesn't ensure it's 100% closed, we need to do that at somepoint
	e.portForwards[k]()

	return nil
}

// Wait waits for all exposes to be shut down
func (e *Exposer) Wait() {
	<-e.doneChan

	e.log.Info("exposes cleaned up")
}

func (e *Exposer) Start(ports []kube.ResolvedServicePort, namespace, serviceName string) error {
	e.workerChan <- newExpose{
		ports:       ports,
		namespace:   namespace,
		serviceName: serviceName,
	}

	// TODO(jaredallard): propregate error
	return nil
}

func (h *GRPCServiceHandler) StopExpose(req *api.StopExposeRequest, res api.LocalizerService_StopExposeServer) error {
	return h.exp.Close(req.Namespace, req.Service)
}

func (h *GRPCServiceHandler) ExposeService(req *api.ExposeServiceRequest, res api.LocalizerService_ExposeServiceServer) error {
	log := h.log
	ctx := h.ctx

	// discover the service's ports
	key := fmt.Sprintf("%s/%s", req.Namespace, req.Service)
	s, err := h.k.CoreV1().Services(req.Namespace).Get(ctx, req.Service, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to get service '%s'", key)
	}

	if len(s.Spec.Ports) == 0 {
		return fmt.Errorf("service had no defined ports")
	}

	servicePorts, err := kube.ResolveServicePorts(log, s)
	if err != nil {
		return errors.Wrap(err, "failed to resolve service ports")
	}

	// handle mapped ports
	if err := mapPorts(req.PortMap, log, servicePorts); err != nil {
		return err
	}

	return h.exp.Start(servicePorts, req.Namespace, req.Service)
}
