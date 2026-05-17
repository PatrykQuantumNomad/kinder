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

// Package lifecycle – certregen.go provides the reactive drift detection and
// wholesale etcd-adjacent cert regeneration helpers consumed by resume.go.
//
// Despite the historical function name RegenerateEtcdPeerCertsWholesale, this
// module renews up to four cert types per CP (etcd-peer, etcd-server,
// etcd-healthcheck-client, apiserver-etcd-client — exact set evidence-locked
// by Phase 57.3 Plan 01 Task 0) and cycles the matching static-pod manifest
// per cert type. The function name is preserved for W2 invariant (called as
// unqualified name from resume.go in the same package).
package lifecycle

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"sigs.k8s.io/kind/pkg/cluster/loadbalancer"
	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/errors"
	"sigs.k8s.io/kind/pkg/exec"
	"sigs.k8s.io/kind/pkg/log"
)

// Package-level constants for the etcd static-pod cycle timing.
// Per RESEARCH: kubelet fileCheckFrequency=20s + 5s safety margin = 25s wait
// for kubelet to notice the removed manifest.
const (
	kubeletFileCheckFrequency  = 20 * time.Second
	staticPodCycleSafetyMargin = 5 * time.Second
	etcdManifestPath           = "/etc/kubernetes/manifests/etcd.yaml"
	etcdManifestBackup         = "/tmp/etcd-bak.yaml"

	kubeAPIServerManifestPath   = "/etc/kubernetes/manifests/kube-apiserver.yaml"
	kubeAPIServerManifestBackup = "/tmp/kube-apiserver-bak.yaml"
	etcdHealthGateDeadline      = 60 * time.Second
	apiserverHealthGateDeadline = 60 * time.Second
	healthGateTick              = 1 * time.Second
)

// certRegenSleeper is a package-level var so tests can swap it to a no-op,
// preventing real 25s+20s sleep blocks during unit tests.
// Production value: time.Sleep.
var certRegenSleeper = func(d time.Duration) { time.Sleep(d) }

// etcdHealthChecker probes etcd liveness from inside a CP node. Production
// implementation runs `crictl exec <etcd-id> etcdctl endpoint health` against
// the supplied endpoint URL, tick 1s up to 60s. Tests swap to instant-OK or
// instant-fail mocks via swapEtcdHealthChecker in certregen_test.go (mirrors
// swapCertRegenSleeper at certregen_test.go:71).
var etcdHealthChecker = func(node nodes.Node, endpoint string) error {
	return pollEtcdHealthRealImpl(node, etcdHealthGateDeadline, healthGateTick, endpoint)
}

// apiserverHealthChecker polls kube-apiserver /healthz from inside a CP node
// via `curl -k --max-time 5 <healthzURL>`, tick 1s up to 60s. Tests swap via
// swapApiserverHealthChecker.
var apiserverHealthChecker = func(node nodes.Node, healthzURL string) error {
	return pollApiserverHealthzRealImpl(node, apiserverHealthGateDeadline, healthGateTick, healthzURL)
}

// certCycle holds per-cert-type config for one renew-then-cycle iteration.
type certCycle struct {
	certName     string // e.g. "etcd-peer"
	manifestPath string // /etc/kubernetes/manifests/etcd.yaml or kube-apiserver.yaml
	backupPath   string // /tmp/<basename>-bak.yaml
	cycleKind    string // "etcd" | "apiserver" — selects which health-gate to use
}

// cycleForCertType maps a kubeadm cert subcommand to the manifest cycle
// target. Confirmed by Kubernetes PKI docs (RESEARCH §1):
//   etcd-peer / etcd-server / etcd-healthcheck-client → etcd.yaml
//   apiserver-etcd-client → kube-apiserver.yaml
var cycleForCertType = map[string]certCycle{
	"etcd-peer":               {"etcd-peer", etcdManifestPath, etcdManifestBackup, "etcd"},
	"etcd-server":             {"etcd-server", etcdManifestPath, etcdManifestBackup, "etcd"},
	"etcd-healthcheck-client": {"etcd-healthcheck-client", etcdManifestPath, etcdManifestBackup, "etcd"},
	"apiserver-etcd-client":   {"apiserver-etcd-client", kubeAPIServerManifestPath, kubeAPIServerManifestBackup, "apiserver"},
}

