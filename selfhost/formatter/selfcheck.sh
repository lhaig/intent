#!/usr/bin/env bash
#
# selfhost/formatter/selfcheck.sh — Phase 42 byte-equal self-format gate.
#
# Verifies the stage2 formatter is a fixpoint on its OWN source files:
# format(parse(f)) == f for each selfhost/formatter/*.intent.
#
# Why a built binary instead of an in-language `intentc test`: `intentc test`
# runs rust tests via `cargo test`/libtest, which executes each test on a small
# (~2 MB) thread stack. The deep recursive-descent parse of the ~95 KB
# parser.intent overflows that thread stack and aborts — a libtest artifact, NOT
# a formatter bug. The real stage2 binary runs on the 8 MB main thread and
# self-formats every file fine, so this gate drives the actual binary.
#
# print() appends one trailing newline (println!/console.log), so the binary's
# stdout is `format(...) + "\n"`; we strip exactly one trailing newline before
# comparing.
#
# Usage:  ./selfhost/formatter/selfcheck.sh   (from anywhere)
#         make selfcheck-formatter
#
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
cd "$ROOT"
INTENTC="$ROOT/intentc"
FMT_DIR="$ROOT/selfhost/formatter"
MAIN_SRC="$FMT_DIR/main.intent"

if [ ! -x "$INTENTC" ]; then
  echo "intentc not built; running make build..." >&2
  make build >/dev/null 2>&1 || { echo "make build failed" >&2; exit 2; }
fi

# Build the stage2 formatter binary into a temp dir (intentc build writes the
# binary named after the entry file into the current working directory).
BUILD_DIR=$(mktemp -d "${TMPDIR:-/tmp}/intent-selfcheck.XXXXXX")
trap 'rm -rf "$BUILD_DIR"' EXIT
if ! ( cd "$BUILD_DIR" && "$INTENTC" build --target rust "$MAIN_SRC" ) >"$BUILD_DIR/build.log" 2>&1; then
  echo "stage2 formatter build failed:" >&2
  tail -20 "$BUILD_DIR/build.log" >&2
  exit 2
fi
BIN="$BUILD_DIR/main"
if [ ! -x "$BIN" ]; then
  echo "build succeeded but binary not found at $BIN" >&2
  exit 2
fi

SHARED_DIR="$ROOT/selfhost/shared"
COMPILER_DIR="$ROOT/selfhost/compiler"

fail=0
for entry in \
    "shared/lexer.intent:$SHARED_DIR/lexer.intent" \
    "shared/ast.intent:$SHARED_DIR/ast.intent" \
    "shared/parser.intent:$SHARED_DIR/parser.intent" \
    "formatter/format.intent:$FMT_DIR/format.intent" \
    "compiler/ir.intent:$COMPILER_DIR/ir.intent" \
    "compiler/lower.intent:$COMPILER_DIR/lower.intent" \
    "compiler/rustbe.intent:$COMPILER_DIR/rustbe.intent"; do
  label="${entry%%:*}"
  src="${entry#*:}"
  base=$(basename "$src" .intent)
  "$BIN" "$src" 2>/dev/null | perl -0777 -pe 's/\n\z//' > "$BUILD_DIR/$base.out"
  if diff -q "$BUILD_DIR/$base.out" "$src" >/dev/null; then
    printf '%-26s %s\n' "$label" "EQUAL"
  else
    printf '%-26s %s\n' "$label" "DIFF"
    fail=1
  fi
done

if [ "$fail" -ne 0 ]; then
  echo "self-format gate FAILED: a stage2 file is not a formatter fixpoint" >&2
  exit 1
fi
echo "self-format gate OK: all stage2 files are formatter fixpoints"
exit 0
