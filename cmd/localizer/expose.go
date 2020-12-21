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
	"os"
	"strings"
	"time"

	apiv1 "github.com/jaredallard/localizer/api/v1"
	"github.com/jaredallard/localizer/internal/server"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"google.golang.org/grpc"
)

func NewExposeCommand(log logrus.FieldLogger) *cli.Command {
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

			if _, err := os.Stat(server.SocketPath); os.IsNotExist(err) {
				return fmt.Errorf("localizer daemon not running (run localizer by itself?)")
			}

			ctx, cancel := context.WithTimeout(c.Context, 30*time.Second)
			defer cancel()

			log.Info("connecting to localizer daemon")
			conn, err := grpc.DialContext(ctx, "unix://"+server.SocketPath,
				grpc.WithBlock(), grpc.WithInsecure())
			if err != nil {
				return errors.Wrap(err, "failed to talk to localizer daemon")
			}

			client := apiv1.NewLocalizerServiceClient(conn)

			var stream apiv1.LocalizerService_ExposeServiceClient
			if c.Bool("stop") {
				log.Info("sending stop expose request to daemon")
				stream, err = client.StopExpose(ctx, &apiv1.StopExposeRequest{
					Namespace: serviceNamespace,
					Service:   serviceName,
				})
			} else {
				log.Info("sending expose request to daemon")
				stream, err = client.ExposeService(ctx, &apiv1.ExposeServiceRequest{
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
				case apiv1.ConsoleLevel_CONSOLE_LEVEL_INFO, apiv1.ConsoleLevel_CONSOLE_LEVEL_UNSPECIFIED:
				case apiv1.ConsoleLevel_CONSOLE_LEVEL_WARN:
					logger = log.Warn
				case apiv1.ConsoleLevel_CONSOLE_LEVEL_ERROR:
					logger = log.Error
				}

				logger(res.Message)
			}
		},
	}
}
