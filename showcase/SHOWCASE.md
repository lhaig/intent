# Intent Showcase

This showcase demonstrates Intent source files compiled to multiple targets. All options use **unmodified compiler output** -- the generated code is never hand-edited.

## The Application

A priority task queue with contract-verified correctness:

- 5 jobs with priorities [5, 9, 3, 7, 1]
- 2 workers that process jobs in priority order
- One job intentionally fails to show error handling
- Contracts enforce: valid priority range (1-10), exclusive worker assignment, monotonic completion count

## Options

### Option A: Native Binary (CLI)

**Source:** `examples/task_queue.intent`

```bash
# Build and run
intentc build examples/task_queue.intent
./task_queue
```

The same source compiled to Rust, producing a native binary. All contracts are `assert!()` calls in Rust.

### Option B: Browser Dashboard

**Directory:** `showcase/option-b/`

```bash
# Generate the JavaScript
intentc build --target js examples/task_queue.intent
cp task_queue.js showcase/option-b/task_queue.generated.js

# Open in browser
open showcase/option-b/index.html
```

The HTML page loads `task_queue.generated.js` (unmodified compiler output) and wraps it with a visual dashboard. The generated code provides the classes (`Job`, `Worker`, `JobStatus`) and functions (`find_highest_priority`, `count_by_status`). The UI code is hand-written but all business logic and contract enforcement comes from the generated file.

**Key files:**
- `task_queue.generated.js` -- Compiler output (do not edit)
- `index.html` -- Dashboard UI (hand-written wrapper)

### Option C: Node.js Server with REST API

**Directory:** `showcase/option-c/`

```bash
# Generate the JavaScript
intentc build --target js examples/task_queue.intent
cp task_queue.js showcase/option-c/task_queue.generated.js

# Start the server
node showcase/option-c/server.js

# Open dashboard at http://localhost:3000
```

A Node.js HTTP server that loads the compiler-generated code using `vm.runInContext()` and exposes a REST API. Contract enforcement happens server-side.

**API endpoints:**
- `GET /` -- Dashboard HTML
- `GET /api/state` -- Current queue state (jobs, workers, stats)
- `POST /api/step` -- Advance one step
- `POST /api/reset` -- Reset the demo
- `POST /api/break` -- Try violating contracts (returns caught errors)

**Key files:**
- `task_queue.generated.js` -- Compiler output (do not edit)
- `server.js` -- Node.js server (hand-written, loads generated code)
- `index.html` -- Dashboard UI (fetches state from API)

### Option D: WASM in Browser

**Directory:** `showcase/option-d/`

```bash
# Generate the WASM binary (no Rust toolchain needed)
intentc build --target wasm examples/fibonacci.intent
cp fibonacci.wasm showcase/option-d/fibonacci.wasm

# Open in browser
open showcase/option-d/index.html
```

The HTML page loads `fibonacci.wasm` (unmodified compiler output) via `WebAssembly.instantiate()` and calls the exported `fib()` function directly. The WASM binary is emitted directly from the IR -- no Rust intermediary, no cargo, no wasm32-unknown-unknown target. Compilation is instant.

**Key files:**
- `fibonacci.wasm` -- Compiler output (do not edit)
- `index.html` -- Interactive demo UI (hand-written wrapper)

**What it demonstrates:**
- Direct WASM binary emission from Intent IR (Phase 6.4)
- Exported functions callable from JavaScript via `WebAssembly.instantiate()`
- Pure computation (recursive fibonacci with contracts) running as native WASM
- Module size: 155 bytes (compared to ~30KB+ via the Rust intermediary path)

---

## Reproducing This Showcase

### Prerequisites

- Go 1.21+ (to build the Intent compiler)
- Rust with cargo (for Option A native binary)
- Node.js 14+ (for Option C server)
- A modern browser (for Options B, C, and D)

### Step 1: Build the Compiler

```bash
cd intent
make build
```

### Step 2: Write the Intent Source

The prompt given to the AI code assistant was:

> Write a priority task queue in Intent that demonstrates:
> - Enums with data-carrying variants (JobStatus with Pending, Running, Complete, Failed)
> - Entities with contracts (Job with priority invariants, Worker with exclusive assignment)
> - Match expressions for status inspection
> - Helper functions with array contracts
> - Intent blocks linking goals to formal contracts
> - An entry function that creates 5 jobs and 2 workers, processes them in priority order, and includes one failure case
>
> Use parallel arrays for queue operations since the compiler doesn't support entity arrays yet.

The resulting file is `examples/task_queue.intent` (273 lines).

### Step 3: Compile to Each Target

```bash
# Option A: Native binary
intentc build examples/task_queue.intent
./task_queue

# Option B + C: JavaScript
intentc build --target js examples/task_queue.intent
cp task_queue.js showcase/option-b/task_queue.generated.js
cp task_queue.js showcase/option-c/task_queue.generated.js

# Option D: WebAssembly
intentc build --target wasm examples/fibonacci.intent
cp fibonacci.wasm showcase/option-d/fibonacci.wasm
```

