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

// Package decode implements the `kinder doctor decode` subcommand.
package decode

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cmd"
	idoctor "sigs.k8s.io/kind/pkg/internal/doctor"
	"sigs.k8s.io/kind/pkg/internal/lifecycle"
	"sigs.k8s.io/kind/pkg/internal/runtime"
	"sigs.k8s.io/kind/pkg/log"
)

// nodeStringer is the minimal interface decode.go needs from a cluster node.
// Both the real nodes.Node and test fakes satisfy this interface.
type nodeStringer interface {
	String() string
	Role() (string, error)
}

// flagpole holds the parsed flag values for `kinder doctor decode`.
type flagpole struct {
	Name          string
	Since         time.Duration
	Output        string
	AutoFix       bool
	IncludeNormal bool
}

// ---------------------------------------------------------------------------
// Test injection points — production = real impls; tests swap via t.Cleanup.
// ---------------------------------------------------------------------------

// resolveClusterName resolves the cluster name from args (auto-detects when
// exactly one cluster exists). Tests swap with a fixed-name closure.
var resolveClusterName = func(args []string) (string, error) {
	provider := cluster.NewProvider(
		cluster.ProviderWithLogger(log.NoopLogger{}),
		runtime.GetDefault(log.NoopLogger{}),
	)
	return lifecycle.ResolveClusterName(args, provider)
}

// listNodesFn enumerates all nodes of a cluster by name. Production code uses
// a real *cluster.Provider; tests substitute a fake.
var listNodesFn = func(clusterName string, logger log.Logger) ([]nodeStringer, error) {
	provider := cluster.NewProvider(
		cluster.ProviderWithLogger(logger),
		runtime.GetDefault(logger),
	)
	nodes, err := provider.ListNodes(clusterName)
	if err != nil {
		return nil, err
	}
	out := make([]nodeStringer, len(nodes))
	for i, n := range nodes {
		out[i] = n
	}
	return out, nil
}

// binaryNameFn returns the first available container runtime binary found in
// $PATH. Production uses lifecycle.ProviderBinaryName; tests return "docker".
var binaryNameFn = lifecycle.ProviderBinaryName

// runDecodeFn is the engine entry-point. Tests stub to return synthetic results.
var runDecodeFn = idoctor.RunDecode

// previewAutoFixFn produces the human-readable preview of pending mitigations.
var previewAutoFixFn = idoctor.PreviewDecodeAutoFix

// applyAutoFixFn applies whitelisted mitigations. Tests record invocations.
var applyAutoFixFn = idoctor.ApplyDecodeAutoFix

// formatHumanFn renders a DecodeResult in human-readable form.
var formatHumanFn func(w io.Writer, result *idoctor.DecodeResult) = idoctor.FormatDecodeHumanReadable

// formatJSONFn builds the JSON envelope for a DecodeResult.
var formatJSONFn func(result *idoctor.DecodeResult) map[string]interface{} = idoctor.FormatDecodeJSON

// ---------------------------------------------------------------------------
// NewCommand returns the `kinder doctor decode` cobra command.
// ---------------------------------------------------------------------------

// NewCommand returns the `kinder doctor decode` cobra subcommand.
func NewCommand(logger log.Logger, streams cmd.IOStreams) *cobra.Command {
	flags := &flagpole{}
	c := &cobra.Command{
		Args:  cobra.MaximumNArgs(1),
		Use:   "decode [cluster-name]",
		Short: "Decode runtime errors from docker logs and kubectl events into plain-English fixes",
		Long: "Scans recent docker logs across all node containers and `kubectl get events` " +
			"from a control-plane node, matches a catalog of known patterns, and prints a " +
			"plain-English explanation + suggested fix for every hit.\n\n" +
			"Use --auto-fix to apply whitelisted, non-destructive remediations automatically.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runE(logger, streams, flags, args)
		},
	}
	c.Flags().StringVar(&flags.Name, "name", "", "cluster name (auto-detected when exactly one cluster exists)")
	c.Flags().DurationVar(&flags.Since, "since", 30*time.Minute, "time window for log/event scan (e.g. 30m, 1h)")
	c.Flags().StringVar(&flags.Output, "output", "", "output format; supported values: \"\", \"json\"")
	c.Flags().BoolVar(&flags.AutoFix, "auto-fix", false, "apply whitelisted, non-destructive remediations (preview shown before each apply)")
	c.Flags().BoolVar(&flags.IncludeNormal, "include-normal", false, "include type=Normal events in the scan (default: warnings only)")
	return c
}

