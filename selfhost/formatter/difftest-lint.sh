#!/usr/bin/env bash
#
# selfhost/formatter/difftest-lint.sh — Phase 43.12 differential test.
#
# Compares the stage2 (Intent) linter against stage1 `intentc lint` across:
#   - examples/*.intent  (22 files; the ADR 0050 baseline corpus)
#   - selfhost/formatter/lint-fixtures/*.intent  (4 fixture files; non-corpus rules)
#
# For each file:
#   1. Run stage1: ./intentc lint <file>
#   2. Run stage2: ./lint_main <file>  (built once into a temp dir)
#   3. PASS iff stdout is byte-identical.
#   4. PARSE-ERR if stage2 exits with a parse-error message (stage2 parser
#      does not support the file's constructs — the extern syntax split, see
#      NOTE below).
#
# NOTE: R4 (extern no-contracts) cannot be gated by this harness because
# stage1 uses `extern function ... from "path";` syntax while stage2 uses
# `extern "target" function ...;`. No single source file is valid for both
# parsers. R4 is verified by lint_test.intent (unit tests in stage2) instead.
# A PARSE-ERR outcome on a fixture file is therefore treated as "SKIP" if it
# is listed in SKIP_PARSE_ERR below.
#
# Usage:  ./selfhost/formatter/difftest-lint.sh            (from anywhere)
#         make diff-linter
#
set -uo pipefail

# --- Fixtures whose stage2 PARSE-ERR is expected (extern syntax split) -------
# Empty by default — no such fixture currently exists.
SKIP_PARSE_ERR="${SKIP_PARSE_ERR:-}"

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
INTENTC="$ROOT/intentc"
FMT_DIR="$ROOT/selfhost/formatter"
FIXTURES_DIR="$FMT_DIR/lint-fixtures"

if [ ! -x "$INTENTC" ]; then
  echo "intentc not built; running make build..." >&2
  make -C "$ROOT" build >/dev/null 2>&1 || { echo "make build failed" >&2; exit 2; }
fi

# Build the stage2 lint binary once into a temp dir.
BUILD_DIR=$(mktemp -d "${TMPDIR:-/tmp}/intent-difflint.XXXXXX")
trap 'rm -rf "$BUILD_DIR"' EXIT

if ! ( cd "$BUILD_DIR" && "$INTENTC" build --target rust "$FMT_DIR/lint_main.intent" ) >"$BUILD_DIR/build.log" 2>&1; then
  echo "stage2 lint_main build failed:" >&2
  tail -20 "$BUILD_DIR/build.log" >&2
  exit 2
fi
LINTBIN="$BUILD_DIR/lint_main"
if [ ! -x "$LINTBIN" ]; then
  echo "build succeeded but binary not found at $LINTBIN" >&2
  exit 2
fi

# Collect files: examples then fixtures.
FILES=()
while IFS= read -r f; do FILES+=("$f"); done < <(find "$ROOT/examples" -maxdepth 1 -name '*.intent' | sort)
if [ -d "$FIXTURES_DIR" ]; then
  while IFS= read -r f; do FILES+=("$f"); done < <(find "$FIXTURES_DIR" -maxdepth 1 -name '*.intent' | sort)
fi
if [ "${#FILES[@]}" -eq 0 ]; then
  echo "no intent files found to test" >&2
  exit 2
fi

# Compare stage1 vs stage2 per file.
pass=0; diverge=0; parseerr=0; skipped=0

printf '%-36s %s\n' "FILE" "RESULT"
printf '%-36s %s\n' "----" "------"

for f in "${FILES[@]}"; do
  base=$(basename "$f" .intent)
  label=$(basename "$(dirname "$f")")/$(basename "$f")

  s1=$("$INTENTC" lint "$f" 2>&1) || true
  s2=$("$LINTBIN" "$f" 2>&1) || true

  # Detect stage2 parse error (lint_main exits 3 and prints "parse error: ...")
  if [[ "$s2" == parse\ error:* ]]; then
    if [[ " $SKIP_PARSE_ERR " == *" $base "* ]]; then
      printf '%-36s %s\n' "$label" "SKIP (expected parse-err)"
      skipped=$((skipped + 1))
    else
      printf '%-36s %s\n' "$label" "PARSE-ERR"
      echo "  stage2: $s2"
      parseerr=$((parseerr + 1))
    fi
    continue
  fi

  if [ "$s1" = "$s2" ]; then
    printf '%-36s %s\n' "$label" "PASS"
    pass=$((pass + 1))
  else
    printf '%-36s %s\n' "$label" "DIVERGE"
    diff <(echo "$s1") <(echo "$s2") | head -20
    diverge=$((diverge + 1))
  fi
done

total=${#FILES[@]}
echo ""
echo "Summary: $pass/$total PASS, $diverge diverged, $parseerr parse-err, $skipped skipped"

if [ "$diverge" -gt 0 ] || [ "$parseerr" -gt 0 ]; then
  exit 1
fi
exit 0