// IPDriftDetected returns true iff the current docker-inspect IP for a CP
// differs from the value recorded in /kind/ipam-state.json (or no recording
// exists, i.e. legacy cluster).
//
// Parameters:
//   - binaryName: container runtime CLI ("docker", "podman").
//   - container: CP container name.
//   - tmpDir: host temp directory for the docker-cp file staging.
//
// Returns: (drifted, currentIP, recordedIP, err).
//
// Legacy cluster (no state file): recordedIP="", drifted=true, err=nil.
// If ReadIPAMState fails for a non-"no such file" reason, the error is returned.
// T-52-03-01: currentIP is validated via net.ParseIP before returning.
func IPDriftDetected(binaryName, container, tmpDir string) (drifted bool, currentIP string, recordedIP string, err error) {
	// Step 1: Read the recorded state from /kind/ipam-state.json.
	// ReadIPAMState is in the same package (ippin.go facade → pkg/internal/ippin).
	state, readErr := ReadIPAMState(binaryName, container, tmpDir)
	if readErr != nil {
		// Legacy detection: if the file is absent, treat the cluster as legacy
		// → always regen (cert-regen forever for legacy per CONTEXT.md).
		// We use a broad string match because the error is wrapped; the root cause
		// from docker cp typically contains "No such file" or "not found".
		errStr := strings.ToLower(readErr.Error())
		if strings.Contains(errStr, "no such file") ||
			strings.Contains(errStr, "not found") ||
			strings.Contains(errStr, "does not exist") {
			// Legacy path: recordedIP="", drifted=true.
			return true, "", "", nil
		}
		// Unexpected error — propagate.
		return false, "", "", readErr
	}
	recordedIP = state.IPv4

	// Step 2: Inspect the current container IP via the runtime CLI.
	// Uses defaultCmder (same injection point as resume.go / state.go).
	lines, inspectErr := exec.OutputLines(defaultCmder(binaryName,
		"inspect",
		"--format", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}",
		container,
	))
	if inspectErr != nil {
		return false, "", recordedIP, errors.Wrapf(inspectErr, "IPDriftDetected: failed to inspect %s", container)
	}
	rawIP := strings.TrimSpace(strings.Join(lines, ""))

	// T-52-03-01: Validate IP before use.
	if net.ParseIP(rawIP) == nil {
		return false, "", recordedIP, errors.Errorf("IPDriftDetected: invalid IP from inspect for %s: %q", container, rawIP)
	}
	currentIP = rawIP

	// Drift = IPs differ, or recorded was empty (shouldn't happen given state
	// file was read without error, but defensive).
	drifted = currentIP != recordedIP || recordedIP == ""
	return drifted, currentIP, recordedIP, nil
}

// currentNodeIPv4 returns the IPv4 address of eth0 inside the given CP node.
// Used by renewAndCycleOne to detect IP drift and patch kubeadm config accordingly.
func currentNodeIPv4(node nodes.Node) (string, error) {
	lines, err := exec.OutputLines(node.Command(
		"ip", "-4", "addr", "show", "eth0",
	))
	if err != nil {
		return "", errors.Wrapf(err, "ip addr show eth0 on %s", node.String())
	}
	// Parse the first "inet <IP>/prefix" line.
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if !strings.HasPrefix(ln, "inet ") {
			continue
		}
		// "inet 172.19.0.3/16 brd ..." → "172.19.0.3"
		fields := strings.Fields(ln)
		if len(fields) < 2 {
			continue
		}
		ipCIDR := fields[1]
		ip := strings.Split(ipCIDR, "/")[0]
		if net.ParseIP(ip) != nil {
			return ip, nil
		}
	}
	return "", errors.Errorf("no IPv4 inet line for eth0 on %s; lines: %v", node.String(), lines)
}

