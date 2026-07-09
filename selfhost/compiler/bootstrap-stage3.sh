#!/usr/bin/env bash
#
# selfhost/compiler/bootstrap-stage3.sh — Phase 56 slice 4: the stage3 bootstrap.
#
# The end-to-end proof that the self-hosted compiler is a true fixpoint:
#
#   stage1 = the Go compiler (intentc).
#   stage2 = the Intent compiler, compiled by stage1
#            (stage1 emits Rust from compile_main.intent -> cargo -> stage2 binary).
#   stage3 = the Intent compiler, compiled by STAGE2
#            (stage2 emits Rust from compile_main.intent -> cargo -> stage3 binary).
#
# Then verify stage3 reproduces the ENTIRE toolchain's source byte-equal with stage1.
# Because diff-emit-self already shows stage2's emit == stage1's emit, stage3 is built
# from the same bytes as stage2 and must behave identically; this script proves it
# end-to-end by actually building stage3 and running it over the toolchain.
#
# Requires cargo. Slow (two cargo builds). Own gate; not in `make validate`.
#
# Usage:  ./selfhost/compiler/bootstrap-stage3.sh   (from anywhere)
#         make bootstrap-stage3
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
cd "$ROOT"
INTENTC="$ROOT/intentc"
COMPILER="$ROOT/selfhost/compiler/compile_main.intent"

if [ ! -x "$INTENTC" ]; then
  echo "intentc not built; running make build..." >&2
  make build >/dev/null 2>&1 || { echo "make build failed" >&2; exit 2; }
fi
if ! command -v cargo >/dev/null 2>&1; then
  echo "cargo not found — the stage3 bootstrap needs a Rust toolchain" >&2
  exit 2
fi

WORK=$(mktemp -d "${TMPDIR:-/tmp}/intent-bootstrap.XXXXXX")
trap 'rm -rf "$WORK"' EXIT

# --- stage2: stage1 compiles the Intent compiler ---------------------------------
echo "[1/4] building stage2 (stage1 compiles compile_main.intent)..."
if ! ( cd "$WORK" && "$INTENTC" build --target rust "$COMPILER" ) >"$WORK/s2build.log" 2>&1 \
   || [ ! -x "$WORK/compile_main" ]; then
  echo "stage2 build failed:" >&2; tail -20 "$WORK/s2build.log" >&2; exit 1
fi
STAGE2="$WORK/compile_main"

# --- stage3 source: stage2 emits the Intent compiler -----------------------------
echo "[2/4] stage2 emits compile_main.intent (the stage3 source)..."
export INTENT_STAGE2_COMPILE="$STAGE2"
if ! ( cd "$WORK" && "$INTENTC" build --emit --self-hosted "$COMPILER" ) >"$WORK/s3emit.log" 2>&1 \
   || [ ! -f "$WORK/compile_main.rs" ]; then
  echo "stage2 emit of the compiler failed:" >&2; tail -20 "$WORK/s3emit.log" >&2; exit 1
fi

# --- stage3: cargo builds stage2's emit ------------------------------------------
echo "[3/4] building stage3 (cargo compiles stage2's emit)..."
S3="$WORK/stage3"
mkdir -p "$S3/src"
cp "$WORK/compile_main.rs" "$S3/src/main.rs"
cat > "$S3/Cargo.toml" <<'TOML'
[package]
name = "intent_output"
version = "0.1.0"
edition = "2021"
TOML
# --release to match intentc's own native build (compiler.go: `cargo build --release`),
# so stage3 emits at the same speed stage2 does (a debug build is ~20x slower).
if ! ( cd "$S3" && cargo build --release --quiet ) >"$WORK/s3build.log" 2>&1 \
   || [ ! -x "$S3/target/release/intent_output" ]; then
  echo "stage3 cargo build failed:" >&2; tail -30 "$WORK/s3build.log" >&2; exit 1
fi
STAGE3="$S3/target/release/intent_output"

# --- verify: stage3 reproduces the whole toolchain byte-equal with stage1 --------
echo "[4/4] verifying stage3 emits the toolchain byte-equal with stage1..."
export INTENT_STAGE2_COMPILE="$STAGE3"
CORPUS=(
  "selfhost/compiler/compile_main.intent"
  "selfhost/checker/check_main.intent"
  "selfhost/formatter/main.intent"
  "selfhost/linter/lint_main.intent"
)
fail=0
pass=0
for f in "${CORPUS[@]}"; do
  base=$(basename "$f" .intent)
  w=$(mktemp -d "${TMPDIR:-/tmp}/intent-s3-$base.XXXXXX")
  if ! ( cd "$w" && "$INTENTC" build --emit "$ROOT/$f" ) >/dev/null 2>&1 || [ ! -f "$w/$base.rs" ]; then
    printf '  %-42s %s\n' "$f" "STAGE1-EMIT-ERR"; fail=1; rm -rf "$w"; continue
  fi
  mv "$w/$base.rs" "$w/stage1.rs"
  if ! ( cd "$w" && "$INTENTC" build --emit --self-hosted "$ROOT/$f" ) >/dev/null 2>&1 || [ ! -f "$w/$base.rs" ]; then
    printf '  %-42s %s\n' "$f" "STAGE3-EMIT-ERR"; fail=1; rm -rf "$w"; continue
  fi
  if diff -q "$w/stage1.rs" "$w/$base.rs" >/dev/null; then
    printf '  %-42s %s\n' "$f" "EQUAL"; pass=$((pass + 1))
  else
    printf '  %-42s %s\n' "$f" "DIVERGE"; diff "$w/stage1.rs" "$w/$base.rs" | head -20; fail=1
  fi
  rm -rf "$w"
done

echo ""
if [ "$fail" -ne 0 ]; then
  echo "Summary: stage3 bootstrap FAILED" >&2; exit 1
fi
echo "Summary: $pass/${#CORPUS[@]} — stage3 (built by stage2) reproduces the whole toolchain byte-for-byte. Bootstrap is a fixpoint."
exit 0
