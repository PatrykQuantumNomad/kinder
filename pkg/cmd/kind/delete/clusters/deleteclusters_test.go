/*
Copyright 2024 The Kubernetes Authors.

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

package clusters

// NOTE: The deleteClusters function is unexported and internally creates a
// concrete cluster.Provider (not an interface), which makes it impractical
// to unit test the error-propagation logic without either:
//   1. Refactoring to accept a provider interface (recommended for future work)
//   2. Running integration tests against a real container runtime
//
// The fix ensures that when provider.Delete() fails for one or more clusters,
// the function now returns an aggregate error instead of nil. Previously,
// errors were logged but the function always returned nil, which caused the
// CLI to exit with code 0 even on failure.
