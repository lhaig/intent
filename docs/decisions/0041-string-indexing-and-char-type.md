# 0041: String Indexing and `Char` Type (First Self-Hosting Gap)

**Date:** 2026-06-03
**Status:** accepted
**Phase:** v1.2 — Self-Improvement Foundations (Phase 31 — first language extension for stage2 formatter)

## Context

[ADR 0040](0040-self-hosted-formatter-strategy.md) committed to a stage2 (Intent-in-Intent) formatter. The first concrete sub-goal is a lexer, which converts Intent source text to a token stream. Today's Intent cannot express that:

| Operation needed by a lexer | Today |
|---|---|
| `s[i]` to read the i-th character | Rejected — `IndexExpr` only accepts `Array<T>` ([checker.go:2698](../../internal/checker/checker.go)). |
| `len(s)` for character count | Rejected — `len()` only accepts `Array` or `Map` ([checker.go:1741](../../internal/checker/checker.go)). |
| `s[i..j]` for token text extraction | Rejected — `RangeExpr` only appears in `for-in` loops. |
| Char literal `'a'` for keyword/punct comparison | No syntax. |
| `is_digit('0')`, `is_alpha('A')`, `is_whitespace(' ')` | No builtins. |
| `c.to_codepoint() : Int` for range checks | No method. |

This ADR specifies the minimum viable surface to write a hand-rolled lexer. Larger string operations (search, split, case conversion, parsing) are *out of scope* — they get their own ADRs as parser phases surface the need.

### Precedent

| Language | Char model | Indexing semantics | Notes |
|---|---|---|---|
| **Rust** | `char` is a Unicode scalar value (32-bit). | Byte-indexed; codepoint iteration via `s.chars()`. | `s[i]` doesn't compile — you have to choose `bytes()` or `chars()`. Verbose but explicit. |
| **Go** | `rune` is an alias for `int32`. | Byte-indexed for `s[i]`; codepoint iteration via `for range s`. | `s[i]` returns a `byte`, not a `rune` — a famous footgun. |
| **Python 3** | Strings are codepoint sequences; `s[i]` returns a one-char string. | Codepoint-indexed. | No separate char type. Mental model is simple; performance pays a constant factor for the indexing. |
| **Swift** | `Character` is an extended grapheme cluster. | Grapheme-cluster-indexed via `String.Index`. | Most correct for human languages; slowest indexing. Not codepoint-indexed. |
| **JavaScript** | Strings are UTF-16 code units. | UTF-16-indexed; surrogate pairs leak through. | The classic "string length lies about emojis" model. |
| **Java** | `char` is a 16-bit UTF-16 code unit. | UTF-16-indexed. | Same problem as JS for non-BMP characters. |
| **Ada** | `Character` is Latin-1; `Wide_Character` is BMP; `Wide_Wide_Character` is full Unicode. | Type-indexed. | Three string types is too much. |
| **D** | `char`/`wchar`/`dchar` (8/16/32-bit). | Type-indexed; `string` is UTF-8. | Like Ada — multiple character types complicates the surface. |
| **Dafny** | `char` is a Unicode scalar value. | Codepoint-indexed. | Contract-friendly: char predicates are pure functions; well-suited to Intent's verification stance. |

### What Intent should pick

Intent's design point is **contracts and correctness over ergonomics**. The chosen model is closest to **Rust + Dafny**:

- **Dedicated `Char` type** representing a Unicode scalar value. Not codepoint-as-Int — the type distinction lets `requires c == 'a'` carry weight in the checker and verifier without sprinkling range guards.
- **Codepoint-indexed** strings. `s[i]` returns the i-th Unicode scalar value, not the i-th byte. Mental model: "string = sequence of code points." Cost of slow indexing is acceptable for a v1 formatter; performance optimisations come later.
- **`len(s)` returns codepoint count.** Consistent with indexing.

Notably *not* picked: byte indexing (would force every lexer line to do UTF-8 decoding manually) or grapheme-cluster indexing (overkill for a programming-language tokenizer; would require ICU-grade data tables).

## Decision

### 1. `Char` type

A new primitive type:

