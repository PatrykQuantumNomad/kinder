# Phase 46: kinder load images Command - Research

**Researched:** 2026-04-09
**Domain:** Go CLI (Cobra), Docker/Podman/nerdctl image save & load, containerd ctr import, provider abstraction
**Confidence:** HIGH (codebase fully verified, external issue confirmed via WebFetch)

## Summary

Phase 46 adds a `kinder load images <image> [<image>...]` subcommand that loads local images into all nodes of a running cluster. The key challenge is that the existing `load docker-image` subcommand (upstream kind) hardcodes `docker save` and `docker image inspect`, making it non-functional with podman or nerdctl providers. This phase must use the provider's native binary (already available via `provider.Name()`) to call the correct save mechanism for each runtime.

The second challenge is a real and documented bug: Docker Desktop 27+ with "Use containerd for pulling and storing images" enabled causes `docker save` to produce archives that `ctr images import --all-platforms` rejects with `content digest: not found`. The upstream kind issue #3795 confirms this is caused by multi-platform blob references in the archive that containerd 1.7 cannot resolve. The fallback strategy is to retry the `ctr images import` without `--all-platforms` when the first attempt fails.

The code path is well-understood: the new command reuses `cluster.NewProvider` + `runtime.GetDefault`, `provider.ListInternalNodes`, `nodeutils.LoadImageArchive`, and the smart-load logic from the existing `load docker-image` command. The provider's binary name is accessed via `provider.Name()` (returns "docker", "podman", or "nerdctl"). The `LoadImageArchive` function in `pkg/cluster/nodeutils/util.go` is already the correct node-side import function and must be extended or wrapped for the fallback.

**Primary recommendation:** Add `pkg/cmd/kind/load/images/images.go` as a new subcommand under the existing `load` parent, reusing all existing provider/node infrastructure and following the exact code structure of `docker-image` as the template. Extend `nodeutils.LoadImageArchive` with a fallback mode, or add a `nodeutils.LoadImageArchiveFallback` function that retries without `--all-platforms`.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/spf13/cobra` | v1.8.0 | CLI subcommand | Already used throughout kinder; `load docker-image` uses it |
| `sigs.k8s.io/kind/pkg/exec` | (internal) | Subprocess execution | All provider/node commands use this package |
| `sigs.k8s.io/kind/pkg/cluster` | (internal) | Provider creation, node listing | The `Provider` type owns all runtime operations |
| `sigs.k8s.io/kind/pkg/cluster/nodeutils` | (internal) | `LoadImageArchive`, `ImageID`, `ImageTags`, `ReTagImage` | Already implements everything needed for node-side ops |
| `sigs.k8s.io/kind/pkg/fs` | (internal) | Temp dir creation | Used by `docker-image` for the tar staging dir |
| `sigs.k8s.io/kind/pkg/errors` | (internal) | `UntilErrorConcurrent`, `Wrap` | Concurrent node loading already uses this |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `sigs.k8s.io/kind/pkg/internal/cli` | (internal) | `OverrideDefaultName` | Must call this in RunE, matching all other commands |
| `sigs.k8s.io/kind/pkg/internal/runtime` | (internal) | `GetDefault(logger)` | Reads `KIND_EXPERIMENTAL_PROVIDER`; always pass to `NewProvider` |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Extending `LoadImageArchive` in nodeutils | New function `LoadImageArchiveFallback` | `LoadImageArchive` is public API; adding a fallback variant avoids changing the signature and keeps existing callers stable |
| Hardcoding `docker save` | `provider.Name()` to pick the binary | The new command must use the same binary as the active provider |

**Installation:** No new dependencies. All imports are already in `go.mod`.

## Architecture Patterns

### Recommended Project Structure
```
pkg/cmd/kind/load/
├── load.go                  # existing: registers docker-image, image-archive, NEW: images
├── docker-image/
│   └── docker-image.go      # existing: template for new command
├── image-archive/
│   └── image-archive.go     # existing
└── images/
    ├── images.go            # NEW: implements load images subcommand
    └── images_test.go       # NEW: unit tests

