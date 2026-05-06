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

package snapshot

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"sigs.k8s.io/kind/pkg/cluster/nodes"
)

// ---------------------------------------------------------------------------
// Helpers for constructing a pvs outer-tar (Plan 02 nested layout)
// ---------------------------------------------------------------------------

// makePVsOuterTar builds an in-memory outer tar with one entry per node:
// "<nodeName>/local-path-provisioner.tar" → tarData bytes.
func makePVsOuterTar(entries map[string][]byte) []byte {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for nodeName, data := range entries {
		hdr := &tar.Header{
			Name:     nodeName + "/local-path-provisioner.tar",
			Mode:     0600,
			Size:     int64(len(data)),
			Typeflag: tar.TypeReg,
		}
		_ = tw.WriteHeader(hdr)
		_, _ = tw.Write(data)
	}
	_ = tw.Close()
	return buf.Bytes()
}

// writePVsFile writes pvsTarBytes to a temp file and returns the path.
func writePVsFile(t *testing.T, pvsTarBytes []byte) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "pvs-*.tar")
	if err != nil {
		t.Fatalf("create temp pvs tar: %v", err)
	}
	if _, err := f.Write(pvsTarBytes); err != nil {
		t.Fatalf("write pvs tar bytes: %v", err)
	}
	f.Close()
	return f.Name()
}

// ---------------------------------------------------------------------------
// TestRestorePVsEmptyFile
// ---------------------------------------------------------------------------

// TestRestorePVsEmptyFile verifies that RestorePVs on a 0-byte file returns
// nil without invoking any node commands.
func TestRestorePVsEmptyFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "empty-pvs-*.tar")
	if err != nil {
		t.Fatalf("create empty pvs file: %v", err)
	}
	f.Close()
	emptyPath := f.Name()

	worker := &snapFakeNode{
		name: "worker",
		role: "worker",
		lookup: func(name string, args []string) (string, error) {
			return "", fmt.Errorf("no commands should be issued on empty pvs.tar")
		},
	}

	allNodes := []nodes.Node{worker}
	if err := RestorePVs(context.Background(), allNodes, emptyPath); err != nil {
		t.Fatalf("RestorePVs on empty file returned error: %v", err)
	}
	if len(worker.snapshot()) > 0 {
		t.Errorf("expected no node commands on empty pvs.tar; got: %v", worker.snapshot())
	}
}

// ---------------------------------------------------------------------------
// TestRestorePVsSingleNodeData
// ---------------------------------------------------------------------------

// TestRestorePVsSingleNodeData verifies that when the outer tar has one entry
// "worker/local-path-provisioner.tar", the matching node receives a
// `tar -xf -` command with the inner tar bytes as stdin, targeted at /opt.
func TestRestorePVsSingleNodeData(t *testing.T) {
	innerTarData := []byte("inner-tar-bytes-for-worker")
	pvsTar := makePVsOuterTar(map[string][]byte{
		"worker": innerTarData,
	})
	pvsTarPath := writePVsFile(t, pvsTar)

	worker := &snapFakeNode{
		name: "worker",
		role: "worker",
		lookup: func(name string, args []string) (string, error) {
			return "", nil // all commands succeed
		},
	}

	allNodes := []nodes.Node{worker}
	if err := RestorePVs(context.Background(), allNodes, pvsTarPath); err != nil {
		t.Fatalf("RestorePVs returned error: %v", err)
	}

	calls := worker.snapshot()
	// Find the tar -xf - -C /opt call.
	var tarCall *snapCall
	for i := range calls {
		c := calls[i]
		if c.name == "tar" && containsStr(c.args, "-xf") && containsStr(c.args, "-") && containsStr(c.args, "/opt") {
			tarCall = &c
			break
		}
	}
	if tarCall == nil {
		t.Fatalf("expected 'tar -xf - -C /opt' call on worker node; calls: %v", calls)
	}
	// Verify stdin bytes == inner tar data.
	if !bytes.Equal(tarCall.stdin, innerTarData) {
		t.Errorf("tar stdin bytes differ from inner tar data:\n  want %q\n  got  %q",
			innerTarData, tarCall.stdin)
	}
}

// ---------------------------------------------------------------------------
// TestRestorePVsUnknownNodeIgnored
// ---------------------------------------------------------------------------

// TestRestorePVsUnknownNodeIgnored verifies that an outer tar entry whose
// leading path component does not match any node name is skipped without
// causing an error.
func TestRestorePVsUnknownNodeIgnored(t *testing.T) {
	pvsTar := makePVsOuterTar(map[string][]byte{
		"ghost": []byte("bytes-for-ghost-node"),
	})
	pvsTarPath := writePVsFile(t, pvsTar)

	realNode := &snapFakeNode{
		name: "worker",
		role: "worker",
		lookup: func(name string, args []string) (string, error) {
			return "", fmt.Errorf("unexpected command on worker: %s %v", name, args)
		},
	}

	allNodes := []nodes.Node{realNode}
	err := RestorePVs(context.Background(), allNodes, pvsTarPath)
	// Should succeed (unknown node is warn-and-skip, not an error).
	if err != nil {
		t.Fatalf("RestorePVs should succeed when entry references unknown node; got: %v", err)
	}
	// Real node should not have received any commands.
	if len(realNode.snapshot()) > 0 {
		t.Errorf("unexpected commands on realNode: %v", realNode.snapshot())
	}
}

// ---------------------------------------------------------------------------
// TestRestorePVsTarUntarFails
// ---------------------------------------------------------------------------

// TestRestorePVsTarUntarFails verifies that when the tar -xf command on a node
// returns an error, that error is included in the aggregated return value and
// references the node name.
func TestRestorePVsTarUntarFails(t *testing.T) {
	innerTarData := []byte("bad-tar-data")
	pvsTar := makePVsOuterTar(map[string][]byte{
		"worker": innerTarData,
	})
	pvsTarPath := writePVsFile(t, pvsTar)

	worker := &snapFakeNode{
		name: "worker",
		role: "worker",
		lookup: func(name string, args []string) (string, error) {
			if name == "tar" {
				return "", fmt.Errorf("simulated tar extraction failure")
			}
			return "", nil
		},
	}

	allNodes := []nodes.Node{worker}
	err := RestorePVs(context.Background(), allNodes, pvsTarPath)
	if err == nil {
		t.Fatal("RestorePVs should return error when tar extraction fails; got nil")
	}
	// Error must mention the node name or the underlying error.
	errStr := err.Error()
	if !strings.Contains(errStr, "worker") && !strings.Contains(errStr, "simulated tar") {
		t.Errorf("error should mention node name 'worker' or underlying error; got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// warnCaptureLogger is a simple logger that captures warning messages.
type warnCaptureLogger struct {
	warnings []string
}

func (l *warnCaptureLogger) warn(msg string) {
	l.warnings = append(l.warnings, msg)
}

func (l *warnCaptureLogger) warned() bool { return len(l.warnings) > 0 }

// writeReader adapts a byte slice into an io.Reader for SetStdin.
func writeReader(data []byte) io.Reader {
	return bytes.NewReader(data)
}
