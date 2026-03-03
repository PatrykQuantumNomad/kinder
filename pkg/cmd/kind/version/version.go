/*
Copyright 2018 The Kubernetes Authors.

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

// Package version implements the `version` command
package version

import (
	"fmt"

	"github.com/spf13/cobra"

	"sigs.k8s.io/kind/pkg/cmd"
	"sigs.k8s.io/kind/pkg/internal/kindversion"
	"sigs.k8s.io/kind/pkg/log"
)

// NewCommand returns a new cobra.Command for version
func NewCommand(logger log.Logger, streams cmd.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Prints the kind CLI version",
		Long:  "Prints the kind CLI version",
		RunE: func(cmd *cobra.Command, args []string) error {
			if logger.V(0).Enabled() {
				// if not -q / --quiet, show lots of info
				fmt.Fprintln(streams.Out, kindversion.DisplayVersion()) //nolint:errcheck
			} else {
				// otherwise only show semver
				fmt.Fprintln(streams.Out, kindversion.Version()) //nolint:errcheck
			}
			return nil
		},
	}
	return cmd
}
