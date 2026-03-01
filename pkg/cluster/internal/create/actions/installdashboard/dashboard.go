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

// Package installdashboard implements the action to install the Headlamp
// Kubernetes dashboard.
package installdashboard

import (
	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions"
)

type action struct{}

// NewAction returns a new action for installing the Dashboard
func NewAction() actions.Action {
	return &action{}
}

// Execute runs the action
func (a *action) Execute(ctx *actions.ActionContext) error {
	ctx.Status.Start("Installing Dashboard (stub)")
	defer ctx.Status.End(false)
	// TODO: Real implementation in Phase 6
	ctx.Status.End(true)
	return nil
}
