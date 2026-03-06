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
	"testing"

	"sigs.k8s.io/kind/pkg/log"
)

// testLogger is a minimal log.Logger for testing.
type testLogger struct{}

func (t *testLogger) Warn(message string)                          {}
func (t *testLogger) Warnf(format string, args ...interface{})     {}
func (t *testLogger) Error(message string)                         {}
func (t *testLogger) Errorf(format string, args ...interface{})    {}
func (t *testLogger) V(level log.Level) log.InfoLogger             { return &testInfoLogger{} }

type testInfoLogger struct{}

func (t *testInfoLogger) Info(message string)                      {}
func (t *testInfoLogger) Infof(format string, args ...interface{}) {}
func (t *testInfoLogger) Enabled() bool                            { return true }

func TestSafeMitigations_ReturnsNil(t *testing.T) {
	t.Parallel()
	mitigations := SafeMitigations()
	if mitigations != nil {
		t.Errorf("SafeMitigations() should return nil (skeleton), got %v", mitigations)
	}
}

func TestApplySafeMitigations_EmptyReturnsNil(t *testing.T) {
	t.Parallel()
	logger := &testLogger{}
	errs := ApplySafeMitigations(logger)
	if errs != nil {
		t.Errorf("ApplySafeMitigations() with empty mitigations should return nil, got %v", errs)
	}
}