### Step 4: Create Option A (no extra work needed)

Option A is just the compiled binary. No wrapper code required.

### Step 5: Create Option B (Browser Dashboard)

The prompt given to the AI code assistant was:

> Create a self-contained HTML page at `showcase/option-b/index.html` that:
> - Loads the unmodified compiler-generated `task_queue.generated.js` via a `<script src>` tag
> - Provides a `process = { exit: function() {} }` shim before the script loads (the generated code calls `process.exit()`)
> - Wraps the compiler-generated classes (Job, Worker, JobStatus, find_highest_priority) with a visual dashboard
> - Dark theme UI with cards for: job list (with priority bars and status badges), worker status, summary stats, active contracts, event log
> - "Run" button that auto-advances through the demo steps with 800ms delays
> - "Step" button for manual one-at-a-time advancement
> - "Break a Contract" button that demonstrates 3 contract violations: invalid priority (priority=0), double worker assignment, and finishing a job on an idle worker
> - An explanatory section at the top explaining what the page is, how contracts work, and that the same source compiles to a native binary
> - The UI code is hand-written, but ALL business logic and contract enforcement must come from the generated classes -- do not rewrite the contract checks
>
> The generated JS defines: JobStatus (object with factory functions), Job (class), Worker (class), find_highest_priority (function), count_by_status (function). Access job status via `job.status._tag` since the generated code doesn't expose a status_name() method.

### Step 6: Create Option C (Node.js Server)

The prompt given to the AI code assistant was:

> Create a Node.js HTTP server at `showcase/option-c/server.js` (pure Node, no dependencies) that:
> - Loads the unmodified compiler-generated `task_queue.generated.js` using Node's `vm` module
> - Uses `vm.createContext()` with `process: { exit: () => {} }` and `console: { log: () => {} }` to suppress the generated entry point
> - Wraps the generated code in an IIFE that returns the classes: `(function() { <generated code> return { JobStatus, Job, Worker, find_highest_priority, count_by_status }; })()`
> - Manages demo state server-side (array of Job instances, Worker instances, step counter, log entries)
> - Exposes REST API endpoints:
>   - GET /api/state -- returns JSON with jobs, workers, stats, logs, step number
>   - POST /api/step -- advances demo one step (same sequence as option B: find highest priority, assign, complete, fail one, process remaining)
>   - POST /api/reset -- reinitializes all state
>   - POST /api/break -- attempts 3 contract violations and returns the caught error messages
>   - GET / -- serves the dashboard HTML
> - Port 3000 by default (configurable via PORT env var)
>
> Also create `showcase/option-c/index.html` -- a dashboard similar to Option B's dark theme but fetching state from the server API via fetch(). Include Run/Step/Reset/Break buttons that call the API endpoints.
>
> Copy `task_queue.generated.js` from option-b. The generated file must not be modified.

### Step 7: Run Each Option

```bash
# Option A
./task_queue

# Option B
open showcase/option-b/index.html

# Option C
node showcase/option-c/server.js
# then open http://localhost:3000

# Option D
open showcase/option-d/index.html
```

## Compiler Bugs Found During Development

Building this showcase uncovered 4 compiler bugs, all fixed:

1. **Entity fields can't reference enum types** -- `registerEntities()` ran before `registerEnums()`, so enum types weren't available during field type resolution. Fix: swapped initialization order in `checker.go`.

2. **Empty array literal `[]` can't infer type** -- Workaround: used pre-populated parallel arrays instead of entity arrays. Not fixed (legitimate limitation).

3. **`constructor` keyword in `verified_by` causes parser hang** -- The parser's `parseVerifiedByRef()` only allowed `IDENT` tokens after dots, but `constructor` is a keyword token. The `break` in the default case only broke the switch, not the for loop. Fix: added `CONSTRUCTOR` to allowed tokens in parser, added constructor handling in intent checker.

4. **Rust backend generates invalid default for enum-typed fields** -- `defaultValue()` fell through to generic struct literal for enum types. Fix: added enum detection that selects the first unit variant (e.g., `JobStatus::Pending`).

## Contract Validation

All three options enforce identical contracts at runtime:

| Contract | What it checks |
|----------|---------------|
| `Job.invariant: priority >= 1 and priority <= 10` | Priority stays in valid range |
| `Worker.start_job.requires: is_busy == false` | Worker must be idle to accept a job |
| `Worker.finish_job.requires: is_busy == true` | Worker must be busy to finish |
| `Worker.finish_job.ensures: jobs_completed == old(jobs_completed) + 1` | Completed count increments exactly |
| `Worker.invariant: jobs_completed >= 0` | Completed count never goes negative |
| `Job.constructor.requires: priority >= 1 and priority <= 10` | Constructor rejects invalid priority |

Try breaking these in Option B (click "Break a Contract") or Option C (`POST /api/break`).
