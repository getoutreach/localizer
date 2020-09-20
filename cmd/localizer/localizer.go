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
	"os"
	"os/signal"
	"os/user"
	"strconv"
	"strings"
	"syscall"

	"github.com/jaredallard/localizer/internal/expose"
	"github.com/jaredallard/localizer/internal/kube"
	"github.com/jaredallard/localizer/internal/proxier"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	log := logrus.New()

	var kconf *rest.Config
	var k kubernetes.Interface

	app := cli.App{
		Version: "1.0.0",
		Name:    "localizer",
		Flags: []cli.Flag{
			// Note: KUBECONFIG is respected, but we don't allow passing a
			// CLI argument to reduce the complexity and re-parsing of it.
			&cli.StringFlag{
				Name:    "context",
				Usage:   "Specify Kubernetes context to use",
				EnvVars: []string{"KUBECONTEXT"},
			},
			&cli.StringSliceFlag{
				Name:  "skip-app",
				Usage: "Skip forwarding an application locally",
			},
			&cli.StringSliceFlag{
				Name:  "skip-namespace",
				Usage: "Skip forwarding to a namespace",
			},
			&cli.StringFlag{
				Name:        "log-level",
				Usage:       "Set the log level",
				EnvVars:     []string{"LOG_LEVEL"},
				DefaultText: "INFO",
			},
		},
		Commands: []*cli.Command{
			{
				Name:        "expose",
				Description: "Expose a local port to a remote service in Kubernetes",
				Usage:       "expose <localPort>[:remotePort] <service>",
				Action: func(c *cli.Context) error {
					localPortStr := c.Args().Get(0)
					remotePortStr := localPortStr
					service := c.Args().Get(1)

					if localPortStr == "" {
						return fmt.Errorf("missing localPort")
					}

					if service == "" {
						return fmt.Errorf("missing service")
					}

					serviceSplit := strings.Split(service, "/")
					if len(serviceSplit) != 2 {
						return fmt.Errorf("serviceName should be in the format: namespace/serviceName")
					}

					portSplit := strings.Split(localPortStr, ":")
					if len(portSplit) > 2 {
						return fmt.Errorf("localPort/remotePort was invalid, expected format: localPort[:remotePort], got '%v'", localPortStr)
					}

					if len(portSplit) == 2 {
						localPortStr = portSplit[0]
						localPortStr = portSplit[1]
					}

					localPort, err := strconv.ParseUint(localPortStr, 10, 0)
					if err != nil {
						return fmt.Errorf("localPort is not an unsigned integer")
					}
					remotePort, err := strconv.Atoi(remotePortStr)
					if err != nil {
						return fmt.Errorf("remotePort is not an unsigned integer")
					}

					e := expose.NewExposer(k, log)
					if err := e.Start(ctx); err != nil {
						return err
					}

					p, err := e.Expose(ctx, uint(localPort), uint(remotePort), serviceSplit[0], serviceSplit[1])
					if err != nil {
						return errors.Wrap(err, "failed to create reverse port-forward")
					}

					return p.Start(ctx)
				},
			},
		},
		Before: func(c *cli.Context) error {
			u, err := user.Current()
			if err != nil {
				return errors.Wrap(err, "failed to get current user")
			}

			if u.Uid != "0" {
				return fmt.Errorf("must be run as root/Administrator")
			}

			sigC := make(chan os.Signal)
			signal.Notify(sigC, os.Interrupt, syscall.SIGTERM)
			go func() {
				<-sigC
				cancel()
			}()

			if strings.ToLower(c.String("log-level")) == "debug" {
				log.SetLevel(logrus.DebugLevel)
				log.Debug("set logger to debug")
			}

			kconf, k, err = kube.GetKubeClient(c.String("context"))
			if err != nil {
				return errors.Wrap(err, "failed to create kube client")
			}

			return nil
		},
		Action: func(c *cli.Context) error {
			d := proxier.NewDiscoverer(k, log)
			p := proxier.NewProxier(k, kconf, log)

			log.Debug("waiting for caches to sync")
			if err := p.Start(ctx); err != nil {
				return errors.Wrap(err, "failed to start proxy informers")
			}

			services, err := d.Discover(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to discover services")
			}

			nameFilterHM := make(map[string]bool)
			for _, serv := range append(c.StringSlice("skip-app"), "kubernetes") {
				nameFilterHM[serv] = true
			}
			namespaceFilterHM := make(map[string]bool)
			for _, serv := range append(c.StringSlice("skip-namespace"), "kube-system") {
				namespaceFilterHM[serv] = true
			}

			filteredServices := make([]proxier.Service, 0)
			for _, serv := range services {
				if nameFilterHM[serv.Name] {
					continue
				}

				if namespaceFilterHM[serv.Namespace] {
					continue
				}

				filteredServices = append(filteredServices, serv)
			}

			if len(filteredServices) == 0 {
				log.Info("found no services, exiting ...")
				return nil
			}

			if err := p.Add(filteredServices...); err != nil {
				return errors.Wrap(err, "failed to add discovered services to proxy")
			}

			return errors.Wrap(p.Proxy(ctx), "failed to start proxier")
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Errorf("failed to run: %v", err)
		os.Exit(1)
	}
}
