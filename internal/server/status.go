package server

import (
	"context"

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
		services[i] = &apiv1.ListService{
			Namespace: s.ServiceInfo.Namespace,
			Name:      s.ServiceInfo.Name,

			// TODO: support sending all statuses
			Status: string(s.Statuses[0]),
		}
	}

	return &apiv1.ListResponse{Services: services}, nil
}
