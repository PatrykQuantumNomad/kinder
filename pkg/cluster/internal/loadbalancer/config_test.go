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

package loadbalancer

import (
	"strings"
	"testing"
)

func TestImageConstantIsEnvoy(t *testing.T) {
	t.Parallel()
	const want = "docker.io/envoyproxy/envoy:v1.36.2"
	if Image != want {
		t.Errorf("Image = %q, want %q", Image, want)
	}
}

func TestProxyConfigPathConstants(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"ProxyConfigPath", ProxyConfigPath, "/home/envoy/envoy.yaml"},
		{"ProxyConfigPathCDS", ProxyConfigPathCDS, "/home/envoy/cds.yaml"},
		{"ProxyConfigPathLDS", ProxyConfigPathLDS, "/home/envoy/lds.yaml"},
		{"ProxyConfigDir", ProxyConfigDir, "/home/envoy"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.got != tc.want {
				t.Errorf("%s = %q, want %q", tc.name, tc.got, tc.want)
			}
		})
	}
}

func TestConfigLDSRendersControlPlanePort(t *testing.T) {
	t.Parallel()
	out, err := Config(&ConfigData{ControlPlanePort: 6443, IPv6: false}, ProxyLDSConfigTemplate)
	if err != nil {
		t.Fatalf("Config() error: %v", err)
	}
	if !strings.Contains(out, "port_value: 6443") {
		t.Errorf("output does not contain 'port_value: 6443'; got:\n%s", out)
	}
	if !strings.Contains(out, `"0.0.0.0"`) {
		t.Errorf("output does not contain '\"0.0.0.0\"' for IPv4; got:\n%s", out)
	}
}

func TestConfigLDSIPv6(t *testing.T) {
	t.Parallel()
	out, err := Config(&ConfigData{ControlPlanePort: 6443, IPv6: true}, ProxyLDSConfigTemplate)
	if err != nil {
		t.Fatalf("Config() error: %v", err)
	}
	if !strings.Contains(out, `"::"`) {
		t.Errorf("output does not contain '\"::\"' for IPv6; got:\n%s", out)
	}
}

func TestConfigCDSRendersBackendServers(t *testing.T) {
	t.Parallel()
	data := &ConfigData{
		BackendServers: map[string]string{
			"cp-1": "172.18.0.4:6443",
			"cp-2": "172.18.0.5:6443",
		},
	}
	out, err := Config(data, ProxyCDSConfigTemplate)
	if err != nil {
		t.Fatalf("Config() error: %v", err)
	}
	if !strings.Contains(out, "172.18.0.4") {
		t.Errorf("output does not contain '172.18.0.4'; got:\n%s", out)
	}
	if !strings.Contains(out, "172.18.0.5") {
		t.Errorf("output does not contain '172.18.0.5'; got:\n%s", out)
	}
	if !strings.Contains(out, "health_checks") {
		t.Errorf("output does not contain 'health_checks' block; got:\n%s", out)
	}
}

func TestGenerateBootstrapCommandShape(t *testing.T) {
	t.Parallel()
	args := GenerateBootstrapCommand("my-cluster", "my-cluster-external-load-balancer")
	if len(args) != 3 {
		t.Fatalf("len(args) = %d, want 3; args = %v", len(args), args)
	}
	if args[0] != "bash" {
		t.Errorf("args[0] = %q, want %q", args[0], "bash")
	}
	if args[1] != "-c" {
		t.Errorf("args[1] = %q, want %q", args[1], "-c")
	}
	cmd := args[2]
	for _, substr := range []string{
		"mkdir -p /home/envoy",
		"/home/envoy/envoy.yaml",
		"/home/envoy/cds.yaml",
		"/home/envoy/lds.yaml",
		"while true; do envoy -c",
	} {
		if !strings.Contains(cmd, substr) {
			t.Errorf("args[2] does not contain %q; got:\n%s", substr, cmd)
		}
	}
}
