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

// Package kindversion contains the kinder CLI version constants and functions.
package kindversion

import (
	"fmt"
	"runtime"
)

// Version returns the kind CLI Semantic Version
func Version() string {
	return version(versionCore, versionPreRelease, gitCommit, gitCommitCount)
}

func version(core, preRelease, commit, commitCount string) string {
	v := core
	// add pre-release version info if we have it
	if preRelease != "" {
		v += "-" + preRelease
		// If commitCount was set, add to the pre-release version
		if commitCount != "" {
			v += "." + commitCount
		}
		// if commit was set, add the + <build>
		// we only do this for pre-release versions
		if commit != "" {
			// NOTE: use 14 character short hash, like Kubernetes
			v += "+" + truncate(commit, 14)
		}
	}
	return v
}

// DisplayVersion is Version() display formatted, this is what the version
// subcommand prints
func DisplayVersion() string {
	return fmt.Sprintf("kind v%s %s %s/%s", Version(), runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

// versionCore is the core portion of the kind CLI version per Semantic Versioning 2.0.0
const versionCore = "1.4.0"

// versionPreRelease is the base pre-release portion of the kind CLI version per
// Semantic Versioning 2.0.0
var versionPreRelease = ""

// gitCommitCount count the commits since the last release.
// It is injected at build time.
var gitCommitCount = ""

// gitCommit is the commit used to build the kind binary, if available.
// It is injected at build time.
var gitCommit = ""

func truncate(s string, maxLen int) string {
	if len(s) < maxLen {
		return s
	}
	return s[:maxLen]
}
