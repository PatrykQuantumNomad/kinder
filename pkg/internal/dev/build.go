/*
Copyright 2026 The Kubernetes Authors.

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

package dev

import (
	"context"
	"fmt"

	"sigs.k8s.io/kind/pkg/exec"
)

// devCmder is the package-level exec.Cmder used by build/load/rollout
// primitives so unit tests can swap in a recording fake without spinning
// real subprocesses. In production it forwards to exec.DefaultCmder.
//
// Keeping the indirection here (rather than threading a Cmder through every
// call) matches the pkg/internal/lifecycle pattern (state.go:36).
var devCmder exec.Cmder = exec.DefaultCmder

// BuildImage shells out to the container runtime binary (docker, podman,
// nerdctl, finch, ...) to build an image from contextDir and tag it
// imageTag. The build context is the entire directory; .dockerignore is
// honored automatically by the runtime (no kinder-side filtering).
//
// The user's Deployment must already reference imageTag and use
// imagePullPolicy: Never (otherwise the cached image will not be picked up
// after the rollout). Plan 04 documents this in --help.
//
// BuildImage uses the package-level exec.Cmder so test fakes can intercept.
// Arguments are passed individually — there is no shell layer, hence zero
// shell-injection risk (RESEARCH security domain V5).
func BuildImage(ctx context.Context, binaryName, imageTag, contextDir string) error {
	if binaryName == "" {
		return fmt.Errorf("dev build: binaryName empty")
	}
	if imageTag == "" {
		return fmt.Errorf("dev build: imageTag empty")
	}
	if contextDir == "" {
		return fmt.Errorf("dev build: contextDir empty")
	}
	cmd := devCmder.CommandContext(ctx, binaryName, "build", "-t", imageTag, contextDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker build (%s build -t %s %s): %w", binaryName, imageTag, contextDir, err)
	}
	return nil
}

// BuildImageFn is the package-level test-injection point Plan 03's cycle
// runner calls (dev.BuildImageFn(...)). Tests can override:
//
//	prev := dev.BuildImageFn
//	dev.BuildImageFn = fakeBuilder
//	t.Cleanup(func(){ dev.BuildImageFn = prev })
var BuildImageFn = BuildImage
