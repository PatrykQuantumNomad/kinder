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

// Package resume implements the `kinder resume [cluster-name]` command, which
// starts every container in a paused kinder cluster in quorum-safe order and
// waits for all nodes to report Ready before exiting.
//
// This file currently scaffolds the command surface and flag set. The full
// orchestration body is implemented in plan 47-03.
package resume

import (
	"github.com/spf13/cobra"

	"sigs.k8s.io/kind/pkg/cmd"
	"sigs.k8s.io/kind/pkg/errors"
	"sigs.k8s.io/kind/pkg/log"

	"sigs.k8s.io/kind/pkg/internal/cli"
)

// flagpole holds the parsed flag values for `kinder resume`.
type flagpole struct {
	// Timeout is the per-container graceful start timeout, in seconds.
	Timeout int
	// WaitSecs is the maximum number of seconds to wait for every node to
	// report Ready via the Kubernetes API after containers are running.
	WaitSecs int
	// JSON enables JSON output for scripted consumers.
	JSON bool
}

// NewCommand returns a new cobra.Command for `kinder resume [cluster-name]`.
func NewCommand(logger log.Logger, streams cmd.IOStreams) *cobra.Command {
	flags := &flagpole{}
	c := &cobra.Command{
		Args:  cobra.MaximumNArgs(1),
		Use:   "resume [cluster-name]",
		Short: "Resumes a paused cluster, starting all containers and waiting for nodes Ready",
		Long: "Resumes a paused kinder cluster by starting every node container in quorum-safe order\n" +
			"(control-plane before workers) and waiting for the Kubernetes API to report all nodes Ready.\n" +
			"If no cluster name is given and exactly one cluster exists, it is auto-selected.",
		// IMPLEMENTED IN: 47-03-PLAN.md
		RunE: func(cmd *cobra.Command, args []string) error {
			cli.OverrideDefaultName(cmd.Flags())
			_ = flags // silence unused-write linter until plan 03 wires the body
			_ = logger
			_ = streams
			_ = args
			return errors.New("kinder resume: not yet implemented (phase 47 plan 03)")
		},
	}
	c.Flags().IntVar(&flags.Timeout, "timeout", 30, "graceful start timeout in seconds")
	c.Flags().IntVar(&flags.WaitSecs, "wait", 300, "max seconds to wait for all nodes Ready")
	c.Flags().BoolVar(&flags.JSON, "json", false, "output JSON")
	return c
}
