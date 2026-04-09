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

package docker

import (
	"strings"
	"testing"

	"sigs.k8s.io/kind/pkg/internal/apis/config"
	"sigs.k8s.io/kind/pkg/internal/cli"
	"sigs.k8s.io/kind/pkg/log"
)

// makeStatus returns a no-op *cli.Status suitable for tests.
func makeStatus() *cli.Status {
	return cli.StatusForLogger(log.NoopLogger{})
}

// makeAirGappedConfig returns a minimal air-gapped Cluster config with the
// given node images and MetalLB addon enabled (so RequiredAllImages returns
// both node images and addon images).
func makeAirGappedConfig(nodeImages []string, metalLB bool) *config.Cluster {
	nodes := make([]config.Node, len(nodeImages))
	for i, img := range nodeImages {
		nodes[i] = config.Node{Image: img}
	}
	return &config.Cluster{
		AirGapped: true,
		Nodes:     nodes,
		Addons: config.Addons{
			MetalLB: metalLB,
		},
	}
}

// TestCheckAllImagesPresent_AllPresent verifies that checkAllImagesPresent
// returns nil when every required image is found locally.
func TestCheckAllImagesPresent_AllPresent(t *testing.T) {
	orig := inspectImageFunc
	t.Cleanup(func() { inspectImageFunc = orig })
	inspectImageFunc = func(image string) bool { return true }

	cfg := makeAirGappedConfig([]string{"kindest/node:v1.29.0", "kindest/node:v1.28.0"}, false)
	err := checkAllImagesPresent(log.NoopLogger{}, makeStatus(), cfg)
	if err != nil {
		t.Errorf("expected nil error when all images are present, got: %v", err)
	}
}

// TestCheckAllImagesPresent_SomeMissing verifies that checkAllImagesPresent
// returns a non-nil error listing every missing image when some images are absent.
func TestCheckAllImagesPresent_SomeMissing(t *testing.T) {
	orig := inspectImageFunc
	t.Cleanup(func() { inspectImageFunc = orig })

	missingImages := map[string]bool{
		"kindest/node:v1.28.0": true,
	}
	inspectImageFunc = func(image string) bool {
		return !missingImages[image]
	}

	cfg := makeAirGappedConfig([]string{"kindest/node:v1.29.0", "kindest/node:v1.28.0"}, false)
	err := checkAllImagesPresent(log.NoopLogger{}, makeStatus(), cfg)
	if err == nil {
		t.Fatal("expected non-nil error when images are missing, got nil")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "kindest/node:v1.28.0") {
		t.Errorf("error message should contain missing image name, got: %s", errMsg)
	}
	if strings.Contains(errMsg, "kindest/node:v1.29.0") {
		t.Errorf("error message should NOT contain present image, got: %s", errMsg)
	}
}

// TestCheckAllImagesPresent_AllMissing verifies that checkAllImagesPresent
// reports all missing images in the error, not just the first one.
func TestCheckAllImagesPresent_AllMissing(t *testing.T) {
	orig := inspectImageFunc
	t.Cleanup(func() { inspectImageFunc = orig })
	inspectImageFunc = func(image string) bool { return false }

	cfg := makeAirGappedConfig([]string{"kindest/node:v1.29.0", "kindest/node:v1.28.0"}, false)
	err := checkAllImagesPresent(log.NoopLogger{}, makeStatus(), cfg)
	if err == nil {
		t.Fatal("expected non-nil error when all images are missing, got nil")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "kindest/node:v1.29.0") {
		t.Errorf("error message should contain first missing image, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "kindest/node:v1.28.0") {
		t.Errorf("error message should contain second missing image, got: %s", errMsg)
	}
}

// TestFormatMissingImagesError verifies the human-readable error format includes
// image names, the air-gapped prefix, and docker pre-load instructions.
func TestFormatMissingImagesError(t *testing.T) {
	missing := []string{"img1:latest", "img2:v1.0"}
	err := formatMissingImagesError(missing)
	if err == nil {
		t.Fatal("expected non-nil error, got nil")
	}
	msg := err.Error()

	// Must contain both image names
	for _, img := range missing {
		if !strings.Contains(msg, img) {
			t.Errorf("error message should contain image %q, got: %s", img, msg)
		}
	}

	// Must have the air-gapped prefix
	if !strings.Contains(msg, "air-gapped mode") {
		t.Errorf("error message should contain 'air-gapped mode' prefix, got: %s", msg)
	}

	// Must include docker pre-load instructions
	for _, instruction := range []string{"docker pull", "docker save", "docker load"} {
		if !strings.Contains(msg, instruction) {
			t.Errorf("error message should contain %q instruction, got: %s", instruction, msg)
		}
	}
}
