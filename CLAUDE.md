# CLAUDE.md

This file provides guidance to Claude Code when working in the Intent compiler project.

## Essential References

Read these before making changes:

- **`INTENT.md`** -- AI code-generation guide for writing `.intent` programs. Covers the full language: types, contracts, entities, enums, traits, error handling, and I/O builtins.
- **`AGENTS.md`** -- Multi-agent workflow and task orchestration guidelines.
- **`docs/DESIGN.md`** -- Full language specification.
- **`docs/grammar.ebnf`** -- Formal grammar (EBNF).
- **`docs/decisions/`** -- Architectural decision records (ADRs). Check here before proposing structural changes.

## Build & Test

```bash
make build          # produces ./intentc
make test           # go test ./... -timeout 30s
make test-v         # verbose
make clean          # remove all build artifacts and binaries
```

Single package: `go test ./internal/parser/... -v -timeout 30s`

Always run `make clean` after building or testing to remove generated binaries and .rs files.

## Architecture

Classic multi-phase compiler pipeline in Go with zero external dependencies:

1. **Lexer** (`internal/lexer/`) -- tokenizes `.intent` source
2. **Parser** (`internal/parser/`) -- recursive-descent, produces AST
3. **Checker** (`internal/checker/`) -- type checking, scope resolution, contract validation
4. **IR** (`internal/ir/`) -- intermediate representation and lowering
5. **Codegen** (`internal/codegen/`) -- AST to Rust source
6. **Rust backend** (`internal/rustbe/`) -- Rust-specific code generation
7. **JS backend** (`internal/jsbe/`) -- JavaScript target
8. **Linter** (`internal/linter/`) -- style and best-practice warnings
9. **Testgen** (`internal/testgen/`) -- property-based tests from contracts
10. **Formatter** (`internal/formatter/`) -- canonical code formatting

Supporting: `internal/ast/`, `internal/compiler/`, `internal/diagnostic/`, `cmd/intentc/`

## Conventions

- Go only, no external dependencies.
- Contracts are the product -- write `requires`/`ensures`/`invariant` before implementations.
- New example programs: add the binary name to both `.gitignore` and the `clean` target in `Makefile`.
- Run `make clean` after any build or emit operation. Do not commit generated binaries or .rs files.
- Tests use `t.TempDir()` for temporary files (auto-cleaned by Go test framework).
