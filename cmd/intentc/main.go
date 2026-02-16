package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lhaig/intent/internal/compiler"
	"github.com/lhaig/intent/internal/formatter"
	"github.com/lhaig/intent/internal/linter"
	"github.com/lhaig/intent/internal/parser"
	"github.com/lhaig/intent/internal/verify"
)

// version is set at build time via -ldflags "-X main.version=..."
var version = "dev"

const usage = `intentc - The Intent language compiler

Usage:
  intentc build [--target <target>] [--emit] <file.intent>    Compile to binary or source
  intentc check <file.intent>                                  Parse and type-check only
  intentc verify <file.intent>                                 Verify contracts using Z3 SMT solver
  intentc test-gen [--emit] <file.intent>                      Generate Rust with property-based contract tests
  intentc fmt [--check] <file.intent>                          Format source to canonical style
  intentc lint <file.intent>                                   Run lint checks for style/best practices

Options:
  --target <target>   Target platform: rust (default), js, wasm
  --emit              Output generated source instead of building a binary
  --emit-rust         (deprecated) Same as --emit with --target rust

Targets:
  rust    Compile to native binary via Rust (default)
  js      Generate JavaScript source
  wasm    Compile to WebAssembly via Rust

Multi-file support:
  When the entry file contains import declarations, intentc automatically
  discovers all imported files, performs cross-file type checking, and
  produces a single output from all modules.

Examples:
  intentc build hello.intent                    Build hello.intent -> hello (native binary)
  intentc build --emit hello.intent             Emit hello.rs (Rust source)
  intentc build --target js hello.intent        Build hello.intent -> hello.js
  intentc build --target js --emit hello.intent Emit hello.js (JS source)
  intentc build --target wasm hello.intent      Build hello.intent -> hello.wasm
  intentc build main.intent                     Build multi-file project (auto-detects imports)
  intentc check hello.intent                    Check for errors without building
  intentc verify hello.intent                   Verify contracts with Z3 (requires z3 on PATH)
  intentc test-gen fibonacci.intent             Generate Rust with contract tests to stdout
  intentc test-gen --emit fibonacci.intent      Write to fibonacci_test.rs
  intentc fmt hello.intent                      Format hello.intent in-place
  intentc fmt --check hello.intent              Check if already formatted (exit 1 if not)
  intentc lint hello.intent                     Lint for style/best practice issues
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	command := os.Args[1]

	if command == "--version" || command == "version" {
		fmt.Printf("intentc %s\n", version)
		os.Exit(0)
	}

	switch command {
	case "build":
		handleBuild(os.Args[2:])
	case "check":
		handleCheck(os.Args[2:])
	case "verify":
		handleVerify(os.Args[2:])
	case "test-gen":
		handleTestGen(os.Args[2:])
	case "fmt":
		handleFmt(os.Args[2:])
	case "lint":
		handleLint(os.Args[2:])
	case "help", "--help", "-h":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}
}

func handleBuild(args []string) {
	emit := false
	target := "rust"
	var filePath string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--emit-rust":
			// deprecated but still supported
			emit = true
			target = "rust"
		case "--emit":
			emit = true
		case "--target":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "Error: --target requires an argument")
				os.Exit(1)
			}
			i++
			target = args[i]
			if target != "rust" && target != "js" && target != "wasm" {
				fmt.Fprintf(os.Stderr, "Error: unknown target: %s\n", target)
				os.Exit(1)
			}
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(os.Stderr, "Unknown option: %s\n", arg)
				os.Exit(1)
			}
			filePath = arg
		}
	}

	if filePath == "" {
		fmt.Fprintln(os.Stderr, "Error: no input file specified")
		os.Exit(1)
	}

	// Check if this is a multi-file project
	isMulti, err := compiler.IsMultiFile(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %s\n", err)
		os.Exit(1)
	}

	baseName := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))

	if isMulti {
		// Multi-file compilation path
		if emit {
			if err := compiler.EmitProjectToTarget(filePath, target, baseName); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				os.Exit(1)
			}
		} else {
			if err := compiler.BuildProjectToTarget(filePath, target, baseName); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				os.Exit(1)
			}
		}
	} else {
		// Single-file compilation path
		source, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file: %s\n", err)
			os.Exit(1)
		}

		if emit {
			if err := compiler.EmitToTarget(string(source), target, baseName); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				os.Exit(1)
			}
		} else {
			if err := compiler.BuildToTarget(string(source), target, baseName); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				os.Exit(1)
			}
		}
	}
}

func handleCheck(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Error: no input file specified")
		os.Exit(1)
	}

	filePath := args[0]

	// Check if this is a multi-file project
	isMulti, err := compiler.IsMultiFile(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %s\n", err)
		os.Exit(1)
	}

	if isMulti {
		// Multi-file check path
		diag := compiler.CheckProject(filePath)
		if diag.HasErrors() {
			fmt.Fprintf(os.Stderr, "%s", diag.Format(filePath))
			os.Exit(1)
		}
		for _, d := range diag.All() {
			if d.Severity != 0 {
				fmt.Printf("%s:%d:%d: warning: %s\n", filePath, d.Line, d.Column, d.Message)
			}
		}
	} else {
		// Single-file check path
		source, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file: %s\n", err)
			os.Exit(1)
		}
		diag := compiler.Check(string(source))
		if diag.HasErrors() {
			fmt.Fprintf(os.Stderr, "%s", diag.Format(filePath))
			os.Exit(1)
		}
		for _, d := range diag.All() {
			if d.Severity != 0 {
				fmt.Printf("%s:%d:%d: warning: %s\n", filePath, d.Line, d.Column, d.Message)
			}
		}
	}

	fmt.Println("No errors found.")
}

func handleTestGen(args []string) {
	emitFile := false
	var filePath string

	for _, arg := range args {
		switch arg {
		case "--emit":
			emitFile = true
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(os.Stderr, "Unknown option: %s\n", arg)
				os.Exit(1)
			}
			filePath = arg
		}
	}

	if filePath == "" {
		fmt.Fprintln(os.Stderr, "Error: no input file specified")
		os.Exit(1)
	}

	// Check if this is a multi-file project
	isMulti, err := compiler.IsMultiFile(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %s\n", err)
		os.Exit(1)
	}

	baseName := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))

	var res *compiler.Result
	if isMulti {
		res = compiler.GenerateTestsProject(filePath)
	} else {
		source, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file: %s\n", err)
			os.Exit(1)
		}
		res = compiler.GenerateTests(string(source))
	}

	if res.Diagnostics != nil && res.Diagnostics.HasErrors() {
		fmt.Fprintf(os.Stderr, "Error: %s\n", res.Diagnostics.Format(filePath))
		os.Exit(1)
	}

	if emitFile {
		outPath := baseName + "_test.rs"
		if err := os.WriteFile(outPath, []byte(res.RustSource), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing file: %s\n", err)
			os.Exit(1)
		}
		fmt.Printf("Wrote %s\n", outPath)
	} else {
		fmt.Print(res.RustSource)
	}
}

func handleFmt(args []string) {
	checkOnly := false
	var filePath string

	for _, arg := range args {
		switch arg {
		case "--check":
			checkOnly = true
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(os.Stderr, "Unknown option: %s\n", arg)
				os.Exit(1)
			}
			filePath = arg
		}
	}

	if filePath == "" {
		fmt.Fprintln(os.Stderr, "Error: no input file specified")
		os.Exit(1)
	}

	source, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %s\n", err)
		os.Exit(1)
	}

	p := parser.New(string(source))
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		fmt.Fprintf(os.Stderr, "%s", p.Diagnostics().Format(filePath))
		os.Exit(1)
	}

	formatted := formatter.Format(prog)

	if checkOnly {
		if formatted != string(source) {
			fmt.Fprintf(os.Stderr, "%s is not formatted\n", filePath)
			os.Exit(1)
		}
		return
	}

	if err := os.WriteFile(filePath, []byte(formatted), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file: %s\n", err)
		os.Exit(1)
	}
}

func handleVerify(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Error: no input file specified")
		os.Exit(1)
	}

	filePath := args[0]

	// Check if this is a multi-file project
	isMulti, err := compiler.IsMultiFile(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %s\n", err)
		os.Exit(1)
	}

	var results []*verify.VerifyResult
	if isMulti {
		results, err = compiler.VerifyProject(filePath)
	} else {
		source, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file: %s\n", err)
			os.Exit(1)
		}
		results, err = compiler.Verify(string(source))
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	// Track verification status
	hasError := false
	hasUnverified := false
	verified := 0
	unverified := 0
	errors := 0
	timeouts := 0

	// Print results
	for _, result := range results {
		contractType := "requires"
		if result.IsEnsures {
			contractType = "ensures"
		}

		switch result.Status {
		case "verified":
			fmt.Printf("VERIFIED: %s %s: %s\n", result.FunctionName, contractType, result.ContractText)
			verified++
		case "unverified":
			fmt.Printf("UNVERIFIED: %s %s: %s\n", result.FunctionName, contractType, result.ContractText)
			fmt.Printf("  %s\n", result.Message)
			unverified++
			hasUnverified = true
		case "error":
			fmt.Printf("ERROR: %s\n", result.Message)
			errors++
			hasError = true
		case "timeout":
			fmt.Printf("TIMEOUT: %s %s: %s\n", result.FunctionName, contractType, result.ContractText)
			fmt.Printf("  %s\n", result.Message)
			timeouts++
			hasUnverified = true
		}
	}

	// Print summary
	fmt.Println()
	fmt.Printf("Verification summary: %d verified, %d unverified, %d timeouts, %d errors\n",
		verified, unverified, timeouts, errors)

	// Exit with appropriate code
	if hasError || hasUnverified {
		os.Exit(1)
	}
}

func handleLint(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Error: no input file specified")
		os.Exit(1)
	}

	filePath := args[0]

	source, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %s\n", err)
		os.Exit(1)
	}

	p := parser.New(string(source))
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		fmt.Fprintf(os.Stderr, "%s", p.Diagnostics().Format(filePath))
		os.Exit(1)
	}

	diag := linter.Lint(prog)

	if diag.Count() == 0 {
		fmt.Println("No lint warnings.")
		return
	}

	fmt.Print(diag.Format(filePath))
	fmt.Println()
	fmt.Printf("%d warning(s) found.\n", diag.Count())
}