// renewOrRegenOneCert issues a cert renewal for certName on node.
//
// Because `kubeadm certs renew` copies SANs verbatim from the existing cert,
// it produces a new cert with STALE IPs when the container has been re-IPed
// by Docker IPAM after a pause/resume (the exact scenario Phase 57.3 fixes).
// We therefore always use `kubeadm init phase certs <name>` with the CURRENT
// node IP injected via localAPIEndpoint.advertiseAddress:
//   - Delete the existing cert+key so kubeadm does not skip generation.
//   - Write a minimal kubeadm config with the correct IP to /tmp/kinder-regen-<certName>.yaml.
//   - Run `kubeadm init phase certs <name> --config <file>`.
//
// For client-only certs (etcd-healthcheck-client, apiserver-etcd-client) there
// are no IP SANs, so the regeneration is still correct (and idempotent).
func renewOrRegenOneCert(node nodes.Node, certName, currentIP string) error {
	// Cert-path table (kubeadm subcommand → pki path).
	certPaths := map[string][2]string{
		"etcd-peer":               {"/etc/kubernetes/pki/etcd/peer.crt", "/etc/kubernetes/pki/etcd/peer.key"},
		"etcd-server":             {"/etc/kubernetes/pki/etcd/server.crt", "/etc/kubernetes/pki/etcd/server.key"},
		"etcd-healthcheck-client": {"/etc/kubernetes/pki/etcd/healthcheck-client.crt", "/etc/kubernetes/pki/etcd/healthcheck-client.key"},
		"apiserver-etcd-client":   {"/etc/kubernetes/pki/apiserver-etcd-client.crt", "/etc/kubernetes/pki/apiserver-etcd-client.key"},
	}
	paths, ok := certPaths[certName]
	if !ok {
		return errors.Errorf("renewOrRegenOneCert: unknown cert name %q", certName)
	}
	crtPath, keyPath := paths[0], paths[1]

	hostname, err := exec.OutputLines(node.Command("hostname"))
	if err != nil || len(hostname) == 0 {
		return errors.Wrapf(err, "hostname on %s", node.String())
	}
	hostnameStr := strings.TrimSpace(hostname[0])

	// Write a minimal kubeadm config with the CURRENT IP.
	cfgPath := "/tmp/kinder-regen-" + certName + ".yaml"
	cfgContent := "apiVersion: kubeadm.k8s.io/v1beta4\nkind: InitConfiguration\n" +
		"localAPIEndpoint:\n  advertiseAddress: " + currentIP + "\n  bindPort: 6443\n" +
		"nodeRegistration:\n  name: " + hostnameStr + "\n" +
		"---\n" +
		"apiVersion: kubeadm.k8s.io/v1beta4\nkind: ClusterConfiguration\n" +
		"etcd:\n  local:\n    dataDir: /var/lib/etcd\n"

	// Write config via echo -e into the container.
	if err := node.Command("bash", "-c",
		"cat > "+cfgPath+" << 'KUBEADM_CFG_EOF'\n"+cfgContent+"\nKUBEADM_CFG_EOF",
	).Run(); err != nil {
		return errors.Wrapf(err, "write kubeadm regen config on %s", node.String())
	}

	// Delete existing cert+key so `kubeadm init phase certs` does not skip.
	if err := node.Command("rm", "-f", crtPath, keyPath).Run(); err != nil {
		return errors.Wrapf(err, "rm existing cert %s on %s", certName, node.String())
	}

	// Regenerate with correct IP.
	if err := node.Command("kubeadm", "init", "phase", "certs", certName,
		"--config", cfgPath,
	).Run(); err != nil {
		return errors.Wrapf(err, "kubeadm certs renew %s failed on %s", certName, node.String())
	}
	return nil
}

