#!/usr/bin/env bash
#
# selfhost/formatter/difftest.sh — Phase 42 differential test.
#
# Compares the stage2 (Intent) formatter against stage1 `intentc fmt` across the
# examples/*.intent corpus. For each single-file example:
#
#   1. stage1-canonicalize a copy (`intentc fmt`) — this IS intentc fmt's output.
#   2. run the stage2 formatter (format_program(parse(.))) on that canonical copy
#      via an in-language probe (absolute paths — `intentc test` runs from a temp
#      cwd, so relative read_file would silently skip).
#   3. PASS iff the stage2 output reproduces the canonical form byte-for-byte,
#      i.e. the stage2 formatter agrees with `intentc fmt`.
#
# Prints a per-file table (PASS / DIVERGE / PARSE-ERR) and a summary. Exits
# non-zero if any example is not PASS, unless it is listed in ALLOW_DIVERGE
# below (use for fixtures with a known, accepted divergence).
#
# Usage:  ./selfhost/formatter/difftest.sh            (from anywhere)
#         make diff-formatter
#
set -uo pipefail

# --- Known-divergence allow-list (space-separated basenames, no extension) ----
# Empty by default: every example should eventually agree with intentc fmt.
ALLOW_DIVERGE="${ALLOW_DIVERGE:-}"

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
cd "$ROOT"
INTENTC="$ROOT/intentc"
FMT_DIR="$ROOT/selfhost/formatter"

if [ ! -x "$INTENTC" ]; then
  echo "intentc not built; running make build..." >&2
  make build >/dev/null 2>&1 || { echo "make build failed" >&2; exit 2; }
fi

TMP=$(mktemp -d "${TMPDIR:-/tmp}/intent-difftest.XXXXXX")
CANON="$TMP/canon"
RES="$TMP/res"
mkdir -p "$CANON" "$RES"
trap 'rm -rf "$TMP"' EXIT

# Collect the single-file examples (directories like multi_file/ are out of
# scope). Portable array fill — macOS ships bash 3.2, which lacks `mapfile`.
EXAMPLES=()
while IFS= read -r line; do
  EXAMPLES+=("$line")
done < <(find "$ROOT/examples" -maxdepth 1 -name '*.intent' | sort)
if [ "${#EXAMPLES[@]}" -eq 0 ]; then
  echo "no examples/*.intent found" >&2
  exit 2
fi

# Step 1: stage1-canonicalize a copy of each example.
for ex in "${EXAMPLES[@]}"; do
  base=$(basename "$ex" .intent)
  cp "$ex" "$CANON/$base.intent"
  if ! "$INTENTC" fmt "$CANON/$base.intent" >/dev/null 2>&1; then
    # stage1 itself could not format it; record and skip the stage2 check.
    printf 'STAGE1ERR %s\n' "$base" > "$RES/$base.txt"
    rm -f "$CANON/$base.intent"
  fi
done

# Step 2: generate the in-language probe over the canonical copies.
PROBE="$FMT_DIR/_difftest_probe.intent"
{
  echo 'module difftest_probe version "0.1.0";'
  echo ''
  echo 'import "format.intent";'
  echo 'import "parser.intent";'
  echo 'import "ast.intent";'
  echo 'import "lexer.intent";'
  echo ''
  echo '// Differential check: stage2 format_program(parse(canon)) vs canon, where'
  echo '// canon is intentc fmt'"'"'s output. EQUAL => stage2 agrees with stage1.'
  echo 'public function difftest_one(src_path: String, out_path: String) returns Void {'
  echo '    let r: Result<String, String> = read_file(src_path);'
  echo '    let src: String = match r {'
  echo '        Ok(s) => s,'
  echo '        Err(_) => "",'
  echo '    };'
  echo '    if src == "" {'
  echo '        write_file(out_path, "UNREAD");'
  echo '        return;'
  echo '    }'
  echo '    let prog: Program = formatter_parser.parse(src);'
  echo '    if prog.error != "" {'
  echo '        write_file(out_path, "PARSEERR :: " + prog.error);'
  echo '        return;'
  echo '    }'
  echo '    let out: String = formatter_format.format_program(prog);'
  echo '    if out == src {'
  echo '        write_file(out_path, "PASS");'
  echo '        return;'
  echo '    }'
  echo '    write_file(out_path, "DIVERGE");'
  echo '}'
  echo ''
  echo 'test "differential: stage2 formatter vs intentc fmt over examples" {'
  for ex in "${EXAMPLES[@]}"; do
    base=$(basename "$ex" .intent)
    [ -f "$CANON/$base.intent" ] || continue
    printf '    difftest_one("%s", "%s");\n' "$CANON/$base.intent" "$RES/$base.txt"
  done
  echo '    assert_eq(1, 1);'
  echo '}'
} > "$PROBE"

# Step 3: run the probe (rust target; no cargo needed beyond what stage2 tests use).
if ! "$INTENTC" test --target rust "$PROBE" >"$TMP/testlog" 2>&1; then
  echo "stage2 probe failed to run:" >&2
  tail -20 "$TMP/testlog" >&2
  rm -f "$PROBE"
  exit 2
fi
rm -f "$PROBE"

# Step 4: tally + report.
pass=0; diverge=0; parseerr=0; other=0; allowed=0
printf '%-26s %s\n' "EXAMPLE" "RESULT"
printf '%-26s %s\n' "-------" "------"
for ex in "${EXAMPLES[@]}"; do
  base=$(basename "$ex" .intent)
  result=$(cat "$RES/$base.txt" 2>/dev/null || echo "NORESULT")
  case "$result" in
    PASS) printf '%-26s %s\n' "$base" "PASS"; pass=$((pass+1)) ;;
    PARSEERR*) printf '%-26s %s\n' "$base" "PARSE-ERR ${result#PARSEERR :: }"; parseerr=$((parseerr+1)) ;;
    DIVERGE)
      if [[ " $ALLOW_DIVERGE " == *" $base "* ]]; then
        printf '%-26s %s\n' "$base" "DIVERGE (allowed)"; allowed=$((allowed+1))
      else
        printf '%-26s %s\n' "$base" "DIVERGE"; diverge=$((diverge+1))
      fi ;;
    STAGE1ERR) printf '%-26s %s\n' "$base" "STAGE1-ERR (skipped)"; other=$((other+1)) ;;
    *) printf '%-26s %s\n' "$base" "$result"; other=$((other+1)) ;;
  esac
done

total=${#EXAMPLES[@]}
echo ""
echo "Summary: $pass/$total PASS, $diverge diverged, $parseerr parse-err, $allowed allowed-diverge, $other other"

# Gate: fail if any non-allowed divergence, parse error, or other anomaly.
if [ "$diverge" -gt 0 ] || [ "$parseerr" -gt 0 ] || [ "$other" -gt 0 ]; then
  exit 1
fi
exit 0
