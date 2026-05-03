/*
Copyright 2026 The Kubernetes Authors.

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

// Package pause implements the `kinder pause [cluster-name]` command, which
// stops every container in a kinder cluster in quorum-safe order so the host
// can reclaim CPU and RAM. Cluster state survives the pause and is restored
// by `kinder resume`.
//
// This file currently scaffolds the command surface and flag set. The full
// orchestration body is implemented in plan 47-02.
package pause

import (
	"github.com/spf13/cobra"

	"sigs.k8s.io/kind/pkg/cmd"
	"sigs.k8s.io/kind/pkg/errors"
	"sigs.k8s.io/kind/pkg/log"

	"sigs.k8s.io/kind/pkg/internal/cli"
)

// flagpole holds the parsed flag values for `kinder pause`.
type flagpole struct {
	// Timeout is the per-container graceful stop timeout, in seconds, before
	// SIGKILL is sent. Generous default leaves room for kubelet/etcd flush.
	Timeout int
	// JSON enables JSON output for scripted consumers.
	JSON bool
}

// NewCommand returns a new cobra.Command for `kinder pause [cluster-name]`.
func NewCommand(logger log.Logger, streams cmd.IOStreams) *cobra.Command {
	flags := &flagpole{}
	c := &cobra.Command{
		Args:  cobra.MaximumNArgs(1),
		Use:   "pause [cluster-name]",
		Short: "Pauses a running cluster, stopping all containers to reclaim CPU/RAM",
		Long: "Pauses a running kinder cluster by gracefully stopping every node container.\n" +
			"Workers stop before control-plane nodes to keep etcd quorum safe.\n" +
			"If no cluster name is given and exactly one cluster exists, it is auto-selected.",
		// IMPLEMENTED IN: 47-02-PLAN.md
		RunE: func(cmd *cobra.Command, args []string) error {
			cli.OverrideDefaultName(cmd.Flags())
			_ = flags // silence unused-write linter until plan 02 wires the body
			_ = logger
			_ = streams
			_ = args
			return errors.New("kinder pause: not yet implemented (phase 47 plan 02)")
		},
	}
	c.Flags().IntVar(&flags.Timeout, "timeout", 30, "graceful stop timeout in seconds before SIGKILL")
	c.Flags().BoolVar(&flags.JSON, "json", false, "output JSON")
	return c
}
