.PHONY: build test test-v clean install check-examples lint-examples fmt-check-examples emit-examples test-gen-examples test-examples gofmt-check validate diff-formatter selfcheck-formatter

# Flat examples (single-file). Subdirectory examples (attractor, multi_file, packages, ffi_blake3)
# are exercised via their own entry points.
FLAT_EXAMPLES := \
	examples/hello.intent \
	examples/bank_account.intent \
	examples/fibonacci.intent \
	examples/array_sum.intent \
	examples/sorted_check.intent \
	examples/enum_basic.intent \
	examples/shape_area.intent \
	examples/result_option.intent \
	examples/try_operator.intent \
	examples/error_handling.intent \
	examples/io_demo.intent \
	examples/js_demo.intent \
	examples/map_demo.intent \
	examples/handler_trait.intent \
	examples/task_queue.intent \
	examples/verify_example.intent \
	examples/closure_demo.intent \
	examples/generic_stack.intent \
	examples/async_demo.intent \
	examples/target_specific_demo.intent

# Build the intentc compiler
build:
	go build -o intentc ./cmd/intentc

# Run all tests
test:
	go test ./... -timeout 30s

# Run tests with verbose output
test-v:
	go test ./... -v -timeout 30s

# Clean build artifacts
clean:
	rm -f intentc
	rm -f examples/*.rs
	rm -f examples/*_test.intent
	rm -f main.rs *.rs
	rm -rf target/
	rm -f main integration_test result_option sorted_check array_sum fibonacci hello bank_account enum_basic shape_area try_operator io_demo js_demo map_demo handler_trait task_queue verify_example error_handling

# Install intentc to GOPATH/bin
install:
	go install ./cmd/intentc

# Type-check every flat example
check-examples: build
	@for f in $(FLAT_EXAMPLES); do \
		echo "check: $$f"; \
		./intentc check $$f || exit 1; \
	done

# Lint every flat example
lint-examples: build
	@for f in $(FLAT_EXAMPLES); do \
		echo "lint:  $$f"; \
		./intentc lint $$f || exit 1; \
	done

# Format-check every flat example (no rewrites)
fmt-check-examples: build
	@for f in $(FLAT_EXAMPLES); do \
		echo "fmt:   $$f"; \
		./intentc fmt --check $$f || exit 1; \
	done

# Emit Rust source from every flat example (does not require cargo)
emit-examples: build
	@for f in $(FLAT_EXAMPLES); do \
		echo "emit:  $$f"; \
		./intentc build --emit $$f || exit 1; \
	done

# Differential test: stage2 (Intent) formatter vs stage1 `intentc fmt` over the
# examples corpus (Phase 42). Reports per-file PASS/DIVERGE/PARSE-ERR; exits
# non-zero on any non-allowed divergence. Drives stage2 formatter gap-closing.
diff-formatter: build
	@./selfhost/formatter/difftest.sh

# Byte-equal self-format gate: the stage2 formatter is a fixpoint on its own
# source files (Phase 42). Drives the built binary (not in-language tests, whose
# libtest thread stacks overflow on the ~95KB parser.intent).
selfcheck-formatter: build
	@./selfhost/formatter/selfcheck.sh

# Generate sibling _test.intent files from contracts (Phase 29 / ADR 0038:
# this used to emit Rust property tests; now emits Intent test blocks).
test-gen-examples: build
	./intentc test-gen --emit examples/fibonacci.intent
	./intentc test-gen --emit examples/bank_account.intent
	./intentc test-gen --emit examples/array_sum.intent
	./intentc test-gen --emit examples/sorted_check.intent

# Examples that carry at least one in-language test block (phase 16). Add to
# this list as tests are added to more examples.
TESTED_EXAMPLES := \
	examples/fibonacci.intent \
	examples/array_sum.intent \
	examples/sorted_check.intent \
	examples/bank_account.intent \
	examples/generic_stack.intent \
	examples/async_demo.intent \
	examples/hello.intent \
	examples/enum_basic.intent \
	examples/shape_area.intent \
	examples/result_option.intent \
	examples/try_operator.intent \
	examples/error_handling.intent \
	examples/closure_demo.intent \
	examples/verify_example.intent \
	examples/map_demo.intent \
	examples/handler_trait.intent \
	examples/task_queue.intent \
	examples/io_demo.intent \
	examples/js_demo.intent \
	examples/target_specific_demo.intent \
	examples/char_string_demo.intent

# Run `intentc test` over each example that has tests, on the default target.
test-examples: build
	@for f in $(TESTED_EXAMPLES); do \
		echo "test:  $$f"; \
		./intentc test $$f || exit 1; \
	done

# Verify Go source is gofmt-clean. Matches the CI Format Check job exactly
# so a green local validate predicts a green CI run.
gofmt-check:
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt: the following files are not formatted:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

# Full mechanical-truth gate. The single command an agent should run before
# claiming a non-trivial change is done.
validate: gofmt-check build test check-examples lint-examples fmt-check-examples test-examples
	@echo "validate: OK"
