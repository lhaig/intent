#!/usr/bin/env bash
#
# selfhost/compiler/diff-emit-self.sh — Phase 56 / ADR 0059 FULL self-hosting gate.
#
# The capstone of the self-hosted compiler: verify the stage2 (Intent) compiler emits
# the ENTIRE self-hosted toolchain's OWN multi-module source byte-equal with stage1.
# For each tool's entry point:
#
#     intentc build --emit <entry>                 (stage1, Go backend)
#     intentc build --emit --self-hosted <entry>   (stage2, Intent -> lower_all/generate_all)
#
# must produce byte-identical Rust. This closes the bootstrap: stage2 regenerates
# every tool, INCLUDING ITSELF (compile_main), to the same bytes stage1 does — so a
# stage3 built from the stage2 emit is identical to stage2.
#
# Separate from diff-emit (the example/fixture corpus) because these are the compiler's
# own large multi-module sources, analogous to selfcheck-checker vs diff-checker.
#
# Usage:  ./selfhost/compiler/diff-emit-self.sh   (from anywhere)
#         make diff-emit-self
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
cd "$ROOT"
INTENTC="$ROOT/intentc"

if [ ! -x "$INTENTC" ]; then
  echo "intentc not built; running make build..." >&2
  make build >/dev/null 2>&1 || { echo "make build failed" >&2; exit 2; }
fi

# The four self-hosted tools' entry points (each a multi-module program).
CORPUS=(
  "selfhost/compiler/compile_main.intent"
  "selfhost/checker/check_main.intent"
  "selfhost/formatter/main.intent"
  "selfhost/linter/lint_main.intent"
)

# Build the stage2 compiler binary once; reuse it across the corpus.
BUILD_DIR=$(mktemp -d "${TMPDIR:-/tmp}/intent-diff-emit-self.XXXXXX")
trap 'rm -rf "$BUILD_DIR"' EXIT
if ! ( cd "$BUILD_DIR" && "$INTENTC" build --target rust "$ROOT/selfhost/compiler/compile_main.intent" ) >"$BUILD_DIR/build.log" 2>&1; then
  echo "stage2 compiler build failed:" >&2
  tail -20 "$BUILD_DIR/build.log" >&2
  exit 2
fi
if [ ! -x "$BUILD_DIR/compile_main" ]; then
  echo "build succeeded but binary not found at $BUILD_DIR/compile_main" >&2
  exit 2
fi
export INTENT_STAGE2_COMPILE="$BUILD_DIR/compile_main"

fail=0
pass=0
for f in "${CORPUS[@]}"; do
  base=$(basename "$f" .intent)
  work=$(mktemp -d "${TMPDIR:-/tmp}/intent-emit-self-$base.XXXXXX")
  if ! ( cd "$work" && "$INTENTC" build --emit "$ROOT/$f" ) >/dev/null 2>&1 || [ ! -f "$work/$base.rs" ]; then
    printf '%-42s %s\n' "$f" "STAGE1-EMIT-ERR"
    fail=1
    rm -rf "$work"
    continue
  fi
  mv "$work/$base.rs" "$work/stage1.rs"
  if ! ( cd "$work" && "$INTENTC" build --emit --self-hosted "$ROOT/$f" ) >/dev/null 2>&1 || [ ! -f "$work/$base.rs" ]; then
    printf '%-42s %s\n' "$f" "STAGE2-EMIT-ERR"
    fail=1
    rm -rf "$work"
    continue
  fi
  if diff -q "$work/stage1.rs" "$work/$base.rs" >/dev/null; then
    printf '%-42s %s (%s lines)\n' "$f" "EQUAL" "$(wc -l < "$work/stage1.rs" | tr -d ' ')"
    pass=$((pass + 1))
  else
    printf '%-42s %s\n' "$f" "DIVERGE"
    diff "$work/stage1.rs" "$work/$base.rs" | head -30
    fail=1
  fi
  rm -rf "$work"
done

echo ""
if [ "$fail" -ne 0 ]; then
  echo "Summary: FULL self-hosting gate FAILED (stage2 self-emit diverges from stage1)" >&2
  exit 1
fi
echo "Summary: $pass/${#CORPUS[@]} EQUAL — the stage2 compiler emits the whole toolchain (incl. itself) byte-for-byte"
exit 0
