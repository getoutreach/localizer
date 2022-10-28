// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file has the package localizer.

// Package localizer is meant to contain useful helper functions, variables, and
// constants in order to better programatically interact with localizer.
package localizer

import (
	"context"
	"fmt"
	"os"

	"github.com/getoutreach/localizer/api"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

// Socket is the communication endpoint that the localizer server is listening
// on.
const Socket = "/var/run/localizer.sock"

// IsRunning checks to see if the localizer socket exists.
func IsRunning() bool {
	if _, err := os.Stat(Socket); err != nil {
		return false
	}

	return true
}

// Connect returns a new instance of LocalizerServiceClient given a gRPC client
// connection (returned from grpc.Dial*).
func Connect(ctx context.Context, opts ...grpc.DialOption) (client api.LocalizerServiceClient,
	closer func(), err error) {
	clientConn, err := grpc.DialContext(ctx, fmt.Sprintf("unix://%s", Socket), opts...)
	if err != nil {
		return nil, nil, errors.Wrap(err, "dial localizer")
	}

	return api.NewLocalizerServiceClient(clientConn), func() {
		_ = clientConn.Close() //nolint:errcheck // Why: We can't do anything about an error regarding closing the client connection.
		// We eat the error here so we don't have to nolint on every call.
	}, nil
}
