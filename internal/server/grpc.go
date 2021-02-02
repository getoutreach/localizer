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
	"google.golang.org/grpc/reflection"

	apiv1 "github.com/jaredallard/localizer/api/v1"
	"github.com/jaredallard/localizer/internal/kevents"
)

const SocketPath = "/var/run/localizer.sock"

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
func (g *GRPCService) CleanupPreviousInstance(ctx context.Context) error {
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	conn, err := grpc.DialContext(ctx, "unix://"+SocketPath,
		grpc.WithBlock(), grpc.WithInsecure())

	// if we made a connection, see if it's responding to pings
	// eventually we can expose useful information here?
	if err == nil {
		client := apiv1.NewLocalizerServiceClient(conn)
		_, err = client.Ping(ctx, &apiv1.PingRequest{})
		if err == nil {
			return fmt.Errorf("localizer instance is already running")
		}
	}

	return errors.Wrap(os.Remove(SocketPath), "failed to cleanup socket from old localizer instance")
}

// Run starts a grpc server with the internal server handler
func (g *GRPCService) Run(ctx context.Context, log logrus.FieldLogger) error {
	if _, err := os.Stat(SocketPath); err == nil {
		// if we found an existing instance, attempt to cleanup after it
		err = g.CleanupPreviousInstance(ctx)
		if err != nil {
			return err
		}
	}

	l, err := net.Listen("unix", SocketPath)
	if err != nil {
		return errors.Wrap(err, "failed to listen on socket")
	}
	defer os.Remove(SocketPath)

	err = os.Chmod(SocketPath, 0777)
	if err != nil {
		return err
	}

	g.lis = l

	h, err := NewServiceHandler(ctx, log, g.opts)
	if err != nil {
		return err
	}

	g.srv = grpc.NewServer()
	reflection.Register(g.srv)
	apiv1.RegisterLocalizerServiceServer(g.srv, h)

	// handle closing the server
	go func() {
		<-ctx.Done()
		log.Info("shutting down server")
		g.srv.GracefulStop()
	}()

	// One day Serve() will accept a context?
	log.Infof("starting GRPC server on '%s'", SocketPath)
	go func() {
		err := g.srv.Serve(g.lis)
		if err != nil {
			log.WithError(err).Error("grpc server exited")
		}
	}()

	// Triggers population, needed for pkg/kube
	kevents.GlobalCache.Apps().V1().Deployments().Informer()
	kevents.GlobalCache.Apps().V1().StatefulSets().Informer()

	//start the informers
	kevents.GlobalCache.Start(ctx.Done())
	log.Info("Waiting for caches to sync...")
	kevents.GlobalCache.WaitForCacheSync(ctx.Done())
	log.Info("Caches synced")

	if err := h.p.Start(ctx); err != nil {
		log.WithError(err).Error("failed to start proxy informers")
	}

	h.exp.Wait()

	return nil
}
