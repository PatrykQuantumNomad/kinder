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

// Package doctor implements the `doctor` command
package doctor

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"sigs.k8s.io/kind/pkg/cmd"
	"sigs.k8s.io/kind/pkg/internal/doctor"
	"sigs.k8s.io/kind/pkg/log"
)

type flagpole struct {
	Output string
}

// NewCommand returns a new cobra.Command for the doctor subcommand
func NewCommand(logger log.Logger, streams cmd.IOStreams) *cobra.Command {
	flags := &flagpole{}
	c := &cobra.Command{
		Args:  cobra.NoArgs,
		Use:   "doctor",
		Short: "Checks prerequisite binaries and reports actionable fix messages",
		Long:  "Checks for required binaries (container runtime, kubectl) and GPU drivers with category-grouped output. Exits 0 if all ok, 1 on failure, 2 on warnings",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runE(streams, flags)
		},
	}
	c.Flags().StringVar(&flags.Output, "output", "", "output format; supported values: \"\", \"json\"")
	return c
}

func runE(streams cmd.IOStreams, flags *flagpole) error {
	switch flags.Output {
	case "", "json":
		// valid
	default:
		return fmt.Errorf("unsupported output format %q; supported values: \"\", \"json\"", flags.Output)
	}

	results := doctor.RunAllChecks()

	if flags.Output == "json" {
		if err := json.NewEncoder(streams.Out).Encode(doctor.FormatJSON(results)); err != nil {
			return err
		}
	} else {
		doctor.FormatHumanReadable(streams.ErrOut, results)
	}

	// Exit with structured codes. os.Exit bypasses Cobra's error handling,
	// which is necessary because Cobra always exits 1 for any non-nil error.
	os.Exit(doctor.ExitCodeFromResults(results))
	return nil
}
