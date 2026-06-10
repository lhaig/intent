# 0044: Stage2 Comment Preservation via Token-Attached Leading Comments

**Date:** 2026-06-09
**Status:** accepted
**Phase:** v1.2 — Self-Improvement Foundations (Phase 40, sub-piece A)

## Context

Stage2's lexer skips comments in `skip_whitespace_and_comments`. They never reach the parser or AST. The formatter therefore emits a comment-free version of any input — fine for `examples/hello.intent` (no comments), fatal for `selfhost/formatter/*.intent` (dense module-level commentary explaining design choices, Phase references, and trade-offs).

Byte-equal self-format on stage2 files is impossible until comments survive the round-trip.

This ADR records *how* comments are plumbed through.

## Decision

Comments are captured by the lexer as **leading trivia attached to the next token**. Each `Token` gains a `comments_before: Array<String>` field. When `skip_whitespace_and_comments` encounters a comment, the comment's text (including the `//` or `/* */` delimiters) is appended to a lexer-level buffer. The buffer is drained onto the next non-whitespace, non-comment token emitted.

Each top-level decl entity gains `comments_before: Array<String>` populated by the parser from the leading keyword token's `comments_before`. The formatter emits these comments on their own lines before each decl.

**Scope for v1 (this phase):**

- Leading comments on top-level declarations — the most common documentation pattern in stage2 source.
- Single-line (`// ...`) and block (`/* ... */`) comments both supported. Block comments retain their multi-line structure.
- Inline-after comments (`let x = 1; // explanation`) deferred to a follow-up (Phase 40A.2 or later).
- Comments inside function bodies deferred (same scope).

## Considered alternatives

| Option | Description | Trade-off |
|---|---|---|
| **(1) Token-attached leading comments** [**chosen**] | Comments captured in lexer; attached to next emitted token's `comments_before` field. Parser threads them into AST decl nodes. | Localised: one new Token field, one new field per top-level decl entity, one lexer change, one parser change per decl method. Granularity: leading-only. Lexer remains the source of truth for comment placement. |
| **(2) Separate comment-token stream** | Lexer emits `tk_comment` tokens interleaved with real tokens. Parser ignores them; formatter walks both streams in sync. | Lower-touch on the parser (just skip `tk_comment` in the dispatch). Higher-touch on the formatter (coordinated walk of two streams indexed by source line). Comment placement gets fuzzy at decl boundaries — is a comment "before X" or "after Y"? |
| **(3) AST-attached comments** | Every AST node (decl, stmt, expr) gets a `leading_comments` field. Lexer emits `tk_comment` tokens; parser distributes them to nodes. | Semantically the most precise — comments live where they belong. But the AST surface widens significantly, and inline-after comments still need a "trailing" field. Diff is large. |

### Why (1) over (2) and (3)

(2) is appealingly minimal at the parser layer but pushes complexity into the formatter, which would need to interleave the comment stream with the decl emit by source line. For block comments that span multiple lines the line-indexing gets fragile. The cost of (1)'s parser plumbing is modest because we only need to thread comments at top-level decl boundaries (one field-write per `parse_*_decl`).

(3) is the long-term right answer for full-fidelity formatting (inline comments, comments inside expression contexts, etc.). The diff to land it now would be large — every AST node would gain a field, every parse method would need to thread comments through every recursive boundary. (1) gets us 80% of the round-trip fidelity for ~15% of the AST surface change. (3) remains available as a future refactor once stage2 stabilises.

### Why leading-only scope for v1

Inline-after comments are conceptually simple (extend Token to have `comment_after: Option<String>`) but the parser would need to look ahead after consuming the statement-terminator semicolon to attach the comment, then back-fill. For Phase 40A.1, leading-only:

- Captures the dominant pattern (module-level documentation, decl headers).
- Lets stage2 source round-trip most of its documentation.
- Keeps the lexer's "drain buffer onto next token" rule simple — no special-case for "what's after this token."

Inline-after is a follow-up that can land independently without restructuring this work.

