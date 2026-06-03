# 0040: Self-Hosted Formatter Strategy

**Date:** 2026-06-03
**Status:** accepted (strategic; per-phase ADRs follow)
**Phase:** v1.2 — Self-Improvement Foundations (kicks off the self-hosting milestone)

## Context

Per `[[project-self-hosting-priority]]`, bootstrapping Intent with itself is the near-term goal. Phase 29 ([ADR 0038](0038-retire-legacy-rust-testgen.md)) removed the Rust-shaped exception in the toolchain; Phase 30 ([ADR 0039](0039-package-registry-git-mvs.md)) gave Intent a working package registry. The next step is to actually write a piece of tooling *in Intent*. The smallest-surface candidate is the formatter (the linter and compiler are larger).

This ADR records the architectural direction for self-hosting the formatter. It deliberately does **not** specify language extensions, an implementation timeline, or the precise tool surface — those are per-phase ADRs (ADR 0041+) and per-phase PRDs. The point of this ADR is to nail down the *direction* so future ADRs share a common frame.

### Architectural choice (selected 2026-06-03)

Three options were considered:

| Option | What it means | Trade-offs |
|---|---|---|
| **JSON bridge** | Go parses, emits AST as JSON; Intent reads JSON and emits formatted source. | Smallest first step; keeps Go-side parser as truth; surfaces only printing-layer gaps. Not real self-hosting — still depends on Go parser. |
| **Native (Intent parses Intent)** [**chosen**] | Lexer, parser, AST, and formatter all in Intent. | Real self-hosting. Massive surface; surfaces every language gap. Multi-phase by necessity. |
| **IR bridge** | Go produces IR; Intent prints from IR. | Awkward — IR drops source-fidelity information (comments, original parenthesisation, trailing commas). |

The chosen option (Native) was selected explicitly to surface *every* language gap rather than incrementally. This is the harder path but produces a real self-hosted artefact when it ships.

### Scope target

Three scope targets were considered:

| Target | What it means |
|---|---|
| **Smallest demo** | Format one file (`hello.intent`); skip almost everything. |
| **Standalone subset** | Top-level shape (module, imports, signatures); bodies pass through. |
| **Full parity (multi-phase)** [**chosen**] | Byte-for-byte parity with the Go formatter; delivered across multiple phases. |

### CLI integration

Three integration choices were considered:

| Choice | What it means |
|---|---|
| `intentc fmt --self-hosted` flag | Opt-in; Go stays default; mismatches diff'd via `--compare`. |
| `intentc fmt-intent` separate command | Visibly experimental until parity. |
| **External wrapper script** [**chosen**] | No `intentc` surgery — invoke compiled `selfhost/formatter/format` directly. CLI integration deferred until the experiment proves out. |

### Precedent

Self-hosted toolchains in roughly chronological order:

| Project | Self-hosting milestone | Notable |
|---|---|---|
| **Pascal** (Wirth, 1970) | Compiler bootstrapped from itself within ~1 year of the language existing. | Demonstrated the *technique*: hand-write a minimal compiler in the new language, compile it with the previous version. The Niklaus-Wirth-built bootstrap loop is the canonical reference. |
| **Rust** (Mozilla, ~2010) | The first Rust compiler was OCaml; rustc became self-hosted ~v0.6 (2012). | Multi-year transition. The OCaml compiler was deleted long after rustc could compile itself, only when "rustc compiles rustc" was reliable. |
| **Go** | Initially in C; transitioned to Go in 1.5 (2015). | The whole-tree rewrite was held back until the language had stabilised — explicitly because half-self-hosted compilers carry double maintenance. |
| **TypeScript** | Self-hosted from very early (2014). | The compiler is one of the largest TypeScript programs; serves as a stress test for the language. |
| **Crystal** | Hand-written in Ruby; transitioned to Crystal-in-Crystal ~v0.1 (2014). | Rewriting in itself surfaced language design problems that hadn't shown up at smaller scale. |
| **Elixir** | Erlang-hosted; never fully self-hosted because the BEAM is the runtime. | Demonstrates that "self-hosted" is a choice, not a requirement. Elixir's design instead leans on Erlang's runtime; the *compiler* is Elixir, but the runtime stays Erlang. |
| **Zig** (Andrew Kelley, ~2016) | C++ initially; transitioned to Zig-in-Zig over years. | Andrew's "stage1 / stage2" terminology is useful: stage1 is the previous-language compiler used to compile the language definition; stage2 is the self-hosted product. Adopted here as the mental model. |
| **D** (Walter Bright) | C++; partial self-hosting only. | Counter-example: many years in, dmd is still mostly C++. Demonstrates that self-hosting is an explicit prioritisation. |

