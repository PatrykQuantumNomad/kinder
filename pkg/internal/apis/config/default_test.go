/*
Copyright 2024 The Kubernetes Authors.

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

package config

import "testing"

func TestSetDefaultsCluster_AddonFields(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		input      Addons
		wantAddons Addons
	}{
		{
			name: "all addons explicitly false remain false",
			input: Addons{
				MetalLB: false, EnvoyGateway: false,
				MetricsServer: false, CoreDNSTuning: false, Dashboard: false,
				NvidiaGPU: false,
			},
			wantAddons: Addons{
				MetalLB: false, EnvoyGateway: false,
				MetricsServer: false, CoreDNSTuning: false, Dashboard: false,
				NvidiaGPU: false,
			},
		},
		{
			name:       "zero-value addons remain zero-value",
			input:      Addons{},
			wantAddons: Addons{},
		},
		{
			name:       "NvidiaGPU defaults to false (opt-in)",
			input:      Addons{},
			wantAddons: Addons{},
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := &Cluster{Addons: tc.input}
			SetDefaultsCluster(c)
			if c.Addons != tc.wantAddons {
				t.Errorf("SetDefaultsCluster() Addons = %+v, want %+v", c.Addons, tc.wantAddons)
			}
		})
	}
}

func TestSetDefaultsCluster_NonAddonDefaults(t *testing.T) {
	t.Parallel()
	c := &Cluster{}
	SetDefaultsCluster(c)
	if c.Name == "" {
		t.Error("SetDefaultsCluster should set default Name")
	}
	if len(c.Nodes) == 0 {
		t.Error("SetDefaultsCluster should set default Nodes")
	}
	if c.Networking.IPFamily == "" {
		t.Error("SetDefaultsCluster should set default IPFamily")
	}
}
