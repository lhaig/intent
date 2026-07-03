#!/usr/bin/env bash
#
# selfhost/checker/selfcheck-check.sh — Phase 54 self-hosting readiness gate.
#
# Verifies the stage2 checker produces byte-identical output to the stage1 (Go)
# checker on the compiler's OWN multi-file source — the real self-hosting
# workload. This is the counterpart to `make diff-checker` (which only exercises
# single-file examples + fixtures) and `make selfcheck-formatter` (the formatter
# fixpoint gate).
#
# It drives the integrated CLI for both stages:
#     intentc check <f>                 (stage1, Go; resolves imports via CheckProject)
#     intentc check --self-hosted <f>   (stage2, Intent; harness discovers the
#                                        import closure and check_main merges it)
# so it covers the whole path: module discovery, cross-module type resolution,
# and the merge that eliminates the `unknown type` / `undeclared variable`
# false positives the single-file stage2 checker used to emit.
#
# NOT wired into `make validate`: the stage2 checker (compiled Intent) is slow on
# the large merged programs (tens of seconds each), so this runs as its own gate.
#
# Usage:  ./selfhost/checker/selfcheck-check.sh   (from anywhere)
#         make selfcheck-checker
#
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
cd "$ROOT"
INTENTC="$ROOT/intentc"

if [ ! -x "$INTENTC" ]; then
  echo "intentc not built; running make build..." >&2
  make build >/dev/null 2>&1 || { echo "make build failed" >&2; exit 2; }
fi

# Core self-hosting source modules (the multi-file compiler itself). The
# single-file check-fixtures and examples are covered byte-for-byte by
# `make diff-checker`; this gate targets the real, import-bearing source.
MODULES=(
  "selfhost/shared/lexer.intent"
  "selfhost/shared/ast.intent"
  "selfhost/shared/parser.intent"
  "selfhost/checker/check.intent"
  "selfhost/checker/check_main.intent"
  "selfhost/linter/lint.intent"
  "selfhost/linter/lint_main.intent"
  "selfhost/formatter/format.intent"
  "selfhost/formatter/main.intent"
)

fail=0
pass=0
for f in "${MODULES[@]}"; do
  s1=$("$INTENTC" check "$f" 2>&1)
  s2=$("$INTENTC" check --self-hosted "$f" 2>&1)
  if [ "$s1" = "$s2" ]; then
    printf '%-40s %s\n' "$f" "PASS"
    pass=$((pass + 1))
  else
    printf '%-40s %s\n' "$f" "DIVERGE"
    diff <(printf '%s\n' "$s1") <(printf '%s\n' "$s2") | head -20
    fail=1
  fi
done

echo ""
if [ "$fail" -ne 0 ]; then
  echo "Summary: self-hosting checker gate FAILED (stage2 diverges from stage1 on real source)" >&2
  exit 1
fi
echo "Summary: $pass/${#MODULES[@]} PASS — stage2 checker matches stage1 on all core self-hosting source"
exit 0