The **stage1 / stage2** pattern from Zig is the closest fit to Intent's situation:

- **Stage1** = the Go-implemented toolchain that exists today (intentc).
- **Stage2** = the Intent-implemented formatter (and eventually linter, compiler).
- Stage2 is *compiled by* stage1 until stage2 can compile itself.

This ADR commits to building stage2 of the formatter. Other tools (linter, compiler) follow the same pattern but are not in scope here.

### Why multi-phase

A real self-hosted formatter needs, at minimum:

1. **Language extensions** that Intent doesn't have today. Currently absent: string indexing (`s[i]`), char type, string slicing, predicates (`starts_with`, `is_digit`), parse helpers. A lexer is impossible without these.
2. **A lexer** — converts source text to a token stream.
3. **A parser** — converts tokens to AST.
4. **A printer / formatter** — converts AST back to formatted source.

Each layer requires the previous. Each language extension needs its own ADR + phase. The total surface is well into 5k+ LOC of Intent across multiple sub-phases. Trying to do it in one phase is not realistic.

This ADR commits to a **gap-driven** delivery model: each phase first identifies the language extension it needs, lands that extension in stage1 (Go), then writes the stage2 Intent code that uses it.

## Decision

### 1. Direction

Build a stage2 formatter in Intent that parses and re-emits Intent source. Stage1 (Go) remains the production formatter until stage2 reaches parity. The two are kept side-by-side in the toolchain (no premature retirement).

### 2. Location

```
selfhost/
  formatter/
    intent.toml             # package manifest
    lexer.intent            # source → tokens
    parser.intent           # tokens → AST
    ast.intent              # AST entity declarations
    format.intent           # AST → formatted source string
    main.intent             # entry point: stdin → stdout
    tests/                  # in-language tests for each layer
```

`selfhost/` is a new top-level directory. Future self-hosted tools (linter, eventually the compiler) get sibling packages.

### 3. Delivery phases

| Phase | Scope | Pre-req language extensions |
|---|---|---|
| **31** | String indexing primitives (Char type, `s[i]`, `s[i..j]`, predicates, parse helpers) — *no Intent code yet*. | None (extends stage1). |
| **32** | Lexer in Intent: tokenise a useful subset of source. | Phase 31. |
| **33** | AST entity layout + parser for top-level decls (module, function signatures, imports). | Phase 32; may need union/sum-type extensions (TBD via discovery). |
| **34** | Statement-level parser (blocks, if, while, let, return). | Phase 33; may need pattern-matching extensions. |
| **35** | Expression parser with precedence. | Phase 33. |
| **36** | Entity, trait, impl, intent block parsing. | Phase 33-34. |
| **37** | Formatter (AST → string), targeting byte-parity on a corpus of `examples/*.intent`. | Phase 33-36. |
| **38** | Async / pattern-matching / generics — full-feature parser parity. | Phases 33-37. |
| **39** | Differential-corpus test gate + CLI integration. | Phase 37-38. |

Each phase has its own PRD; this ADR's phase numbering is **indicative** and may shift as language gaps surface.

### 4. CLI integration

Deliberately **deferred**. Stage2 lives in `selfhost/formatter/` and is invoked as a compiled binary directly (`./selfhost/formatter/format < hello.intent`). Once parity is reached (Phase 39), `intentc fmt --self-hosted` lands as a thin shim. Until then, the Go formatter is the only user-facing path.

