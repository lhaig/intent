# Phase 31: String Indexing Primitives + `Char` Type

**Status:** Planning (ADRs 0040, 0041 accepted 2026-06-03)
**Milestone:** v1.2 — Self-Improvement Foundations
**Decision:** [ADR 0041](../../docs/decisions/0041-string-indexing-and-char-type.md)
**Strategic frame:** [ADR 0040](../../docs/decisions/0040-self-hosted-formatter-strategy.md) — first language extension for the stage2 formatter

## Goal

Add the minimum viable set of string and `Char` operations to stage1 so a hand-rolled lexer can be written in Intent. No stage2 Intent code lands in this phase — this is a host-runtime phase that *unblocks* Phase 32.

## Success Criteria

- [ ] `Char` is a registered primitive type; checker accepts it everywhere `Int`/`Bool` are accepted (param/return/let/field).
- [ ] Char literals parse: `'a'`, `'0'`, `'\n'`, `'\t'`, `'\\'`, `'\''`, `'\"'`, `'\u{1234}'`. Invalid forms (empty, multi-char, surrogate, out-of-range) are checker errors with file:line:col.
- [ ] String indexing: `s[i]` produces `Char` when `s: String`, `i: Int`. Out-of-bounds is a runtime contract violation on every backend.
- [ ] String slicing: `s[i..j]` produces `String`. Same out-of-bounds semantics. Empty slice `s[k..k]` returns `""`.
- [ ] `len(s)` accepts `String` and returns codepoint count (was Array/Map only).
- [ ] Char built-ins land: `to_codepoint`, `Char.from_codepoint`, `is_digit`, `is_alpha`, `is_alphanumeric`, `is_whitespace`, `is_lowercase`, `is_uppercase`, `to_string`. All ASCII-only per ADR 0041 §2.
- [ ] Comparisons `==`, `!=`, `<`, `<=`, `>`, `>=` work on `Char`.
- [ ] All three backends (rust, js, wasm) emit working code for every new operation. Differential `intentc test --all-targets` passes on a new feature-coverage example.
- [ ] Z3 verifier reasons about `Char` as a bounded integer; a contract using char predicates verifies on a test case.
- [ ] No regression: `make validate` green; `examples/*.intent` continue to type-check and run.
- [ ] `INTENT.md` documents the new surface; `docs/DESIGN.md` updates the type system table; `docs/grammar.ebnf` documents char literals + slice syntax.

## Reference

- [ADR 0041](../../docs/decisions/0041-string-indexing-and-char-type.md) — full design
- [ADR 0040](../../docs/decisions/0040-self-hosted-formatter-strategy.md) — strategic frame
- Existing primitive registration: `internal/checker/types.go` — `TypeInt`, `TypeBool`, `TypeString` for the pattern
- Existing IndexExpr: `internal/checker/checker.go:2697` `checkIndexExpr` — relax to accept String
- Existing len() handler: `internal/checker/checker.go:1741` — relax to accept String
- Lexer / parser: `internal/lexer/`, `internal/parser/` — add char-literal tokenisation + parse rule
- Backends: `internal/rustbe/`, `internal/jsbe/`, `internal/wasmbe/` — emit per-target runtime helpers
- Z3 bindings: `internal/verify/` — encode Char as bounded Int

## Tasks

### 31.1 Lexer: char literal token

**Files:** `internal/lexer/lexer.go`, `internal/lexer/lexer_test.go`

- Add `TokenChar` kind carrying the parsed scalar value.
- Recognise `'<content>'` where `<content>` is one of: a single non-`\` non-`'` char; an escape sequence (`\n`, `\t`, `\r`, `\\`, `\'`, `\"`, `\0`, `\u{HEX}`).
- Reject empty `''`, multi-char `'ab'`, unterminated `'a`, bad escape `'\q'`, out-of-range `'\u{110000}'`, surrogate `'\u{D800}'`.
- Carry the codepoint value (as `int32` or `int64` — pick one and stick with it) on the token.

**Acceptance:** Table-driven tests for every form above; round-trip via existing lexer error harness for invalid forms.

### 31.2 AST + parser: CharLit and slice syntax

**Files:** `internal/ast/expr.go` (or wherever literals live), `internal/parser/parser.go`, `internal/parser/parser_test.go`

- New AST node `CharLit { Value int32; Line, Col int }`.
- Parser builds `CharLit` from `TokenChar`.
- Slice expression: extend the existing `IndexExpr` handling so `s[a..b]` parses where `a..b` is the existing `RangeExpr`. Either reuse `IndexExpr.Index` holding a `RangeExpr`, or introduce `SliceExpr { Object; Start; End }`. Pick the path that touches the fewest backends — likely letting `IndexExpr.Index` be a `RangeExpr` and disambiguating in the checker.

