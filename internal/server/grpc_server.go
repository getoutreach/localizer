// Copyright 2022 Outreach Corporation. Licensed under the Apache License 2.0.
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

// Description: This file has the package server.
package server

import (
	"context"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	///StartBlock(imports)
	"github.com/getoutreach/localizer/api"
	"github.com/getoutreach/localizer/internal/kube"
	"github.com/getoutreach/localizer/internal/proxier"
	///EndBlock(imports)
)

///StartBlock(globalVars)
///EndBlock(globalVars)

type GRPCServiceHandler struct {
	log logrus.FieldLogger
	api.UnimplementedLocalizerServiceServer

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

func NewServiceHandler(ctx context.Context, log logrus.FieldLogger, opts *RunOpts) (*GRPCServiceHandler, error) {
	///StartBlock(grpcInit)
	log = log.WithField("service", "*api.GRPCServiceHandler")

	// TODO(jaredallard): pass context
	kconf, k, err := kube.GetKubeClient(opts.KubeContext)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create kube client")
	}

	exp, err := NewExposer(ctx, k, kconf, log)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start expose container")
	}

	p, err := proxier.NewProxier(ctx, k, kconf, log, &proxier.ProxyOpts{
		ClusterDomain:  opts.ClusterDomain,
		IPCidr:         opts.IPCidr,
		SkipNamespaces: opts.SkipNamespaces,
	})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create proxier")
	}
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
