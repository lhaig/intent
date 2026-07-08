#!/usr/bin/env bash
#
# selfhost/compiler/diff-emit.sh — Phase 55 / ADR 0059 byte-equal emit gate.
#
# The compiler counterpart to diff-checker / selfcheck-formatter: for each corpus
# file, verify the stage2 (Intent) compiler emits Rust BYTE-EQUAL with stage1:
#
#     intentc build --emit <f>                 (stage1, Go backend -> <base>.rs)
#     intentc build --emit --self-hosted <f>   (stage2, Intent -> <base>.rs)
#
# The stage2 emitter must be COMPLETE for every construct it claims (unlike the
# sound-but-incomplete checker), so the corpus only holds programs whose every
# construct is supported. It starts at 1/1 (hello.intent) and grows per slice as
# constructs land. A divergence fails the gate.
#
# Both emits write <base>.rs into the current directory, so each file is emitted
# in its own temp dir (stage1 output moved aside before the stage2 run). The
# stage2 compiler binary is built once and passed via INTENT_STAGE2_COMPILE so
# the emits can run from a temp cwd (keeping the repo root clean).
#
# Usage:  ./selfhost/compiler/diff-emit.sh   (from anywhere)
#         make diff-emit
#
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
cd "$ROOT"
INTENTC="$ROOT/intentc"

if [ ! -x "$INTENTC" ]; then
  echo "intentc not built; running make build..." >&2
  make build >/dev/null 2>&1 || { echo "make build failed" >&2; exit 2; }
fi

# Byte-equal emit corpus: programs whose every construct the stage2 emitter fully
# supports. GROW THIS LIST as each construct slice lands (mirror TESTED_EXAMPLES).
CORPUS=(
  "examples/hello.intent"
  "examples/divergence_demo.intent"
  "examples/fibonacci.intent"
  "examples/target_specific_demo.intent"
  "examples/array_sum.intent"
  "selfhost/compiler/emit-fixtures/let_locals.intent"
  "selfhost/compiler/emit-fixtures/binops.intent"
  "selfhost/compiler/emit-fixtures/control_flow.intent"
  "selfhost/compiler/emit-fixtures/functions.intent"
  "selfhost/compiler/emit-fixtures/strings.intent"
  "selfhost/compiler/emit-fixtures/arrays.intent"
)

# Build the stage2 compiler binary once; reuse it across corpus files.
BUILD_DIR=$(mktemp -d "${TMPDIR:-/tmp}/intent-diff-emit.XXXXXX")
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
  work=$(mktemp -d "${TMPDIR:-/tmp}/intent-emit-$base.XXXXXX")
  # stage1 emit -> <base>.rs, moved aside.
  if ! ( cd "$work" && "$INTENTC" build --emit "$ROOT/$f" ) >/dev/null 2>&1 || [ ! -f "$work/$base.rs" ]; then
    printf '%-32s %s\n' "$f" "STAGE1-EMIT-ERR"
    fail=1
    rm -rf "$work"
    continue
  fi
  mv "$work/$base.rs" "$work/stage1.rs"
  # stage2 emit -> <base>.rs (uses the prebuilt binary via INTENT_STAGE2_COMPILE).
  if ! ( cd "$work" && "$INTENTC" build --emit --self-hosted "$ROOT/$f" ) >/dev/null 2>&1 || [ ! -f "$work/$base.rs" ]; then
    printf '%-32s %s\n' "$f" "STAGE2-EMIT-ERR"
    fail=1
    rm -rf "$work"
    continue
  fi
  if diff -q "$work/stage1.rs" "$work/$base.rs" >/dev/null; then
    printf '%-32s %s\n' "$f" "EQUAL"
    pass=$((pass + 1))
  else
    printf '%-32s %s\n' "$f" "DIVERGE"
    diff "$work/stage1.rs" "$work/$base.rs" | head -30
    fail=1
  fi
  rm -rf "$work"
done

echo ""
if [ "$fail" -ne 0 ]; then
  echo "Summary: byte-equal emit gate FAILED (stage2 emit diverges from stage1)" >&2
  exit 1
fi
echo "Summary: $pass/${#CORPUS[@]} EQUAL — stage2 emit matches stage1 byte-for-byte"
exit 0