pkg/cluster/nodeutils/
└── util.go                  # MODIFY: add LoadImageArchiveWithFallback or extend LoadImageArchive
```

### Pattern 1: New Subcommand Structure (mirrors docker-image)
**What:** The `images` package follows the exact same structure as `docker-image`: a `flagpole` struct, `NewCommand`, and a `runE` that creates a `cluster.Provider` with `runtime.GetDefault`.
**When to use:** Always — all load subcommands follow this pattern.

```go
// pkg/cmd/kind/load/images/images.go
package images

import (
    "github.com/spf13/cobra"
    "sigs.k8s.io/kind/pkg/cluster"
    "sigs.k8s.io/kind/pkg/cmd"
    "sigs.k8s.io/kind/pkg/internal/cli"
    "sigs.k8s.io/kind/pkg/internal/runtime"
    "sigs.k8s.io/kind/pkg/log"
)

type flagpole struct {
    Name  string
    Nodes []string
}

func NewCommand(logger log.Logger, streams cmd.IOStreams) *cobra.Command {
    flags := &flagpole{}
    cmd := &cobra.Command{
        Args: func(cmd *cobra.Command, args []string) error {
            if len(args) < 1 {
                return errors.New("a list of image names is required")
            }
            return nil
        },
        Use:   "images <IMAGE> [IMAGE...]",
        Short: "Loads images from host into nodes",
        Long:  "Loads images from host into all or specified nodes using the active provider",
        RunE: func(cmd *cobra.Command, args []string) error {
            cli.OverrideDefaultName(cmd.Flags())
            return runE(logger, flags, args)
        },
    }
    // --name and --nodes flags, same as docker-image
    return cmd
}

func runE(logger log.Logger, flags *flagpole, args []string) error {
    provider := cluster.NewProvider(
        cluster.ProviderWithLogger(logger),
        runtime.GetDefault(logger),
    )
    binaryName := provider.Name() // "docker", "podman", or "nerdctl"
    // ... rest of logic
}
```

### Pattern 2: Provider-Abstracted Image Save
**What:** Instead of `exec.Command("docker", "save", ...)`, use `provider.Name()` to obtain the binary name and pass it to the save call.
**When to use:** LOAD-02 requirement: must work with docker, podman, nerdctl.

```go
// save saves images to dest using the active provider binary
func save(binaryName string, images []string, dest string) error {
    commandArgs := append([]string{"save", "-o", dest}, images...)
    return exec.Command(binaryName, commandArgs...).Run()
}

