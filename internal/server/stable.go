package server

import (
	"context"

	"github.com/getoutreach/localizer/api"
)

// Stable implements the Stable RPC for the localizer gRPC server.
func (g *GRPCServiceHandler) Stable(ctx context.Context, _ *api.Empty) (*api.StableResponse, error) {
	return &api.StableResponse{
		Stable: g.p.IsStable(),
	}, nil
}
