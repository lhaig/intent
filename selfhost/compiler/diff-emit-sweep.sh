#!/usr/bin/env bash
#
# selfhost/compiler/diff-emit-sweep.sh — Phase 57 emitter-hardening sweep.
#
# Differential emit over EVERY Intent program in the repo (a superset of the
# diff-emit corpus): for each file stage1 can emit, compare stage1 vs stage2
# (`--self-hosted`) byte-for-byte. Finds emitter gaps the curated corpus misses.
#
# Files stage1 can't `build --emit` (library modules with no entry, intentionally
# invalid checker fixtures) are auto-skipped. Files with KNOWN, catalogued gaps are
# allow-listed below (grep `KNOWN_GAPS`) — the sweep FAILS only on an UNEXPECTED
# divergence (a regression) or when an allow-listed file starts passing (prune it).
#
# Usage:  ./selfhost/compiler/diff-emit-sweep.sh   (from anywhere)
#         make diff-emit-sweep
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
cd "$ROOT"
INTENTC="$ROOT/intentc"
[ -x "$INTENTC" ] || { make build >/dev/null 2>&1 || { echo "make build failed" >&2; exit 2; }; }

# KNOWN_GAPS — programs with catalogued, not-yet-closed emitter/parser gaps. Each entry
# is a repo-relative path. Pruthis list as gaps close (a passing allow-listed file fails
# the sweep, prompting removal). See prds/progress.md (Phase 57) for the gap taxonomy.
KNOWN_GAPS=(
  # EMITTER gap: HTTP builtins (http_post/http_get/json_get/json_path) + reqwest/serde_json
  # `use` injection + the __intent_http_* helper block. These files import llm.intent.
  "examples/attractor/async_retry.intent"
  "examples/attractor/handlers.intent"
  "examples/attractor/main_async.intent"
  "examples/attractor/parallel.intent"
  "examples/attractor/retry.intent"
  "examples/attractor/llm.intent"
  # extern / FFI emit.
  "examples/ffi_blake3/ffi_blake3.intent"
  # multi-file DETECTION for package members / project files (header + intent-block omission):
  # stage1 build --emit treats a lone package/project member as multi-file; stage2CompilePaths
  # uses IsMultiFile (no imports -> single-file) so a 1-module closure emits the single-file form.
  "examples/attractor/types.intent"
  "examples/packages/app_pkg/main.intent"
  "examples/packages/types_pkg/types.intent"
)
is_known() { local f="$1"; for k in "${KNOWN_GAPS[@]}"; do [ "$k" = "$f" ] && return 0; done; return 1; }

BUILD_DIR=$(mktemp -d "${TMPDIR:-/tmp}/intent-sweep.XXXXXX")
trap 'rm -rf "$BUILD_DIR"' EXIT
( cd "$BUILD_DIR" && "$INTENTC" build --target rust "$ROOT/selfhost/compiler/compile_main.intent" ) >"$BUILD_DIR/b.log" 2>&1 \
  || { echo "stage2 build failed" >&2; tail -20 "$BUILD_DIR/b.log" >&2; exit 2; }
export INTENT_STAGE2_COMPILE="$BUILD_DIR/compile_main"

equal=0; skip=0; known=0; regress=0; fixed=0
for f in $(find examples selfhost -name "*.intent" | sort); do
  base=$(basename "$f" .intent)
  w=$(mktemp -d)
  if ! ( cd "$w" && "$INTENTC" build --emit "$ROOT/$f" ) >/dev/null 2>&1 || [ ! -f "$w/$base.rs" ]; then
    skip=$((skip+1)); rm -rf "$w"; continue
  fi
  mv "$w/$base.rs" "$w/s1.rs"
  ok=1
  if ! ( cd "$w" && "$INTENTC" build --emit --self-hosted "$ROOT/$f" ) >/dev/null 2>&1 || [ ! -f "$w/$base.rs" ]; then
    ok=0
  elif ! diff -q "$w/s1.rs" "$w/$base.rs" >/dev/null; then
    ok=0
  fi
  if [ "$ok" -eq 1 ]; then
    if is_known "$f"; then printf '  %-52s NOW-PASSES (prune from KNOWN_GAPS)\n' "$f"; fixed=$((fixed+1)); else equal=$((equal+1)); fi
  else
    if is_known "$f"; then known=$((known+1)); else printf '  %-52s REGRESSION\n' "$f"; regress=$((regress+1)); fi
  fi
  rm -rf "$w"
done

echo ""
echo "SWEEP: equal=$equal  known-gaps=$known  skipped=$skip  regressions=$regress  now-passing=$fixed"
if [ "$regress" -ne 0 ] || [ "$fixed" -ne 0 ]; then
  echo "Summary: sweep FAILED — $regress regression(s), $fixed allow-listed file(s) now passing (prune KNOWN_GAPS)" >&2
  exit 1
fi
echo "Summary: OK — $equal programs emit byte-equal beyond the corpus; $known catalogued gaps unchanged"
exit 0
