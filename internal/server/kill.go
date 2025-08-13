// Copyright 2022 Outreach Corporation. Licensed under the Apache License 2.0.

// Description: This file has the package server.
package server

import (
	"context"
	"os"
	"time"

	"github.com/getoutreach/localizer/api"
	"github.com/getoutreach/localizer/pkg/localizer"
	"github.com/pkg/errors"
)

// Kill implements the Kill RPC for the localizer gRPC server.
//
// This RPC just kills the current localizer process. Note that it actually waits until
// after the RPC returns (responds) before killing the process because if it kills it
// before it attempts to respond, the transport will have been closed already, resulting
// in a perceived error. Because of this stipulation, this RPC is only BEST EFFORT.
func (*GRPCServiceHandler) Kill(ctx context.Context, _ *api.Empty) (*api.Empty, error) {
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		return nil, errors.Wrap(err, "find localizer process")
	}

	go func(process *os.Process) {
		// Give the RPC time to respond. It doesn't need much time, because it is using the local network.
		time.Sleep(time.Second * 1)

		_ = os.Remove(localizer.Socket) //nolint:errcheck // Why: We can't do anything about this error, it's best effort.
		_ = process.Kill()              //nolint:errcheck // Why: We can't do anything about this error, it's best effort.
	}(p)

	return &api.Empty{}, nil
}