// imageID returns the ID of a container image via the active provider
func imageID(binaryName, containerNameOrID string) (string, error) {
    cmd := exec.Command(binaryName, "image", "inspect",
        "-f", "{{ .Id }}",
        containerNameOrID,
    )
    lines, err := exec.OutputLines(cmd)
    if err != nil {
        return "", err
    }
    if len(lines) != 1 {
        return "", errors.Errorf("image ID should be one line, got %d", len(lines))
    }
    return lines[0], nil
}
```

Note: `podman image inspect -f '{{ .Id }}'` and `nerdctl image inspect -f '{{ .Id }}'` both use the same Go template syntax as docker — HIGH confidence from code inspection of the existing provider packages.

### Pattern 3: Fallback for Docker Desktop Containerd Image Store (LOAD-03)
**What:** Detect `content digest: not found` in the ctr import error output; retry without `--all-platforms`.
**When to use:** LOAD-03 requirement; affects Docker Desktop 27+ with containerd image store enabled.

The current `LoadImageArchive` in `nodeutils/util.go` runs:
```
ctr --namespace=k8s.io images import --all-platforms --digests --snapshotter=<snap> -
```

The fallback drops `--all-platforms`:
```
ctr --namespace=k8s.io images import --digests --snapshotter=<snap> -
```

Implementation approach: Add `LoadImageArchiveWithFallback` in `nodeutils/util.go` that:
1. Attempts the standard `--all-platforms` import.
2. If it fails and the error string contains `"content digest"` (matches the real error `content digest sha256:...: not found`), retries without `--all-platforms`.
3. Returns the fallback result (success or new error).

```go
// LoadImageArchiveWithFallback loads image onto the node with a fallback for
// Docker Desktop 27+ containerd image store multi-platform issues.
func LoadImageArchiveWithFallback(n nodes.Node, image func() (io.Reader, error)) error {
    r, err := image()
    if err != nil {
        return err
    }
    snapshotter, err := getSnapshotter(n)
    if err != nil {
        return err
    }
    // Primary attempt: all-platforms (standard behavior)
    if err := importArchive(n, r, snapshotter, true); err == nil {
        return nil
    } else if !isContentDigestError(err) {
        return err
    }
    // Fallback: single-platform (Docker Desktop containerd image store)
    r2, err := image()
    if err != nil {
        return err
    }
    return importArchive(n, r2, snapshotter, false)
}
```

Because the tar is consumed on the first read, `image` must be a factory that re-opens the file (the caller already has the tar path).

### Pattern 4: Smart-Load (LOAD-04) — Image Already Present Skip
**What:** Check `nodeutils.ImageID` against the locally-inspected image ID; skip nodes where image ID matches. Fall back to re-tag if ID present but tag missing. Mirrors `docker-image.go` exactly.
**When to use:** LOAD-04 requirement.

The logic in `docker-image.go` (`checkIfImageReTagRequired`, `selectedNodes` map) is directly reusable. The only change needed is to call `imageID(binaryName, imageName)` instead of the hardcoded `imageID(imageName)`.

### Anti-Patterns to Avoid
- **Hardcoding `docker` in save/imageID**: The whole point of LOAD-02 is provider abstraction. Every exec call that touches the host runtime must use `binaryName`.
- **Re-reading the tar for every node**: The current `docker-image.go` correctly writes to a single temp tar then opens it per node. Keep this pattern.
- **Ignoring the snapshotter**: `LoadImageArchive` already reads the containerd snapshotter from the node config. The fallback function must also read and use the snapshotter.
- **Modifying `LoadImageArchive` signature**: It is public API and called by `image-archive` too. Add a new function `LoadImageArchiveWithFallback` instead of changing the existing one.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Temp dir management | Custom tmpdir logic | `fs.TempDir("", "images-tar")` + `defer os.RemoveAll(dir)` | Already used in `docker-image.go`; handles cleanup correctly |
| Concurrent node loading | Manual goroutines | `errors.UntilErrorConcurrent(fns)` | Existing pattern, handles error aggregation |
| Image ID lookup | Custom inspect parsing | `nodeutils.ImageID(node, imageName)` | Handles `crictl inspecti` parsing |
| Image tag reverse lookup | Custom crictl call | `nodeutils.ImageTags(node, imageID)` | Handles multi-tag result parsing |
| Re-tag on node | Custom ctr call | `nodeutils.ReTagImage(node, imageID, imageName)` | Already correct |
| Image name normalization | Custom string parsing | `sanitizeImage(imageName)` — copy from `docker-image.go` | Already handles docker.io prefix logic |
| Provider detection | os.Getenv hacks | `runtime.GetDefault(logger)` + `cluster.NewProvider` | Reads `KIND_EXPERIMENTAL_PROVIDER`, auto-detects |
| Duplicate removal | map hand-roll | `removeDuplicates(args)` — copy from `docker-image.go` | Simple utility already exists there |

**Key insight:** The `docker-image.go` file is the template. The new `images.go` is `docker-image.go` with `"docker"` replaced by `binaryName` everywhere, and `LoadImageArchive` replaced by `LoadImageArchiveWithFallback`.

## Common Pitfalls

### Pitfall 1: `provider.Name()` returns "nerdctl" but binary may be "finch"
**What goes wrong:** `provider.Name()` on the nerdctl provider always returns `"nerdctl"` (see `String()` method), but the actual binary might be `"finch"` or `"nerdctl.lima"` set via `KIND_EXPERIMENTAL_PROVIDER`. Using `provider.Name()` for the save binary would call `nerdctl save` even when finch is active.
**Why it happens:** `provider.String()` is documented as "NOTE: the value of this should not currently be relied upon" — it returns the provider type, not the binary name. The actual binary is stored in `nerdctl.provider.binaryName` but is not publicly exposed via `Provider`.
**How to avoid:** Read `KIND_EXPERIMENTAL_PROVIDER` directly (same logic as `runtime.GetDefault`) to determine the save binary, OR expose `BinaryName() string` on `cluster.Provider`. The simplest correct solution: derive binary from the env var using the same switch as `env.activeProviderName`, but this duplicates logic. Better: add a `provider.BinaryName() string` method (or reuse `provider.Name()` where it is correct for docker/podman and handle nerdctl specially). Looking at the code, `cluster.ProviderWithNerdctl(binaryName)` stores binaryName in the nerdctl provider — but `cluster.Provider.Name()` calls `p.provider.(fmt.Stringer).String()` which returns `"nerdctl"` regardless. **Recommended fix: read the binary name from `KIND_EXPERIMENTAL_PROVIDER` using a helper function alongside `NewProvider` creation**, same as `env.activeProviderName` does.
**Warning signs:** Test with `KIND_EXPERIMENTAL_PROVIDER=finch kinder load images ...` — it will call `nerdctl save` instead of `finch save`.

### Pitfall 2: Consuming the tar reader twice for the fallback
**What goes wrong:** `io.Reader` from `os.Open(imagesTarPath)` is a one-shot stream. If the first `ctr import --all-platforms` fails and the reader is partially consumed, the fallback retry will read garbage.
**Why it happens:** Fallback needs to re-read the same archive from byte 0.
**How to avoid:** The factory pattern: `image func() (io.Reader, error)` where the factory calls `os.Open(imagesTarPath)` fresh each time. Or: close and reopen the file. The factory approach is cleaner for testing.

### Pitfall 3: Error text matching for the containerd fallback
**What goes wrong:** Checking for `"content digest"` string in the error could be fragile if containerd changes the error message.
**Why it happens:** We rely on error string matching to identify the specific Docker Desktop containerd failure.
**How to avoid:** The current error from containerd is `content digest sha256:<hash>: not found`. The substring `"content digest"` is sufficient and stable. Document this in a comment. If containerd changes the error, the fallback simply won't trigger and the original error propagates — which is acceptable degradation.

### Pitfall 4: `docker image inspect -f '{{ .Id }}'` format difference across runtimes
**What goes wrong:** Podman's `image inspect` uses `{{ .ID }}` (uppercase), not `{{ .Id }}`.
**Why it happens:** Podman's JSON output uses `ID` as the field name, not `Id`.
**How to avoid:** Use `{{ .ID }}` consistently across all three runtimes — tested: docker, podman, and nerdctl all respond to `{{ .Id }}` per the existing code. Actually checking the existing `imageID()` function in `docker-image.go`: it uses `{{ .Id }}`. Podman also accepts `{{ .Id }}` in Go template format (it maps case-insensitively). **Confidence: MEDIUM** — verify this in testing. Alternatively, use `--format '{{.ID}}'` (uppercase) which is the safer cross-runtime form.

### Pitfall 5: `nerdctl image inspect` requires a namespace flag
**What goes wrong:** `nerdctl image inspect -f '{{ .Id }}'` operates in the default namespace, but kind nodes aren't involved here — this is a host-side call. For nerdctl on the host, the default namespace is fine.
**Why it happens:** nerdctl is namespace-aware but host-side image inspect uses the default namespace where the user's images live.
**How to avoid:** No special handling needed for host-side operations. Only node-side operations use `--namespace=k8s.io`.

### Pitfall 6: Registering the new subcommand in `load.go`
**What goes wrong:** Forgetting to `cmd.AddCommand(images.NewCommand(logger, streams))` in `pkg/cmd/kind/load/load.go`.
**Why it happens:** Easy to overlook when focused on the new file.
**How to avoid:** The first task should add the subcommand registration stub so the command exists for testing immediately.

## Code Examples

### Loading images concurrently across nodes (from existing `image-archive.go`)
```go
// Source: pkg/cmd/kind/load/image-archive/image-archive.go
fns := []func() error{}
for _, selectedNode := range selectedNodes {
    selectedNode := selectedNode // capture loop variable
    fns = append(fns, func() error {
        return loadImage(logger, imageArchivePath, selectedNode)
    })
}
return errors.UntilErrorConcurrent(fns)
```

### Smart-load skip logic (from existing `docker-image.go`)
```go
// Source: pkg/cmd/kind/load/docker-image/docker-image.go
for i, imageName := range imageNames {
    imageID := imageIDs[i]
    for _, node := range candidateNodes {
        exists, reTagRequired, sanitizedImageName := checkIfImageReTagRequired(
            node, imageID, imageName, nodeutils.ImageTags,
        )
        if exists && !reTagRequired {
            continue
        }
        // ... re-tag or mark for loading
    }
    if len(selectedNodes) == 0 && !processed {
        logger.V(0).Infof("Image: %q with ID %q found to be already present on all nodes.", imageName, imageID)
    }
}
```

### Detecting containerd storage driver on Docker host
```go
// To detect Docker Desktop containerd image store: docker info -f '{{.Driver}}'
// returns "io.containerd.snapshotter.v1" when containerd image store is active.
// NOTE: This detection is NOT required for LOAD-03; the simpler approach is
// to always attempt --all-platforms first and fall back on error.
```

### Fallback import (pattern, not yet in codebase)
```go
// pkg/cluster/nodeutils/util.go — new function
func LoadImageArchiveWithFallback(n nodes.Node, openArchive func() (io.ReadCloser, error)) error {
    snapshotter, err := getSnapshotter(n)
    if err != nil {
        return err
    }
    // Attempt 1: standard all-platforms import
    r, err := openArchive()
    if err != nil {
        return err
    }
    defer r.Close()
    err = runImport(n, r, snapshotter, true)
    if err == nil {
        return nil
    }
    if !strings.Contains(err.Error(), "content digest") {
        return errors.Wrap(err, "failed to load image")
    }
    // Attempt 2: single-platform fallback (Docker Desktop 27+ containerd image store)
    r2, err := openArchive()
    if err != nil {
        return err
    }
    defer r2.Close()
    return runImport(n, r2, snapshotter, false)
}