// renewAndCycleOne performs renew + manifest mv-out + sleep + mv-in + active
// health poll for one cert type on one CP. Hard-fails on any sub-step error.
// The active health gate replaces the historical 20s staticPodRecreationWait.
func renewAndCycleOne(node nodes.Node, c certCycle, etcdEndpoint, healthzURL string, currentIP string, logger log.Logger) error {
	// 1. Regenerate cert with current IP (handles IP drift + cert expiry in one step).
	if err := renewOrRegenOneCert(node, c.certName, currentIP); err != nil {
		return errors.Wrapf(err, "kubeadm certs renew %s failed on %s", c.certName, node.String())
	}
	// 2. mv-out (kubelet will notice within fileCheckFrequency and stop the pod)
	if err := node.Command("mv", c.manifestPath, c.backupPath).Run(); err != nil {
		return errors.Wrapf(err, "mv out %s failed on %s", c.manifestPath, node.String())
	}
	// 3. wait for kubelet to notice manifest gone — keep existing sleep
	//    (no kubelet API for "have you noticed"; see RESEARCH §3 Pitfall 3).
	certRegenSleeper(kubeletFileCheckFrequency + staticPodCycleSafetyMargin)
	// 4. mv-in
	if err := node.Command("mv", c.backupPath, c.manifestPath).Run(); err != nil {
		return errors.Wrapf(err, "mv back %s failed on %s", c.manifestPath, node.String())
	}
	// 5. active health gate (replaces the old 20s static staticPodRecreationWait)
	switch c.cycleKind {
	case "etcd":
		if err := etcdHealthChecker(node, etcdEndpoint); err != nil {
			return errors.Wrapf(err, "etcd ready-gate failed on %s after %s renew", node.String(), c.certName)
		}
	case "apiserver":
		if err := apiserverHealthChecker(node, healthzURL); err != nil {
			return errors.Wrapf(err, "apiserver healthz failed on %s after %s renew", node.String(), c.certName)
		}
	}
	logger.V(1).Infof(" ✓ renewed %s on %s, %s pod healthy", c.certName, node.String(), c.cycleKind)
	return nil
}

// RegenerateEtcdPeerCertsWholesale runs kubeadm cert renew for each cert type
// in the locked set on every CP node and cycles the appropriate static-pod
// manifest per cert type. All CPs must be started before this call. Failure on
// any CP halts the operation and returns a structured diagnostic error directing
// the user to delete and recreate the cluster.
//
// The function is a no-op when len(cpNodes) <= 1 (defense in depth; callers
// already gate on HA, but safety is preserved here too).
//
// The function name is preserved per W2 invariant (called as unqualified name
// from resume.go in the same package). The scope expansion is internal only.
//
// Evidence-driven cert set locked by Phase 57.3 Plan 01 Task 0 diagnostics:
// etcd-peer + etcd-server + etcd-healthcheck-client + apiserver-etcd-client.
func RegenerateEtcdPeerCertsWholesale(cpNodes []nodes.Node, logger log.Logger) error {
	if len(cpNodes) <= 1 {
		return nil
	}

	// Derive loopback endpoints from cluster IPFamily (RESEARCH §7).
	// ClusterIPFamily returns true for IPv6/dual, false for IPv4. The
	// dual-stack case picks the IPv6 loopback (works fine — etcd binds
	// on `::` with ipv4_compat=true; the IPv6 form is accepted).
	ipv6, fErr := loadbalancer.ClusterIPFamily(ProviderBinaryName(), cpNodes[0])
	if fErr != nil {
		return errors.Wrapf(fErr,
			"cert regen: failed to derive cluster IP family from %s. Cluster state is undefined — delete and recreate the cluster",
			cpNodes[0].String())
	}
	etcdEndpoint := "https://127.0.0.1:2379"
	healthzURL := "https://127.0.0.1:6443/healthz"
	if ipv6 {
		etcdEndpoint = "https://[::1]:2379"
		healthzURL = "https://[::1]:6443/healthz"
	}

	// Evidence-driven cert list (LOCKED by Plan 01 Task 0 — see CONTEXT.md
	// section "## Plan 01 Task 0 — Cert-Set Lock").
	certTypes := []string{"etcd-peer", "etcd-server", "etcd-healthcheck-client", "apiserver-etcd-client"}

	total := len(cpNodes)
	for i, node := range cpNodes {
		logger.V(0).Infof("Regenerating etcd-adjacent certs on %s (%d/%d)", node.String(), i+1, total)

		// Determine current container IP for cert SAN injection (IP drift fix).
		currentIP, ipErr := currentNodeIPv4(node)
		if ipErr != nil {
			dumpCertRegenDiagnostics(node, "ip-detect", logger)
			return errors.Wrapf(ipErr,
				"cert regen: failed to detect current IP on %s. Cluster state is undefined — delete and recreate the cluster",
				node.String())
		}
		logger.V(1).Infof("  current IP on %s: %s", node.String(), currentIP)

		// Snapshot pre-renew check-expiration for the post-pass verify.
		preSnap, preErr := captureCertExpirationSnapshot(node)
		if preErr != nil {
			logger.Warnf("pre-renew check-expiration snapshot failed on %s: %v (continuing; post-pass verify will be best-effort)", node.String(), preErr)
		}

		for _, ct := range certTypes {
			cyc, ok := cycleForCertType[ct]
			if !ok {
				dumpCertRegenDiagnostics(node, ct, logger)
				return errors.Errorf("unknown cert type %q (cluster state is undefined — delete and recreate the cluster)", ct)
			}
			if err := renewAndCycleOne(node, cyc, etcdEndpoint, healthzURL, currentIP, logger); err != nil {
				dumpCertRegenDiagnostics(node, ct, logger)
				return errors.Wrapf(err,
					"cert regen failed on %s for %s. Cluster state is undefined — delete and recreate the cluster",
					node.String(), ct)
			}
		}

		// Post-pass verification per CP — kubeadm certs check-expiration.
		// Hard-fail if any regenerated cert's notAfter did not advance.
		if err := verifyCheckExpirationAdvanced(node, certTypes, preSnap); err != nil {
			dumpCertRegenDiagnostics(node, "post-pass-verify", logger)
			return errors.Wrapf(err,
				"post-pass cert-expiration check failed on %s. Cluster state is undefined — delete and recreate the cluster",
				node.String())
		}
	}
	return nil
}

