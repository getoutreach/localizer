// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file has the package server.
package server

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	"github.com/getoutreach/localizer/api"
	"github.com/getoutreach/localizer/internal/kevents"
	"github.com/getoutreach/localizer/pkg/localizer"
)

type GRPCService struct {
	lis net.Listener
	srv *grpc.Server

	opts *RunOpts
}

type RunOpts struct {
	ClusterDomain string
	IPCidr        string
	KubeContext   string
}

func NewGRPCService(opts *RunOpts) *GRPCService {
	return &GRPCService{
		opts: opts,
	}
}

// CleanupPreviousInstance attempts to cleanup after a dead localizer instance
// if a not dead one is found, an error is returned or if it fails to cleanup
func (g *GRPCService) CleanupPreviousInstance(ctx context.Context, log logrus.FieldLogger) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	log.Info("checking if an instance of localizer is already running")
	client, closer, err := localizer.Connect(ctx, grpc.WithBlock(), grpc.WithTransportCredentials(insecure.NewCredentials()))

	// if we made a connection, see if it's responding to pings
	// eventually we can expose useful information here?
	if err == nil {
		defer closer()
		if _, err := client.Ping(ctx, &api.PingRequest{}); err == nil {
			return fmt.Errorf("localizer instance is already running")
		}
	}

	log.Warn("failed to contact existing instance, cleaning up socket")

	return errors.Wrap(os.Remove(localizer.Socket), "failed to cleanup socket from old localizer instance")
}

// Run starts a grpc server with the internal server handler
func (g *GRPCService) Run(ctx context.Context, log logrus.FieldLogger) error {
	if _, err := os.Stat(localizer.Socket); err == nil {
		// if we found an existing instance, attempt to cleanup after it
		if err := g.CleanupPreviousInstance(ctx, log); err != nil {
			return err
		}
	}

	l, err := net.Listen("unix", localizer.Socket)
	if err != nil {
		return errors.Wrap(err, "failed to listen on socket")
	}
	defer os.Remove(localizer.Socket)

	err = os.Chmod(localizer.Socket, 0o777)
	if err != nil {
		return err
	}

	g.lis = l

	// Trigger the population of our informers
	kevents.GlobalCache.Apps().V1().Deployments().Informer()
	kevents.GlobalCache.Apps().V1().StatefulSets().Informer()
	kevents.GlobalCache.Core().V1().Services().Informer()
	kevents.GlobalCache.Core().V1().Endpoints().Informer()
	kevents.GlobalCache.Core().V1().Pods().Informer()

	h, err := NewServiceHandler(ctx, log, g.opts)
	if err != nil {
		return err
	}

	g.srv = grpc.NewServer()
	reflection.Register(g.srv)
	api.RegisterLocalizerServiceServer(g.srv, h)

	// handle closing the server
	go func() {
		<-ctx.Done()
		log.Info("shutting down server")
		g.srv.GracefulStop()
	}()

	// One day Serve() will accept a context?
	log.Infof("starting GRPC server on unix://%s", localizer.Socket)
	go func() {
		err := g.srv.Serve(g.lis)
		if err != nil {
			log.WithError(err).Error("grpc server exited")
		}
	}()

	//start the informers
	kevents.GlobalCache.Start(ctx.Done())
	log.Info("Waiting for caches to sync...")
	kevents.GlobalCache.WaitForCacheSync(ctx.Done())
	log.Info("Caches synced")

	if err := h.exp.e.Start(ctx); err != nil {
		log.WithError(err).Error("failed to start exposer")
	}

	if err := h.p.Start(ctx); err != nil {
		log.WithError(err).Error("failed to start proxy informers")
	}

	h.exp.Wait()

	return nil
}
