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

// currentNodeIPv6 returns the first global-scope IPv6 address of eth0 inside
// the given CP node. Used for IPv6/dual-stack clusters where etcd peer
// connections use IPv6 addresses.
func currentNodeIPv6(node nodes.Node) (string, error) {
	lines, err := exec.OutputLines(node.Command(
		"ip", "-6", "addr", "show", "eth0",
	))
	if err != nil {
		return "", errors.Wrapf(err, "ip -6 addr show eth0 on %s", node.String())
	}
	// Parse the first "inet6 <IP>/prefix scope global" line.
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if !strings.HasPrefix(ln, "inet6 ") {
			continue
		}
		if !strings.Contains(ln, "scope global") {
			continue // skip link-local and other non-global addresses
		}
		// "inet6 fc00:4d57:1afd:1f1b::a/64 scope global nodad" → "fc00:4d57:1afd:1f1b::a"
		fields := strings.Fields(ln)
		if len(fields) < 2 {
			continue
		}
		ipCIDR := fields[1]
		ip := strings.Split(ipCIDR, "/")[0]
		if parsed := net.ParseIP(ip); parsed != nil && parsed.To4() == nil {
			return ip, nil
		}
	}
	return "", errors.Errorf("no global IPv6 inet6 line for eth0 on %s; lines: %v", node.String(), lines)
}

// extractEtcdManifestIP reads the node's own IP from its etcd static-pod
// manifest by parsing the `--initial-advertise-peer-urls` flag. This is the
// IP the node had at cluster creation time (or last kubeadm join/init). After
// a Docker IPAM reassignment on pause/resume, this IP is stale — it no longer
// matches the container's actual eth0 IP.
//
// Format: --initial-advertise-peer-urls=https://<IP>:2380
// Returns the IP string (no port, no scheme), or an error if not found.
func extractEtcdManifestIP(node nodes.Node) (string, error) {
	// Use -- to separate flags from the pattern, preventing grep from treating
	// the -- prefix in --initial-advertise-peer-urls= as an option flag.
	lines, err := exec.OutputLines(node.Command(
		"grep", "-E", "--", `initial-advertise-peer-urls=https?://`, etcdManifestPath,
	))
	if err != nil {
		return "", errors.Wrapf(err, "grep initial-advertise-peer-urls in etcd manifest on %s", node.String())
	}
	// Each line is like: "    - --initial-advertise-peer-urls=https://172.19.0.6:2380"
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		// Strip YAML list marker and flag name.
		for _, prefix := range []string{"- --initial-advertise-peer-urls=", "--initial-advertise-peer-urls="} {
			if idx := strings.Index(ln, prefix); idx >= 0 {
				val := ln[idx+len(prefix):]
				// val = "https://172.19.0.6:2380"
				// Strip scheme.
				val = strings.TrimPrefix(val, "https://")
				val = strings.TrimPrefix(val, "http://")
				// Strip port.
				if colon := strings.LastIndex(val, ":"); colon >= 0 {
					val = val[:colon]
				}
				// IPv6 bracket cleanup: "::1" comes in as "[::1]" in URLs.
				val = strings.Trim(val, "[]")
				if net.ParseIP(val) != nil {
					return val, nil
				}
			}
		}
	}
	return "", errors.Errorf("extractEtcdManifestIP: initial-advertise-peer-urls not found or unparseable on %s; lines: %v", node.String(), lines)
}

