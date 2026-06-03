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
  formatter/    Intent-implemented formatter (in progress; see formatter/README.md)
```

Future siblings: `linter/`, eventually `compiler/`. Each is its own Intent
package with its own `intent.toml`.

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
