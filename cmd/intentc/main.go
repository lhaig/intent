package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lhaig/intent/internal/compiler"
	"github.com/lhaig/intent/internal/linter"
	"github.com/lhaig/intent/internal/parser"
)

const usage = `intentc - The Intent language compiler

Usage:
  intentc build [--emit-rust] <file.intent>    Compile to native binary (or Rust source)
  intentc check <file.intent>                  Parse and type-check only
  intentc lint <file.intent>                   Run lint checks for style/best practices

Options:
  --emit-rust    Output generated Rust source instead of building a binary

Multi-file support:
  When the entry file contains import declarations, intentc automatically
  discovers all imported files, performs cross-file type checking, and
  produces a single binary from all modules.

Examples:
  intentc build hello.intent              Build hello.intent -> hello (native binary)
  intentc build --emit-rust hello.intent  Emit hello.rs (Rust source)
  intentc build main.intent               Build multi-file project (auto-detects imports)
  intentc check hello.intent              Check for errors without building
  intentc lint hello.intent               Lint for style/best practice issues
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "build":
		handleBuild(os.Args[2:])
	case "check":
		handleCheck(os.Args[2:])
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
	emitRust := false
	var filePath string

	for _, arg := range args {
		switch arg {
		case "--emit-rust":
			emitRust = true
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
		if emitRust {
			res := compiler.CompileProject(filePath)
			if res.Diagnostics != nil && res.Diagnostics.HasErrors() {
				fmt.Fprintf(os.Stderr, "Error: %s\n", res.Diagnostics.Format(filePath))
				os.Exit(1)
			}
			outPath := baseName + ".rs"
			if err := os.WriteFile(outPath, []byte(res.RustSource), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing file: %s\n", err)
				os.Exit(1)
			}
			fmt.Printf("Wrote %s (multi-file)\n", outPath)
		} else {
			outPath := baseName
			fmt.Printf("Compiling %s (multi-file project)...\n", filePath)
			if err := compiler.BuildProject(filePath, outPath); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				os.Exit(1)
			}
			fmt.Printf("Built %s\n", outPath)
		}
	} else {
		// Single-file compilation path (backward compatible)
		source, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file: %s\n", err)
			os.Exit(1)
		}

		if emitRust {
			outPath := baseName + ".rs"
			if err := compiler.EmitRust(string(source), outPath); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				os.Exit(1)
			}
			fmt.Printf("Wrote %s\n", outPath)
		} else {
			outPath := baseName
			fmt.Printf("Compiling %s...\n", filePath)
			if err := compiler.Build(string(source), outPath); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				os.Exit(1)
			}
			fmt.Printf("Built %s\n", outPath)
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
