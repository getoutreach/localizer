// Copyright 2022 Outreach Corporation. All Rights Reserved.

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
	"github.com/urfave/cli/v2"
	"google.golang.org/grpc"
)

func NewListCommand(_ logrus.FieldLogger) *cli.Command {
	return &cli.Command{
		Name:        "list",
		Description: "list all port-forwarded services and their status(es)",
		Usage:       "list",
		Action: func(c *cli.Context) error {
			if !localizer.IsRunning() {
				return fmt.Errorf("localizer daemon not running (run localizer by itself?)")
			}

			ctx, cancel := context.WithTimeout(c.Context, 30*time.Second)
			defer cancel()

			client, closer, err := localizer.Connect(ctx, grpc.WithBlock(), grpc.WithInsecure()) // nolint: staticcheck // Why: need to test
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
