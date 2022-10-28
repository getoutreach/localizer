// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file has the package server.
package server

import (
	"context"

	"github.com/getoutreach/localizer/api"
)

func (h *GRPCServiceHandler) Ping(ctx context.Context, req *api.PingRequest) (*api.PingResponse, error) {
	return &api.PingResponse{}, nil
}
