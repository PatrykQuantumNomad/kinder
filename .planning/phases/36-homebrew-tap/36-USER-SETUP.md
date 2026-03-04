# Phase 36: Homebrew Tap — User Setup

## Status: COMPLETE

All user-setup steps for Phase 36 have been completed.

---

## HOMEBREW_TAP_TOKEN — GitHub PAT Setup

**Why this is needed:** GoReleaser needs to push `Casks/kinder.rb` to the `homebrew-kinder` repo from the `kinder` repo CI. `GITHUB_TOKEN` only has permissions to the current repo — a cross-repo push requires a separate Personal Access Token with `Contents: write` on the target repo.

### Setup Steps (Completed)

1. **Create fine-grained PAT**
   - URL: https://github.com/settings/personal-access-tokens/new
   - Token name: `HOMEBREW_TAP_TOKEN`
   - Expiration: 90 days (or custom)
   - Resource owner: `PatrykQuantumNomad`
   - Repository access: "Only select repositories" -> `homebrew-kinder`
   - Repository permissions: Contents -> "Read and write"

2. **Store PAT as repository secret**
   - URL: https://github.com/PatrykQuantumNomad/kinder/settings/secrets/actions
   - Secret name: `HOMEBREW_TAP_TOKEN`
   - Value: PAT generated in step 1

### Verification

The secret is active and will be consumed by `.github/workflows/release.yml`:

```yaml
      - uses: goreleaser/goreleaser-action@v7
        with:
          distribution: goreleaser
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          HOMEBREW_TAP_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}  # PAT with Contents:write on homebrew-kinder repo
```

GoReleaser passes `HOMEBREW_TAP_TOKEN` to the `homebrew_casks` section in `.goreleaser.yaml` via `token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"`.

### End-to-End Verification (requires a tagged release)

After pushing a `v*` tag, confirm the cask was published:

```bash
gh api repos/PatrykQuantumNomad/homebrew-kinder/commits --jq '.[0].commit.message'
# Expected: "Brew cask update for kinder version vX.Y.Z"
```

---

*Completed: 2026-03-04*