// ---------------------------------------------------------------------------
// runE implements the decode subcommand logic.
// ---------------------------------------------------------------------------

func runE(logger log.Logger, streams cmd.IOStreams, flags *flagpole, args []string) error {
	// Validate output format first.
	switch flags.Output {
	case "", "json":
		// valid
	default:
		return fmt.Errorf("unsupported output format %q; supported values: \"\", \"json\"", flags.Output)
	}

	// Cluster resolution: --name overrides positional arg.
	argSlice := args
	if flags.Name != "" {
		argSlice = []string{flags.Name}
	}
	name, err := resolveClusterName(argSlice)
	if err != nil {
		return err
	}

	// Node enumeration.
	allNodes, err := listNodesFn(name, logger)
	if err != nil {
		return err
	}
	if len(allNodes) == 0 {
		return fmt.Errorf("no nodes found for cluster %q", name)
	}

	// Find first control-plane node (mirrors ClassifyNodes logic; avoids
	// importing lifecycle here since lifecycle imports doctor — import cycle).
	cpNodeName := ""
	for _, n := range allNodes {
		role, roleErr := n.Role()
		if roleErr == nil && role == "control-plane" {
			cpNodeName = n.String()
			break
		}
	}
	if cpNodeName == "" {
		return fmt.Errorf("no control-plane nodes found in cluster %q; cannot run kubectl events", name)
	}

	// Enumerate node names for docker logs scan.
	nodeNames := make([]string, 0, len(allNodes))
	for _, n := range allNodes {
		nodeNames = append(nodeNames, n.String())
	}

	// Resolve container runtime binary.
	binary := binaryNameFn()
	if binary == "" {
		return fmt.Errorf("no container runtime binary found in PATH; run `kinder doctor` to diagnose")
	}

	// Scan via engine.
	result, err := runDecodeFn(idoctor.DecodeOptions{
		Cluster:             name,
		BinaryName:          binary,
		CPNodeName:          cpNodeName,
		AllNodes:            nodeNames,
		Since:               flags.Since,
		IncludeNormalEvents: flags.IncludeNormal,
		Logger:              logger,
	})
	if err != nil {
		return err
	}

	// Render results.
	if flags.Output == "json" {
		if err := json.NewEncoder(streams.Out).Encode(formatJSONFn(result)); err != nil {
			return err
		}
	} else {
		formatHumanFn(streams.Out, result)
	}

	// Auto-fix path (only when --auto-fix is explicitly set).
	if !flags.AutoFix {
		return nil
	}
	autoCtx := idoctor.DecodeAutoFixContext{
		BinaryName: binary,
		CPNodeName: cpNodeName,
	}
	preview := previewAutoFixFn(result.Matches, autoCtx)
	if len(preview) == 0 {
		fmt.Fprintln(streams.Out, "auto-fix: no whitelisted remediations apply")
		return nil
	}
	fmt.Fprintln(streams.Out, "auto-fix preview:")
	for _, line := range preview {
		fmt.Fprintf(streams.Out, "  %s\n", line)
	}
	errs := applyAutoFixFn(result.Matches, autoCtx, logger)
	if len(errs) > 0 {
		return fmt.Errorf("auto-fix produced %d error(s); see logs above", len(errs))
	}
	return nil
}
