#!/usr/bin/env bash
#
# selfhost/checker/difftest-check.sh — Phase 45.10 differential test.
#
# Compares the stage2 (Intent) checker against stage1 `intentc check` across:
#   - examples/*.intent  (22 files; the ADR 0052 baseline corpus)
#   - selfhost/checker/check-fixtures/*.intent  (one fixture per implemented check)
#
# For each file:
#   1. Run stage1: ./intentc check <file>   (stdout+stderr combined via 2>&1)
#   2. Run stage2: ./check_main <file>      (built once into a temp dir)
#   3. PASS iff output is byte-identical.
#   4. PARSE-ERR if stage2 exits with a parse-error message (stage2 parser does
#      not support the file's constructs — no such fixture expected currently).
#
# NOTE: stage2 implements a first-slice subset (no type inference). Fixtures are
# crafted so stage1 emits ONLY the target check's error, never a type/other error
# that stage2 lacks. See prd-phase-45 §B for fixture design rules.
#
# Usage:  ./selfhost/checker/difftest-check.sh            (from anywhere)
#         make diff-checker
#
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
INTENTC="$ROOT/intentc"
CHECKER_DIR="$ROOT/selfhost/checker"
FIXTURES_DIR="$CHECKER_DIR/check-fixtures"

if [ ! -x "$INTENTC" ]; then
  echo "intentc not built; running make build..." >&2
  make -C "$ROOT" build >/dev/null 2>&1 || { echo "make build failed" >&2; exit 2; }
fi

# Build the stage2 check binary once into a temp dir.
BUILD_DIR=$(mktemp -d "${TMPDIR:-/tmp}/intent-diffcheck.XXXXXX")
trap 'rm -rf "$BUILD_DIR"' EXIT

if ! ( cd "$BUILD_DIR" && "$INTENTC" build --target rust "$CHECKER_DIR/check_main.intent" ) >"$BUILD_DIR/build.log" 2>&1; then
  echo "stage2 check_main build failed:" >&2
  tail -20 "$BUILD_DIR/build.log" >&2
  exit 2
fi
CHECKBIN="$BUILD_DIR/check_main"
if [ ! -x "$CHECKBIN" ]; then
  echo "build succeeded but binary not found at $CHECKBIN" >&2
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
pass=0; diverge=0; parseerr=0

printf '%-50s %s\n' "FILE" "RESULT"
printf '%-50s %s\n' "----" "------"

for f in "${FILES[@]}"; do
  label=$(basename "$(dirname "$f")")/$(basename "$f")

  s1=$("$INTENTC" check "$f" 2>&1) || true
  s2=$("$CHECKBIN" "$f" 2>&1) || true

  # Detect stage2 parse error (check_main exits 1 and prints "parse error: ...")
  if [[ "$s2" == parse\ error:* ]]; then
    printf '%-50s %s\n' "$label" "PARSE-ERR"
    echo "  stage2: $s2"
    parseerr=$((parseerr + 1))
    continue
  fi

  if [ "$s1" = "$s2" ]; then
    printf '%-50s %s\n' "$label" "PASS"
    pass=$((pass + 1))
  else
    printf '%-50s %s\n' "$label" "DIVERGE"
    diff <(echo "$s1") <(echo "$s2") | head -20
    diverge=$((diverge + 1))
  fi
done

total=${#FILES[@]}
echo ""
echo "Summary: $pass/$total PASS, $diverge diverged, $parseerr parse-err"

if [ "$diverge" -gt 0 ] || [ "$parseerr" -gt 0 ]; then
  exit 1
fi
exit 0
