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

// Package doctor provides the shared diagnostic check infrastructure
// for the kinder doctor command and create-flow mitigations.
package doctor

import (
	"runtime"
	"strings"
)

// Check represents a single diagnostic check.
type Check interface {
	// Name returns the check identifier, e.g., "container-runtime".
	Name() string
	// Category returns the grouping label, e.g., "Runtime".
	Category() string
	// Platforms returns the OS platforms this check applies to.
	// nil means all platforms; []string{"linux"} means Linux-only.
	Platforms() []string
	// Run executes the check and returns one or more results.
	// All error conditions are expressed as warn/fail results, not Go errors.
	Run() []Result
}

// Result is the outcome of a single diagnostic check.
// JSON tags match the user-specified output format.
type Result struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Status   string `json:"status"`           // "ok", "warn", "fail", "skip"
	Message  string `json:"message,omitempty"` // detected value or problem description
	Reason   string `json:"reason,omitempty"`  // WHY it matters (warn/fail only)
	Fix      string `json:"fix,omitempty"`     // fix command (warn/fail only)
}

// allChecks is the explicit registry of all diagnostic checks.
// Order defines category grouping: Runtime, Docker, Tools, GPU, Kernel, Security, Platform, Network.
var allChecks = []Check{
	// Category: Runtime
	newContainerRuntimeCheck(),
	// Category: Docker
	newDiskSpaceCheck(),
	newDaemonJSONCheck(),
	newDockerSnapCheck(),
	newDockerSocketCheck(),
	// Category: Tools
	newKubectlCheck(),
	newKubectlVersionSkewCheck(),
	// Category: GPU
	newNvidiaDriverCheck(),
	newNvidiaContainerToolkitCheck(),
	newNvidiaDockerRuntimeCheck(),
	// Category: Kernel (Phase 40)
	newInotifyCheck(),
	newKernelVersionCheck(),
	// Category: Security (Phase 40)
	newApparmorCheck(),
	newSELinuxCheck(),
	// Category: Platform (Phase 40)
	newFirewalldCheck(),
	newWSL2Check(),
	newRootfsDeviceCheck(),
	// Category: Network (Phase 41)
	newSubnetClashCheck(),
	// Category: Cluster (Phase 42)
	newClusterNodeSkewCheck(),
	newLocalPathCVECheck(),
	// Category: Offline (Phase 43)
	newOfflineReadinessCheck(),
	// Category: Mounts (Phase 45)
	newHostMountPathCheck(),
	newDockerDesktopFileSharingCheck(),
}

// mountPathConfigurable is implemented by checks that need host mount paths
// injected from an external config source (e.g., --config flag).
type mountPathConfigurable interface {
	setMountPaths(paths []string)
}

// SetMountPaths injects host mount paths into all checks that implement
// mountPathConfigurable. Call before RunAllChecks to wire config-derived
// paths into mount checks.
func SetMountPaths(paths []string) {
	for _, check := range allChecks {
		if mc, ok := check.(mountPathConfigurable); ok {
			mc.setMountPaths(paths)
		}
	}
}

// AllChecks returns all registered diagnostic checks.
func AllChecks() []Check {
	return allChecks
}

// RunAllChecks executes all checks with centralized platform filtering.
// Returns ordered results preserving insertion order from the registry.
func RunAllChecks() []Result {
	var results []Result
	for _, check := range AllChecks() {
		platforms := check.Platforms()
		if len(platforms) > 0 && !containsString(platforms, runtime.GOOS) {
			results = append(results, Result{
				Name:     check.Name(),
				Category: check.Category(),
				Status:   "skip",
				Message:  platformSkipMessage(platforms),
			})
			continue
		}
		results = append(results, check.Run()...)
	}
	return results
}

// ExitCodeFromResults computes the exit code from check results.
// 0 for all-ok-or-skip, 1 for any fail, 2 for any warn but no fail.
func ExitCodeFromResults(results []Result) int {
	hasFail := false
	hasWarn := false
	for _, r := range results {
		switch r.Status {
		case "fail":
			hasFail = true
		case "warn":
			hasWarn = true
		}
	}
	if hasFail {
		return 1
	}
	if hasWarn {
		return 2
	}
	return 0
}

// containsString checks if a string is in a slice.
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// platformSkipMessage generates a concise platform tag for skip messages.
// e.g., "(linux only)" or "(linux/darwin only)".
func platformSkipMessage(platforms []string) string {
	return "(" + strings.Join(platforms, "/") + " only)"
}
