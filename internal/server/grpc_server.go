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

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	///StartBlock(imports)
	apiv1 "github.com/jaredallard/localizer/api/v1"
	"github.com/jaredallard/localizer/internal/kube"
	"github.com/jaredallard/localizer/internal/proxier"
	///EndBlock(imports)
)

///StartBlock(globalVars)
///EndBlock(globalVars)

type GRPCServiceHandler struct {
	log logrus.FieldLogger
	apiv1.UnimplementedLocalizerServiceServer

	///StartBlock(grpcConfig)
	k     kubernetes.Interface
	kconf *rest.Config
	ctx   context.Context
	exp   *Exposer
	p     *proxier.Proxier
	///EndBlock(grpcConfig)
}

///StartBlock(global)
///EndBlock(global)

func NewServiceHandler(ctx context.Context, log logrus.FieldLogger) (*GRPCServiceHandler, error) {
	///StartBlock(grpcInit)
	log = log.WithField("service", "*api.GRPCServiceHandler")

	// TODO: pass context
	kconf, k, err := kube.GetKubeClient("")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create kube client")
	}

	exp, err := NewExposer(ctx, k, kconf, log)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start expose container")
	}

	p := proxier.NewProxier(ctx, k, kconf, log)
	///EndBlock(grpcInit)

	return &GRPCServiceHandler{
		log: log,
		///StartBlock(grpcConfigInit)
		k:     k,
		kconf: kconf,
		ctx:   ctx,
		exp:   exp,
		p:     p,
		///EndBlock(grpcConfigInit)
	}, nil
}

///StartBlock(grpcHandlers)
// See *.go files
///EndBlock(grpcHandlers)
