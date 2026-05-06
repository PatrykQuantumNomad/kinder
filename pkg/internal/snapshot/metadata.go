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
	"encoding/json"
	"fmt"
	"time"
)

// MetadataVersion is the current schema version written to metadata.json.
// UnmarshalMetadata rejects documents with an absent schemaVersion to provide
// a forward-compat alarm if a newer version of kinder changes the schema.
const MetadataVersion = "1"

// Bundle entry names — the exact filenames inside the tar.gz archive.
const (
	EntryEtcd     = "etcd.snap"
	EntryImages   = "images.tar"
	EntryPVs      = "pvs.tar"
	EntryConfig   = "kind-config.yaml"
	EntryMetadata = "metadata.json"
)

// TopologyInfo records the cluster topology at snapshot time so restore can
// reject a snapshot that doesn't match the target cluster's node layout.
type TopologyInfo struct {
	ControlPlaneCount int  `json:"controlPlaneCount"`
	WorkerCount       int  `json:"workerCount"`
	HasLoadBalancer   bool `json:"hasLoadBalancer"`
}

// Metadata is the JSON document written as metadata.json inside every snapshot
// archive. It carries all LIFE-08 fields (k8sVersion, addonVersions,
// imagesDigest) plus per-component SHA-256 digests and topology.
//
// Do NOT implement json.Marshaler — stdlib json tags handle all cases.
// time.Time fields round-trip via RFC3339Nano through the stdlib encoder.
type Metadata struct {
	SchemaVersion string            `json:"schemaVersion"` // must be MetadataVersion ("1")
	Name          string            `json:"name"`
	ClusterName   string            `json:"clusterName"`
	CreatedAt     time.Time         `json:"createdAt"`
	K8sVersion    string            `json:"k8sVersion"`    // e.g. "v1.31.2"
	NodeImage     string            `json:"nodeImage"`     // e.g. "kindest/node:v1.31.2"
	Topology      TopologyInfo      `json:"topology"`
	AddonVersions map[string]string `json:"addonVersions"` // empty map = no addons
	EtcdDigest    string            `json:"etcdDigest"`
	ImagesDigest  string            `json:"imagesDigest"`
	PVsDigest     string            `json:"pvsDigest"`     // "" if no local-path PVs
	ConfigDigest  string            `json:"configDigest"`
	ArchiveDigest string            `json:"archiveDigest"` // sha256 of full .tar.gz; also in sidecar
}

// MarshalMetadata encodes m as indented JSON. It is a thin wrapper around
// json.MarshalIndent so callers don't need to import encoding/json.
func MarshalMetadata(m *Metadata) ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

// UnmarshalMetadata decodes data into a *Metadata. It returns an error if
// schemaVersion is absent — a forward-compat alarm that fires whenever a future
// kinder writes a document this version doesn't know how to interpret.
func UnmarshalMetadata(data []byte) (*Metadata, error) {
	var m Metadata
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("snapshot metadata: failed to decode JSON: %w", err)
	}
	if m.SchemaVersion == "" {
		return nil, fmt.Errorf("snapshot metadata: missing schemaVersion (expected %q)", MetadataVersion)
	}
	return &m, nil
}