// patchEtcdManifestIPs rewrites the etcd static-pod manifest on node,
// replacing every occurrence of each key in ipMap (stale IP) with the
// corresponding value (current IP). This updates --initial-cluster,
// --initial-advertise-peer-urls, --listen-peer-urls, --advertise-client-urls,
// --listen-client-urls, and the kubeadm annotation in one pass.
//
// Uses a two-pass sed approach to avoid double-substitution. Sequential sed
// expressions have a hazard: if IP A is replaced by IP B, and IP B is also a
// key in ipMap, a subsequent sed would incorrectly replace the just-written B.
// The two-pass approach replaces old IPs with unique non-IP markers in pass 1,
// then replaces markers with new IPs in pass 2, avoiding any collision.
//
// ipMap must be non-empty; it is the caller's responsibility to populate it
// with at least the local node's entry.
func patchEtcdManifestIPs(node nodes.Node, ipMap map[string]string) error {
	if len(ipMap) == 0 {
		return nil // no-op: no IP drift detected
	}

	// Build two sets of sed expressions:
	//   pass1: old_ip → __KINDER_IP_N__ (unique marker, can't collide with any IP)
	//   pass2: __KINDER_IP_N__ → new_ip
	type ipReplacement struct {
		oldIP  string
		newIP  string
		marker string
	}
	var replacements []ipReplacement
	idx := 0
	for oldIP, newIP := range ipMap {
		if oldIP == newIP {
			continue
		}
		marker := fmt.Sprintf("__KINDER_IP_%d__", idx)
		replacements = append(replacements, ipReplacement{oldIP, newIP, marker})
		idx++
	}
	if len(replacements) == 0 {
		return nil
	}

	for _, manifestPath := range []string{etcdManifestPath, kubeAPIServerManifestPath} {
		// Check if any of the old IPs exist in this manifest before running sed.
		hasAny := false
		for _, r := range replacements {
			checkLines, _ := exec.OutputLines(node.Command("grep", "-c", r.oldIP, manifestPath))
			if len(checkLines) > 0 && strings.TrimSpace(checkLines[0]) != "0" {
				hasAny = true
				break
			}
		}
		if !hasAny {
			continue // no old IPs in this manifest; skip
		}

		// Pass 1: replace all old IPs with unique markers.
		// Build a single sed command with multiple -e expressions for pass 1.
		pass1Args := []string{"-i"}
		for _, r := range replacements {
			pass1Args = append(pass1Args, "-e", "s|"+r.oldIP+"|"+r.marker+"|g")
		}
		pass1Args = append(pass1Args, manifestPath)
		if err := node.Command("sed", pass1Args...).Run(); err != nil {
			return errors.Wrapf(err, "sed pass1 patch %s on %s", manifestPath, node.String())
		}

		// Pass 2: replace markers with new IPs.
		pass2Args := []string{"-i"}
		for _, r := range replacements {
			pass2Args = append(pass2Args, "-e", "s|"+r.marker+"|"+r.newIP+"|g")
		}
		pass2Args = append(pass2Args, manifestPath)
		if err := node.Command("sed", pass2Args...).Run(); err != nil {
			return errors.Wrapf(err, "sed pass2 patch %s on %s", manifestPath, node.String())
		}
	}
	return nil
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
//
// extraIP is an optional additional IP to include in the kubeadm config (for
// dual-stack/IPv6 clusters where the etcd peer cert must include both IPv4 and
// IPv6 addresses in SANs). Pass "" to skip.
func renewOrRegenOneCert(node nodes.Node, certName, currentIP, extraIP string) error {
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
	// If extraIP is provided (dual-stack/IPv6 clusters), include it as an
	// additional SAN via etcd.local.peerCertSANs / serverCertSANs so the cert
	// is valid for both IPv4 and IPv6 peer/server connections.
	cfgPath := "/tmp/kinder-regen-" + certName + ".yaml"
	etcdSANsSection := "    dataDir: /var/lib/etcd\n"
	if extraIP != "" {
		// Include both IPs in etcd SAN lists for dual-stack cluster compatibility.
		// peerCertSANs: for etcd-peer cert (peer mTLS between nodes).
		// serverCertSANs: for etcd-server cert (client→etcd TLS).
		sanList := "    - " + currentIP + "\n    - " + extraIP + "\n"
		etcdSANsSection = "    dataDir: /var/lib/etcd\n" +
			"    peerCertSANs:\n" + sanList +
			"    serverCertSANs:\n" + sanList
	}
	cfgContent := "apiVersion: kubeadm.k8s.io/v1beta4\nkind: InitConfiguration\n" +
		"localAPIEndpoint:\n  advertiseAddress: " + currentIP + "\n  bindPort: 6443\n" +
		"nodeRegistration:\n  name: " + hostnameStr + "\n" +
		"---\n" +
		"apiVersion: kubeadm.k8s.io/v1beta4\nkind: ClusterConfiguration\n" +
		"etcd:\n  local:\n" + etcdSANsSection

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

// cycleManifestOne performs manifest mv-out + sleep + mv-in + active health
// poll for one cert type on one CP. Called only after ALL CPs have had their
// certs regenerated (see two-phase flow in RegenerateEtcdPeerCertsWholesale).
// Hard-fails on any sub-step error.
//
// Separation from cert regeneration is mandatory for HA clusters: etcd peer
// TLS requires ALL members to present valid mutual certs. If we cycle CP1's
// manifest before CP2/CP3 certs are regenerated, CP1's etcd starts with a
// new cert but CP2/CP3 still present old (possibly expired or wrong-SAN) certs,
// causing "remote error: tls: bad certificate" and "ID mismatch" on the Raft
// peer layer. The two-phase approach ensures all peer certs are valid before
// any etcd process is restarted.
func cycleManifestOne(node nodes.Node, c certCycle, etcdEndpoint, healthzURL string, logger log.Logger) error {
	// 1. mv-out (kubelet will notice within fileCheckFrequency and stop the pod)
	if err := node.Command("mv", c.manifestPath, c.backupPath).Run(); err != nil {
		return errors.Wrapf(err, "mv out %s failed on %s", c.manifestPath, node.String())
	}
	// 2. wait for kubelet to notice manifest gone — keep existing sleep
	//    (no kubelet API for "have you noticed"; see RESEARCH §3 Pitfall 3).
	certRegenSleeper(kubeletFileCheckFrequency + staticPodCycleSafetyMargin)
	// 3. mv-in
	if err := node.Command("mv", c.backupPath, c.manifestPath).Run(); err != nil {
		return errors.Wrapf(err, "mv back %s failed on %s", c.manifestPath, node.String())
	}
	// 4. active health gate (replaces the old 20s static staticPodRecreationWait)
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
	logger.V(1).Infof(" ✓ cycled manifest for %s on %s, %s pod healthy", c.certName, node.String(), c.cycleKind)
	return nil
}

// updateEtcdMemberPeerURLs updates the peer URLs for any member whose IP changed
// by calling `etcdctl member update <id> --peer-urls=https://<newIP>:2380` while
// etcd is still running (before the simultaneous restart).
//
// BACKGROUND: etcd stores peer membership (member IDs + peer URLs) in the Raft log
// and WAL. The --initial-cluster flag in the static-pod manifest is IGNORED when
// --initial-cluster-state=existing (used for all joined members). On restart, etcd
// reads peer URLs from the WAL. After Docker IPAM reassignment, the WAL contains
// pre-pause peer URLs — causing "dial tcp <stale-IP>:2380: connection refused"
// after restart even though --initial-cluster was patched in the manifest.
//
// The fix: before the simultaneous restart, update the cluster's membership record
// via the live etcd API. This writes the new peer URL into the Raft log, so when
// etcd restarts it finds the correct URLs in the WAL.
//
// This function uses direct IP comparison (WAL peer URL IP vs node's current IPs)
// rather than the ipMap approach, which handles both IPv4 and IPv6 clusters. For
// each etcd member, we match it to a node (by hostname in the member name), get
// the node's current IPs, and update the peer URL if stale. For IPv6 clusters,
// the WAL may store IPv6 peer URLs, so we check against both IPv4 and IPv6.
//
// This function is always called when IP drift is detected (ipMap non-empty).
func updateEtcdMemberPeerURLs(states []nodeRegenState, ipMap map[string]string, etcdEndpoint string, logger log.Logger) error {
	// Even if ipMap is empty (no IPv4 drift), we still check WAL peer URLs
	// for IPv6 drift on dual-stack/IPv6 clusters. The function uses direct
	// IP comparison (WAL vs current node IPs) rather than the ipMap.
	// Only skip if there are no current IPs to compare against.
	hasCurrentIPs := false
	for _, st := range states {
		if st.currentIP != "" || st.extraIP != "" {
			hasCurrentIPs = true
			break
		}
	}
	if !hasCurrentIPs {
		logger.V(1).Infof("phase 1.6: no node states with current IPs, skipping")
		return nil
	}

	// Step 1: Get member list from the first running CP. We pick states[0].node
	// because it ran Phase 1 cert regen first and should be available.
	firstNode := states[0].node
	etcdID, err := getEtcdContainerID(firstNode)
	if err != nil {
		return errors.Wrapf(err, "phase 1.6: could not get etcd container ID on %s", firstNode.String())
	}

	// Run etcdctl member list --write-out=json to get member IDs + peer URLs.
	args := []string{
		"crictl", "exec", etcdID, "etcdctl",
		"--cacert=/etc/kubernetes/pki/etcd/ca.crt",
		"--cert=/etc/kubernetes/pki/etcd/peer.crt",
		"--key=/etc/kubernetes/pki/etcd/peer.key",
		"--endpoints=" + etcdEndpoint,
		"member", "list", "--write-out=json",
	}
	out, listErr := exec.OutputLines(firstNode.Command(args[0], args[1:]...))
	if listErr != nil {
		return errors.Wrapf(listErr, "phase 1.6: etcdctl member list failed on %s", firstNode.String())
	}

	// Parse the member list JSON.
	members, parseErr := parseEtcdMemberList(strings.Join(out, ""))
	if parseErr != nil {
		return errors.Wrapf(parseErr, "phase 1.6: parse etcdctl member list output")
	}

	// For each member, match to its node state and check if the peer URL IP
	// matches that specific node's current IP. This per-member check is
	// required to handle IP rotation (Docker IPAM "musical chairs") where
	// after pause/resume, node A may receive node B's old IP and vice versa.
	//
	// The previous approach (checking against a global set of all current IPs)
	// fails in rotation: if cp2's new IP happens to be cp1's old peer URL IP,
	// the global check would incorrectly conclude cp1's peer URL is "current"
	// and skip updating it — leaving a stale URL that causes "Peer URLs already
	// exists" when cp2's update tries to claim that IP.
	//
	// Correct algorithm: match member → node, then check peerIP == node.currentIP.
	// Apply updates in release-before-claim order to handle IP rotation without
	// "Peer URLs already exists" conflicts:
	//   1. Collect all (member, newPeerURL) pairs that need updating.
	//   2. Compute "oldPeerIPs" — the set of current WAL IPs that will be released.
	//   3. Sort so that updates whose oldPeerIP is needed by another update's newPeerURL
	//      (i.e., A releases the IP that B wants to claim) come first.
	//
	// Example: cp1 WAL=172.19.0.4→172.19.0.3, cp2 WAL=172.19.0.6→172.19.0.4.
	// cp2 wants 172.19.0.4 which cp1 holds → cp1 must be updated first.

	// Phase A: collect pending updates.
	type pendingUpdate struct {
		member     etcdMember
		newPeerURL string
		oldPeerIP  string // IP being released by this update
	}
	var pending []pendingUpdate

	for _, member := range members {
		peerIP := extractIPFromURL(member.peerURL)
		if peerIP == "" {
			logger.Warnf("  [phase 1.6] could not parse IP from peer URL %q for member %s, skipping", member.peerURL, member.name)
			continue
		}
		var matchedState *nodeRegenState
		for j := range states {
			nodeName := states[j].node.String()
			if nodeName == member.name || strings.HasSuffix(nodeName, member.name) || strings.HasSuffix(member.name, nodeName) {
				matchedState = &states[j]
				break
			}
		}
		if matchedState == nil {
			logger.Warnf("  [phase 1.6] could not match etcd member %q to a node state, skipping", member.name)
			continue
		}
		parsedPeer := net.ParseIP(peerIP)
		isIPv6PeerURL := parsedPeer != nil && parsedPeer.To4() == nil
		var expectedIP string
		if isIPv6PeerURL && matchedState.extraIP != "" {
			expectedIP = matchedState.extraIP
		} else {
			expectedIP = matchedState.currentIP
		}
		if expectedIP == "" {
			logger.Warnf("  [phase 1.6] no expected IP for member %s (currentIP=%q extraIP=%q), skipping",
				member.name, matchedState.currentIP, matchedState.extraIP)
			continue
		}
		if peerIP == expectedIP {
			logger.V(1).Infof("  [phase 1.6] member %s peer URL %s is current (peerIP=%s matches node currentIP), no update needed",
				member.name, member.peerURL, peerIP)
			continue
		}
		newPeerURL := buildNewPeerURL(member.peerURL, peerIP, matchedState)
		if newPeerURL == "" {
			logger.Warnf("  [phase 1.6] could not build new peer URL for member %s (peerURL=%s), skipping", member.name, member.peerURL)
			continue
		}
		pending = append(pending, pendingUpdate{member: member, newPeerURL: newPeerURL, oldPeerIP: peerIP})
	}

	// Phase B: sort updates in release-before-claim order.
	// Build a map: claimed new IP → index in pending (who wants this IP).
	// For each update, check if its oldPeerIP is claimed by another update.
	// If yes, this update must come BEFORE the one claiming its old IP.
	// Simple stable sort: "releasers first" — an update that releases an IP
	// wanted by another update gets sorted to an earlier position.
	newIPSet := make(map[string]int) // newPeerIP → index in pending
	for i, pu := range pending {
		newIP := extractIPFromURL(pu.newPeerURL)
		if newIP != "" {
			newIPSet[newIP] = i
		}
	}
	// Sort: updates whose oldPeerIP is in newIPSet (i.e., someone else needs it) go first.
	sortedPending := make([]pendingUpdate, 0, len(pending))
	var deferred []pendingUpdate
	for _, pu := range pending {
		if _, wantedByOther := newIPSet[pu.oldPeerIP]; wantedByOther {
			// This update releases an IP that another update wants to claim.
			// It must run first (prepend to ensure releasers come first).
			sortedPending = append([]pendingUpdate{pu}, sortedPending...)
		} else {
			deferred = append(deferred, pu)
		}
	}
	sortedPending = append(sortedPending, deferred...)

	// Phase C: apply updates in sorted order.
	updated := 0
	for _, pu := range sortedPending {
		logger.V(0).Infof("  [phase 1.6] updating etcd member %s (%s) peer URL: %s → %s",
			pu.member.name, pu.member.id, pu.member.peerURL, pu.newPeerURL)
		updateArgs := []string{
			"crictl", "exec", etcdID, "etcdctl",
			"--cacert=/etc/kubernetes/pki/etcd/ca.crt",
			"--cert=/etc/kubernetes/pki/etcd/peer.crt",
			"--key=/etc/kubernetes/pki/etcd/peer.key",
			"--endpoints=" + etcdEndpoint,
			"member", "update", pu.member.id, "--peer-urls=" + pu.newPeerURL,
		}
		if err := firstNode.Command(updateArgs[0], updateArgs[1:]...).Run(); err != nil {
			return errors.Wrapf(err,
				"phase 1.6: etcdctl member update %s (peer URL %s→%s) failed",
				pu.member.id, pu.member.peerURL, pu.newPeerURL)
		}
		updated++
	}
	logger.V(0).Infof("  [phase 1.6] updated %d etcd member peer URLs", updated)
	return nil
}

// extractIPFromURL extracts the IP address from an etcd peer URL.
// Handles formats like:
//   - https://172.19.0.3:2380          → "172.19.0.3"
//   - https://[172.19.0.3]:2380        → "172.19.0.3" (IPv4 in brackets)
//   - https://[fc00:4d57:1afd::a]:2380 → "fc00:4d57:1afd::a"
func extractIPFromURL(rawURL string) string {
	// Strip scheme.
	u := rawURL
	for _, prefix := range []string{"https://", "http://"} {
		u = strings.TrimPrefix(u, prefix)
	}
	// Strip port suffix (":2380" etc).
	if colon := strings.LastIndex(u, ":"); colon >= 0 {
		// For IPv6, the last colon is the port separator after ']'.
		if strings.HasSuffix(u[:colon], "]") || !strings.Contains(u[:colon], ":") {
			u = u[:colon]
		}
	}
	// Strip IPv6 brackets.
	u = strings.Trim(u, "[]")
	if net.ParseIP(u) != nil {
		return u
	}
	return ""
}

// buildNewPeerURL constructs the new peer URL for a member given the old peer
// URL, the stale IP, and the matched node state.
func buildNewPeerURL(oldPeerURL, staleIP string, st *nodeRegenState) string {
	// Determine whether the original peer URL used IPv6 (bracketed non-IPv4 address).
	parsedStale := net.ParseIP(staleIP)
	useIPv6 := parsedStale != nil && parsedStale.To4() == nil
	var newIP string
	if useIPv6 && st.extraIP != "" {
		newIP = st.extraIP
	} else {
		newIP = st.currentIP
	}
	if newIP == "" {
		return ""
	}
	// For IPv6, wrap in brackets.
	if net.ParseIP(newIP) != nil && strings.Contains(newIP, ":") {
		newIP = "[" + newIP + "]"
	}
	// Replace the stale IP in the original URL.
	staleInURL := staleIP
	// If the original URL had the stale IP in brackets (IPv4 in brackets notation).
	if strings.Contains(oldPeerURL, "["+staleIP+"]") {
		return strings.Replace(oldPeerURL, "["+staleIP+"]", newIP, 1)
	}
	// Plain IP without brackets.
	return strings.Replace(oldPeerURL, staleInURL, newIP, 1)
}

// etcdMember holds a parsed etcd cluster member record.
type etcdMember struct {
	id      string // hex member ID, e.g. "6f1e1de96aea0868"
	name    string // container/node name
	peerURL string // e.g. "https://172.19.0.6:2380"
}

// parseEtcdMemberList parses `etcdctl member list --write-out=json` output.
// The JSON schema is: {"header":..., "members":[{"ID":uint64,"name":"...","peerURLs":["..."],...}]}
func parseEtcdMemberList(raw string) ([]etcdMember, error) {
	var doc struct {
		Members []struct {
			ID       uint64   `json:"ID"`
			Name     string   `json:"name"`
			PeerURLs []string `json:"peerURLs"`
		} `json:"members"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &doc); err != nil {
		return nil, errors.Errorf("parseEtcdMemberList: JSON unmarshal failed: %v (raw: %q)", err, raw)
	}
	out := make([]etcdMember, 0, len(doc.Members))
	for _, m := range doc.Members {
		// Format ID as hex (same representation etcdctl uses in update commands).
		id := fmt.Sprintf("%x", m.ID)
		peerURL := ""
		if len(m.PeerURLs) > 0 {
			peerURL = m.PeerURLs[0]
		}
		out = append(out, etcdMember{id: id, name: m.Name, peerURL: peerURL})
	}
	return out, nil
}

// getEtcdContainerID returns the crictl container ID of the running etcd container.
func getEtcdContainerID(node nodes.Node) (string, error) {
	lines, err := exec.OutputLines(node.Command("crictl", "ps", "--name", "etcd", "-q"))
	if err != nil {
		return "", errors.Wrapf(err, "crictl ps --name etcd on %s", node.String())
	}
	for _, ln := range lines {
		if id := strings.TrimSpace(ln); id != "" {
			return id, nil
		}
	}
	return "", errors.Errorf("no running etcd container on %s", node.String())
}

// cycleEtcdClusterSimultaneous performs a cluster-wide simultaneous etcd restart
// across all CP nodes. This is required to prevent etcd WAL stale peer URL failures.
//
// BACKGROUND: etcd --initial-cluster-state=existing reads peer membership from
// the WAL on every startup, NOT from the --initial-cluster flag. After a Docker
// IPAM reassignment, the WAL contains stale peer URLs (pre-pause container IPs).
// If etcd members are restarted sequentially, the first member to restart connects
// to WAL-stored stale URLs for its peers — which are down or have different IPs —
// causing "dial tcp <stale-IP>:2380: connection refused" and cluster degradation.
//
// SOLUTION: mv-out etcd.yaml from ALL CPs simultaneously, sleep once, then
// mv-in ALL simultaneously. All 3 etcd instances start at approximately the same
// time with the fresh cert files. When they all come up together, each member can
// immediately reach its peers (which have new IPs in the updated manifests from
// Phase 1.5), form quorum, and update its own WAL entry for its current peer URL.
//
// The health gate fires sequentially (one CP at a time) to avoid parallel health
// checker state corruption, starting with cpNodes[0].
func cycleEtcdClusterSimultaneous(cpNodes []nodes.Node, etcdEndpoint string, logger log.Logger) error {
	total := len(cpNodes)
	logger.V(0).Infof("  [phase 2 etcd] simultaneous etcd restart across %d CPs", total)

	// Step 1: mv-out etcd.yaml from ALL CPs.
	logger.V(1).Infof("    [etcd simultaneous] mv-out etcd.yaml from all %d CPs", total)
	for i, node := range cpNodes {
		if err := node.Command("mv", etcdManifestPath, etcdManifestBackup).Run(); err != nil {
			return errors.Wrapf(err, "etcd simultaneous mv-out failed on %s (%d/%d)", node.String(), i+1, total)
		}
	}

	// Step 2: sleep once — kubelet must notice the manifest is gone on ALL CPs.
	certRegenSleeper(kubeletFileCheckFrequency + staticPodCycleSafetyMargin)

	// Step 3: mv-in etcd.yaml to ALL CPs simultaneously.
	logger.V(1).Infof("    [etcd simultaneous] mv-in etcd.yaml to all %d CPs", total)
	for i, node := range cpNodes {
		if err := node.Command("mv", etcdManifestBackup, etcdManifestPath).Run(); err != nil {
			return errors.Wrapf(err, "etcd simultaneous mv-in failed on %s (%d/%d)", node.String(), i+1, total)
		}
	}

	// Step 4: health gate — wait for etcd on each CP (sequentially, starting with CP0).
	for i, node := range cpNodes {
		if err := etcdHealthChecker(node, etcdEndpoint); err != nil {
			return errors.Wrapf(err, "etcd ready-gate failed on %s (%d/%d) after simultaneous restart", node.String(), i+1, total)
		}
		logger.V(1).Infof("    [etcd simultaneous] etcd healthy on %s (%d/%d)", node.String(), i+1, total)
	}

	logger.V(0).Infof("  [phase 2 etcd] simultaneous etcd restart complete: all %d CPs healthy", total)
	return nil
}

// waitForEtcdContainerGone polls `crictl ps --name etcd -q` until no container
// ID is returned (etcd has stopped) or deadline expires.
func waitForEtcdContainerGone(node nodes.Node, deadline, tick time.Duration) error {
	end := time.Now().Add(deadline)
	for time.Now().Before(end) {
		lines, err := exec.OutputLines(node.Command("crictl", "ps", "--name", "etcd", "-q"))
		if err != nil {
			// crictl error may mean the container runtime is busy; treat as "not gone yet"
			certRegenSleeper(tick)
			continue
		}
		running := false
		for _, ln := range lines {
			if strings.TrimSpace(ln) != "" {
				running = true
				break
			}
		}
		if !running {
			return nil
		}
		certRegenSleeper(tick)
	}
	return errors.Errorf("etcd container on %s still running after %v", node.String(), deadline)
}

// peerURLForState builds the etcd peer URL for a node state.
// For IPv6/dual-stack clusters where currentIP is an IPv6 address, the URL
// uses the bracketed IPv6 form: https://[<ip>]:2380.
// For IPv4 clusters it uses: https://<ip>:2380.
func peerURLForState(st *nodeRegenState) string {
	ip := st.currentIP
	if ip == "" {
		return ""
	}
	// Wrap IPv6 in brackets.
	if strings.Contains(ip, ":") {
		ip = "[" + ip + "]"
	}
	return "https://" + ip + ":2380"
}

// addForceNewClusterFlag injects `    - --force-new-cluster` into the etcd
// static-pod manifest on node, immediately after the `    - etcd` command line.
// Uses sed append-after to avoid double-injection and to survive manifest
// reformatting (the YAML indentation is always 4 spaces in kubeadm output).
//
// manifestPath is the current on-node path to the manifest file. When called
// from forceNewClusterBootstrap step 3, the manifest has been mv-out to
// etcdManifestBackup (/tmp/etcd-bak.yaml) — callers must pass the correct path.
func addForceNewClusterFlag(node nodes.Node, manifestPath string) error {
	// Idempotency guard: skip if already present.
	// Use `grep -c -- pattern file` with `--` to prevent grep from treating
	// --force-new-cluster as a flag. The `-F` flag treats the pattern as a
	// fixed string (not a regex), which is correct here.
	lines, _ := exec.OutputLines(node.Command("grep", "-cF", "--", "--force-new-cluster", manifestPath))
	if len(lines) > 0 && strings.TrimSpace(lines[0]) != "0" {
		return nil // already present
	}
	// Insert after the `    - etcd` line (the container command entrypoint).
	// The sed `a` command appends a line after the matched line.
	if err := node.Command("sed", "-i",
		`/^    - etcd$/a\    - --force-new-cluster`,
		manifestPath,
	).Run(); err != nil {
		return errors.Wrapf(err, "sed inject --force-new-cluster into %s on %s", manifestPath, node.String())
	}
	return nil
}

// removeForceNewClusterFlag deletes the `--force-new-cluster` line from the
// etcd static-pod manifest on node using sed -i delete.
func removeForceNewClusterFlag(node nodes.Node) error {
	if err := node.Command("sed", "-i", "/--force-new-cluster/d", etcdManifestPath).Run(); err != nil {
		return errors.Wrapf(err, "sed remove --force-new-cluster from %s on %s", etcdManifestPath, node.String())
	}
	return nil
}

// forceNewClusterBootstrap recovers an etcd cluster where ALL members have
// stale WAL peer URLs (the "all-isolated" scenario on macOS Docker Desktop,
// where Docker IPAM assigns new IPs to ALL containers on every pause/resume).
//
// BACKGROUND: Normally, Phase 1.5 waits for the bootstrap CP's etcd to reach
// quorum, then Phase 1.6 uses `etcdctl member update` to fix stale WAL peer
// URLs. But `etcdctl member update` requires etcd quorum — which is impossible
// when ALL 3 etcd nodes are isolated (each has the other members' stale IPs in
// its WAL). This is a chicken-and-egg deadlock.
//
// SOLUTION: Use etcd's --force-new-cluster flag to bootstrap a single-member
// cluster on cp1, bypassing the quorum requirement. Then re-add cp2/cp3 as
// members via etcdctl member add (which only needs cp1's quorum, i.e. 1/1).
//
// PROCEDURE:
//  1. Patch ALL etcd+apiserver manifests with corrected IPs (ipMap).
//     This must happen before stopping etcd so that when etcd restarts it
//     reads the correct --initial-cluster IPs from the updated manifest.
//  2. Stop ALL etcd: mv-out etcd.yaml from all CPs, wait for containers to stop.
//  3. Add --force-new-cluster to cp1's manifest.
//  4. Start cp1: mv-in cp1's manifest (--force-new-cluster + corrected IPs).
//     cp1 bootstraps as a 1-member cluster from its existing WAL snapshot.
//  5. Wait for cp1 healthy.
//  6. Register cp2 and cp3 as new etcd members via `etcdctl member add`.
//  7. Start cp2/cp3: mv-in their manifests (--initial-cluster-state=existing).
//     They join the cluster using the new peer URLs registered in step 6.
//  8. Wait for cp2 and cp3 healthy.
//  9. Remove --force-new-cluster from cp1's manifest.
// 10. Wait for cp1 to be healthy again after kubelet-triggered restart.
//
// After this function returns, etcd is healthy on all CPs with correct WAL
// peer URLs and manifest IPs. The caller must still run Phase 2b (apiserver).
func forceNewClusterBootstrap(states []nodeRegenState, ipMap map[string]string, etcdEndpoint string, logger log.Logger) error {
	total := len(states)
	logger.V(0).Infof("  [force-new-cluster] starting recovery bootstrap for %d CP nodes", total)

	// Step 1: Patch ALL manifests with corrected IPs.
	// Must happen before step 2 (stop etcd) so that when etcd restarts it reads
	// correct --initial-cluster values from the manifest.
	if len(ipMap) > 0 {
		logger.V(0).Infof("  [force-new-cluster] step 1: patching etcd/apiserver manifests with %d IP remappings", len(ipMap))
		for _, st := range states {
			logger.V(1).Infof("    patching manifests on %s", st.node.String())
			if err := patchEtcdManifestIPs(st.node, ipMap); err != nil {
				return errors.Wrapf(err, "force-new-cluster: manifest patch failed on %s", st.node.String())
			}
		}
		logger.V(0).Infof("  [force-new-cluster] step 1 complete: manifests patched")
	} else {
		logger.V(1).Infof("  [force-new-cluster] step 1: no IP drift, manifests unchanged")
	}

	// Step 2: Stop ALL etcd instances.
	//
	// APPROACH: mv-out etcd.yaml (prevents kubelet from restarting the pod), then
	// explicitly run `crictl stop <id>` to stop the etcd container immediately.
	//
	// WHY NOT rely on kubelet static pod file watching alone:
	// On macOS Docker Desktop with kindest/node v1.35.x, kubelet's static pod
	// file watcher does NOT stop the etcd container when the manifest is removed
	// while kube-apiserver is down/in CrashLoopBackOff. Kubelet enters a degraded
	// state where it only processes status_manager updates (trying to contact the
	// API server) and does not run static pod reconciliation. Waiting 120s produces
	// "etcd container still running after 2m0s" — see UAT run 5 forensics.
	//
	// SOLUTION: After mv-out (to prevent kubelet from restarting), explicitly stop
	// the container via `crictl stop <id>`. This bypasses kubelet entirely and
	// directly instructs the container runtime. crictl stop sends SIGTERM and waits
	// up to --timeout seconds (default 10s) before SIGKILL.
	logger.V(0).Infof("  [force-new-cluster] step 2: stopping all etcd instances (mv-out + crictl stop)")

	// Sub-step 2a: Get etcd container IDs before mv-out (while containers are running).
	etcdContainerIDs := make([]string, total)
	for i, st := range states {
		id, err := getEtcdContainerID(st.node)
		if err != nil {
			// Container may already be stopped — that's fine, record empty ID.
			logger.V(1).Infof("    [step 2a] no etcd container on %s (already stopped?): %v", st.node.String(), err)
			etcdContainerIDs[i] = ""
		} else {
			etcdContainerIDs[i] = id
			logger.V(1).Infof("    [step 2a] etcd container on %s: %s", st.node.String(), id)
		}
	}

	// Sub-step 2b: mv-out etcd.yaml from all CPs (prevents kubelet restart).
	for i, st := range states {
		logger.V(1).Infof("    [step 2b] mv-out etcd.yaml on %s (%d/%d)", st.node.String(), i+1, total)
		if err := st.node.Command("mv", etcdManifestPath, etcdManifestBackup).Run(); err != nil {
			return errors.Wrapf(err, "force-new-cluster: mv-out etcd.yaml on %s", st.node.String())
		}
	}

	// Sub-step 2c: Explicitly stop each etcd container via crictl stop.
	// crictl stop <id> sends SIGTERM; after --timeout (default 10s) sends SIGKILL.
	// This is immediate and does not depend on kubelet's static pod reconciliation.
	for i, st := range states {
		id := etcdContainerIDs[i]
		if id == "" {
			logger.V(1).Infof("    [step 2c] no container ID for %s, skipping crictl stop", st.node.String())
			continue
		}
		logger.V(1).Infof("    [step 2c] crictl stop %s on %s (%d/%d)", id, st.node.String(), i+1, total)
		if err := st.node.Command("crictl", "stop", id).Run(); err != nil {
			// Non-fatal: container may have already stopped between get and stop.
			logger.V(1).Infof("    [step 2c] crictl stop %s on %s returned error (may be already stopped): %v", id, st.node.String(), err)
		}
	}

	// Sub-step 2d: Confirm all etcd containers are gone (short deadline — crictl stop
	// is synchronous so containers should be gone immediately, this is a safety check).
	confirmDeadline := 15 * time.Second
	logger.V(1).Infof("  [force-new-cluster] step 2d: confirming etcd containers stopped (deadline %v)", confirmDeadline)
	for _, st := range states {
		if err := waitForEtcdContainerGone(st.node, confirmDeadline, healthGateTick); err != nil {
			return errors.Wrapf(err, "force-new-cluster: etcd container on %s did not stop within %v", st.node.String(), confirmDeadline)
		}
	}
	logger.V(0).Infof("  [force-new-cluster] step 2 complete: all etcd containers stopped")

	// Step 3: Add --force-new-cluster to cp1's manifest.
	// IMPORTANT: The manifest was mv-out to etcdManifestBackup in step 2b.
	// We inject the flag into the backup file so that when step 4 mv-in restores
	// it to etcdManifestPath, the flag is present from first start.
	logger.V(0).Infof("  [force-new-cluster] step 3: injecting --force-new-cluster into %s manifest (at backup path)", states[0].node.String())
	if err := addForceNewClusterFlag(states[0].node, etcdManifestBackup); err != nil {
		return err
	}

	// Step 4: Start cp1: mv-in cp1's manifest.
	logger.V(0).Infof("  [force-new-cluster] step 4: starting cp1 (%s) with --force-new-cluster", states[0].node.String())
	if err := states[0].node.Command("mv", etcdManifestBackup, etcdManifestPath).Run(); err != nil {
		return errors.Wrapf(err, "force-new-cluster: mv-in etcd.yaml on %s", states[0].node.String())
	}

	// Step 5: Wait for cp1 healthy (1/1 quorum as single-member cluster).
	logger.V(0).Infof("  [force-new-cluster] step 5: waiting for cp1 etcd to be healthy")
	if err := etcdHealthChecker(states[0].node, etcdEndpoint); err != nil {
		return errors.Wrapf(err, "force-new-cluster: cp1 (%s) etcd not healthy after --force-new-cluster start", states[0].node.String())
	}
	logger.V(0).Infof("  [force-new-cluster] step 5 complete: cp1 etcd healthy (single-member)")

	// Step 6: Register cp2 and cp3 as new etcd members.
	// etcdctl member add registers the member and writes the new peer URL into
	// the Raft log. cp2/cp3 will use --initial-cluster-state=existing when they
	// start (already set in their manifests by kubeadm), so they expect to find
	// themselves already registered in the cluster — which we do here.
	cp1EtcdID, err := getEtcdContainerID(states[0].node)
	if err != nil {
		return errors.Wrapf(err, "force-new-cluster: get cp1 etcd container ID on %s", states[0].node.String())
	}
	for i := 1; i < total; i++ {
		st := states[i]
		peerURL := peerURLForState(&st)
		if peerURL == "" {
			return errors.Errorf("force-new-cluster: could not build peer URL for %s (currentIP=%q)", st.node.String(), st.currentIP)
		}
		memberName := st.node.String()
		logger.V(0).Infof("  [force-new-cluster] step 6: adding member %s with peer URL %s", memberName, peerURL)
		addArgs := []string{
			"crictl", "exec", cp1EtcdID, "etcdctl",
			"--cacert=/etc/kubernetes/pki/etcd/ca.crt",
			"--cert=/etc/kubernetes/pki/etcd/peer.crt",
			"--key=/etc/kubernetes/pki/etcd/peer.key",
			"--endpoints=" + etcdEndpoint,
			"member", "add", memberName, "--peer-urls=" + peerURL,
		}
		if err := states[0].node.Command(addArgs[0], addArgs[1:]...).Run(); err != nil {
			return errors.Wrapf(err, "force-new-cluster: etcdctl member add %s failed", memberName)
		}
		logger.V(1).Infof("    [force-new-cluster] member %s registered", memberName)
	}
	logger.V(0).Infof("  [force-new-cluster] step 6 complete: %d member(s) registered", total-1)

	// Step 7: Clear cp2/cp3 etcd data directories.
	//
	// CRITICAL: cp2/cp3 cannot restart with their existing WAL data when joining
	// the force-new-cluster bootstrap. Their WAL contains the old 3-member cluster
	// configuration with stale peer URLs. When etcd starts with
	// --initial-cluster-state=existing and an existing data directory, it reads
	// peer membership from the WAL — ignoring the --initial-cluster manifest flag.
	// This would cause cp2/cp3 to try to connect to their old (stale) peer URLs,
	// re-isolating them.
	//
	// Solution: clear the WAL + snapshot data for cp2/cp3 (/var/lib/etcd/member)
	// before starting them. With no WAL, etcd treats this as a fresh join of an
	// existing cluster: it reads --initial-cluster from the manifest to find
	// cp1's new peer URL, connects, and syncs the full cluster state via Raft
	// replication. ALL data is preserved — cp1's data dir (preserved by
	// --force-new-cluster) is Raft-replicated to the newly-joined members.
	//
	// Only the WAL+snapshot subdirectory (/var/lib/etcd/member) is removed.
	// The bind-mount point (/var/lib/etcd) itself is kept as an empty directory.
	logger.V(0).Infof("  [force-new-cluster] step 7: clearing etcd WAL for remaining CPs (member join path)")
	for i := 1; i < total; i++ {
		st := states[i]
		logger.V(1).Infof("    clearing /var/lib/etcd/member on %s (%d/%d)", st.node.String(), i, total-1)
		if err := st.node.Command("rm", "-rf", "/var/lib/etcd/member").Run(); err != nil {
			return errors.Wrapf(err, "force-new-cluster: rm -rf /var/lib/etcd/member on %s", st.node.String())
		}
	}
	logger.V(0).Infof("  [force-new-cluster] step 7 complete: WAL cleared for %d CPs", total-1)

	// Step 8: Start cp2/cp3 — mv-in their manifests.
	// With empty data dirs and --initial-cluster-state=existing, they connect
	// to cp1 at the new peer URL (from the manifest --initial-cluster patched in
	// step 1) and sync the full Raft log from cp1.
	logger.V(0).Infof("  [force-new-cluster] step 8: starting remaining CPs (%d)", total-1)
	for i := 1; i < total; i++ {
		st := states[i]
		logger.V(1).Infof("    mv-in etcd.yaml on %s (%d/%d)", st.node.String(), i, total-1)
		if err := st.node.Command("mv", etcdManifestBackup, etcdManifestPath).Run(); err != nil {
			return errors.Wrapf(err, "force-new-cluster: mv-in etcd.yaml on %s", st.node.String())
		}
	}

	// Step 9: Wait for cp2 and cp3 healthy.
	// cp2/cp3 need to download and replay the full Raft log from cp1. This may
	// take longer than a regular etcd restart — use the full etcdHealthGateDeadline.
	logger.V(0).Infof("  [force-new-cluster] step 9: waiting for remaining CPs to become healthy")
	for i := 1; i < total; i++ {
		if err := etcdHealthChecker(states[i].node, etcdEndpoint); err != nil {
			return errors.Wrapf(err, "force-new-cluster: etcd health gate failed on %s after member join", states[i].node.String())
		}
		logger.V(1).Infof("    [force-new-cluster] %s etcd healthy", states[i].node.String())
	}
	logger.V(0).Infof("  [force-new-cluster] step 9 complete: all %d CPs healthy", total)

	// Step 10: Remove --force-new-cluster from cp1's manifest.
	// kubelet will detect the manifest change and restart cp1's etcd. At this
	// point the WAL has the correct 3-member configuration (from the member add
	// commands in step 6), so the restart is clean with --initial-cluster-state.
	// cp1's data dir was never cleared so all cluster data is preserved.
	logger.V(0).Infof("  [force-new-cluster] step 10: removing --force-new-cluster from %s manifest", states[0].node.String())
	if err := removeForceNewClusterFlag(states[0].node); err != nil {
		return err
	}

	// Step 11: Wait for cp1 healthy after kubelet-triggered restart.
	logger.V(0).Infof("  [force-new-cluster] step 11: waiting for cp1 (%s) to re-join after --force-new-cluster removal", states[0].node.String())
	// Give kubelet time to notice the manifest change and restart the pod.
	certRegenSleeper(kubeletFileCheckFrequency + staticPodCycleSafetyMargin)
	if err := etcdHealthChecker(states[0].node, etcdEndpoint); err != nil {
		return errors.Wrapf(err, "force-new-cluster: cp1 (%s) etcd not healthy after --force-new-cluster removal", states[0].node.String())
	}
	logger.V(0).Infof("  [force-new-cluster] step 11 complete: cp1 healthy, all %d etcd members healthy", total)
	logger.V(0).Infof("  [force-new-cluster] bootstrap recovery complete")
	return nil
}

// nodeRegenState holds per-CP data collected during Phase 1 so Phase 2 can
// run without re-querying the nodes.
type nodeRegenState struct {
	node        nodes.Node
	preSnap     certExpirationSnapshot // may be nil if snapshot capture failed
	currentIP   string                 // current IPv4 address (from ip -4 addr show eth0)
	oldIP       string                 // IPv4 read from etcd manifest before cert regen
	extraIP     string                 // current IPv6 address (for dual-stack/IPv6 clusters; may be empty)
	oldExtraIP  string                 // IPv6 read from etcd manifest before cert regen (may be empty)
}

// RegenerateEtcdPeerCertsWholesale runs cert regeneration in two phases across
// all CP nodes, then cycles each static-pod manifest. All CPs must be started
// before this call. Failure on any CP halts the operation and returns a
// structured diagnostic error directing the user to delete and recreate the
// cluster.
//
// TWO-PHASE DESIGN (HA correctness requirement):
//
// etcd uses mutual TLS for peer communication. All members must present a valid
// peer cert to every other member. If cert regeneration and manifest cycling are
// interleaved per-CP (e.g. regen+cycle CP1, then regen+cycle CP2), etcd on CP1
// restarts with its new cert before CP2/CP3 have their certs regenerated. CP1's
// new etcd then attempts to connect to CP2/CP3 which still present old certs —
// causing "remote error: tls: bad certificate" and Raft "ID mismatch" errors.
// The etcd health gate then times out and the entire resume fails.
//
// The correct sequence is:
//   Phase 1: Regenerate cert files on ALL CPs (no pod restart, manifests intact).
//   Phase 2: Cycle each manifest (mv-out + mv-in) one CP at a time, waiting for
//            the health gate between each, so all certs are valid before any
//            etcd process restarts.
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
	states := make([]nodeRegenState, total)

	// ── PHASE 1: Regenerate cert FILES on ALL CPs ──────────────────────────
	// No manifest cycling yet. All peer certs must be valid before any etcd
	// process restarts (see two-phase design comment above).
	logger.V(0).Infof("cert regen phase 1: regenerating cert files on all %d CP nodes", total)
	for i, node := range cpNodes {
		logger.V(0).Infof("  [phase 1] regenerating certs on %s (%d/%d)", node.String(), i+1, total)

		// Extract the IP recorded in the etcd manifest BEFORE cert regen. This
		// is the node's pre-pause IP. We'll use it to build a cluster-wide
		// oldIP→currentIP map so etcd manifests can be patched with correct IPs.
		//
		// For IPv6/dual-stack clusters the manifest IP is IPv6 (e.g. fc00::4),
		// NOT IPv4. The manifest patching in Phase 1.5 must replace IPv6→IPv6,
		// not IPv6→IPv4. See deviation note: "cross-family ipMap" bug.
		oldIP, oldIPErr := extractEtcdManifestIP(node)
		if oldIPErr != nil {
			logger.Warnf("cert regen: could not read manifest IP on %s: %v (continuing; manifest patch will be skipped for this node)", node.String(), oldIPErr)
			oldIP = "" // empty → will not be added to ipMap
		}

		// Always detect the IPv4 address — needed for kubeadm cert generation
		// (localAPIEndpoint.advertiseAddress must be a reachable IP, and IPv4
		// is always present even on IPv6-only kind clusters).
		currentIPv4, ipErr := currentNodeIPv4(node)
		if ipErr != nil {
			dumpCertRegenDiagnostics(node, "ip-detect", logger)
			return errors.Wrapf(ipErr,
				"cert regen: failed to detect current IPv4 on %s. Cluster state is undefined — delete and recreate the cluster",
				node.String())
		}

		// For IPv6/dual-stack clusters, also detect the IPv6 global address.
		// It is used for two purposes:
		//  1. As the kubeadm advertiseAddress for cert generation (so the cert's
		//     primary SAN matches the etcd manifest's advertise IP).
		//  2. As the manifest-patch target IP for Phase 1.5 (since IPv6 cluster
		//     manifests use IPv6 addresses in peer/client URLs, not IPv4).
		// When ipv6==true, currentIP used for cert gen and manifest patching is
		// the IPv6 address; IPv4 is included as an extra SAN (extraIP) so that
		// etcd can still be reached via IPv4 loopback (127.0.0.1).
		var currentIP, extraIP string
		if ipv6 {
			if ip6, ip6Err := currentNodeIPv6(node); ip6Err == nil {
				currentIP = ip6
				extraIP = currentIPv4
				logger.V(1).Infof("    IPv6 cluster: manifest/cert IP=%s extraSAN=%s on %s", currentIP, extraIP, node.String())
			} else {
				// Fallback: could not detect IPv6; use IPv4 and log a warning.
				logger.Warnf("    cert regen: could not detect IPv6 address on %s: %v (falling back to IPv4 for cert gen; peer certs may lack IPv6 SAN)", node.String(), ip6Err)
				currentIP = currentIPv4
			}
		} else {
			// IPv4-only cluster: use IPv4 for both cert gen and manifest patching.
			currentIP = currentIPv4
		}
		logger.V(1).Infof("    current IP on %s: %s (old manifest IP: %s, extraIP: %s)", node.String(), currentIP, oldIP, extraIP)

		// Snapshot pre-renew check-expiration for the post-pass verify.
		preSnap, preErr := captureCertExpirationSnapshot(node)
		if preErr != nil {
			logger.Warnf("pre-renew check-expiration snapshot failed on %s: %v (continuing; post-pass verify will be best-effort)", node.String(), preErr)
		}

		for _, ct := range certTypes {
			if _, ok := cycleForCertType[ct]; !ok {
				dumpCertRegenDiagnostics(node, ct, logger)
				return errors.Errorf("unknown cert type %q (cluster state is undefined — delete and recreate the cluster)", ct)
			}
			if err := renewOrRegenOneCert(node, ct, currentIP, extraIP); err != nil {
				dumpCertRegenDiagnostics(node, ct, logger)
				return errors.Wrapf(err,
					"cert regen phase 1 failed on %s for %s. Cluster state is undefined — delete and recreate the cluster",
					node.String(), ct)
			}
		}

		states[i] = nodeRegenState{node: node, preSnap: preSnap, currentIP: currentIP, oldIP: oldIP, extraIP: extraIP, oldExtraIP: ""}
		logger.V(1).Infof("    [phase 1] cert files regenerated on %s", node.String())
	}
	logger.V(0).Infof("cert regen phase 1 complete: all %d CP nodes have fresh cert files", total)

	// Build ipMap for Phase 1.7 (manifest patch) and forceNewClusterBootstrap.
	// Must be computed before Phase 1.5 so it is available for both code paths.
	ipMap := make(map[string]string)
	for _, st := range states {
		if st.oldIP != "" && st.oldIP != st.currentIP {
			ipMap[st.oldIP] = st.currentIP
			logger.V(1).Infof("  IP drift map: %s → %s (on %s)", st.oldIP, st.currentIP, st.node.String())
		}
	}

	// ── PHASE 1.5: Wait for etcd quorum on the bootstrap CP ─────────────────
	// ORDERING NOTE: Phase 1.5 (WAL peer URL update via live etcd API) MUST run
	// before Phase 1.7 (manifest IP patch). Reason:
	//   - After Phase 1, etcd is running (it started from the original manifest,
	//     which may have stale peer IPs but etcd adapts and binds to eth0).
	//   - Phase 1.7 (manifest patch) triggers a kubelet-managed etcd restart.
	//     We must call etcdctl BEFORE the manifest patch while the original etcd
	//     instance is still running.
	//
	// IMPORTANT: We wait only on states[0].node (the bootstrap CP that ran Phase
	// 1 cert regen first). This is the same node used for `etcdctl member list`
	// and `member update` in Phase 1.6. Waiting for ALL CPs fails in the IP-drift
	// scenario: after IPAM reassignment, isolated CPs (those whose old peer URL is
	// stale in the WAL) cannot form quorum and will never become healthy on their
	// own local endpoint — they need Phase 1.6's WAL update first.
	//
	// FALLBACK: On macOS with Docker Desktop, every pause/resume randomises all
	// container MAC addresses → Docker DHCP assigns new IPs to ALL containers.
	// This means ALL 3 etcd nodes always have stale WAL peer URLs — even the
	// bootstrap CP cannot form quorum. In this "all-isolated" scenario,
	// etcdHealthChecker will time out. We detect this and fall back to
	// forceNewClusterBootstrap, which uses etcd's --force-new-cluster flag to
	// bootstrap a single-member cluster on cp1, then re-adds cp2/cp3.
	forceBootstrapped := false
	logger.V(0).Infof("cert regen phase 1.5: waiting for etcd quorum on bootstrap CP (%s)", states[0].node.String())
	if healthErr := etcdHealthChecker(states[0].node, etcdEndpoint); healthErr != nil {
		// Phase 1.5 health wait failed — attempt the force-new-cluster fallback
		// to recover the all-isolated scenario.
		logger.V(0).Infof("cert regen phase 1.5: etcd quorum wait failed on %s, attempting force-new-cluster bootstrap: %v",
			states[0].node.String(), healthErr)
		if fErr := forceNewClusterBootstrap(states, ipMap, etcdEndpoint, logger); fErr != nil {
			dumpCertRegenDiagnostics(states[0].node, "force-new-cluster", logger)
			return errors.Wrapf(fErr,
				"cert regen phase 1.5 (force-new-cluster bootstrap) failed on %s. Cluster state is undefined — delete and recreate the cluster",
				states[0].node.String())
		}
		forceBootstrapped = true
		logger.V(0).Infof("cert regen phase 1.5 complete: force-new-cluster bootstrap succeeded, etcd cluster reformed")
	} else {
		logger.V(0).Infof("cert regen phase 1.5 complete: etcd quorum confirmed on %s", states[0].node.String())
	}

	// ── PHASE 1.6: Update etcd WAL peer URLs via live cluster API ───────────
	// With etcd running, update the cluster membership record for any member
	// whose WAL peer URL is stale. This writes the new peer URLs into the Raft
	// log so they are present in the WAL when etcd restarts in Phase 2.
	//
	// Without this step, the simultaneous restart in Phase 2a would fail:
	// etcd reads peer URLs from the WAL on startup (--initial-cluster-state=existing),
	// ignoring the --initial-cluster manifest flag.
	//
	// Phase 1.6 runs unconditionally because IPv6/dual-stack clusters may have
	// stale IPv6 peer URLs in the WAL even when the IPv4 address mapping shows
	// no drift. The function compares WAL peer URL IPs directly against node
	// current IPs, handling all IP family cases.
	//
	// SKIP if forceNewClusterBootstrap already reformed the cluster: that path
	// re-registers cp2/cp3 via etcdctl member add with correct peer URLs, so
	// there are no stale WAL entries left to fix.
	if !forceBootstrapped {
		logger.V(0).Infof("cert regen phase 1.6: verifying and updating etcd member peer URLs")
		if err := updateEtcdMemberPeerURLs(states, ipMap, etcdEndpoint, logger); err != nil {
			dumpCertRegenDiagnostics(states[0].node, "member-update", logger)
			return errors.Wrapf(err,
				"cert regen phase 1.6 (etcd member peer URL update) failed. Cluster state is undefined — delete and recreate the cluster")
		}
		logger.V(0).Infof("cert regen phase 1.6 complete: etcd member peer URLs verified")
	} else {
		logger.V(1).Infof("cert regen phase 1.6: skipped (force-new-cluster bootstrap handled WAL peer URL registration)")
	}

	// ── PHASE 1.7: Patch etcd (and kube-apiserver) manifests with current IPs ─
	// Now that WAL peer URLs are updated (Phase 1.6), patch the static-pod
	// manifests with current container IPs. This triggers a kubelet-managed
	// etcd restart (old etcd is killed, new one started with correct IPs).
	// The manifest patch must happen AFTER Phase 1.6 so that the live etcd
	// instance (running from the old manifest) processes the WAL update before
	// it is restarted by kubelet.
	//
	// This must happen BEFORE Phase 2 (manifest cycling) — cycling the manifest
	// again before Phase 2 would be a no-op but the IPs must be correct for
	// etcd to form quorum when Phase 2a restarts it simultaneously.
	//
	// SKIP if forceNewClusterBootstrap already patched manifests: that path
	// patches manifests before starting cp1 with --force-new-cluster, and also
	// patches cp2/cp3 manifests before starting them as rejoining members.
	if !forceBootstrapped {
		if len(ipMap) > 0 {
			logger.V(0).Infof("cert regen phase 1.7: patching etcd/apiserver manifests with %d IP remappings", len(ipMap))
			for _, st := range states {
				logger.V(1).Infof("    patching manifests on %s", st.node.String())
				if err := patchEtcdManifestIPs(st.node, ipMap); err != nil {
					dumpCertRegenDiagnostics(st.node, "manifest-patch", logger)
					return errors.Wrapf(err,
						"cert regen phase 1.7 (manifest IP patch) failed on %s. Cluster state is undefined — delete and recreate the cluster",
						st.node.String())
				}
			}
			logger.V(0).Infof("cert regen phase 1.7 complete: manifests patched")
		} else {
			logger.V(1).Infof("cert regen phase 1.7: no IP drift detected, manifests unchanged")
		}
	} else {
		logger.V(1).Infof("cert regen phase 1.7: skipped (force-new-cluster bootstrap patched manifests)")
	}

	// ── PHASE 2: Cycle static-pod manifests ───────────────────────────────
	// All peer certs are now valid. Restart etcd cluster-wide simultaneously
	// (all CPs at once) so every etcd instance starts with fresh certs and can
	// form quorum without hitting WAL stale peer URL failures. Then cycle
	// kube-apiserver.yaml per CP sequentially.
	//
	// WHY SIMULTANEOUS for etcd:
	// etcd --initial-cluster-state=existing reads peer membership from the WAL
	// on every restart, NOT from the --initial-cluster manifest flag. After a
	// Docker IPAM reassignment, the WAL contains stale peer URLs (pre-pause IPs).
	// Sequential per-cert cycling (etcd-peer, etcd-server, etcd-healthcheck-client)
	// causes 3 separate etcd restarts. On the 2nd restart, etcd reads the WAL
	// with stale peer URLs → can't reach peers → health gate times out.
	// A cluster-wide simultaneous restart avoids this: all 3 etcd instances start
	// together, can find each other at the new IPs (from Phase 1.7 manifest patch),
	// form quorum, and each overwrites its own WAL peer URL entry.
	logger.V(0).Infof("cert regen phase 2: cycling manifests on all %d CP nodes", total)

	// ── PHASE 2a: Simultaneous etcd restart (all CPs at once) ─────────────
	// etcd-peer, etcd-server, etcd-healthcheck-client all share etcd.yaml.
	// We do a single mv-out-ALL / sleep / mv-in-ALL / health-gate cycle.
	//
	// SKIP if forceNewClusterBootstrap already performed the etcd restart:
	// that path stops all etcd, patches manifests, starts cp1 with --force-new-cluster,
	// adds cp2/cp3 as members, starts them, and removes --force-new-cluster.
	// The etcd cluster is already healthy on all CPs at this point.
	if !forceBootstrapped {
		logger.V(0).Infof("cert regen phase 2a: simultaneous etcd restart (all %d CPs)", total)
		cpNodesList := make([]nodes.Node, total)
		for i, st := range states {
			cpNodesList[i] = st.node
		}
		if err := cycleEtcdClusterSimultaneous(cpNodesList, etcdEndpoint, logger); err != nil {
			// Report the failure on the first CP for diagnostics.
			dumpCertRegenDiagnostics(states[0].node, "etcd-simultaneous", logger)
			return errors.Wrapf(err,
				"cert regen phase 2a (simultaneous etcd restart) failed. Cluster state is undefined — delete and recreate the cluster")
		}
		logger.V(0).Infof("cert regen phase 2a complete: etcd cluster restarted simultaneously")
	} else {
		logger.V(0).Infof("cert regen phase 2a: skipped (force-new-cluster bootstrap already restarted etcd cluster)")
	}

	// ── PHASE 2b: Per-CP kube-apiserver manifest cycle ────────────────────
	// apiserver-etcd-client uses kube-apiserver.yaml. Each apiserver connects
	// to its local etcd only, so sequential per-CP cycling is safe here.
	logger.V(0).Infof("cert regen phase 2b: cycling kube-apiserver manifests on all %d CPs", total)
	apiserverCyc := cycleForCertType["apiserver-etcd-client"]
	for i, st := range states {
		logger.V(1).Infof("    [phase 2b] cycling kube-apiserver on %s (%d/%d)", st.node.String(), i+1, total)
		if err := cycleManifestOne(st.node, apiserverCyc, etcdEndpoint, healthzURL, logger); err != nil {
			dumpCertRegenDiagnostics(st.node, "apiserver-etcd-client", logger)
			return errors.Wrapf(err,
				"cert regen phase 2b (apiserver manifest cycle) failed on %s. Cluster state is undefined — delete and recreate the cluster",
				st.node.String())
		}
	}
	logger.V(0).Infof("cert regen phase 2b complete: all %d kube-apiserver pods healthy", total)

	// ── POST-PASS VERIFICATION: kubeadm certs check-expiration per CP ─────
	for i, st := range states {
		if err := verifyCheckExpirationAdvanced(st.node, certTypes, st.preSnap); err != nil {
			dumpCertRegenDiagnostics(st.node, "post-pass-verify", logger)
			return errors.Wrapf(err,
				"post-pass cert-expiration check failed on %s. Cluster state is undefined — delete and recreate the cluster",
				st.node.String())
		}
		logger.V(1).Infof("    [post-pass] certs verified on %s (%d/%d)", st.node.String(), i+1, total)
	}
	logger.V(0).Infof("cert regen phase 2 complete: all %d CP nodes healthy", total)
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
