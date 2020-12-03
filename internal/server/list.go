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
package server

import (
	"context"
	"fmt"

	apiv1 "github.com/jaredallard/localizer/api/v1"
)

func (h *GRPCServiceHandler) List(ctx context.Context, req *apiv1.ListRequest) (*apiv1.ListResponse, error) {
	statuses, err := h.p.List(ctx)
	if err != nil {
		return nil, err
	}

	services := make([]*apiv1.ListService, len(statuses))
	for i := range statuses {
		s := &statuses[i]

		ports := make([]string, len(s.Ports))
		for i, p := range s.Ports {
			ports[i] = fmt.Sprintf("%d/tcp", p)
		}

		services[i] = &apiv1.ListService{
			Namespace:    s.ServiceInfo.Namespace,
			Name:         s.ServiceInfo.Name,
			Endpoint:     s.Endpoint.Name,
			StatusReason: s.Reason,
			Status:       string(s.Statuses[0]),
			Ip:           s.IP,
			Ports:        ports,
		}
	}

	return &apiv1.ListResponse{Services: services}, nil
}
