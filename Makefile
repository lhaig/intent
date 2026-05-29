.PHONY: build test test-v clean install check-examples lint-examples fmt-check-examples emit-examples test-gen-examples validate

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
	examples/async_demo.intent

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

# Generate property-test-augmented Rust for examples with contracts
# (kept as-is until Phase 16 migrates test-gen to emit Intent test blocks)
test-gen-examples: build
	./intentc test-gen --emit examples/fibonacci.intent
	./intentc test-gen --emit examples/bank_account.intent
	./intentc test-gen --emit examples/array_sum.intent
	./intentc test-gen --emit examples/sorted_check.intent

# Full mechanical-truth gate. The single command an agent should run before
# claiming a non-trivial change is done. Phase 16 will add `intentc test`
# over every example to this target.
validate: build test check-examples lint-examples fmt-check-examples
	@echo "validate: OK"
