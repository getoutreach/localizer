// Copyright 2025 Outreach Corporation. Licensed under the Apache License 2.0.

// Description: This file is the entrypoint for the localizer CLI
// command for localizer.
// Managed: true

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"os/user"
	"strings"
	"syscall"

	oapp "github.com/getoutreach/gobox/pkg/app"
	gcli "github.com/getoutreach/gobox/pkg/cli"
	"github.com/getoutreach/localizer/internal/kevents"
	"github.com/getoutreach/localizer/internal/kube"
	"github.com/getoutreach/localizer/internal/server"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"k8s.io/klog/v2"

	// Place any extra imports for your startup code here
	// <<Stencil::Block(imports)>>
	logrusr "github.com/bombsimon/logrusr/v2"
	// <</Stencil::Block>>
)

// <<Stencil::Block(global)>>

// <</Stencil::Block>>

// main is the entrypoint for the localizer CLI.
func main() {
	ctx, cancel := context.WithCancel(context.Background())
	log := logrus.New()

	// <<Stencil::Block(init)>>

	// <</Stencil::Block>>

	app := cli.App{
		Version: oapp.Version,
		Name:    "localizer",
		// <<Stencil::Block(app)>>
		EnableBashCompletion: true,
		// <</Stencil::Block>>
	}
	app.Flags = []cli.Flag{
		// <<Stencil::Block(flags)>>
		// Note: KUBECONFIG is respected, but we don't allow passing a
		// CLI argument to reduce the complexity and re-parsing of it.
		&cli.StringFlag{
			Name:    "context",
			Usage:   "Specify Kubernetes context to use",
			EnvVars: []string{"KUBECONTEXT"},
		},
		&cli.StringFlag{
			Name:    "log-level",
			Usage:   "Set the log level. Valid values are: debug",
			EnvVars: []string{"LOG_LEVEL"},
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
		&cli.StringSliceFlag{
			Name:  "skip-namespace",
			Usage: "Skip forwarding services from the following namespace",
		},
		// <</Stencil::Block>>
	}
	app.Commands = []*cli.Command{
		// <<Stencil::Block(commands)>>
		NewListCommand(log),
		NewExposeCommand(log),
		// <</Stencil::Block>>
	}

	// <<Stencil::Block(postApp)>>
	log.Formatter = &logrus.TextFormatter{
		ForceColors: true,
	}

	app.Before = func(c *cli.Context) error {
		// Automatic updater is currently disabled
		// until consumers have time to pass in
		// --skip-update if required.
		c.Set("skip-update", "true") //nolint:errcheck // Why: Best effort

		sigC := make(chan os.Signal, 1)
		signal.Notify(sigC, os.Interrupt, syscall.SIGTERM)
		go func() {
			sig := <-sigC
			log.WithField("signal", sig.String()).Info("shutting down")
			cancel()
		}()

		if strings.EqualFold(c.String("log-level"), "debug") {
			log.SetLevel(logrus.DebugLevel)
		}

		if strings.EqualFold(c.String("log-format"), "JSON") {
			log.SetFormatter(&logrus.JSONFormatter{})
		}

		klog.SetLogger(logrusr.New(log, logrusr.WithReportCaller()))

		// setup the global kubernetes cache interface
		config, k, err := kube.GetKubeClient(c.String("context"))
		if err != nil {
			return err
		}
		log.Infof("using apiserver %s", config.Host)
		kevents.ConfigureGlobalCache(k, c.String("namespace"))

		return nil
	}

	app.Action = func(c *cli.Context) error {
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
	}
	// <</Stencil::Block>>

	// Insert global flags, tracing, updating and start the application.
	gcli.Run(ctx, cancel, &app, &gcli.Config{
		Logger: log,
		Telemetry: gcli.TelemetryConfig{
			UseDelibird: true,
		},
	})
}
