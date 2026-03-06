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
	osexec "os/exec"
	"strings"

	"sigs.k8s.io/kind/pkg/exec"
)

// firewalldCheck detects whether firewalld is running with the nftables
// backend, which breaks Docker networking on Fedora 32+ and similar distros.
type firewalldCheck struct {
	readFile func(string) ([]byte, error)
	lookPath func(string) (string, error)
	execCmd  func(name string, args ...string) exec.Cmd
}

// newFirewalldCheck creates a firewalldCheck with real system deps.
func newFirewalldCheck() Check {
	return &firewalldCheck{
		readFile: os.ReadFile,
		lookPath: osexec.LookPath,
		execCmd:  exec.Command,
	}
}

func (c *firewalldCheck) Name() string       { return "firewalld-backend" }
func (c *firewalldCheck) Category() string    { return "Platform" }
func (c *firewalldCheck) Platforms() []string { return []string{"linux"} }

func (c *firewalldCheck) Run() []Result {
	// Step 1: Check if firewall-cmd is installed.
	if _, err := c.lookPath("firewall-cmd"); err != nil {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "ok",
			Message:  "firewalld not installed",
		}}
	}

	// Step 2: Check if firewalld is running.
	lines, err := exec.CombinedOutputLines(c.execCmd("firewall-cmd", "--state"))
	if err != nil || len(lines) == 0 || strings.TrimSpace(lines[0]) != "running" {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "ok",
			Message:  "firewalld not running",
		}}
	}

	// Step 3: Read firewalld config.
	data, err := c.readFile("/etc/firewalld/firewalld.conf")
	if err != nil {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "ok",
			Message:  "firewalld running but config not readable",
		}}
	}

	// Step 4: Parse config for FirewallBackend.
	backend := "nftables" // Fedora 32+ default when line is absent
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "FirewallBackend=") {
			backend = strings.TrimPrefix(line, "FirewallBackend=")
			break
		}
	}

	// Step 5: Warn if nftables.
	if strings.EqualFold(backend, "nftables") {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "warn",
			Message:  "firewalld is using nftables backend",
			Reason:   "nftables backend breaks Docker/kind networking (iptables rules are bypassed)",
			Fix:      "Change FirewallBackend=iptables in /etc/firewalld/firewalld.conf and run: sudo systemctl restart firewalld",
		}}
	}

	return []Result{{
		Name:     c.Name(),
		Category: c.Category(),
		Status:   "ok",
		Message:  fmt.Sprintf("firewalld using %s backend", backend),
	}}
}
