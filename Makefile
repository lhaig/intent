.PHONY: build test clean install check-examples lint-examples test-gen-examples

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
	rm -f examples/hello examples/bank_account examples/fibonacci

# Install intentc to GOPATH/bin
install:
	go install ./cmd/intentc

# Type-check example programs
check-examples: build
	./intentc check examples/hello.intent
	./intentc check examples/bank_account.intent
	./intentc check examples/fibonacci.intent

# Lint example programs
lint-examples: build
	./intentc lint examples/hello.intent
	./intentc lint examples/bank_account.intent
	./intentc lint examples/fibonacci.intent

# Emit Rust from examples (does not require cargo)
emit-examples: build
	./intentc build --emit-rust examples/hello.intent
	./intentc build --emit-rust examples/bank_account.intent
	./intentc build --emit-rust examples/fibonacci.intent

# Generate test-augmented Rust for examples
test-gen-examples: build
	./intentc test-gen --emit examples/fibonacci.intent
	./intentc test-gen --emit examples/bank_account.intent
	./intentc test-gen --emit examples/array_sum.intent
	./intentc test-gen --emit examples/sorted_check.intent
