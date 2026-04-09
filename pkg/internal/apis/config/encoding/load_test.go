/*
Copyright 2018 The Kubernetes Authors.

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

package encoding

import (
	"testing"
)

func TestLoadCurrent(t *testing.T) {
	t.Parallel()
	cases := []struct {
		TestName    string
		Path        string
		ExpectError bool
	}{
		{
			TestName:    "example config",
			Path:        "./../../../../../site/content/docs/user/kind-example-config.yaml",
			ExpectError: false,
		},
		{
			TestName:    "no config",
			Path:        "",
			ExpectError: false,
		},
		{
			TestName:    "v1alpha4 minimal",
			Path:        "./testdata/v1alpha4/valid-minimal.yaml",
			ExpectError: false,
		},
		{
			TestName:    "v1alpha4 config with 2 nodes",
			Path:        "./testdata/v1alpha4/valid-minimal-two-nodes.yaml",
			ExpectError: false,
		},
		{
			TestName:    "v1alpha4 full HA",
			Path:        "./testdata/v1alpha4/valid-full-ha.yaml",
			ExpectError: false,
		},
		{
			TestName:    "v1alpha4 many fields set",
			Path:        "./testdata/v1alpha4/valid-many-fields.yaml",
			ExpectError: false,
		},
		{
			TestName:    "v1alpha4 config with patches",
			Path:        "./testdata/v1alpha4/valid-kind-patches.yaml",
			ExpectError: false,
		},
		{
			TestName:    "v1alpha4 config with workers patches",
			Path:        "./testdata/v1alpha4/valid-kind-workers-patches.yaml",
			ExpectError: false,
		},
		{
			TestName:    "v1alpha4 config with port mapping and mount",
			Path:        "./testdata/v1alpha4/valid-port-and-mount.yaml",
			ExpectError: false,
		},
		{
			TestName:    "v1alpha4 non-existent field",
			Path:        "./testdata/v1alpha4/invalid-bogus-field.yaml",
			ExpectError: true,
		},
		{
			TestName:    "v1alpha4 bad indentation",
			Path:        "./testdata/v1alpha4/invalid-bad-indent.yaml",
			ExpectError: true,
		},
		{
			TestName:    "invalid path",
			Path:        "./testdata/not-a-file.bogus",
			ExpectError: true,
		},
		{
			TestName:    "Invalid apiversion",
			Path:        "./testdata/invalid-apiversion.yaml",
			ExpectError: true,
		},
		{
			TestName:    "Invalid kind",
			Path:        "./testdata/invalid-kind.yaml",
			ExpectError: true,
		},
		{
			TestName:    "Invalid yaml",
			Path:        "./testdata/invalid-yaml.yaml",
			ExpectError: true,
		},
		{
			TestName:    "v1alpha4 config with all addons enabled",
			Path:        "./testdata/v1alpha4/valid-addons-all-enabled.yaml",
			ExpectError: false,
		},
		{
			TestName:    "v1alpha4 config with some addons disabled",
			Path:        "./testdata/v1alpha4/valid-addons-some-disabled.yaml",
			ExpectError: false,
		},
		{
			TestName:    "v1alpha4 config with addons section absent",
			Path:        "./testdata/v1alpha4/valid-addons-absent.yaml",
			ExpectError: false,
		},
		{
			TestName:    "v1alpha4 config with new addon fields disabled",
			Path:        "./testdata/v1alpha4/valid-addons-new-fields.yaml",
			ExpectError: false,
		},
	}
	for _, c := range cases {
		c := c // capture loop variable
		t.Run(c.TestName, func(t *testing.T) {
			t.Parallel()
			_, err := Load(c.Path)
			// the error can be:
			// - nil, in which case we should expect no errors or fail
			if err != nil {
				if !c.ExpectError {
					t.Fatalf("unexpected error while Loading config: %v", err)
				}
				return
			}
			// - not nil, in which case we should expect errors or fail
			if c.ExpectError {
				t.Fatalf("unexpected lack or error while Loading config")
			}
		})
	}
}

func TestAddonsDefaults(t *testing.T) {
	t.Parallel()

	// Test 1: No addons section — all default to enabled
	cfg, err := Load("./testdata/v1alpha4/valid-addons-absent.yaml")
	if err != nil {
		t.Fatalf("failed to load config without addons: %v", err)
	}
	if !cfg.Addons.MetalLB {
		t.Error("expected MetalLB to default to true when addons section absent")
	}
	if !cfg.Addons.EnvoyGateway {
		t.Error("expected EnvoyGateway to default to true when addons section absent")
	}
	if !cfg.Addons.MetricsServer {
		t.Error("expected MetricsServer to default to true when addons section absent")
	}
	if !cfg.Addons.CoreDNSTuning {
		t.Error("expected CoreDNSTuning to default to true when addons section absent")
	}
	if !cfg.Addons.Dashboard {
		t.Error("expected Dashboard to default to true when addons section absent")
	}
	if !cfg.Addons.LocalRegistry {
		t.Error("expected LocalRegistry to default to true when addons section absent")
	}
	if !cfg.Addons.CertManager {
		t.Error("expected CertManager to default to true when addons section absent")
	}
	if !cfg.Addons.LocalPath {
		t.Error("expected LocalPath to default to true when addons section absent")
	}

	// Test 2: Some addons disabled — explicit false should be respected
	cfg2, err := Load("./testdata/v1alpha4/valid-addons-some-disabled.yaml")
	if err != nil {
		t.Fatalf("failed to load config with some addons disabled: %v", err)
	}
	if cfg2.Addons.MetalLB {
		t.Error("expected MetalLB to be false when explicitly set to false")
	}
	if !cfg2.Addons.EnvoyGateway {
		t.Error("expected EnvoyGateway to be true when explicitly set to true")
	}
	if !cfg2.Addons.MetricsServer {
		t.Error("expected MetricsServer to default to true when not specified in addons")
	}
	if !cfg2.Addons.CoreDNSTuning {
		t.Error("expected CoreDNSTuning to default to true when not specified in addons")
	}
	if !cfg2.Addons.Dashboard {
		t.Error("expected Dashboard to default to true when not specified in addons")
	}

	// Test 3: All addons enabled
	cfg3, err := Load("./testdata/v1alpha4/valid-addons-all-enabled.yaml")
	if err != nil {
		t.Fatalf("failed to load config with all addons enabled: %v", err)
	}
	if !cfg3.Addons.MetalLB || !cfg3.Addons.EnvoyGateway || !cfg3.Addons.MetricsServer || !cfg3.Addons.CoreDNSTuning || !cfg3.Addons.Dashboard {
		t.Error("expected all addons to be true when explicitly set to true")
	}

	// Test 4: New addon fields can be set to false
	cfg4, err := Load("./testdata/v1alpha4/valid-addons-new-fields.yaml")
	if err != nil {
		t.Fatalf("failed to load config with new addon fields disabled: %v", err)
	}
	if cfg4.Addons.LocalRegistry {
		t.Error("expected LocalRegistry to be false when explicitly set to false")
	}
	if cfg4.Addons.CertManager {
		t.Error("expected CertManager to be false when explicitly set to false")
	}
	if cfg4.Addons.LocalPath {
		t.Error("expected LocalPath to be false when explicitly set to false")
	}
}
