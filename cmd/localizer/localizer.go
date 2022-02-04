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
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/bombsimon/logrusr/v2"
	"github.com/getoutreach/localizer/internal/kevents"
	"github.com/getoutreach/localizer/internal/kube"
	"github.com/getoutreach/localizer/internal/server"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"k8s.io/klog/v2"
)

/// Deviation(unbootstrapped): waiting on OSS bootstrap

var Version = "v0.0.0-unset"

func main() { //nolint:funlen
	ctx, cancel := context.WithCancel(context.Background())
	log := logrus.New()
	log.Formatter = &logrus.TextFormatter{
		ForceColors: true,
	}

	tmpFilePath := filepath.Join(os.TempDir(), "localizer-"+strings.ReplaceAll(time.Now().Format(time.RFC3339), ":", "-")+".log")
	tmpFile, err := os.Create(tmpFilePath)
	if err == nil {
		defer tmpFile.Close()

		log.Out = io.MultiWriter(os.Stderr, tmpFile)
	}

	// this prevents the CLI from clobbering context cancellation
	cli.OsExiter = func(code int) {
		if code != 0 {
			os.Exit(code)
		}
	}

	app := cli.App{
		Version:              Version,
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
				Value:       "DEBUG",
				DefaultText: "DEBUG",
			},
			&cli.StringFlag{
				Name:        "log-format",
				Usage:       "Set the log format",
				EnvVars:     []string{"LOG_FORMAT"},
				DefaultText: "TEXT",
			},
			&cli.StringFlag{
				Name:  "cluster-domain",
				Usage: "Configure the cluster domain used for service DNS endpoints",
				Value: "cluster.local",
			},
			&cli.StringFlag{
				Name:  "ip-cidr",
				Usage: "Set the IP address CIDR, must include the /",
				Value: "127.0.0.1/8",
			},
			&cli.StringFlag{
				Name:  "namespace",
				Usage: "Restrict forwarding to the given namespace. (default: all namespaces)",
			},
		},
		Commands: []*cli.Command{
			NewListCommand(log),
			NewExposeCommand(log),
		},
		Before: func(c *cli.Context) error {
			sigC := make(chan os.Signal, 1)
			signal.Notify(sigC, os.Interrupt, syscall.SIGTERM)
			go func() {
				sig := <-sigC
				log.WithField("signal", sig.String()).Info("shutting down")
				cancel()
			}()

			// good for testing shut down issues
			// go func() {
			// 	time.Sleep(2 * time.Second)
			// 	sigC <- os.Interrupt
			// }()

			if strings.EqualFold(c.String("log-level"), "debug") {
				log.SetLevel(logrus.DebugLevel)
			}

			if strings.EqualFold(c.String("log-format"), "JSON") {
				log.SetFormatter(&logrus.JSONFormatter{})
			}

			klog.SetLogger(logrusr.New(log))

			// setup the global kubernetes cache interface
			config, k, err := kube.GetKubeClient(c.String("context"))
			if err != nil {
				return err
			}
			log.Infof("using apiserver %s", config.Host)
			kevents.ConfigureGlobalCache(k, c.String("namespace"))

			return nil
		},
		Action: func(c *cli.Context) error {
			u, err := user.Current()
			if err != nil {
				return errors.Wrap(err, "failed to get current user")
			}

			if u.Uid != "0" {
				return fmt.Errorf("must be run as root/Administrator")
			}

			clusterDomain := c.String("cluster-domain")
			ipCidr := c.String("ip-cidr")

			log.Infof("using cluster domain: %v", clusterDomain)
			log.Infof("using ip cidr: %v", ipCidr)

			srv := server.NewGRPCService(&server.RunOpts{
				ClusterDomain: clusterDomain,
				IPCidr:        ipCidr,
				KubeContext:   c.String("context"),
			})
			return srv.Run(ctx, log)
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Errorf("failed to run: %v", err)
		return
	}
}
