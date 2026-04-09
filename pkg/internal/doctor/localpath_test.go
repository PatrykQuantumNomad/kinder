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
	"errors"
	"strings"
	"testing"
)

func TestLocalPathCVE_Safe(t *testing.T) {
	t.Parallel()
	check := &localPathCVECheck{
		getProvisionerVersion: func() (string, error) { return "v0.0.35", nil },
	}
	results := check.Run()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Status != "ok" {
		t.Errorf("expected ok, got %q: %s", r.Status, r.Message)
	}
	if !strings.Contains(r.Message, "v0.0.35") {
		t.Errorf("Message = %q, want to contain %q", r.Message, "v0.0.35")
	}
}

func TestLocalPathCVE_ExactThreshold(t *testing.T) {
	t.Parallel()
	// v0.0.34 is the fix version — not vulnerable.
	check := &localPathCVECheck{
		getProvisionerVersion: func() (string, error) { return "v0.0.34", nil },
	}
	results := check.Run()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Status != "ok" {
		t.Errorf("expected ok, got %q: %s", r.Status, r.Message)
	}
}

func TestLocalPathCVE_Vulnerable(t *testing.T) {
	t.Parallel()
	check := &localPathCVECheck{
		getProvisionerVersion: func() (string, error) { return "v0.0.33", nil },
	}
	results := check.Run()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Status != "warn" {
		t.Errorf("expected warn, got %q: %s", r.Status, r.Message)
	}
	if !strings.Contains(r.Message, "below v0.0.34") {
		t.Errorf("Message = %q, want to contain %q", r.Message, "below v0.0.34")
	}
	if !strings.Contains(r.Reason, "CVE-2025-62878") {
		t.Errorf("Reason = %q, want to contain %q", r.Reason, "CVE-2025-62878")
	}
}

func TestLocalPathCVE_NoCluster(t *testing.T) {
	t.Parallel()
	check := &localPathCVECheck{
		getProvisionerVersion: func() (string, error) { return "", nil },
	}
	results := check.Run()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Status != "skip" {
		t.Errorf("expected skip, got %q: %s", r.Status, r.Message)
	}
}

func TestLocalPathCVE_Error(t *testing.T) {
	t.Parallel()
	check := &localPathCVECheck{
		getProvisionerVersion: func() (string, error) { return "", errors.New("boom") },
	}
	results := check.Run()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Status != "warn" {
		t.Errorf("expected warn, got %q: %s", r.Status, r.Message)
	}
	if !strings.Contains(r.Message, "could not determine") {
		t.Errorf("Message = %q, want to contain %q", r.Message, "could not determine")
	}
}

func TestLocalPathCVE_UnparseableVersion(t *testing.T) {
	t.Parallel()
	check := &localPathCVECheck{
		getProvisionerVersion: func() (string, error) { return "latest", nil },
	}
	results := check.Run()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Status != "warn" {
		t.Errorf("expected warn, got %q: %s", r.Status, r.Message)
	}
	if !strings.Contains(r.Message, "unparseable") {
		t.Errorf("Message = %q, want to contain %q", r.Message, "unparseable")
	}
}

func TestLocalPathCVE_NoVPrefix(t *testing.T) {
	t.Parallel()
	// "0.0.35" without "v" prefix — code should add "v" before parsing.
	check := &localPathCVECheck{
		getProvisionerVersion: func() (string, error) { return "0.0.35", nil },
	}
	results := check.Run()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Status != "ok" {
		t.Errorf("expected ok, got %q: %s", r.Status, r.Message)
	}
}
