# selfhost/

Stage2 toolchain — Intent code that implements parts of the Intent toolchain
itself. Stage1 (the Go-implemented `intentc`) builds stage2 until stage2 can
compile itself.

See [ADR 0040](../docs/decisions/0040-self-hosted-formatter-strategy.md) for
the strategic frame, the stage1/stage2 mental model (Zig precedent), and the
multi-phase delivery plan.

## Layout

```
selfhost/
  shared/       the stage2 front-end shared by every tool: lexer.intent,
                ast.intent, parser.intent  (modules shared_lexer/ast/parser)
  formatter/    Intent-implemented `intentc fmt --self-hosted` (Phase 38-42)
  linter/       Intent-implemented `intentc lint --self-hosted` (Phase 43)
  checker/      Intent-implemented `intentc check --self-hosted` (Phases 45-54)
  compiler/     Intent-implemented `intentc build --emit --self-hosted` — IR lowering +
                Rust backend (Phases 55-57; byte-equal with stage1 on every repo program)
```

The shared front-end lives in `selfhost/shared/`; each tool is a sibling that
imports it via `../shared/…` ([ADR 0051](../docs/decisions/0051-selfhost-shared-restructure.md),
Phase 44). This split was deferred (ADR 0050 D1) until a third stage2 tool — the
checker ([ADR 0052](../docs/decisions/0052-self-hosted-checker-strategy.md)) — was
about to land, then done as a pure refactor before the checker arrived. The checker
is multi-phase (Phase 45 ships structural + name-resolution + arity checks; type
inference follows, Phases 46-54). The self-hosted `compiler/` (IR + Rust backend) landed
in Phases 55-57 — the toolchain now compiles itself (ADR 0059; Milestone 9 in the roadmap).

## How stage2 is built

Stage2 is plain Intent source — build it with the stage1 toolchain:

```bash
cd selfhost/<tool>/
intentc pkg install            # phase 30 registry pulls any deps
intentc build main.intent      # phase 14+ build emits a binary
./main < input                 # invoke directly
```

Each stage2 tool dogfoods stage1: the package manager, compiler, runtime,
and standard library are all exercised by the existence of the stage2 code.
When a stage2 tool can't be expressed in today's Intent, the relevant
language gap is captured in a new ADR + phase before stage2 work continues.
