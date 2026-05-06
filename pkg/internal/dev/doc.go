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

// Package dev implements the inner loop for `kinder dev`: watch a source
// directory, debounce file changes, and run a build → load → rollout cycle
// against a target Deployment in a kinder cluster.
//
// The package is split into focused files:
//   - watch.go    : fsnotify-backed file watcher (default)
//   - poll.go     : stdlib polling watcher (--poll fallback for Docker
//                   Desktop volume mounts on macOS where fsnotify is
//                   unreliable)
//   - debounce.go : channel debouncer coalescing rapid edits into one
//                   cycle trigger
//   - cycle.go    : per-cycle build → load → rollout step runner (Plan 03)
//   - build.go    : docker build wrapper (Plan 02)
//   - load.go     : image load via cluster nodeutils (Plan 02)
//   - rollout.go  : host kubectl rollout restart + status (Plan 02)
//   - dev.go      : Run() entrypoint stitching everything together (Plan 03)
package dev
