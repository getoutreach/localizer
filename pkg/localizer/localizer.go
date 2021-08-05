// Package localizer is meant to contain useful helper functions, variables, and
// constants in order to better programatically interact with localizer.
package localizer

import (
	"os"

	apiv1 "github.com/getoutreach/localizer/api/v1"
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
func Connect(clientConn *grpc.ClientConn) apiv1.LocalizerServiceClient {
	return apiv1.NewLocalizerServiceClient(clientConn)
}
