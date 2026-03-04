/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package actions defines the Action interface and utilities for cluster creation actions.
package actions

import (
	"context"
	"sync"

	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/internal/apis/config"
	"sigs.k8s.io/kind/pkg/internal/cli"
	"sigs.k8s.io/kind/pkg/log"

	"sigs.k8s.io/kind/pkg/cluster/internal/providers"
)

// Action defines a step of bringing up a kind cluster after initial node
// container creation
type Action interface {
	Execute(ctx *ActionContext) error
}

// ActionContext is data supplied to all actions
type ActionContext struct {
	// Context carries cancellation and deadline for node commands. Never nil.
	Context    context.Context
	Logger     log.Logger
	Status     *cli.Status
	Config     *config.Cluster
	Provider   providers.Provider
	nodesOnce  func() ([]nodes.Node, error) // sync.OnceValues: race-free cached ListNodes
}

// NewActionContext returns a new ActionContext
func NewActionContext(
	ctx context.Context,
	logger log.Logger,
	status *cli.Status,
	provider providers.Provider,
	cfg *config.Cluster,
) *ActionContext {
	return &ActionContext{
		Context:  ctx,
		Logger:   logger,
		Status:   status,
		Provider: provider,
		Config:   cfg,
		nodesOnce: sync.OnceValues(func() ([]nodes.Node, error) {
			return provider.ListNodes(cfg.Name)
		}),
	}
}

// Nodes returns the list of cluster nodes, cached after the first call.
// Concurrent calls are safe: sync.OnceValues guarantees exactly-once execution.
func (ac *ActionContext) Nodes() ([]nodes.Node, error) {
	return ac.nodesOnce()
}