### 5. Stage2 build flow

Stage2 is itself an Intent program — built by stage1:

```bash
cd selfhost/formatter
intentc pkg install     # phase 30 registry: pulls any deps
intentc build main.intent
./main < ../../examples/hello.intent
```

The build flow surfaces dogfooding bugs: stage1's package manager, compiler, runtime, and standard library are all exercised by stage2's existence.

### 6. Parity strategy

Byte-for-byte parity with stage1's formatter is the success criterion (Phase 39). Achieved by:

1. **Differential testing.** A new test gate runs stage1 and stage2 against every `examples/*.intent` and `internal/**/*.intent` file, diffing the outputs. Stage2 passes only when the diff is empty.
2. **Migration of the canonical formatter spec.** Currently, the Go formatter *is* the spec ("whatever the Go code does"). Once stage2 reaches parity, the truth shifts: any future formatting change must land in stage2 first; stage1 is updated to match for as long as it exists.
3. **Stage1 retirement.** Out of scope for this ADR. Deferred until stage2 has been running in production-ish use for a credible period.

## Open questions

These are explicitly *not* resolved in this ADR; each gets its own follow-up ADR as the relevant phase arrives:

- **Char type design.** Codepoint-as-Int vs. dedicated `Char` type. Phase 31 ADR (0041) decides.
- **Sum types beyond enums.** If parsing AST needs ergonomic discriminated unions richer than Intent's current `enum`s, that's its own ADR.
- **Dynamic dispatch.** The formatter's visitor pattern wants traits with dynamic dispatch (so a `format_node` function can take any node kind). Today's traits are static-dispatch only. ADR likely needed in the parser phases.
- **I/O surface.** `read_file` / `print` exist; stdin/stdout streaming for a real pipe-friendly formatter doesn't. May need a new ADR.
- **Stage1 retirement criteria.** Defined when stage2 parity holds.

## Consequences

### Aspirational benefits

- **Toolchain trust.** A self-hosted formatter validates that Intent is *usable* for non-trivial programs. The compiler-trust loop closes.
- **Language gap discovery.** Each phase will surface real missing pieces (string ops, dispatch, I/O) and force them to be designed properly via ADRs.
- **Standard library kickstart.** The pieces added for the formatter (string handling, file I/O streaming) compose into a usable stdlib.

### Costs

- **Double maintenance window.** Stage1 and stage2 coexist for the duration of phases 32-39. Formatting bugs may need fixing in both until stage2 retires stage1.
- **Phase 31 has zero stage2 deliverable.** This is the right shape — landing language primitives before code is the gap-driven philosophy — but it can read as "we're not doing anything." The ADR makes that scope explicit.
- **Risk of permanent stage1.** Per the D precedent, half-self-hosted compilers can ossify if the work stalls. Mitigation: each phase has concrete acceptance criteria; if parity stalls past Phase 39, an explicit go/no-go ADR is required.

### Non-goals

- Self-hosting the compiler. Bigger surface, more language extensions, much later.
- Self-hosting the linter. Smaller than the compiler but still its own multi-phase project.
- Performance parity. Stage2 may be slower than stage1 at first; that's fine. Correctness first.

## References

- [ADR 0038](0038-retire-legacy-rust-testgen.md) — Phase 29 testgen retirement (same self-hosting motivation)
- [ADR 0039](0039-package-registry-git-mvs.md) — Phase 30 package registry (unblocks multi-package self-hosted code)
- Niklaus Wirth, "Compiler Construction" (1996) — canonical self-hosting reference
- Rust's bootstrap history: rust-lang.org/governance/teams/release (early Rust release notes)
- Zig's stage1/stage2 transition: ziglang.org/news/0.10.0-release-notes/ (and related)
- Andrew Kelley's stage2 talks (~2020-2022) for the gap-driven approach
- `selfhost/formatter/` (directory created alongside this ADR as a scaffold)
