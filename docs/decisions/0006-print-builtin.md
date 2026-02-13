# 0006: Print as Built-in Function

**Date:** 2025-02-12
**Status:** accepted
**Phase:** POC

## Context

Programs need observable output. Without print, there is no way to
verify a program did anything useful.

## Options

- **print as keyword** -- Requires lexer/parser changes for new syntax.
- **print as built-in function** -- Recognized by checker, no new syntax.
- **Standard library** -- Requires an import system.
- **Extern declarations** -- Requires FFI support.

## Decision

Built-in function recognized by the checker. print() is already a valid
call expression syntactically, so no lexer or parser modifications are
needed.

## Consequences

- Minimal implementation change. The checker registers print alongside
  user-defined functions. Codegen maps print() to println!() in Rust.
- No new syntax to learn. print("hello") looks like any other function
  call.
- A standard library would require an import system (Phase 2.3) which
  does not exist yet. Built-in avoids that dependency.
- As more built-ins are needed, this pattern extends naturally. Each
  built-in is registered in the checker and mapped in codegen.
- print is not user-overridable. This is acceptable for now.
