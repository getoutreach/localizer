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
package kube

import (
	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"

	// Needed for external authenticators
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/client-go/tools/clientcmd"

	"k8s.io/client-go/rest"
)

// GetKubeClient returns a kubernetes client, and the config used by it, based on
// a given context. If no context is provided then the default will be used
func GetKubeClient(context string) (*rest.Config, kubernetes.Interface, error) {
	lr := clientcmd.NewDefaultClientConfigLoadingRules()
	apiconfig, err := lr.Load()
	if err != nil {
		return nil, nil, err
	}

	overrides := &clientcmd.ConfigOverrides{}
	if context != "" {
		overrides.CurrentContext = context
	}

	ccc := clientcmd.NewDefaultClientConfig(*apiconfig, overrides)

	config, err := ccc.ClientConfig()
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get kubernetes client config")
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create kubernetes client")
	}

	return config, client, nil
}
