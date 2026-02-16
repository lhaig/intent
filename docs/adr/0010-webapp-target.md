# ADR 0010: Web Application Target (`--target webapp`)

## Status: Proposed

## Context

The Intent compiler generates business logic (classes, contracts, functions) but UI wrappers must be hand-written. The compiler already has full metadata about entities, fields, enums, contracts, and intent blocks via the IR. This is sufficient to auto-generate a complete browser application.

## Decision

Add a `webapp` target that generates a self-contained HTML file bundling:
1. The JS backend output (from `jsbe`)
2. An auto-generated dashboard UI derived from IR metadata
3. Contract enforcement visualization

## Architecture

```
AST -> Checker -> IR -> jsbe.Generate()  -> JS source
                    \-> webbe.Generate() -> HTML with embedded JS + auto-UI
```

The `webbe` (web backend) package:
- Calls `jsbe.Generate()` to get the JS business logic
- Reads IR metadata to build the UI scaffold
- Outputs a single self-contained HTML file

## Implementation Plan

### New package: `internal/webbe/`

**webbe.go** (~400 lines estimated):

```go
package webbe

func Generate(mod *ir.Module) (string, error) {
    // 1. Generate JS via jsbe
    jsCode := jsbe.Generate(mod)

    // 2. Extract UI metadata from IR
    entities := extractEntities(mod)    // name, fields, contracts
    enums := extractEnums(mod)          // name, variants
    contracts := extractContracts(mod)  // all requires/ensures/invariants
    intents := extractIntents(mod)      // intent blocks with goals

    // 3. Render HTML template
    return renderTemplate(jsCode, entities, enums, contracts, intents)
}
```

**metadata.go** (~150 lines):

```go
type EntityMeta struct {
    Name        string
    Fields      []FieldMeta      // name, type, has constraints
    Methods     []MethodMeta     // name, params, contracts
    Invariants  []string         // raw contract text
}

type EnumMeta struct {
    Name     string
    Variants []VariantMeta      // name, fields (if data-carrying)
}

type ContractMeta struct {
    Entity   string             // "" for free functions
    Method   string
    Kind     string             // "requires", "ensures", "invariant"
    Text     string             // human-readable form
}

func extractEntities(mod *ir.Module) []EntityMeta { ... }
func extractEnums(mod *ir.Module) []EnumMeta { ... }
func extractContracts(mod *ir.Module) []ContractMeta { ... }
```

**template.go** (~300 lines):

HTML template with:
- Dark theme CSS (reuse Option B's proven design)
- Entity cards: auto-generated from field metadata
  - Each entity type gets a card
  - Fields displayed with type info
  - Enum fields get colored status badges (variant names as labels)
  - Numeric fields with constraints get visual bars
- Contract panel: list all contracts with badges (requires/ensures/invariant)
- Intent block panel: display goals and guarantees
- Event log panel
- Control buttons: Run, Step, Reset, Break Contract
- Auto-generated step sequence from `main()` function body
- Process shim: `var process = { exit: function(){} };`

### Auto-generating the step sequence

The IR's entry function body contains the demo sequence as statements. The webbe can walk these statements and group them into logical steps:

```go
func extractSteps(mainFunc *ir.Function) []Step {
    // Group statements between print() calls
    // Each print("...") starts a new visual step
    // Method calls on entities become logged actions
}
```

This is the most complex part. A simpler v1: just run `__intent_main()` and capture console.log output, displaying it in the log panel. The step-by-step UI can come in v2.

### CLI integration

In `cmd/intentc/main.go`, add `"webapp"` to the target switch:

```go
case "webapp":
    html, err := webbe.Generate(module)
    // write to <name>.html
```

### Usage

```bash
# Generate self-contained web app
intentc build --target webapp examples/task_queue.intent
# -> task_queue.html (open in browser, no server needed)
```

## What gets auto-generated vs. what doesn't

**Auto-generated from IR:**
- Entity cards with field names and types
- Enum variant badges with colors
- Contract list with kind badges
- Intent block display (goals, guarantees, verified_by)
- Basic entry point execution

**NOT auto-generated (would need future work):**
- Custom layouts or styling
- Interactive step-by-step demo (v1 just runs main())
- Server-side rendering (that stays as Option C pattern)

## Phases

### Phase 1: Basic webapp target
- Extract entity/enum/contract metadata from IR
- HTML template with entity cards, contract panel, intent panel
- Embed jsbe output with process shim
- Run entry function on load, capture console output to log panel
- Estimate: ~800 lines of Go

### Phase 2: Interactive stepping
- Parse main() body into step groups
- Step/Run/Reset buttons that execute steps sequentially
- Render entity state changes in real-time
- Estimate: ~400 lines additional

### Phase 3: Contract violation demo
- Auto-generate "Break a Contract" scenarios from contract metadata
- For each `requires` with a numeric bound, generate a call that violates it
- For each precondition on state, generate a sequence that reaches invalid state
- Estimate: ~200 lines additional

## Files to create/modify

| File | Action |
|------|--------|
| `internal/webbe/webbe.go` | NEW: main generation entry point |
| `internal/webbe/metadata.go` | NEW: IR metadata extraction |
| `internal/webbe/template.go` | NEW: HTML template rendering |
| `internal/webbe/webbe_test.go` | NEW: tests |
| `cmd/intentc/main.go` | MODIFY: add "webapp" target case |
| `internal/compiler/compiler.go` | MODIFY: add webapp compilation path |

## Example output structure

For `task_queue.intent`, the generated `task_queue.html` would contain:

```
<!DOCTYPE html>
<html>
<head>
  <title>task_queue - Intent Web App</title>
  <style>/* dark theme CSS */</style>
</head>
<body>
  <!-- Auto-generated from IR metadata -->
  <h1>task_queue</h1>

  <!-- Intent blocks -->
  <div class="card">
    <h2>Intent: Job priority is always valid</h2>
    <p>Goal: Priority stays within 1-10 range</p>
    <p>Guarantee: Constructor enforces bounds, invariant checked on every method</p>
    <p>Verified by: Job.invariant, Job.constructor.requires</p>
  </div>

  <!-- Entity: Job (auto-generated) -->
  <div class="card">
    <h2>Entity: Job</h2>
    <div>Fields: id (Int), priority (Int), status (JobStatus)</div>
    <div>Invariants: priority >= 1, priority <= 10</div>
    <div>Methods: get_id, get_priority, is_pending, assign, complete, fail, status_code</div>
  </div>

  <!-- Entity: Worker (auto-generated) -->
  <!-- Enum: JobStatus (auto-generated) -->
  <!-- Contract list (auto-generated) -->
  <!-- Log panel -->

  <script>var process = { exit: function(){} };</script>
  <script>/* jsbe output embedded here */</script>
  <script>
    // Auto-generated runner
    // Captures console.log to display in log panel
    const _origLog = console.log;
    const logPanel = document.getElementById('log');
    console.log = function(...args) {
      logPanel.innerHTML += '<div>' + args.join(' ') + '</div>';
      _origLog.apply(console, args);
    };
    __intent_main();
  </script>
</body>
</html>
```

## Risks

- **Template complexity**: The HTML template needs to be maintainable. Use Go's `text/template` or `html/template` package rather than string concatenation.
- **Entry function parsing**: Extracting logical steps from main() is fragile. Phase 1 should just run it; Phase 2 can add stepping.
- **CSS bloat**: Keep the template minimal. A good dark theme is ~100 lines of CSS.
