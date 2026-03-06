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
	"fmt"
	"os"
	"runtime"

	"sigs.k8s.io/kind/pkg/log"
)

// SafeMitigation represents an idempotent, non-destructive fix
// that can be auto-applied during cluster creation.
type SafeMitigation struct {
	// Name is the human-readable name of the mitigation.
	Name string
	// NeedsFix returns true if the mitigation should be applied.
	NeedsFix func() bool
	// Apply executes the mitigation. Must be idempotent.
	Apply func() error
	// NeedsRoot is true if the mitigation requires root/sudo.
	NeedsRoot bool
}

// SafeMitigations returns the list of mitigations safe for auto-apply.
// No tier-1 auto-fixable mitigations currently; infrastructure ready for future use.
func SafeMitigations() []SafeMitigation {
	return []SafeMitigation{}
}

// ApplySafeMitigations runs all safe mitigations. Called from create flow.
// Returns errors for mitigations that failed (informational, not fatal).
func ApplySafeMitigations(logger log.Logger) []error {
	if runtime.GOOS != "linux" {
		return nil
	}
	mitigations := SafeMitigations()
	if len(mitigations) == 0 {
		return nil
	}
	var errs []error
	for _, m := range mitigations {
		if !m.NeedsFix() {
			continue
		}
		if m.NeedsRoot && os.Geteuid() != 0 {
			logger.Warnf("Skipping %s mitigation (requires root)", m.Name)
			continue
		}
		if err := m.Apply(); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", m.Name, err))
		} else {
			logger.V(0).Infof("Applied mitigation: %s", m.Name)
		}
	}
	return errs
}
