// Copyright 2022 Outreach Corporation. Licensed under the Apache License 2.0.
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

// Description: This file has the package main.
package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/getoutreach/localizer/api"
	"github.com/getoutreach/localizer/pkg/localizer"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func NewListCommand(_ logrus.FieldLogger) *cli.Command {
	return &cli.Command{
		Name:        "list",
		Description: "list all port-forwarded services and their status(es)",
		Usage:       "list",
		Action: func(ctx context.Context, c *cli.Command) error {
			if !localizer.IsRunning() {
				return fmt.Errorf("localizer daemon not running (run localizer by itself?)")
			}

			ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			// nolint: staticcheck // Why: we are not upgrading to the new grpc API yet.
			client, closer, err := localizer.Connect(ctx, grpc.WithBlock(), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				return errors.Wrap(err, "failed to connect to localizer daemon")
			}
			defer closer()

			resp, err := client.List(ctx, &api.ListRequest{})
			if err != nil {
				return err
			}

			w := tabwriter.NewWriter(os.Stdout, 10, 0, 3, ' ', 0)
			defer w.Flush()

			fmt.Fprintf(w, "NAMESPACE\tNAME\tSTATUS\tREASON\tENDPOINT\tIP ADDRESS\tPORT(S)\t\n")

			// sort by namespace and then by name
			sort.Slice(resp.Services, func(i, j int) bool {
				return resp.Services[i].Namespace < resp.Services[j].Namespace
			})
			sort.Slice(resp.Services, func(i, j int) bool {
				return resp.Services[i].Name < resp.Services[j].Name
			})

			for _, s := range resp.Services {
				status := strings.ToUpper(s.Status[:1]) + s.Status[1:]
				ip := s.Ip
				if ip == "" {
					ip = "None"
				}

				fmt.Fprintf(w,
					"%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					s.Namespace, s.Name, status, s.StatusReason, s.Endpoint, ip, strings.Join(s.Ports, ","),
				)
			}

			return nil
		},
	}
}
