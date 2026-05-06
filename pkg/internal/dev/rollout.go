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
	"time"
)

// RolloutRestartAndWait runs `kubectl rollout restart deployment/<name>`
// followed by `kubectl rollout status --timeout=<dur> deployment/<name>`
// against the HOST kubectl using the supplied kubeconfig file.
//
// This is the HOST-side rollout pattern (RESEARCH §3) — distinct from the
// in-node pattern used by system addons during cluster create (e.g.
// pkg/cluster/internal/create/actions/installcorednstuning/corednstuning.go
// uses node.CommandContext with the in-cluster /etc/kubernetes/admin.conf).
// The user's Deployment for `kinder dev` lives in a user-managed namespace,
// and `kinder dev` runs on the host, so we use the host kubectl plus the
// EXTERNAL (host-reachable) kubeconfig produced by WriteKubeconfigTemp.
//
// Why `rollout status` rather than `kubectl wait --for=condition=Available`:
// rollout status is the canonical kubectl idiom for "wait for the new
// ReplicaSet to converge after a restart". It blocks until ALL pods are in
// the new generation and Ready. `kubectl wait` checks Deployment-level
// Available which is satisfied for a single Ready pod and does NOT confirm
// the new image is fully rolled out.
//
// All side effects route through the package-level devCmder so test fakes
// can capture argv shape and inject failures.
func RolloutRestartAndWait(ctx context.Context, kubeconfigPath, namespace, deployment string, timeout time.Duration) error {
	if kubeconfigPath == "" {
		return fmt.Errorf("dev rollout: kubeconfigPath empty")
	}
	if namespace == "" {
		return fmt.Errorf("dev rollout: namespace empty")
	}
	if deployment == "" {
		return fmt.Errorf("dev rollout: deployment empty")
	}
	if timeout <= 0 {
		return fmt.Errorf("dev rollout: timeout %v must be > 0", timeout)
	}

	restartCmd := devCmder.CommandContext(ctx,
		"kubectl",
		"--kubeconfig="+kubeconfigPath,
		"rollout", "restart",
		"--namespace="+namespace,
		"deployment/"+deployment,
	)
	if err := restartCmd.Run(); err != nil {
		return fmt.Errorf("rollout restart deployment/%s in namespace %s: %w", deployment, namespace, err)
	}

	statusCmd := devCmder.CommandContext(ctx,
		"kubectl",
		"--kubeconfig="+kubeconfigPath,
		"rollout", "status",
		"--namespace="+namespace,
		"deployment/"+deployment,
		"--timeout="+timeout.String(),
	)
	if err := statusCmd.Run(); err != nil {
		return fmt.Errorf("rollout status deployment/%s in namespace %s (timeout=%s): %w", deployment, namespace, timeout, err)
	}
	return nil
}

// RolloutFn is the package-level test-injection point. Plan 03's cycle
// runner calls dev.RolloutFn(...). Tests can override:
//
//	prev := dev.RolloutFn
//	dev.RolloutFn = fakeRollout
//	t.Cleanup(func(){ dev.RolloutFn = prev })
var RolloutFn = RolloutRestartAndWait
