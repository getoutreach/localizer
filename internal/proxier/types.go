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

// Description: This file has the package proxier.
package proxier

import (
	"fmt"
	"net/netip"

	"k8s.io/client-go/tools/portforward"
)

const PodKind = "Pod"

type PodInfo struct {
	// Name is the name of this pod
	Name string

	// Namespace is the namespace this pod is in
	Namespace string
}

func (s *PodInfo) Key() string {
	return fmt.Sprintf("%s/%s", s.Namespace, s.Name)
}

type ServiceInfo struct {
	// Name is the name of this service
	Name string

	// Namespace is the namespace of this service
	Namespace string
}

func (s *ServiceInfo) Key() string {
	return fmt.Sprintf("%s/%s", s.Namespace, s.Name)
}

// CreatePortForwardRequest is a request to create port-forward
type CreatePortForwardRequest struct {
	// Service is the service this port-forward implements.
	Service ServiceInfo

	// Hostnames are the DNS entries to inject into our host's DNS
	// for this port-forward. This will be attached to the IP that
	// is created for this service.
	Hostnames []string

	// Ports are the ports this port-forward exposes
	Ports []string

	// Endpoint is the specific pod to use for this service.
	Endpoint *PodInfo

	// Recreate specifies if this should be recreated if it already
	// exists
	Recreate       bool
	RecreateReason string
}

// DeletePortForwardRequest is a request to delete a port-forward
type DeletePortForwardRequest struct {
	// Service is the service that should delete being port-forwarded
	Service ServiceInfo

	// IsShuttingDown denotes if the entire process is shutting down as
	// opposed to a single port-forward, or group, being deleted.
	IsShuttingDown bool
}

// PortForwardRequest is a port-forward request, the non-nil struct is the type
// of request this is. There should only ever be one non-nil struct.
type PortForwardRequest struct {
	DeletePortForwardRequest *DeletePortForwardRequest
	CreatePortForwardRequest *CreatePortForwardRequest
}

// PortForwardConnection is a port-forward that is managed by the port-forward
// worker.
type PortForwardConnection struct {
	Service      ServiceInfo
	Pod          PodInfo
	Status       PortForwardStatus
	StatusReason string

	// IP that this port-forward allocates
	IP        netip.Addr
	Hostnames []string

	// Ports is a local -> remote port list
	Ports []string

	pf *portforward.PortForwarder
}

type PortForwardStatus string

var (
	PortForwardStatusRunning    PortForwardStatus = "running"
	PortForwardStatusRecreating PortForwardStatus = "recreating"
	PortForwardStatusWaiting    PortForwardStatus = "waiting"
)
