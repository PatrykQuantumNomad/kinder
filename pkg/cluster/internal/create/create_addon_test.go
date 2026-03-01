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

package create

import (
	"fmt"
	"runtime"
	"strings"
	"testing"

	"sigs.k8s.io/kind/pkg/log"
)

// testLogger implements log.Logger and log.InfoLogger for testing.
// All log output is captured into the lines slice.
type testLogger struct {
	lines []string
}

func (l *testLogger) Warn(message string) {
	l.lines = append(l.lines, message)
}

func (l *testLogger) Warnf(format string, args ...interface{}) {
	l.lines = append(l.lines, fmt.Sprintf(format, args...))
}

func (l *testLogger) Error(message string) {
	l.lines = append(l.lines, message)
}

func (l *testLogger) Errorf(format string, args ...interface{}) {
	l.lines = append(l.lines, fmt.Sprintf(format, args...))
}

func (l *testLogger) V(log.Level) log.InfoLogger {
	return l
}

func (l *testLogger) Info(message string) {
	l.lines = append(l.lines, message)
}

func (l *testLogger) Infof(format string, args ...interface{}) {
	l.lines = append(l.lines, fmt.Sprintf(format, args...))
}

func (l *testLogger) Enabled() bool {
	return true
}

func (l *testLogger) output() string {
	return strings.Join(l.lines, "\n")
}

func TestLogAddonSummary(t *testing.T) {
	tests := []struct {
		name     string
		results  []addonResult
		contains []string
	}{
		{
			name: "single enabled addon shows installed",
			results: []addonResult{
				{name: "MetalLB", enabled: true},
			},
			contains: []string{"MetalLB", "installed"},
		},
		{
			name: "single disabled addon shows skipped",
			results: []addonResult{
				{name: "MetalLB", enabled: false},
			},
			contains: []string{"MetalLB", "skipped"},
		},
		{
			name: "single failed addon shows FAILED",
			results: []addonResult{
				{name: "MetalLB", enabled: true, err: fmt.Errorf("timeout")},
			},
			contains: []string{"MetalLB", "FAILED", "timeout"},
		},
		{
			name: "multiple addons each appear",
			results: []addonResult{
				{name: "MetalLB", enabled: true},
				{name: "Metrics Server", enabled: false},
				{name: "CoreDNS Tuning", enabled: true, err: fmt.Errorf("apply failed")},
			},
			contains: []string{"MetalLB", "Metrics Server", "CoreDNS Tuning"},
		},
		{
			name:     "empty results does not panic",
			results:  []addonResult{},
			contains: []string{"Addons:"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &testLogger{}
			logAddonSummary(logger, tt.results)
			out := logger.output()
			for _, want := range tt.contains {
				if !strings.Contains(out, want) {
					t.Errorf("output missing %q\ngot:\n%s", want, out)
				}
			}
		})
	}
}

func TestLogMetalLBPlatformWarning(t *testing.T) {
	logger := &testLogger{}
	logMetalLBPlatformWarning(logger)
	out := logger.output()

	switch runtime.GOOS {
	case "darwin", "windows":
		if !strings.Contains(out, "port-forward") {
			t.Errorf("on %s expected output to contain 'port-forward', got:\n%s", runtime.GOOS, out)
		}
	default:
		if out != "" {
			t.Errorf("on %s expected empty output, got:\n%s", runtime.GOOS, out)
		}
	}
}