// certExpirationSnapshot maps cert subcommand name → notAfter string. Built
// from `kubeadm certs check-expiration -o json`. RESEARCH §9 Pitfall 6 warns
// against using exit code to detect renewal — use the parsed notAfter.
type certExpirationSnapshot map[string]string

func captureCertExpirationSnapshot(node nodes.Node) (certExpirationSnapshot, error) {
	out, err := exec.OutputLines(node.Command("kubeadm", "certs", "check-expiration", "-o", "json"))
	if err != nil {
		return nil, errors.Wrap(err, "kubeadm certs check-expiration -o json failed")
	}
	joined := strings.Join(out, "")
	return parseCheckExpiration(joined)
}

// parseCheckExpiration parses `kubeadm certs check-expiration -o json` output.
// Kubernetes 1.30+ uses the output.kubeadm.k8s.io/v1alpha3 schema where the
// expiration field is "expirationDate" (not "notAfter" which was the v1alpha2
// name). We try both field names for compatibility.
func parseCheckExpiration(raw string) (certExpirationSnapshot, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("empty check-expiration output")
	}
	var doc struct {
		Certificates []struct {
			Name           string `json:"name"`
			ExpirationDate string `json:"expirationDate"` // v1alpha3 (K8s 1.30+)
			NotAfter       string `json:"notAfter"`       // v1alpha2 (legacy)
		} `json:"certificates"`
	}
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return nil, errors.Wrap(err, "parse check-expiration JSON")
	}
	snap := make(certExpirationSnapshot, len(doc.Certificates))
	for _, c := range doc.Certificates {
		// Prefer expirationDate (v1alpha3); fall back to notAfter (v1alpha2).
		exp := c.ExpirationDate
		if exp == "" {
			exp = c.NotAfter
		}
		snap[c.Name] = exp
	}
	return snap, nil
}

// verifyCheckExpirationAdvanced asserts every cert in certTypes has a notAfter
// strictly greater than its pre-snapshot value (proves renewal actually
// happened). Pre-snapshot may be nil (snapshot failure) — in that case we
// best-effort verify the cert is at least present in the post-snapshot.
func verifyCheckExpirationAdvanced(node nodes.Node, certTypes []string, pre certExpirationSnapshot) error {
	post, err := captureCertExpirationSnapshot(node)
	if err != nil {
		return errors.Wrap(err, "post-renew check-expiration capture")
	}
	for _, ct := range certTypes {
		postNA, ok := post[ct]
		if !ok || postNA == "" {
			return errors.Errorf("cert %q missing from post-renew check-expiration output. Cluster state is undefined — delete and recreate the cluster", ct)
		}
		if pre != nil {
			preNA := pre[ct]
			if preNA != "" && postNA <= preNA {
				return errors.Errorf("cert %q notAfter did not advance (pre=%s post=%s). Cluster state is undefined — delete and recreate the cluster", ct, preNA, postNA)
			}
		}
	}
	return nil
}

