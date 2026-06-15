# Pickup Notes — 2026-06-15 (after Phase 40A.2 complete)

Handoff after the comment-preservation phase (40A.2) landed and a byte-equal probe
re-scoped the remaining work into Phase 40A.3.

## Where we are

Recent landings:
- **Phase 30-39** — see git history / `prds/done/`.
- **Phase 40C** — source-order tracking. ADR 0042.
- **Phase 40B** — precedence-aware paren stripping. ADR 0043.
- **Phase 40A.1** — leading-decl comment preservation. ADR 0044.
- **Phase 40A.2 (complete)** — comments now round-trip in four positions:
  - step (1) trailing-EOF (`Program.trailing_comments`),
  - step (2) body/between-statement (`Stmt.comments_before` via `parse_block`),
  - step (3) inline-after on statements (`Token.comment_after` + lexer same-line detection + `Stmt.comment_after`),
  - step (4) comprehensive synthetic round-trip test (partial gate).

Tooling: aiki removed; **norman** drives tracking (`prds/`). Full ops/→prds/ migration done.

**147 in-language tests pass on rust + js** for `selfhost/formatter/format_test.intent`.

## Byte-equal self-format gate progress

| Sub-piece | Status |
|---|---|
| C — source-order tracking | done (40C) |
| B — paren stripping | done (40B) |
| A.1 — leading-decl comments | done (40A.1) |
| A.2 — trailing / body / inline-after-statement comments | done (40A.2) |
| A.3 — module-leading, entity field/method, end-of-block, inline-after-field comments + canonicalize | **next (see below)** |

## Probe finding (2026-06-15)

A throwaway probe ran `format(parse(src)) == src` on all four stage2 files. All diverge.
First divergence is at index 0 on every file: the **file-header comment before `module`
is dropped**. Length deltas (lexer −2004, ast −2568, parser −6566, format −1112 chars)
show many comments still dropped, concentrated in non-statement positions. Plus the
files use **column-aligned** inline comments (`field x: T;       // ...`) which a
canonical formatter can't reproduce as-is.

Conclusion: real-file byte-equal is not a single test — it's Phase 40A.3.

## Immediate next step: Phase 40A.3 (real-file byte-equal)

Order by impact (all mirror the established add-field / capture-from-token / emit pattern):

1. **40A.3.1 module-leading comments.** The index-0 diff. Capture comments before the
   `module` token (e.g. `Program.module_comments` or `ModuleDecl.comments_before`) in
   `parse_module_decl`; emit before the module line in `format_program`.
2. **40A.3.3 comments before entity methods/constructor.** Biggest volume
   (parser.intent's ~30 method-doc comments). `FunctionDecl.comments_before` already
   exists; populate it for methods/ctor inside entity/impl parsing and emit in
   `format_entity_decl` / `format_method_decl` / `format_impl_decl`.
3. **40A.3.2 comments before entity fields.** Add `FieldDecl.comments_before`; capture
   in entity-field parsing; emit in `format_entity_decl`'s field loop.
4. **40A.3.4 end-of-block comments.** Comments before a block's closing `}` attach to
   the rbrace token; capture in `parse_block` (the `expect(tk_rbrace)` token) into a
   `Block.trailing_comments`; emit after the last statement.
5. **40A.3.5 inline-after on fields.** Extend the 40A.2.3 `comment_after` mechanism to
   `FieldDecl`.
6. **40A.3.6 canonicalize + gate.** Run the formatter over the four stage2 files to
   normalize them (de-aligns inline comments to single space, etc.); verify the
   normalized files still compile and self-parse and the suite stays green; commit the
   reformatted files; then add the real-file gate:

```intent
test "byte-equal self-format on stage2 files" {
    let r = read_file("selfhost/formatter/lexer.intent");
    let src = match r { Ok(s) => s, Err(_) => "" };
    if src != "" {
        let prog = formatter_parser.parse(src);
        assert_eq(prog.error, "");
        assert_eq(formatter_format.format_program(prog), src);
    }
}
```

Anything still diverging after 40A.3 (structural, not comments) goes to Phase 41.

## Phase 41 outlook (parser surface widening)

Independent additions, any order:
- `requires` / `ensures` + `result` keyword. Unblocks `examples/fibonacci.intent`.
- `match` over `Result` / `Option`.
- `for in` loops.
- `try ?` operator.

## Other candidates (orthogonal)

- Verify-aware stripping (ADR 0033 deferred).
- Phase 23 — VS Code Marketplace publish (blocked on credentials; in `prds/backlog/`).
- Backend ADRs surfaced (cross-module fn qualification, auto-`&mut` for entity params, multi-use String auto-clone).

## Language / backend gaps still open

- Mutating an already-pushed Array element (`arr[i].field = x`) is unreliable in the stage1 Rust backend — hold a local, mutate, push (used in `scan_all`).
- Local `String` re-use across expressions / loop iterations can trigger `borrow of moved value`; call helpers (e.g. `indent_string`) fresh each iteration.
- `let _:` rejected → use bare expression statements to discard a Result.
- Cross-module free-function calls need a module prefix; entity-type qualification across modules rejected.
- No `String.to_int()` / `parse_float` builtins.
- Stage2 parser doesn't handle `requires` / `ensures` / `match` / `for-in` / `try ?` / `break`. Phase 41+.
- Column-aligned inline comments can't survive a canonical formatter without de-aligning the source (40A.3.6).

## How to resume

1. `git log --oneline -12`.
2. `prds/TASKS.md` for the open task list (norman; aiki removed 2026-06-15).
3. Recommended start: **Phase 40A.3.1** — module-leading comments (the index-0 byte-equal
   divergence). Then 40A.3.3 (entity method comments, biggest volume). Re-run the probe
   pattern (a temporary read_file + format + first-diff test) to track shrinking divergence.
