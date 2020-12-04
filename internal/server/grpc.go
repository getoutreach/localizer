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

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	apiv1 "github.com/jaredallard/localizer/api/v1"
)

const SocketPath = "/var/run/localizer.sock"

type GRPCService struct {
	lis net.Listener
	srv *grpc.Server
}

func NewGRPCService() *GRPCService {
	return &GRPCService{}
}

// Run starts a grpc server with the internal server handler
func (g *GRPCService) Run(ctx context.Context, log logrus.FieldLogger) error {
	if _, err := os.Stat(SocketPath); err == nil {
		return fmt.Errorf("localizer instance already running")
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

	h, err := NewServiceHandler(ctx, log)
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

	if err := h.p.Start(ctx); err != nil {
		log.WithError(err).Error("failed to start proxy informers")
	}

	h.exp.Wait()

	return nil
}
