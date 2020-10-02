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
	"io/ioutil"
	"os"
	"os/signal"
	"os/user"
	"strconv"
	"strings"
	"syscall"

	"github.com/go-logr/logr"
	"github.com/jaredallard/localizer/internal/expose"
	"github.com/jaredallard/localizer/internal/kube"
	"github.com/jaredallard/localizer/internal/proxier"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

type klogToLogrus struct {
	log logrus.FieldLogger
}

// Enabled tests whether this Logger is enabled.  For example, commandline
// flags might be used to set the logging verbosity and disable some info
// logs.
func (l *klogToLogrus) Enabled() bool {
	return true
}

// Info logs a non-error message with the given key/value pairs as context.
//
// The msg argument should be used to add some constant description to
// the log line.  The key/value pairs can then be used to add additional
// variable information.  The key/value pairs should alternate string
// keys and arbitrary values.
func (l *klogToLogrus) Info(msg string, keysAndValues ...interface{}) {
	l.log.Info(msg)
}

// Error logs an error, with the given message and key/value pairs as context.
// It functions similarly to calling Info with the "error" named value, but may
// have unique behavior, and should be preferred for logging errors (see the
// package documentations for more information).
//
// The msg field should be used to add context to any underlying error,
// while the err field should be used to attach the actual error that
// triggered this log line, if present.
func (l *klogToLogrus) Error(err error, msg string, keysAndValues ...interface{}) {
	l.log.WithError(err).Error(msg)
}

// V returns an Logger value for a specific verbosity level, relative to
// this Logger.  In other words, V values are additive.  V higher verbosity
// level means a log message is less important.  It's illegal to pass a log
// level less than zero.
func (l *klogToLogrus) V(level int) logr.Logger {
	return l
}

// WithValues adds some key-value pairs of context to a logger.
// See Info for documentation on how key/value pairs work.
func (l *klogToLogrus) WithValues(keysAndValues ...interface{}) logr.Logger {
	return l
}

// WithName adds a new element to the logger's name.
// Successive calls with WithName continue to append
// suffixes to the logger's name.  It's strongly reccomended
// that name segments contain only letters, digits, and hyphens
// (see the package documentation for more information).
func (l *klogToLogrus) WithName(name string) logr.Logger {
	return l
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
			&cli.StringFlag{
				Name:        "log-format",
				Usage:       "Set the log format",
				EnvVars:     []string{"LOG_FORMAT"},
				DefaultText: "TEXT",
			},
		},
		Commands: []*cli.Command{
			{
				Name:        "expose",
				Description: "Expose ports for a given service to Kubernetes",
				Usage:       "expose <service>",
				Flags: []cli.Flag{
					&cli.StringSliceFlag{
						Name:  "map",
						Usage: "Map a local port to a remote port, i.e --map 80:8080 will bind what is normally :8080 to :80 locally",
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

					// if we couldn't find endpoints, then we fall back to binding whatever the
					// public port of the service is if it is named
					if !exists {
						for i, sp := range servicePorts {
							if servicePorts[i].TargetPort.Type == intstr.String {
								log.Warnf("failed to determine the value of port %s, using public port %d", sp.TargetPort.String(), sp.Port)
								servicePorts[i].TargetPort = intstr.FromInt(int(sp.Port))
							}
						}

						log.Debug("service has no endpoints")
					}

					log.Debugf("map %v", c.StringSlice("map"))
					for _, portOverride := range c.StringSlice("map") {
						spl := strings.Split(portOverride, ":")
						if len(spl) != 2 {
							return fmt.Errorf("invalid port map '%s', expected 'local:remote'", portOverride)
						}

						local, err := strconv.ParseUint(spl[0], 10, 0)
						if err != nil {
							return errors.Wrapf(err, "failed to parse port map '%s'", portOverride)
						}

						rem, err := strconv.ParseUint(spl[1], 10, 0)
						if err != nil {
							return errors.Wrapf(err, "failed to parse port map '%s'", portOverride)
						}

						// TODO: this is slow...
						for i, sp := range servicePorts {
							log.Debugf("checking if we need to map %s, using %d:%d", sp.TargetPort.String(), rem, local)
							if uint(servicePorts[i].TargetPort.IntValue()) == uint(rem) {
								log.Debugf("mapping remote port %d -> %d locally", rem, local)
								servicePorts[i].MappedPort = uint(local)
							}
						}
					}

					e := expose.NewExposer(k, kconf, log)
					if err := e.Start(ctx); err != nil {
						return err
					}

					p, err := e.Expose(ctx, servicePorts, serviceSplit[0], serviceSplit[1])
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

			if strings.EqualFold(c.String("log-format"), "JSON") {
				log.SetFormatter(&logrus.JSONFormatter{})
				log.Debug("set log format to JSON")
			}

			// disable client-go logging
			discardLogger := logrus.New()
			discardLogger.Out = ioutil.Discard
			klog.SetLogger(&klogToLogrus{log: discardLogger})

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
