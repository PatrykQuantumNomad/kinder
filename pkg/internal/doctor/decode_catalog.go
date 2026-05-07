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

// Catalog is the v1 seed of known runtime error patterns for kinder doctor decode.
// It ships 16 HIGH-confidence entries sourced from RESEARCH.md §"Pattern Catalog Seed".
//
// IDs shipped in v1:
//   KUB-01, KUB-02, KUB-03, KUB-04, KUB-05
//   KADM-01, KADM-02, KADM-03
//   CTD-01, CTD-02, CTD-03
//   DOCK-01, DOCK-02, DOCK-03
//   ADDON-01, ADDON-02
//
// AutoFixable=false and AutoFix=nil for all entries — Plan 50-04 will flip the
// relevant flags for KUB-01, KUB-02, KADM-02, KUB-05.
var Catalog = []DecodePattern{
	// -------------------------------------------------------------------------
	// ScopeKubelet (5 entries: KUB-01..KUB-05)
	// -------------------------------------------------------------------------
	{
		ID:          "KUB-01",
		Scope:       ScopeKubelet,
		Match:       "too many open files",
		Explanation: "Inotify watch limit exhausted — kubelet cannot watch required files. This is the most common kind cluster failure on Linux hosts with default kernel settings.",
		Fix:         "sudo sysctl fs.inotify.max_user_watches=524288 && sudo sysctl fs.inotify.max_user_instances=512",
		DocLink:     "https://kind.sigs.k8s.io/docs/user/known-issues/#pod-errors-due-to-too-many-open-files",
		AutoFixable: false,
		AutoFix:     nil,
	},
	{
		ID:          "KUB-02",
		Scope:       ScopeKubelet,
		Match:       "failed to create fsnotify watcher",
		Explanation: "Same root cause as KUB-01 — kubelet failed to set up an inotify watcher because the inotify instance limit is exhausted.",
		Fix:         "sudo sysctl fs.inotify.max_user_watches=524288 && sudo sysctl fs.inotify.max_user_instances=512",
		DocLink:     "https://kind.sigs.k8s.io/docs/user/known-issues/",
		AutoFixable: false,
		AutoFix:     nil,
	},
	{
		ID:          "KUB-03",
		Scope:       ScopeKubelet,
		Match:       "kubelet is not running",
		Explanation: "Kubelet service has not started — usually caused by a cgroup driver mismatch or a CRI socket misconfiguration.",
		Fix:         "Check docker logs <node> for prior errors; verify the cgroup driver matches the container runtime configuration.",
		DocLink:     "https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/troubleshooting-kubeadm/",
		AutoFixable: false,
		AutoFix:     nil,
	},
	{
		ID:          "KUB-04",
		Scope:       ScopeKubelet,
		Match:       "Get \"http://127.0.0.1:10248/healthz\": context deadline exceeded",
		Explanation: "Kubelet health check timed out — kubelet is not responding, often caused by missing cgroup support or insufficient Docker memory limits.",
		Fix:         "Increase Docker memory limit (≥4 GB recommended); check docker logs <node> for cgroup errors.",
		DocLink:     "https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/troubleshooting-kubeadm/",
		AutoFixable: false,
		AutoFix:     nil,
	},
	{
		ID:          "KUB-05",
		Scope:       ScopeKubelet,
		Match:       "regex:error adding pid \\d+ to cgroups",
		Explanation: "Cgroup v2 hierarchy conflict — the node entrypoint had not finished cgroup setup when a container exec ran.",
		Fix:         "Wait for node readiness before exec; run kinder doctor before re-creating the cluster.",
		DocLink:     "https://github.com/kubernetes-sigs/kind/issues/2409",
		AutoFixable: false,
		AutoFix:     nil,
	},

	// -------------------------------------------------------------------------
	// ScopeKubeadm (3 entries: KADM-01..KADM-03)
	// -------------------------------------------------------------------------
	{
		ID:          "KADM-01",
		Scope:       ScopeKubeadm,
		Match:       "[ERROR CRI]: container runtime is not running",
		Explanation: "kubeadm pre-flight check failed: containerd is present but its CRI service is disabled. Typically caused by the CRI plugin being explicitly disabled in containerd config.",
		Fix:         "docker exec <node> systemctl restart containerd; or recreate the cluster.",
		DocLink:     "https://github.com/containerd/containerd/issues/8139",
		AutoFixable: false,
		AutoFix:     nil,
	},
	{
		ID:          "KADM-02",
		Scope:       ScopeKubeadm,
		Match:       "coredns",
		Explanation: "CoreDNS pod detected in events — if CoreDNS is stuck in Pending, the CNI plugin is not installed or misconfigured. Check pod status to confirm.",
		Fix:         "kubectl get pods -n kube-system; verify kindnet/flannel is running. If Pending, check CNI DaemonSet logs.",
		DocLink:     "https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/troubleshooting-kubeadm/",
		AutoFixable: false,
		AutoFix:     nil,
	},
	{
		ID:          "KADM-03",
		Scope:       ScopeKubeadm,
		Match:       "context deadline exceeded",
		Explanation: "kubeadm timed out waiting for a component to become ready — usually kubelet health or etcd. Often caused by insufficient Docker resources.",
		Fix:         "Increase Docker CPU/memory limits; check docker logs <node> for prior errors.",
		DocLink:     "https://github.com/kubernetes/kubeadm/issues/3069",
		AutoFixable: false,
		AutoFix:     nil,
	},

	// -------------------------------------------------------------------------
	// ScopeContainerd (3 entries: CTD-01..CTD-03)
	// -------------------------------------------------------------------------
	{
		ID:          "CTD-01",
		Scope:       ScopeContainerd,
		Match:       "failed to pull image",
		Explanation: "Image tag or digest not found in registry — wrong tag, stale digest, or private registry without credentials.",
		Fix:         "Verify image tag; if loading a local image: kinder load images <image>.",
		DocLink:     "",
		AutoFixable: false,
		AutoFix:     nil,
	},
	{
		ID:          "CTD-02",
		Scope:       ScopeContainerd,
		Match:       "connection refused",
		Explanation: "Registry unreachable — network issue or air-gapped environment preventing image pull.",
		Fix:         "Check network connectivity; pre-load required images with kinder load images.",
		DocLink:     "",
		AutoFixable: false,
		AutoFix:     nil,
	},
	{
		ID:          "CTD-03",
		Scope:       ScopeContainerd,
		Match:       "ImagePullBackOff",
		Explanation: "Containerd cannot pull the image after repeated retries — see CTD-01/CTD-02 for root cause.",
		Fix:         "kubectl describe pod <name>; check events for specific pull error.",
		DocLink:     "",
		AutoFixable: false,
		AutoFix:     nil,
	},

	// -------------------------------------------------------------------------
	// ScopeDocker (3 entries: DOCK-01..DOCK-03)
	// -------------------------------------------------------------------------
	{
		ID:          "DOCK-01",
		Scope:       ScopeDocker,
		Match:       "no space left on device",
		Explanation: "Docker has run out of disk space — kind cluster creation or image loading cannot proceed.",
		Fix:         "docker system prune to reclaim space; check df -h for usage.",
		DocLink:     "",
		AutoFixable: false,
		AutoFix:     nil,
	},
	{
		ID:          "DOCK-02",
		Scope:       ScopeDocker,
		Match:       "docker.sock",
		Explanation: "Docker socket not accessible — user is not in the docker group or socket permissions are wrong.",
		Fix:         "sudo usermod -aG docker $USER && newgrp docker",
		DocLink:     "",
		AutoFixable: false,
		AutoFix:     nil,
	},
	{
		ID:          "DOCK-03",
		Scope:       ScopeDocker,
		Match:       "cannot create temp file",
		Explanation: "Docker installed via Snap lacks access to the system TMPDIR — kind cannot write temporary files.",
		Fix:         "export TMPDIR=$HOME/tmp && mkdir -p $HOME/tmp",
		DocLink:     "https://kind.sigs.k8s.io/docs/user/known-issues/#docker-installed-with-snap",
		AutoFixable: false,
		AutoFix:     nil,
	},

	// -------------------------------------------------------------------------
	// ScopeAddon (2 entries: ADDON-01, ADDON-02)
	// ADDON-02 is MEDIUM confidence in research but required to provide addon
	// scope coverage beyond the generic CrashLoopBackOff (DIAG-02 SC2).
	// -------------------------------------------------------------------------
	{
		ID:          "ADDON-01",
		Scope:       ScopeAddon,
		Match:       "CrashLoopBackOff",
		Explanation: "A system addon pod is crash-looping — check kubectl logs -n kube-system <pod> for details.",
		Fix:         "kubectl logs -n kube-system <pod> --previous; look for config or image errors.",
		DocLink:     "",
		AutoFixable: false,
		AutoFix:     nil,
	},
	{
		ID:          "ADDON-02",
		Scope:       ScopeAddon,
		Match:       "MountVolume.SetUp failed",
		Explanation: "A ConfigMap required by an addon pod does not exist — cluster creation was incomplete or a resource was deleted.",
		Fix:         "kubectl get configmap -n kube-system; recreate the cluster if resources are missing.",
		DocLink:     "",
		AutoFixable: false,
		AutoFix:     nil,
	},
}
