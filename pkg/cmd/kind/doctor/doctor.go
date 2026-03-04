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
	osexec "os/exec"

	"github.com/spf13/cobra"

	"sigs.k8s.io/kind/pkg/cmd"
	"sigs.k8s.io/kind/pkg/exec"
	"sigs.k8s.io/kind/pkg/log"
)

// result holds the outcome of a single doctor check.
type result struct {
	name    string
	status  string // "ok", "warn", or "fail"
	message string
}

// checkResult is the JSON-serializable form of a single doctor check.
type checkResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

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
		Long:  "Checks for required binaries (container runtime, kubectl) and exits 0 if all ok, 1 on failure, 2 on warnings",
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

	var results []result

	// Container runtime check: try docker, podman, nerdctl in order.
	runtimes := []string{"docker", "podman", "nerdctl"}
	foundRuntime := false
	for _, rt := range runtimes {
		found, working := checkBinary(rt)
		if found && working {
			results = append(results, result{
				name:   rt,
				status: "ok",
			})
			foundRuntime = true
			break
		} else if found {
			// Binary present but daemon not responding.
			results = append(results, result{
				name:    rt,
				status:  "warn",
				message: rt + " found but not responding — is the daemon running?",
			})
			foundRuntime = true
			break
		}
	}
	if !foundRuntime {
		results = append(results, result{
			name:    "container-runtime",
			status:  "fail",
			message: "no container runtime found — install Docker (https://docs.docker.com/get-docker/), Podman (https://podman.io/getting-started/installation), or nerdctl (https://github.com/containerd/nerdctl)",
		})
	}

	// kubectl check
	found, working := checkKubectl()
	if !found {
		results = append(results, result{
			name:    "kubectl",
			status:  "fail",
			message: "kubectl not found — install from https://kubernetes.io/docs/tasks/tools/",
		})
	} else if !working {
		results = append(results, result{
			name:    "kubectl",
			status:  "warn",
			message: "kubectl found but 'kubectl version --client' failed — check your installation",
		})
	} else {
		results = append(results, result{
			name:   "kubectl",
			status: "ok",
		})
	}

	// Compute exit codes from results (must happen before branching on output format).
	hasFail := false
	hasWarn := false
	for _, r := range results {
		switch r.status {
		case "fail":
			hasFail = true
		case "warn":
			hasWarn = true
		}
	}

	if flags.Output == "json" {
		var out []checkResult
		for _, r := range results {
			out = append(out, checkResult{
				Name:    r.name,
				Status:  r.status,
				Message: r.message,
			})
		}
		if err := json.NewEncoder(streams.Out).Encode(out); err != nil {
			return err
		}
		// Exit with structured codes after JSON output.
		if hasFail {
			os.Exit(1)
		}
		if hasWarn {
			os.Exit(2)
		}
		return nil
	}

	// Human-readable output to stderr.
	for _, r := range results {
		switch r.status {
		case "ok":
			fmt.Fprintf(streams.ErrOut, "[ OK ] %s\n", r.name) //nolint:errcheck
		case "warn":
			fmt.Fprintf(streams.ErrOut, "[WARN] %s: %s\n", r.name, r.message) //nolint:errcheck
		case "fail":
			fmt.Fprintf(streams.ErrOut, "[FAIL] %s: %s\n", r.name, r.message) //nolint:errcheck
		}
	}

	// Exit with structured codes. os.Exit bypasses Cobra's error handling,
	// which is necessary because Cobra always exits 1 for any non-nil error.
	if hasFail {
		os.Exit(1)
	}
	if hasWarn {
		os.Exit(2)
	}
	return nil
}

// checkBinary checks whether a binary is on PATH and responding to version commands.
// Returns (found, working).
func checkBinary(name string) (found bool, working bool) {
	if _, err := osexec.LookPath(name); err != nil {
		return false, false
	}
	// Binary is on PATH; try "version" first.
	lines, err := exec.OutputLines(exec.Command(name, "version"))
	if err == nil && len(lines) > 0 {
		return true, true
	}
	// Fall back to "-v".
	lines, err = exec.OutputLines(exec.Command(name, "-v"))
	if err == nil && len(lines) > 0 {
		return true, true
	}
	return true, false
}

// checkKubectl checks whether kubectl is on PATH and responds to version --client.
// Returns (found, working).
func checkKubectl() (found bool, working bool) {
	if _, err := osexec.LookPath("kubectl"); err != nil {
		return false, false
	}
	// Use --client to avoid contacting the API server.
	lines, err := exec.OutputLines(exec.Command("kubectl", "version", "--client"))
	if err == nil && len(lines) > 0 {
		return true, true
	}
	return true, false
}