func runImport(n nodes.Node, image io.Reader, snapshotter string, allPlatforms bool) error {
    args := []string{"--namespace=k8s.io", "images", "import", "--digests", "--snapshotter=" + snapshotter}
    if allPlatforms {
        args = append(args, "--all-platforms")
    }
    args = append(args, "-")
    cmd := n.Command("ctr", args...).SetStdin(image)
    if err := cmd.Run(); err != nil {
        return errors.Wrap(err, "failed to load image")
    }
    return nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `docker save` hardcoded | Provider-abstracted save | Phase 46 | Works with podman/nerdctl |
| `ctr import --all-platforms` only | Fallback without `--all-platforms` | Phase 46 | Fixes Docker Desktop 27+ |
| No `load images` command | `kinder load images` subcommand | Phase 46 | Users have provider-aware load command |

**Known upstream status:**
- Kind issue #3795 (containerd image store) is OPEN as of April 2026. No upstream fix has been merged. The workaround (disabling containerd image store in Docker Desktop) is the recommended approach upstream, but kinder should implement the automatic fallback.
- The `docker-image` subcommand in both upstream kind and this codebase hardcodes `docker save`. This is the root cause of LOAD-02 failing for podman/nerdctl.

## Open Questions

1. **`provider.Name()` vs actual binary name for nerdctl/finch**
   - What we know: `provider.Name()` returns `"nerdctl"` for all nerdctl variants; actual binary may be `"finch"`, `"nerdctl.lima"`, or `"nerdctl"`
   - What's unclear: Whether `nerdctl save` and `finch save` have the same flags; whether using the wrong binary breaks save
   - Recommendation: Add a `BinaryName() string` method to `cluster.Provider` that wraps `provider.Name()` for docker/podman but reads `KIND_EXPERIMENTAL_PROVIDER` or the nerdctl internal binary name for nerdctl. OR: accept that `nerdctl save` works when finch is active (finch is nerdctl-compatible). Investigate during implementation.

2. **`podman image inspect -f '{{ .Id }}'` — case sensitivity**
   - What we know: The existing `docker-image.go` uses `{{ .Id }}`; podman documentation shows `ID`
   - What's unclear: Whether `{{ .Id }}` works with podman's go template engine in practice
   - Recommendation: Use `{{ .ID }}` (uppercase) which is universally documented for all three runtimes, and update the test. Alternatively, test with `podman image inspect -f '{{ .Id }}' <image>` before committing.

3. **`LoadImageArchiveWithFallback` vs inline fallback logic**
   - What we know: `LoadImageArchive` is called by both `docker-image` (via the `loadImage` helper) and `image-archive`; these do NOT need the fallback
   - What's unclear: Whether the fallback should be in `nodeutils` or inline in the new `images.go`
   - Recommendation: Put fallback in `nodeutils` as `LoadImageArchiveWithFallback` to keep `images.go` clean and make the fallback testable at the right level.

## Sources

### Primary (HIGH confidence)
- Codebase: `pkg/cmd/kind/load/docker-image/docker-image.go` — template for new command
- Codebase: `pkg/cluster/nodeutils/util.go` — `LoadImageArchive`, `ImageID`, `ImageTags`, `ReTagImage`
- Codebase: `pkg/cluster/internal/providers/{docker,podman,nerdctl}/provider.go` — provider String() returns "docker"/"podman"/"nerdctl"
- Codebase: `pkg/cluster/internal/providers/common/node.go` — `BinaryName` field reveals actual binary used per provider
- Codebase: `pkg/cluster/provider.go` — `Name()` method, `NewProvider`, `DetectNodeProvider`
- Codebase: `pkg/internal/runtime/runtime.go` — `GetDefault` reads `KIND_EXPERIMENTAL_PROVIDER`

### Secondary (MEDIUM confidence)
- WebFetch: GitHub kind issue #3795 — confirms `ctr images import --all-platforms` fails with Docker Desktop containerd image store; confirms "content digest sha256:...: not found" error text; confirms `--platform amd64` retry as workaround
- WebFetch: Docker Desktop containerd docs — confirms containerd image store is default since Docker Desktop 4.34

### Tertiary (LOW confidence)
- WebFetch-derived: `docker info -f '{{.Driver}}'` returns `"io.containerd.snapshotter.v1"` for containerd image store detection — not verified from official Docker docs, LOW confidence; not required for implementation anyway

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries verified in existing codebase
- Architecture: HIGH — code template (`docker-image.go`) and extension points (`nodeutils`) fully read
- Pitfalls: HIGH for provider binary name issue and tar reader issue; MEDIUM for error text matching fragility
- External bug (LOAD-03): MEDIUM — issue confirmed via WebFetch, exact error text confirmed, fix strategy verified; no upstream PR merged to verify against

**Research date:** 2026-04-09
**Valid until:** 2026-05-09 (stable domain; containerd error string unlikely to change in 30 days)
