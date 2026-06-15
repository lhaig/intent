# Norman Config

## Mode

scaffolding: full
notes: |
  Full migration from the previous ops/ layout (completed 2026-06-15). All phase
  PRDs now live under prds/{done,active,backlog,research}/. prds/TASKS.md is the
  live task list; prds/TASKS-archive.md holds the collapsed completed-phase index.

  What stays OUTSIDE norman (norman does not own these):
  - docs/decisions/*.md  — ADRs (architectural decision records)
  - docs/ROADMAP.md      — milestone view with rich per-phase shipped summaries
  - docs/HARNESS.md      — agent harness contract (updated to the norman workflow)

## Session Limits
max_tasks_per_session: 15
warn_at_tasks: 12

## Project
name: Intent compiler (intent/)
repo: /Users/lance.haig/dev/ai/exp/intent
created: 2026-06-15

## Commands
build: make build                 # produces ./intentc (go build -o intentc ./cmd/intentc)
test: make test                   # go test ./... -timeout 30s
validate: make validate           # gofmt-check + build + test + check/lint examples
stage2_test: ./intentc test --all-targets selfhost/formatter/format_test.intent
lint: gosec ./...

## Subagent Defaults
default_subagent: general-purpose

## Model Strategy
advisor_mode: always        # Opus advisor reviews plans + completed work
default_model: sonnet       # workers implement on Sonnet
quick_model: haiku
progress_compress_after: 10
