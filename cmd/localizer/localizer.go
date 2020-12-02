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
	"io/ioutil"
	"os"
	"os/signal"
	"os/user"
	"sort"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/go-logr/logr"
	"github.com/jaredallard/localizer/internal/server"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"google.golang.org/grpc"
	"k8s.io/klog/v2"

	apiv1 "github.com/jaredallard/localizer/api/v1"
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
// suffixes to the logger's name.  It's strongly recommended
// that name segments contain only letters, digits, and hyphens
// (see the package documentation for more information).
func (l *klogToLogrus) WithName(name string) logr.Logger {
	return l
}

func main() { //nolint:funlen,gocyclo
	ctx, cancel := context.WithCancel(context.Background())
	log := logrus.New()

	// this prevents the CLI from clobbering context cancellation
	cli.OsExiter = func(code int) {
		if code != 0 {
			os.Exit(code)
		}
	}

	app := cli.App{
		Version:              "1.0.0",
		EnableBashCompletion: true,
		Name:                 "localizer",
		Flags: []cli.Flag{
			// Note: KUBECONFIG is respected, but we don't allow passing a
			// CLI argument to reduce the complexity and re-parsing of it.
			&cli.StringFlag{
				Name:    "context",
				Usage:   "Specify Kubernetes context to use",
				EnvVars: []string{"KUBECONTEXT"},
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
				Name:        "list",
				Description: "list all port-forwarded services and their status(es)",
				Usage:       "list",
				Action: func(c *cli.Context) error {
					if _, err := os.Stat(server.SocketPath); os.IsNotExist(err) {
						return fmt.Errorf("localizer daemon not running (run localizer by itself?)")
					}

					ctx, cancel = context.WithTimeout(ctx, 30*time.Second)

					conn, err := grpc.DialContext(ctx, "unix://"+server.SocketPath,
						grpc.WithBlock(), grpc.WithInsecure())
					if err != nil {
						return errors.Wrap(err, "failed to talk to localizer daemon")
					}

					cli := apiv1.NewLocalizerServiceClient(conn)

					resp, err := cli.List(ctx, &apiv1.ListRequest{})
					if err != nil {
						return err
					}

					w := tabwriter.NewWriter(os.Stdout, 10, 0, 5, ' ', 0)
					defer w.Flush()

					fmt.Fprintf(w, "NAMESPACE\tNAME\tSTATUS\t\n")
					sort.Slice(resp.Services, func(i, j int) bool {
						return resp.Services[i].Namespace < resp.Services[j].Namespace
					})
					for _, s := range resp.Services {
						fmt.Fprintf(w, "%s\t%s\t%s\t\n", s.Namespace, s.Name, strings.ToUpper(s.Status[:1])+s.Status[1:])
					}

					return nil
				},
			},
			{
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
				// TODO: multiple service support before this gets released
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

					log.Info("connecting to localizer daemon")
					conn, err := grpc.DialContext(ctx, "unix://"+server.SocketPath,
						grpc.WithBlock(), grpc.WithInsecure(), grpc.WithTimeout(30*time.Second))
					if err != nil {
						return errors.Wrap(err, "failed to talk to localizer daemon")
					}

					cli := apiv1.NewLocalizerServiceClient(conn)

					var stream apiv1.LocalizerService_ExposeServiceClient
					if c.Bool("stop") {
						log.Info("sending stop expose request to daemon")
						stream, err = cli.StopExpose(ctx, &apiv1.StopExposeRequest{
							Namespace: serviceNamespace,
							Service:   serviceName,
						})
					} else {
						log.Info("sending expose request to daemon")
						stream, err = cli.ExposeService(ctx, &apiv1.ExposeServiceRequest{
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
						}

						// errors responses get caught here, so we just return them
						if err != nil {
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

			return nil
		},
		Action: func(c *cli.Context) error {
			srv := server.NewGRPCService()
			return srv.Run(ctx, log)
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Errorf("failed to run: %v", err)
		os.Exit(1)
	}
}
