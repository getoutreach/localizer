// Copyright 2021 Outreach.io
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
package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/getoutreach/localizer/api"
	"github.com/getoutreach/localizer/pkg/localizer"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"google.golang.org/grpc"
)

func NewExposeCommand(log logrus.FieldLogger) *cli.Command { //nolint:funlen
	return &cli.Command{
		Name:        "expose",
		Description: "Expose ports for a given service to Kubernetes",
		Usage:       "expose <namespace/service>",
		Flags: []cli.Flag{
			&cli.StringSliceFlag{
				Name:  "map",
				Usage: "Map a local port to a remote port, i.e --map 80:8080 will bind what is normally :8080 to :80 locally",
			},
			&cli.BoolFlag{
				Name:  "stop",
				Usage: "stop exposing a service",
			},
		},
		Action: func(c *cli.Context) error {
			split := strings.Split(c.Args().First(), "/")
			if len(split) != 2 {
				return fmt.Errorf("invalid service, expected namespace/name")
			}

			serviceNamespace := split[0]
			serviceName := split[1]

			if !localizer.IsRunning() {
				return fmt.Errorf("localizer daemon not running (run localizer by itself?)")
			}

			ctx, cancel := context.WithTimeout(c.Context, 30*time.Second)
			defer cancel()

			log.Info("connecting to localizer daemon")

			client, closer, err := localizer.Connect(ctx, grpc.WithBlock(), grpc.WithInsecure())
			if err != nil {
				return errors.Wrap(err, "failed to connect to localizer daemon")
			}
			defer closer() //nolint:errcheck // Why: Nothing we can really do about the error that comes from closing the gRPC client connection.

			var stream api.LocalizerService_ExposeServiceClient
			if c.Bool("stop") {
				log.Info("sending stop expose request to daemon")
				stream, err = client.StopExpose(ctx, &api.StopExposeRequest{
					Namespace: serviceNamespace,
					Service:   serviceName,
				})
			} else {
				log.Info("sending expose request to daemon")
				stream, err = client.ExposeService(ctx, &api.ExposeServiceRequest{
					PortMap:   c.StringSlice("map"),
					Namespace: serviceNamespace,
					Service:   serviceName,
				})
			}
			if err != nil {
				return err
			}

			for {
				res, err := stream.Recv()
				if err == io.EOF {
					return nil
				} else if err != nil {
					return err
				}

				logger := log.Info
				switch res.Level {
				case api.ConsoleLevel_CONSOLE_LEVEL_INFO, api.ConsoleLevel_CONSOLE_LEVEL_UNSPECIFIED:
				case api.ConsoleLevel_CONSOLE_LEVEL_WARN:
					logger = log.Warn
				case api.ConsoleLevel_CONSOLE_LEVEL_ERROR:
					logger = log.Error
				}

				logger(res.Message)
			}
		},
	}
}