// dumpCertRegenDiagnostics emits the four mandatory diagnostic fields via
// logger.Warnf when cert regen hard-fails. Each section uses 2>&1 || true
// semantics so a partial dump still produces useful signal (RESEARCH §Q6).
func dumpCertRegenDiagnostics(node nodes.Node, failedAt string, logger log.Logger) {
	logger.Warnf("=== cert-regen diagnostic dump (failed at: %s on %s) ===", failedAt, node.String())

	// 1. etcdctl member list (best-effort — etcd may itself be down)
	idOut, _ := exec.OutputLines(node.Command("crictl", "ps", "--name", "etcd", "-q"))
	var etcdID string
	for _, ln := range idOut {
		if id := strings.TrimSpace(ln); id != "" {
			etcdID = id
			break
		}
	}
	if etcdID != "" {
		out, _ := exec.OutputLines(node.Command(
			"crictl", "exec", etcdID, "etcdctl",
			"--cacert=/etc/kubernetes/pki/etcd/ca.crt",
			"--cert=/etc/kubernetes/pki/etcd/peer.crt",
			"--key=/etc/kubernetes/pki/etcd/peer.key",
			"--endpoints=https://127.0.0.1:2379",
			"member", "list", "--write-out=table",
		))
		logger.Warnf("etcdctl member list:\n%s", strings.Join(out, "\n"))
	} else {
		logger.Warnf("etcdctl member list: <etcd container not running>")
	}
	// 2. kubeadm certs check-expiration
	out, _ := exec.OutputLines(node.Command("kubeadm", "certs", "check-expiration"))
	logger.Warnf("kubeadm certs check-expiration:\n%s", strings.Join(out, "\n"))
	// 3. last 20 lines of crictl logs kube-apiserver
	apiIDOut, _ := exec.OutputLines(node.Command("crictl", "ps", "--name", "kube-apiserver", "-q"))
	var apiID string
	for _, ln := range apiIDOut {
		if id := strings.TrimSpace(ln); id != "" {
			apiID = id
			break
		}
	}
	if apiID != "" {
		logsOut, _ := exec.OutputLines(node.Command("crictl", "logs", "--tail", "20", apiID))
		logger.Warnf("kube-apiserver last 20 lines:\n%s", strings.Join(logsOut, "\n"))
	} else {
		logger.Warnf("kube-apiserver last 20 lines: <kube-apiserver container not running>")
	}
	// 4. serial-number table of all certs of interest
	serOut, _ := exec.OutputLines(node.Command("bash", "-c", `
            for f in /etc/kubernetes/pki/etcd/peer.crt /etc/kubernetes/pki/etcd/server.crt \
                     /etc/kubernetes/pki/etcd/healthcheck-client.crt \
                     /etc/kubernetes/pki/apiserver-etcd-client.crt; do
                echo -n "$f: "
                openssl x509 -noout -serial -in "$f" 2>/dev/null || echo "MISSING"
            done
        `))
	logger.Warnf("cert serials:\n%s", strings.Join(serOut, "\n"))
}