## Implementation outline

### Token field

```intent
public entity Token {
    field kind: Int;
    field text: String;
    field line: Int;
    field column: Int;
    field comments_before: Array<String>;  // NEW

    constructor(kind: Int, text: String, line: Int, column: Int, comments_before: Array<String>) {
        // ...
        self.comments_before = comments_before;
    }
}
```

For existing callers that don't care about comments, a helper `make_token(kind, text, line, col)` constructs a Token with an empty `comments_before`. The bulk of the lexer's `scan_one` and friends emit via this helper; only the wrap-up at the top of `scan_all` drains the accumulated buffer.

### Lexer change

`skip_whitespace_and_comments` becomes a stateful method that pushes comment text onto a mutable `pending_comments: Array<String>` field on the `Lexer` entity. Each comment is captured verbatim (including the `//` or `/* */` delimiters). When `scan_all` emits the next token, it drains the buffer onto that token's `comments_before` and resets the buffer.

### Parser change

Each `parse_*_decl` method captures the comments from the leading keyword token's `comments_before` and passes them to the decl constructor:

```intent
let line: Int = entity_tok.line;
let cmts: Array<String> = entity_tok.comments_before;
// ... parse body ...
return EntityDecl(name_tok.text, is_public, fields, methods, has_ctor, ctor, invariants, line, cmts);
```

### AST change

Each top-level decl entity gains `comments_before: Array<String>` after `line: Int`:

```intent
public entity EntityDecl {
    // ...
    field line: Int;
    field comments_before: Array<String>;

    constructor(/* existing args */, line: Int, comments_before: Array<String>) {
        // ...
    }
}
```

### Formatter change

`format_program`'s k-way merge dispatcher prepends each decl's `comments_before` to the output, one comment per line:

```intent
if best_kind == 0 {
    out = out + format_comments_before(prog.imports[i_imp].comments_before) + format_import_decl(prog.imports[i_imp]) + "\n\n";
    i_imp = i_imp + 1;
}
```

`format_comments_before(cs)` joins the strings with `"\n"` and ensures a trailing `"\n"` so the decl that follows is on its own line.

## Consequences

### Positive

- Stage2 source files (heavy documentation) survive round-trip.
- Token surface gains a clean place to hang per-token trivia. Inline-after comments can extend the same `Token` entity later without a structural change.
- Lexer-level capture means the source of truth for comment position stays at the lowest layer.

### Negative

- Every `Token` constructor call site now passes an `Array<String>` (often empty). Stage2's lexer constructs tokens in dozens of places — mechanical update.
- Every top-level decl entity constructor widens by one parameter. Stage2's parser and any test/helper that constructs decls directly need updating.
- Block comments that span multiple lines are stored as a single `String` with embedded newlines. The formatter emits them verbatim; if the user had unusual internal indentation it's preserved as-is.

### Neutral

- The `tk_comment` token kind is *not* added. Comments are trivia, not tokens, from the parser's perspective.

## Out of scope

- **Inline-after comments** (`let x = 1; // explanation`). Phase 40A.2 (or rolled into Phase 41).
- **Comments inside function bodies** (between statements). Same.
- **Comments inside expressions** (`f(x, /* hint */ y)`). Same.
- **Doc-comment syntax** (`///`, `/**`). Treated as regular line / block comments.
- **Comment-aware re-indentation** (if user wrote `//  doubly indented`, we don't re-indent). Comment text preserved verbatim.

## Reference

- ADR 0040 — strategic frame for self-hosted formatter
- ADR 0042 — source-order tracking (Phase 40C)
- ADR 0043 — paren stripping (Phase 40B)
- `selfhost/formatter/lexer.intent` — `skip_whitespace_and_comments` (currently drops comments)
- `selfhost/formatter/ast.intent` — decl entities (this ADR widens nine of them)
- `selfhost/formatter/parser.intent` — parse methods (this ADR adds one line to each `parse_*_decl`)
- `selfhost/formatter/format.intent` — `format_program` (this ADR adds the prepend)
