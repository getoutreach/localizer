// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file has the package server.
package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/getoutreach/localizer/api"
)

func (h *GRPCServiceHandler) List(ctx context.Context, req *api.ListRequest) (*api.ListResponse, error) {
	statuses, err := h.p.List(ctx)
	if err != nil {
		return nil, err
	}

	services := make([]*api.ListService, len(statuses))
	for i := range statuses {
		s := &statuses[i]

		ports := make([]string, len(s.Ports))
		for i, p := range s.Ports {
			servicePorts := strings.Split(p, ":")
			if len(servicePorts) != 2 {
				continue
			}

			sourcePort := servicePorts[0]
			destPort := servicePorts[1]
			if sourcePort == destPort {
				ports[i] = sourcePort + "/tcp"
			} else {
				ports[i] = fmt.Sprintf("%s->%s/tcp", sourcePort, destPort)
			}
		}

		services[i] = &api.ListService{
			Namespace:    s.ServiceInfo.Namespace,
			Name:         s.ServiceInfo.Name,
			Endpoint:     s.Endpoint.Name,
			StatusReason: s.Reason,
			Status:       string(s.Statuses[0]),
			Ip:           s.IP,
			Ports:        ports,
		}
	}

	return &api.ListResponse{Services: services}, nil
}
