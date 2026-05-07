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

package doctor

import (
	"regexp"
	"strings"
	"sync"
)

// DecodeScope groups error patterns into broad component categories
// for display and filtering in kinder doctor decode output.
type DecodeScope string

const (
	// ScopeKubelet covers kubelet startup, cgroup, and file-descriptor errors.
	ScopeKubelet DecodeScope = "kubelet"
	// ScopeKubeadm covers kubeadm pre-flight and init-phase failures.
	ScopeKubeadm DecodeScope = "kubeadm"
	// ScopeContainerd covers containerd CRI, image-pull, and runtime errors.
	ScopeContainerd DecodeScope = "containerd"
	// ScopeDocker covers Docker daemon and host environment errors.
	ScopeDocker DecodeScope = "docker"
	// ScopeAddon covers addon-startup failures in kube-system pods.
	ScopeAddon DecodeScope = "addon"
)

// DecodePattern is a single entry in the runtime error catalog.
// Plain substrings and "regex:"-prefixed patterns are both supported.
type DecodePattern struct {
	// ID is the unique pattern identifier, e.g. "KUB-01".
	ID string
	// Scope is the component this pattern belongs to.
	Scope DecodeScope
	// Match is the pattern string to match against log/event lines.
	// Plain strings use strings.Contains; "regex:"-prefixed strings compile
	// to a regexp and use MatchString. Regexes are cached after first compile.
	Match string
	// Explanation is the plain-English description of what went wrong.
	Explanation string
	// Fix is the suggested remediation (one-liner or short paragraph).
	Fix string
	// DocLink is an optional URL to docs or the kind issue tracker.
	// Empty string means no link available.
	DocLink string
	// AutoFixable is true when a non-destructive SafeMitigation exists for
	// this pattern. Plan 50-04 populates the relevant catalog entries.
	AutoFixable bool
	// AutoFix is the safe remediation to apply when --auto-fix is set.
	// nil when AutoFixable is false. Plan 50-04 sets this field.
	AutoFix *SafeMitigation
}

// DecodeMatch is a single pattern match found in the collected logs or events.
type DecodeMatch struct {
	// Source identifies where the match was found, e.g. "docker-logs:kind-control-plane"
	// or "k8s-events". Set by the caller of matchLines.
	Source string
	// Line is the raw log or event line that triggered the match.
	Line string
	// Pattern is the full catalog entry that matched.
	Pattern DecodePattern
}

// DecodeResult is the top-level output of a RunDecode call.
type DecodeResult struct {
	// Cluster is the resolved cluster name.
	Cluster string
	// Matches contains all pattern matches found, in source order.
	Matches []DecodeMatch
	// Unmatched is the count of log lines that matched no pattern (debug metric).
	Unmatched int
}

// regexCache stores compiled regexps by their pattern string so each unique
// regex is compiled at most once across the process lifetime.
var regexCache sync.Map // map[string]*regexp.Regexp

// matchesPattern returns true when line satisfies pat.
// pat starting with "regex:" is treated as a regular expression (cached);
// all other strings use substring containment.
func matchesPattern(line, pat string) bool {
	if strings.HasPrefix(pat, "regex:") {
		expr := strings.TrimPrefix(pat, "regex:")
		var compiled *regexp.Regexp
		if v, ok := regexCache.Load(expr); ok {
			compiled = v.(*regexp.Regexp)
		} else {
			// MustCompile panics on malformed regex — intentional: bad catalog
			// entries must be caught at test time, not silently skipped at runtime.
			compiled = regexp.MustCompile(expr)
			regexCache.Store(expr, compiled)
		}
		return compiled.MatchString(line)
	}
	return strings.Contains(line, pat)
}

// matchLines scans lines against patterns and returns one DecodeMatch per
// (line, pattern) hit. First-match-wins per line: once a pattern matches a
// line the inner loop breaks, preventing duplicate output for ambiguous lines
// (RESEARCH common pitfall 3). source is propagated verbatim to each match.
//
// Empty or nil inputs return a non-nil empty []DecodeMatch so callers can
// range without a nil-check.
func matchLines(lines []string, patterns []DecodePattern, source string) []DecodeMatch {
	matches := []DecodeMatch{}
	for _, line := range lines {
		for _, pat := range patterns {
			if matchesPattern(line, pat.Match) {
				matches = append(matches, DecodeMatch{
					Source:  source,
					Line:    line,
					Pattern: pat,
				})
				break // first-match-wins; next line
			}
		}
	}
	return matches
}
