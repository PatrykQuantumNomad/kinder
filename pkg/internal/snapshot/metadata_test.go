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
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestMetadataRoundTrip(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		input   *Metadata
		wantErr string // non-empty = expect error from UnmarshalMetadata
	}{
		{
			name: "full",
			input: &Metadata{
				SchemaVersion: MetadataVersion,
				Name:          "snap-20260506-120000",
				ClusterName:   "dev-cluster",
				CreatedAt:     now,
				K8sVersion:    "v1.31.2",
				NodeImage:     "kindest/node:v1.31.2",
				Topology: TopologyInfo{
					ControlPlaneCount: 3,
					WorkerCount:       2,
					HasLoadBalancer:   true,
				},
				AddonVersions: map[string]string{
					"metallb":    "v0.13.12",
					"ingress":    "v1.10.0",
					"local-path": "v0.0.28",
				},
				EtcdDigest:    "sha256:aabbccdd",
				ImagesDigest:  "sha256:11223344",
				PVsDigest:     "sha256:55667788",
				ConfigDigest:  "sha256:99aabbcc",
				ArchiveDigest: "sha256:ddeeff00",
			},
		},
		{
			name: "minimal",
			input: &Metadata{
				SchemaVersion: MetadataVersion,
				Name:          "snap-minimal",
				ClusterName:   "single-cp",
				CreatedAt:     now,
				K8sVersion:    "v1.30.0",
				NodeImage:     "kindest/node:v1.30.0",
				Topology: TopologyInfo{
					ControlPlaneCount: 1,
					WorkerCount:       0,
					HasLoadBalancer:   false,
				},
				// Empty addon map — no addons installed.
				AddonVersions: map[string]string{},
				EtcdDigest:    "sha256:aabb",
				ImagesDigest:  "sha256:ccdd",
				PVsDigest:     "", // no local-path PVs
				ConfigDigest:  "sha256:eeff",
				ArchiveDigest: "sha256:0011",
			},
		},
		{
			// Forward-compat alarm: if metadata.json arrives without schemaVersion
			// (e.g., corrupt write or future version omits it) UnmarshalMetadata
			// must return an error rather than silently accepting unknown schema.
			// This preserves our ability to detect and reject future schema changes.
			name:    "missing schemaVersion",
			wantErr: "missing schemaVersion",
			input: &Metadata{
				// SchemaVersion intentionally left empty
				Name:          "snap-bad",
				ClusterName:   "bad",
				CreatedAt:     now,
				K8sVersion:    "v1.31.2",
				NodeImage:     "kindest/node:v1.31.2",
				AddonVersions: map[string]string{},
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			data, err := MarshalMetadata(tc.input)
			if err != nil {
				t.Fatalf("MarshalMetadata: unexpected error: %v", err)
			}

			got, err := UnmarshalMetadata(data)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("UnmarshalMetadata: expected error containing %q, got nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("UnmarshalMetadata: expected error containing %q, got %q", tc.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("UnmarshalMetadata: unexpected error: %v", err)
			}

			// Deep equality: time.Time round-trips cleanly via RFC3339Nano via stdlib
			// json tags, so reflect.DeepEqual is safe here.
			if !reflect.DeepEqual(tc.input, got) {
				t.Errorf("round-trip mismatch:\n  input: %+v\n  got:   %+v", tc.input, got)
			}
		})
	}
}
