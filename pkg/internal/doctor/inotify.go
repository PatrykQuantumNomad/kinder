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
	"strconv"
	"strings"
)

const (
	// minWatches is the minimum recommended value for inotify max_user_watches.
	// kind nodes run multiple watchers (kubelet, kube-apiserver file watches).
	minWatches = 524288
	// minInstances is the minimum recommended value for inotify max_user_instances.
	minInstances = 512
)

// inotifyCheck verifies that inotify kernel limits are sufficient for kind.
type inotifyCheck struct {
	readFile func(string) ([]byte, error)
}

// newInotifyCheck creates an inotifyCheck with real system deps.
func newInotifyCheck() Check {
	return &inotifyCheck{
		readFile: os.ReadFile,
	}
}

func (c *inotifyCheck) Name() string       { return "inotify-limits" }
func (c *inotifyCheck) Category() string    { return "Kernel" }
func (c *inotifyCheck) Platforms() []string { return []string{"linux"} }

func (c *inotifyCheck) Run() []Result {
	watches, watchErr := c.readSysctl("/proc/sys/fs/inotify/max_user_watches")
	instances, instErr := c.readSysctl("/proc/sys/fs/inotify/max_user_instances")

	// Both unreadable: single warn result.
	if watchErr != nil && instErr != nil {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "warn",
			Message:  "Could not read inotify limits from /proc/sys/fs/inotify/",
			Reason:   "Unable to verify inotify limits; this may indicate a non-Linux platform or restricted /proc",
			Fix:      "cat /proc/sys/fs/inotify/max_user_watches /proc/sys/fs/inotify/max_user_instances",
		}}
	}

	// One unreadable: warn about it.
	if watchErr != nil || instErr != nil {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "warn",
			Message:  "Could not read one or more inotify limits",
			Reason:   "Partial inotify limit data; check /proc/sys/fs/inotify/ accessibility",
			Fix:      "cat /proc/sys/fs/inotify/max_user_watches /proc/sys/fs/inotify/max_user_instances",
		}}
	}

	var results []Result

	if watches < minWatches {
		results = append(results, Result{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "warn",
			Message:  fmt.Sprintf("max_user_watches is %d (recommended >= %d)", watches, minWatches),
			Reason:   "Low inotify watches can cause 'too many open files' errors in kind nodes",
			Fix:      fmt.Sprintf("sudo sysctl fs.inotify.max_user_watches=%d", minWatches),
		})
	}

	if instances < minInstances {
		results = append(results, Result{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "warn",
			Message:  fmt.Sprintf("max_user_instances is %d (recommended >= %d)", instances, minInstances),
			Reason:   "Low inotify instances can cause 'too many open files' errors in kind nodes",
			Fix:      fmt.Sprintf("sudo sysctl fs.inotify.max_user_instances=%d", minInstances),
		})
	}

	// Both ok: single result.
	if len(results) == 0 {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "ok",
			Message:  fmt.Sprintf("inotify limits sufficient (watches=%d, instances=%d)", watches, instances),
		}}
	}

	return results
}

// readSysctl reads and parses a single integer value from a sysctl proc file.
func (c *inotifyCheck) readSysctl(path string) (int, error) {
	data, err := c.readFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}