**Acceptance:** Parser tests for each char literal escape form; parser tests for `s[i]`, `s[i..j]`, `s[..j]`, `s[i..]` (the open-ended forms can either land here or be deferred to a follow-up; if deferred, the ADR's choice is documented).

### 31.3 Checker: `Char` primitive + relaxed IndexExpr + len()

**Files:** `internal/checker/types.go`, `internal/checker/checker.go`, `internal/checker/checker_test.go`

- Register `TypeChar` analogously to `TypeBool`. Same defaulting rules.
- Allow `Char` in field types, param types, return types, let bindings.
- Comparison operators: extend the existing `==`/`!=`/`<`/`<=`/`>`/`>=` handlers to accept `Char`.
- `checkIndexExpr`: accept `String → Char`; accept `String[RangeExpr] → String`.
- `len()`: accept `String` (returns Int, codepoint count).
- New built-in methods on `Char`: `to_codepoint`, `is_digit`, `is_alpha`, `is_alphanumeric`, `is_whitespace`, `is_lowercase`, `is_uppercase`, `to_string` — register via the same mechanism as `Int.to_string()`.
- New built-in static method `Char.from_codepoint(n: Int) returns Result<Char, String>` — register as a free function or as a `Char.` method per existing conventions.

**Acceptance:** Tests for: `let c: Char = 'a';` typechecks; `let i: Int = c.to_codepoint();` typechecks; `let b: Bool = c.is_digit();` typechecks; `let s: String = "hi"; let c2: Char = s[0];` typechecks; `let sub: String = s[0..1];` typechecks; `len("hi") == 2` typechecks; cross-type comparison `c == 65` is rejected (no implicit conversion).

### 31.4 IR + backend: Rust

**Files:** `internal/ir/`, `internal/rustbe/rustbe.go`, runtime helper file

- IR: lower `CharLit` and slice expressions; carry codepoint values through.
- Rust: emit Intent `Char` as Rust `char`. Indexed strings precompute `Vec<char>` per indexed binding (or per call site — pick the simplest emission strategy and document trade-off).
- Helper: `__intent_string_index(s: &str, i: i64) -> char` panicking on out-of-bounds; `__intent_string_slice(s: &str, i: i64, j: i64) -> String`; `__intent_string_len_chars(s: &str) -> i64`.
- Char predicates: emit Rust `c.is_ascii_digit()` etc. — match the ASCII-only contract.
- `Char.from_codepoint`: emit `std::char::from_u32(n as u32)` with `Result` wrapping.

**Acceptance:** Generated Rust compiles cleanly; cargo run produces expected output; out-of-bounds aborts with the standard Intent panic message format.

### 31.5 IR + backend: JS

**Files:** `internal/jsbe/jsbe.go`, JS runtime helpers

- Char as `number` (codepoint). Helper layer: indexing precomputes `Array.from(s)` per indexed binding.
- `__intent_char_at(s, i)`, `__intent_string_slice(s, i, j)`, `__intent_string_len_chars(s)`.
- Char predicates inlined as ASCII range checks.
- `Char.from_codepoint`: `String.fromCodePoint(n)` with surrogate/out-of-range checks producing `Result`.

**Acceptance:** `intentc test --target js` green on the feature-coverage example.

### 31.6 IR + backend: WASM

**Files:** `internal/wasmbe/wasmbe.go`, WASM runtime helpers

- WASM has no native string; backends-with-strings already encode strings as `(ptr, len)` in linear memory. Extend the runtime ABI with codepoint index helpers; precompute a small offset table per indexed string at first access (cache in a thread-local map keyed by pointer).
- Char as `i32`.
- Predicates inlined.

**Acceptance:** `intentc test --target wasm` green on the feature-coverage example.

### 31.7 Z3 verifier integration

**Files:** `internal/verify/`, `internal/verify/_test.go`

- Encode `Char` as a bounded integer (`(declare-const c Int)` with `(assert (and (>= c 0) (<= c 1114111)))` plus surrogate exclusion).
- Translate the ASCII predicates: `is_digit(c) ↔ c >= 48 and c <= 57`, etc.
- Translate `to_codepoint(c) = c` and `from_codepoint(n)` as conditional construct.
- Translate indexing: `s[i]` as an uninterpreted function `string_index : String × Int → Int` with axioms about `len`.

**Acceptance:** A verify test case proves a `requires`/`ensures` chain over a char predicate: e.g. `function f(c: Char) returns Bool requires c.is_digit() ensures result == true { return c >= '0' and c <= '9'; }` verifies.

### 31.8 Feature-coverage example + differential test

**Files:** `examples/char_string_demo.intent`, `Makefile` (if needed)

- A single `.intent` file exercising every new construct (char literals, indexing, slicing, len, all predicates, comparisons, contract usage).
- The file is added to `TESTED_EXAMPLES` so `intentc test --all-targets` runs it on every backend, catching cross-target divergence.

**Acceptance:** `make validate` green; the example produces identical output on all three backends.

### 31.9 Docs

**Files:** `INTENT.md`, `docs/DESIGN.md`, `docs/grammar.ebnf`, `ops/NEXT-STEPS.md`

- `INTENT.md`: add a Strings & Chars section covering literals, indexing, slicing, predicates; document the codepoint-indexed model and the v1 ASCII-only predicate caveat.
- `docs/DESIGN.md`: add `Char` to the type-system table; document `s[i]` and `s[i..j]` semantics.
- `docs/grammar.ebnf`: char-literal production and the slice-expression production.
- `ops/NEXT-STEPS.md`: mark Phase 31 shipped; Phase 32 lexer becomes the next move.

**Acceptance:** Docs reflect Phase 31 reality; no dangling claims about strings being opaque.

## Validation

- `make validate` green at every commit boundary (per-task).
- `intentc test --all-targets examples/char_string_demo.intent` green on rust + js + wasm.
- New checker tests, parser tests, lexer tests, verify test all green.
- A small `verify_char_demo.intent` example added with a verified contract using char predicates.

## Out of scope

- String search/replace (`starts_with`, `contains`, `split`, `trim`).
- String case conversion (`to_lowercase`, `to_uppercase`).
- String parsing (`s.to_int()`, `s.to_float()`).
- Unicode-aware predicates (full Unicode categories).
- Open-ended slices `s[i..]`, `s[..j]` *unless* they fall naturally out of 31.2; otherwise deferred.
- Stage2 Intent code — that's Phase 32+.

## Estimated size

~1.5-2k LOC across lexer, parser, checker, IR, three backends, verify, plus tests. Comparable in surface to Phase 30. The work spans more files than Phase 30 but each file's diff is smaller.
