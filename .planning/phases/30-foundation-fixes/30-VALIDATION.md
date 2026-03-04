# Phase 30: Foundation Fixes - Validation

**Generated:** 2026-03-04
**Phase:** 30-foundation-fixes
**Source:** 30-RESEARCH.md Validation Architecture section

## Test Framework

| Property | Value |
|----------|-------|
| Framework | `astro build` (compile-time validation) |
| Config file | `kinder-site/astro.config.mjs` |
| Quick run command | `cd kinder-site && npm run build` |
| Full suite command | `cd kinder-site && npm run build` |

No unit test framework is installed in `kinder-site/`. Validation is build-time: `astro build` catches broken slugs, missing files, and MDX syntax errors. Content checks use grep.

## Requirements to Verification Map

| Req ID | Behavior | Automated Command | Expected |
|--------|----------|-------------------|----------|
| FOUND-01 | Comparison component shows all 7 addons | `grep -c "Local Registry\|cert-manager" kinder-site/src/components/Comparison.astro` | >= 2 (one match per addon) |
| FOUND-01 | index.mdx description meta updated with all 7 addons | `grep "Local Registry.*cert-manager\|cert-manager.*Local Registry" kinder-site/src/content/docs/index.mdx` | At least 1 match |
| FOUND-01 | index.mdx addon cards grouped as core vs optional | `grep -c "Core Addons\|Optional Addons" kinder-site/src/content/docs/index.mdx` | 2 (one heading per group) |
| FOUND-02 | Quick-start covers all 7 addon verifications | `grep -c "Local Registry\|cert-manager" kinder-site/src/content/docs/quick-start.md` | >= 4 (section headers + content) |
| FOUND-02 | Quick-start mentions --profile flag | `grep -c -- "--profile" kinder-site/src/content/docs/quick-start.md` | >= 4 (one per preset) |
| FOUND-02 | Quick-start has "Something wrong?" section with kinder doctor | `grep -c "kinder doctor" kinder-site/src/content/docs/quick-start.md` | >= 1 |
| FOUND-03 | Configuration page documents localRegistry and certManager fields | `grep -c "localRegistry\|certManager" kinder-site/src/content/docs/configuration.md` | >= 4 (field tables + YAML examples) |
| FOUND-03 | Configuration page has core/optional grouping | `grep -c "Core Addons\|Optional Addons" kinder-site/src/content/docs/configuration.md` | 2 |
| FOUND-04 | Sidebar has Guides and CLI Reference sections | `grep -c "Guides\|CLI Reference" kinder-site/astro.config.mjs` | 2 |
| FOUND-04 | All 6 placeholder files exist with "Coming soon" | `ls kinder-site/src/content/docs/guides/*.md kinder-site/src/content/docs/cli-reference/*.md 2>/dev/null \| wc -l` | 6 |
| ALL | Site builds without errors | `cd kinder-site && npm run build 2>&1 \| tail -1` | "build completed" or similar success message |

## Composite Verification Script

```bash
#!/bin/bash
# Phase 30 validation script -- run from repo root
set -e

echo "=== FOUND-01: Comparison component ==="
COUNT=$(grep -c "Local Registry\|cert-manager" kinder-site/src/components/Comparison.astro)
echo "  Addon mentions in Comparison.astro: $COUNT (expect >= 2)"
[ "$COUNT" -ge 2 ] || { echo "  FAIL"; exit 1; }

echo "=== FOUND-01: index.mdx description ==="
grep -q "Local Registry" kinder-site/src/content/docs/index.mdx && echo "  Local Registry found" || { echo "  FAIL: Local Registry missing"; exit 1; }
grep -q "cert-manager" kinder-site/src/content/docs/index.mdx && echo "  cert-manager found" || { echo "  FAIL: cert-manager missing"; exit 1; }

echo "=== FOUND-01: index.mdx core/optional card grouping ==="
COUNT=$(grep -c "Core Addons\|Optional Addons" kinder-site/src/content/docs/index.mdx)
echo "  Core/Optional headings in index.mdx: $COUNT (expect 2)"
[ "$COUNT" -ge 2 ] || { echo "  FAIL"; exit 1; }

echo "=== FOUND-02: Quick-start addon coverage ==="
COUNT=$(grep -c "Local Registry\|cert-manager" kinder-site/src/content/docs/quick-start.md)
echo "  New addon mentions in quick-start: $COUNT (expect >= 4)"
[ "$COUNT" -ge 4 ] || { echo "  FAIL"; exit 1; }

echo "=== FOUND-02: --profile flag ==="
COUNT=$(grep -c -- "--profile" kinder-site/src/content/docs/quick-start.md)
echo "  --profile mentions: $COUNT (expect >= 4)"
[ "$COUNT" -ge 4 ] || { echo "  FAIL"; exit 1; }

echo "=== FOUND-02: kinder doctor ==="
grep -q "kinder doctor" kinder-site/src/content/docs/quick-start.md && echo "  kinder doctor found" || { echo "  FAIL"; exit 1; }

echo "=== FOUND-03: Configuration fields ==="
COUNT=$(grep -c "localRegistry\|certManager" kinder-site/src/content/docs/configuration.md)
echo "  New field mentions in configuration: $COUNT (expect >= 4)"
[ "$COUNT" -ge 4 ] || { echo "  FAIL"; exit 1; }

echo "=== FOUND-03: Configuration core/optional grouping ==="
COUNT=$(grep -c "Core Addons\|Optional Addons" kinder-site/src/content/docs/configuration.md)
echo "  Core/Optional headings in configuration: $COUNT (expect 2)"
[ "$COUNT" -ge 2 ] || { echo "  FAIL"; exit 1; }

echo "=== FOUND-04: Sidebar sections ==="
grep -q "Guides" kinder-site/astro.config.mjs && echo "  Guides section found" || { echo "  FAIL"; exit 1; }
grep -q "CLI Reference" kinder-site/astro.config.mjs && echo "  CLI Reference section found" || { echo "  FAIL"; exit 1; }

echo "=== FOUND-04: Placeholder files ==="
EXPECTED_FILES=(
  "kinder-site/src/content/docs/guides/tls-web-app.md"
  "kinder-site/src/content/docs/guides/hpa-auto-scaling.md"
  "kinder-site/src/content/docs/guides/local-dev-workflow.md"
  "kinder-site/src/content/docs/cli-reference/profile-comparison.md"
  "kinder-site/src/content/docs/cli-reference/json-output.md"
  "kinder-site/src/content/docs/cli-reference/troubleshooting.md"
)
for f in "${EXPECTED_FILES[@]}"; do
  [ -f "$f" ] && echo "  $f exists" || { echo "  FAIL: $f missing"; exit 1; }
  grep -q "Coming soon" "$f" && echo "    has 'Coming soon'" || { echo "  FAIL: $f missing 'Coming soon'"; exit 1; }
done

echo "=== ALL: Site build ==="
cd kinder-site && npm run build 2>&1 | tail -5
echo ""
echo "=== ALL CHECKS PASSED ==="
```
