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
	"os"
	"strings"
)

// apparmorCheck detects whether AppArmor is enabled, which can interfere
// with kind containers via stale profile caches (moby/moby#7512).
type apparmorCheck struct {
	readFile func(string) ([]byte, error)
}

// newApparmorCheck creates an apparmorCheck with real system deps.
func newApparmorCheck() Check {
	return &apparmorCheck{
		readFile: os.ReadFile,
	}
}

func (c *apparmorCheck) Name() string       { return "apparmor" }
func (c *apparmorCheck) Category() string    { return "Security" }
func (c *apparmorCheck) Platforms() []string { return []string{"linux"} }

func (c *apparmorCheck) Run() []Result {
	data, err := c.readFile("/sys/module/apparmor/parameters/enabled")
	if err != nil {
		// AppArmor kernel module not loaded -- not a concern.
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "ok",
			Message:  "AppArmor not enabled (module not loaded)",
		}}
	}

	enabled := strings.TrimSpace(string(data))
	if enabled == "Y" {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "warn",
			Message:  "AppArmor is enabled",
			Reason:   "AppArmor can cause container startup failures due to stale profile caches (moby/moby#7512)",
			Fix:      "If kind containers fail to start, run: sudo aa-remove-unknown",
		}}
	}

	return []Result{{
		Name:     c.Name(),
		Category: c.Category(),
		Status:   "ok",
		Message:  "AppArmor not enabled",
	}}
}
