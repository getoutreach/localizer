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
	"net/http"
	"os"
	"os/signal"
	"os/user"
	"strconv"
	"strings"
	"syscall"

	"github.com/davecgh/go-spew/spew"
	"github.com/jaredallard/localizer/internal/expose"
	"github.com/jaredallard/localizer/internal/kube"
	"github.com/jaredallard/localizer/internal/proxier"
	"github.com/omrikiei/ktunnel/pkg/server"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// getUserInput returns user input and prints the given prompt
func getUserInput(prompt string) (string, error) {
	fmt.Print(prompt)

	var input string
	_, err := fmt.Scanln(&input)
	if err != nil {
		return "", err
	}

	return input, err
}

func main() { //nolint:funlen,gocyclo
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
				Name:        "server",
				Description: "Run a server that can be used with `expose`",
				Usage:       "server",
				Action: func(c *cli.Context) error {
					go func() {
						if err := http.ListenAndServe(":51",
							http.HandlerFunc(
								func(w http.ResponseWriter, r *http.Request) { fmt.Fprintln(w, "ready") },
							),
						); err != nil {
							log.WithError(err).Fatal("failed to start readiness prob server")
						}
					}()

					// note: port 50 is chosen as least likely to collide with anything
					// we may want to look into randomizing it in the future
					port := 50
					tls := false
					return errors.Wrap(server.RunServer(ctx, &port, &tls, nil, nil), "server failed")
				},
			},
			{
				Name:        "expose",
				Description: "Expose ports for a given service to Kubernetes",
				Usage:       "expose <service>",
				Flags: []cli.Flag{
					&cli.StringSliceFlag{
						// Note: We should make this true override support
						// right now this will only work for a targetPort -> another targetPort
						// and not the noted localPort
						Name:  "map",
						Usage: "Map a service's target port to another port, --map targetPortName:remotePort",
					},
				},
				Action: func(c *cli.Context) error {
					service := c.Args().Get(0)
					if service == "" {
						return fmt.Errorf("missing service")
					}

					serviceSplit := strings.Split(service, "/")
					if len(serviceSplit) != 2 {
						return fmt.Errorf("serviceName should be in the format: namespace/serviceName")
					}

					// discover the service's ports
					s, err := k.CoreV1().Services(serviceSplit[0]).Get(ctx, serviceSplit[1], metav1.GetOptions{})
					if err != nil {
						return errors.Wrapf(err, "failed to get service '%s'", service)
					}

					if len(s.Spec.Ports) == 0 {
						return fmt.Errorf("service had no defined ports")
					}

					servicePorts, exists, err := kube.ResolveServicePorts(ctx, k, s)
					if err != nil {
						return errors.Wrap(err, "failed to resolve service ports")
					}

					portOverrides := make(map[string]uint)
					for _, portOverride := range c.StringSlice("map") {
						spl := strings.Split(portOverride, ":")
						if len(spl) != 2 {
							return fmt.Errorf("invalid port map '%s', expected 'local:remote'", portOverride)
						}

						local := spl[0]
						rem := spl[1]

						remote, err := strconv.ParseUint(rem, 10, 0)
						if err != nil {
							return errors.Wrapf(err, "failed to parse port map '%s'", portOverride)
						}

						portOverrides[local] = uint(remote)
					}

					// if we couldn't find endpoints, check if we mapped the port, if not
					// then prompt the user
					if !exists {
						log.Info("Failed to resolve ports due to endpoints not exists, please use --map or awnser the below prompt(s)")
						for _, sp := range servicePorts {
							if sp.TargetPort.Type != intstr.String {
								continue
							}

							v, err := getUserInput(fmt.Sprintf("Please enter a port to map '%s' to: ", sp.TargetPort.StrVal))
							if err != nil {
								return errors.Wrap(err, "failed to get user input")
							}

							remote, err := strconv.ParseUint(v, 10, 0)
							if err != nil {
								return errors.Wrapf(err, "failed to parse port map '%s'", v)
							}

							portOverrides[strconv.Itoa(int(sp.Port))] = uint(remote)
						}
					}

					mappedServicePorts := make([]kube.ResolvedServicePort, len(servicePorts))
					for i, sp := range servicePorts {
						mappedPort, ok := portOverrides[strconv.Itoa(int(sp.Port))]
						if ok {
							sp.Port = int32(mappedPort)
							sp.TargetPort = intstr.FromInt(int(mappedPort))
						}
						mappedServicePorts[i] = sp
					}

					log.WithField("portoverride", "").Debug(spew.Sdump(portOverrides))

					// if there's no endpoints
					if !exists {
						log.Debug("service has no endpoints")
					}

					e := expose.NewExposer(k, kconf, log)
					if err := e.Start(ctx); err != nil {
						return err
					}

					p, err := e.Expose(ctx, mappedServicePorts, serviceSplit[0], serviceSplit[1])
					if err != nil {
						return errors.Wrap(err, "failed to create reverse tunnel")
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

			if strings.EqualFold(c.String("log-level"), "debug") {
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