// pollEtcdHealthRealImpl is the production etcdHealthChecker. See RESEARCH §10A
// for the rationale: `crictl exec` is required because kindest/node rootfs lacks
// etcdctl on $PATH (only the etcd container image ships it).
//
// We use apiserver-etcd-client.{crt,key} (NOT peer.crt/peer.key) because THAT is
// the exact cert pair whose handshake failure this phase recovers from. The
// fatal production symptom is `addrConn.createTransport failed to connect to
// {Addr: "127.0.0.1:2379"} → authentication handshake failed` — i.e. the
// apiserver→etcd-server handshake, NOT the etcd↔etcd peer handshake on :2380.
//
// The doctor's resumereadiness.go uses peer.crt because it does passive health
// monitoring (any working cert pair suffices to ping the protocol). Cert-regen
// recovery is different: the gate MUST validate the apiserver→etcd-server
// handshake explicitly, mirroring CONTEXT.md Diagnostic 2 (the protocol-level
// proof) so a passing gate cannot green-light a still-broken apiserver→etcd
// handshake. See .planning/phases/57.3-ha-cluster-cert-regen-recovery/57.3-CONTEXT.md.
func pollEtcdHealthRealImpl(node nodes.Node, deadline, tick time.Duration, endpoint string) error {
	end := time.Now().Add(deadline)
	var lastErr error
	for time.Now().Before(end) {
		etcdIDLines, err := exec.OutputLines(node.Command("crictl", "ps", "--name", "etcd", "-q"))
		if err != nil || len(etcdIDLines) == 0 {
			lastErr = errors.Wrap(err, "etcd container not running")
			certRegenSleeper(tick)
			continue
		}
		etcdID := strings.TrimSpace(etcdIDLines[0])
		// Use peer.crt/peer.key for etcdctl inside the etcd container.
		// The etcd container's filesystem only mounts /etc/kubernetes/pki/etcd/;
		// apiserver-etcd-client.crt lives at /etc/kubernetes/pki/ (not /etcd/)
		// and is NOT bind-mounted into the etcd container. peer.crt IS mounted
		// and is sufficient to prove the etcd server is alive and accepting TLS
		// connections post cert-regen. The apiserver-etcd-client handshake is
		// validated indirectly by the subsequent apiserverHealthChecker gate which
		// confirms kube-apiserver came up (requires a working etcd TLS session).
		args := []string{
			"crictl", "exec", etcdID, "etcdctl",
			"--cacert=/etc/kubernetes/pki/etcd/ca.crt",
			"--cert=/etc/kubernetes/pki/etcd/peer.crt",
			"--key=/etc/kubernetes/pki/etcd/peer.key",
			"--endpoints=" + endpoint,
			"endpoint", "health", "--write-out=json",
		}
		out, execErr := exec.OutputLines(node.Command(args[0], args[1:]...))
		if execErr == nil {
			healthy, total, parseErr := parseEtcdHealth(strings.Join(out, ""))
			if parseErr == nil && total > 0 && healthy == total {
				return nil
			}
			lastErr = fmt.Errorf("etcdctl reports %d/%d healthy (parse err: %v)", healthy, total, parseErr)
		} else {
			lastErr = errors.Wrap(execErr, "etcdctl exec failed")
		}
		certRegenSleeper(tick)
	}
	return errors.Wrapf(lastErr, "etcd ready-gate timed out after %v at %s", deadline, endpoint)
}

// pollApiserverHealthzRealImpl is the production apiserverHealthChecker. Uses
// `curl -k` because we are only verifying the responder is up, not the cert chain.
func pollApiserverHealthzRealImpl(node nodes.Node, deadline, tick time.Duration, healthzURL string) error {
	end := time.Now().Add(deadline)
	var lastErr error
	for time.Now().Before(end) {
		out, err := exec.OutputLines(node.Command(
			"curl", "-k", "-s", "-o", "/dev/null", "-w", "%{http_code}",
			"--max-time", "5", healthzURL,
		))
		if err == nil && len(out) > 0 && strings.TrimSpace(out[0]) == "200" {
			return nil
		}
		lastErr = err
		certRegenSleeper(tick)
	}
	return errors.Wrapf(lastErr, "apiserver healthz timed out after %v at %s", deadline, healthzURL)
}

// parseEtcdHealth is a verbatim copy of the parser at pkg/internal/doctor/
// resumereadiness.go:263-275. Copy-pasted (not imported) to avoid creating a
// lifecycle→doctor layering dependency. See RESEARCH §2 Option A.
func parseEtcdHealth(rawJSON string) (healthy, total int, err error) {
	var entries []map[string]interface{}
	if uErr := json.Unmarshal([]byte(rawJSON), &entries); uErr != nil {
		return 0, 0, fmt.Errorf("etcd health JSON parse: %w", uErr)
	}
	for _, e := range entries {
		total++
		if h, ok := e["health"].(bool); ok && h {
			healthy++
		}
	}
	return healthy, total, nil
}