- `Char` — a single Unicode scalar value (`U+0000` through `U+10FFFF`, excluding the surrogate range `U+D800..U+DFFF`).
- Distinct from `Int`. No implicit conversions in either direction.
- Equality and ordering comparators (`==`, `!=`, `<`, `<=`, `>`, `>=`) defined. Ordering is by codepoint value.
- Char literals: `'a'`, `'0'`, `'\n'`, `'\t'`, `'\\'`, `'\''`, `'\"'`, `'\u{1234}'` (Rust-style escape syntax for arbitrary codepoints).
- Default value (where Intent's defaulting rules apply): `'\0'`.

### 2. `Char` methods and built-ins

For Phase 31, the ASCII-friendly predicate set:

| Method / built-in | Returns | Definition |
|---|---|---|
| `c.to_codepoint()` | `Int` | The numeric codepoint. |
| `Char.from_codepoint(n: Int)` | `Result<Char, String>` | Validates the range; rejects surrogates. |
| `c.is_digit()` | `Bool` | True for `'0'..'9'`. ASCII-only in v1. |
| `c.is_alpha()` | `Bool` | True for `'a'..'z'` or `'A'..'Z'`. ASCII-only in v1. |
| `c.is_alphanumeric()` | `Bool` | True for digits or letters. ASCII-only in v1. |
| `c.is_whitespace()` | `Bool` | True for `' '`, `'\t'`, `'\n'`, `'\r'`. ASCII-only in v1. |
| `c.is_lowercase()` | `Bool` | True for `'a'..'z'`. ASCII-only in v1. |
| `c.is_uppercase()` | `Bool` | True for `'A'..'Z'`. ASCII-only in v1. |
| `c.to_string()` | `String` | The one-character String containing `c`. |

The "ASCII-only in v1" caveat is deliberate. The lexer for `.intent` files needs ASCII predicates only — identifiers and keywords are ASCII. Unicode-aware predicates are a follow-up ADR if and when we want to lex non-ASCII identifiers.

### 3. `String` indexing and slicing

| Operation | Returns | Semantics |
|---|---|---|
| `s[i]` (where `s: String`, `i: Int`) | `Char` | Codepoint at position `i`. Out-of-bounds is a runtime contract violation (same panic semantics as `Array<T>[i]`). |
| `s[i..j]` | `String` | Half-open codepoint slice, `i..j` exclusive. `i == j` returns the empty string. Out-of-bounds is a runtime contract violation. |
| `len(s)` | `Int` | Codepoint count. |

Bounds:
- `i >= 0` is checked at runtime (contract).
- `i < len(s)` for indexing; `i <= j <= len(s)` for slicing.
- Negative-index Python-style indexing is **not** supported. (Intent's design point: every operation should have one obvious meaning. Wrap-around indices break that.)

### 4. Char literal lexer rules

Char literals are single quotes around exactly one Unicode scalar value:

```
'a'         // U+0061
'0'         // U+0030
'\n'        // U+000A
'\t'        // U+0009
'\\'        // U+005C
'\''        // U+0027
'\"'        // U+0022 (escape allowed but optional; '"' also fine)
'\u{1234}'  // arbitrary codepoint
```

Invalid forms (rejected by the parser):

- `''` (empty char)
- `'ab'` (two-char content)
- `'\u{D800}'` (surrogate codepoint)
- `'\u{110000}'` (out of Unicode range)

### 5. String construction

For Phase 31, the lexer can build token text using:

- The slice form `s[i..j]` (cheapest for substring extraction).
- The `+` operator already concatenates Strings.
- `c.to_string()` to lift a Char into a String for concatenation.

Iteration-style construction (e.g. `Array<Char> -> String`) is deferred. The lexer's hot path is slicing, not iteration.

### 6. Backend semantics

Each backend implements the abstraction faithfully:

| Backend | String repr | Indexing impl |
|---|---|---|
| **Rust** | `String` (UTF-8). Lex-time conversion to `Vec<char>` for indexed access, or `s.chars().nth(i)` on each indexing. Phase 31 picks `Vec<char>` precomputed per-indexed-string for O(1) access. |
| **JS** | UTF-16 strings. Indexed access via `Array.from(s)` precomputed once; v1 accepts the constant-factor overhead. |
| **WASM** | UTF-8 stored in linear memory. Indexed access requires precomputed offset table per string; implementation detail of the WASM runtime helpers. |

Performance is *not* a v1 concern. Correctness across backends is. The differential test gate from Phase 39 will catch any divergence.

### 7. Contract integration

`Char` is a verifiable type in contracts:

```intent
function is_keyword_start(c: Char) returns Bool
    ensures result == (c.is_alpha() or c == '_')
{
    return c.is_alpha() or c == '_';
}
```

Char literals are concrete values the verifier (Z3) can reason about as bounded integers.

## Consequences

### Code surface (Phase 31 — implementation)

Phase 31 implements ADR 0041 in stage1 (Go):

- `internal/lexer/`: parse `'...'` char literal; new `Char` token kind.
- `internal/ast/`: new `CharLit` AST node; type ref `Char`.
- `internal/checker/`: register `Char` as a primitive; relax `IndexExpr` to accept `String → Char`; new `SliceExpr` (or extend `IndexExpr`) to accept `String[i..j] → String`; extend `len()` to accept `String`; register Char built-in predicates and `to_codepoint` / `from_codepoint` / `to_string`.
- `internal/ir/`: lower the new constructs.
- `internal/rustbe/`, `internal/jsbe/`, `internal/wasmbe/`: emit per-backend implementations of the above operations.
- Runtime helpers: per-backend `__intent_string_index`, `__intent_string_slice`, `__intent_string_len_chars` (or the equivalent inlined version).
- `internal/verify/`: teach Z3 about `Char` (bounded integer encoding) and the predicates.
- Tests: full coverage of literals, indexing, slicing, predicates, out-of-bounds contract violations, surrogate-codepoint rejection, all backends.

### Migration

No existing `.intent` source uses `s[i]` or `s[i..j]` (today's checker rejects them outright). Phase 31 only *enables* code; nothing breaks.

### Deferred to later ADRs

The following are *not* in Phase 31 — each gets its own ADR when its phase needs it:

- **String search**: `s.starts_with(prefix)`, `s.ends_with(suffix)`, `s.contains(needle)`, `s.index_of(needle)`.
- **String split / trim**: `s.split(sep)`, `s.trim()` and friends. Likely needed by the parser phase, not the lexer.
- **String case conversion**: `s.to_lowercase()`, `s.to_uppercase()`.
- **String parsing**: `s.to_int() : Result<Int, String>`, `s.to_float()`.
- **Unicode-aware char predicates**: full Unicode categories. v1 lexer doesn't need this; defer until non-ASCII identifiers are on the table.
- **String construction from `Array<Char>`**: defer; lexer hot path is slicing.

### Trade-offs

- **Performance.** Codepoint indexing on UTF-8 / UTF-16 is O(n) without per-string caching. Backends are expected to precompute a codepoint table per indexed string, paying space for time. For a v1 formatter, this is acceptable. If perf becomes a concern, a future ADR can add byte indexing as a parallel mode (`s.bytes()` returning `Array<Int>`).
- **ASCII-only predicates.** Won't break non-ASCII identifiers because the lexer rejects them today already; this is a Phase 31 limitation that maps cleanly to existing constraints.
- **Slow-indexing surprise.** Code that does `for i in 0..len(s) { do_something(s[i]); }` is O(n²) on UTF-8 backends without caching. Documented in INTENT.md as a known v1 trade-off; idiomatic Intent should use slicing for substring work.

### Stage1 + stage2 implications

- Stage1 (Go formatter) does not gain a dependency on these primitives; it keeps using Go's strings directly.
- Stage2 (Intent formatter, Phase 32+) depends on these primitives. Phase 31 is therefore a *prerequisite* phase with no direct stage2 deliverable.
- The Z3 verifier learns about `Char` as part of Phase 31 so that contracts using char predicates remain verifiable end-to-end.

## Follow-ups

- **Phase 31 PRD** (`ops/plans/phase-31-string-primitives.md`) — implementation tasks for this ADR.
- **ADR 004x — String search & manipulation.** Deferred until a parser phase surfaces the need; likely Phase 33+.
- **ADR 004x — Stdin/stdout streaming for stage2 binaries.** `selfhost/formatter/main.intent` will want to read source from stdin. Not blocking Phase 31 but on the list.
- **Standard library positioning.** As Phase 31, 33+, and beyond accumulate string ops, file I/O, stdin/stdout, etc., they become a de facto Intent stdlib. A meta-ADR may be worth writing once 3-4 of these have landed.

## References

- [ADR 0040](0040-self-hosted-formatter-strategy.md) — the strategic frame this ADR fits into
- Russ Cox / Pike, "Strings, bytes, runes and characters in Go" (blog.golang.org/strings) — codepoint vs. byte tradeoff in a popular language
- Rust Reference §6.3.5 (str type) — char vs byte indexing in Rust
- Python 3 strings: docs.python.org/3/reference/datamodel.html#strings
- Swift String API: developer.apple.com/documentation/swift/string
- Dafny char & string semantics: dafny.org/dafny/DafnyRef/DafnyRef
- Unicode Standard, Chapter 3 — definitions of scalar value, surrogate, codepoint
