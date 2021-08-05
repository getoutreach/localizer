// Package localizer is meant to contain useful helper functions, variables, and
// constants in order to better programatically interact with localizer.
package localizer

import (
	"context"
	"fmt"
	"os"

	apiv1 "github.com/getoutreach/localizer/api/v1"
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
func Connect(ctx context.Context, opts ...grpc.DialOption) (apiv1.LocalizerServiceClient, error) {
	clientConn, err := grpc.DialContext(ctx, fmt.Sprintf("unix://%s", Socket), opts...)
	if err != nil {
		return nil, errors.Wrap(err, "dial localizer")
	}

	return apiv1.NewLocalizerServiceClient(clientConn), nil
}
