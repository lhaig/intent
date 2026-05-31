package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/lhaig/intent/internal/backend"
	"github.com/lhaig/intent/internal/compiler"
	"github.com/lhaig/intent/internal/formatter"
	"github.com/lhaig/intent/internal/linter"
	"github.com/lhaig/intent/internal/lsp"
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
  intentc test [--target <t>] [--all-targets] [--filter <s>] [--list] [--quiet] <file.intent>
                                                               Run in-language tests on one or more targets
  intentc fmt [--check] <file.intent>                          Format source to canonical style
  intentc lint <file.intent>                                   Run lint checks for style/best practices
  intentc pkg init                                             Create intent.toml from module declarations
  intentc pkg add <name> <version>                             Add dependency with version constraint
  intentc pkg add <name> --path <dir>                          Add local path dependency
  intentc pkg remove <name>                                    Remove dependency from manifest
  intentc pkg install                                          Resolve and cache all dependencies
  intentc pkg list                                             Show dependency tree
  intentc lsp                                                  Start the LSP server on stdio (ADR 0032)

Options:
  --target <target>   Target platform: rust (default), js, wasm
  --emit              Output generated source instead of building a binary
  --emit-rust         (deprecated) Same as --emit with --target rust
  --strip-contracts   Drop runtime contract checks from emitted output
                      (Phase 22 / ADR 0033). Rust: assert! -> debug_assert!,
                      compiled out by cargo's release profile. JS: contract
                      throw-on-violation lines omitted. WASM: no effect.
                      User-written assert(...) in test bodies is unaffected.

Targets:
  rust    Compile to native binary via Rust (default)
  js      Generate JavaScript source
  wasm    Compile to WebAssembly (direct binary emission)

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
  intentc test hello.intent                     Run all tests in hello.intent on the rust target
  intentc test --target js hello.intent         Run tests on the js target (requires node)
  intentc test --all-targets hello.intent       Run on rust + js + wasm; flag cross-target divergence
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
	case "test":
		handleTest(os.Args[2:])
	case "fmt":
		handleFmt(os.Args[2:])
	case "lint":
		handleLint(os.Args[2:])
	case "pkg":
		handlePkg(os.Args[2:])
	case "lsp":
		handleLsp(os.Args[2:])
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
	opts := backend.BuildOptions{}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--emit-rust":
			// deprecated but still supported
			emit = true
			target = "rust"
		case "--emit":
			emit = true
		case "--strip-contracts":
			opts.StripContracts = true
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

	// Phase 22 / ADR 0033: surface the safety trade in CI logs.
	if opts.StripContracts {
		fmt.Fprintln(os.Stderr, "warning: --strip-contracts removes runtime contract checks; run 'intentc verify' to confirm safety properties.")
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
			if err := compiler.EmitProjectToTarget(filePath, target, baseName, opts); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				os.Exit(1)
			}
		} else {
			if err := compiler.BuildProjectToTarget(filePath, target, baseName, opts); err != nil {
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
			if err := compiler.EmitToTarget(string(source), target, baseName, opts); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				os.Exit(1)
			}
		} else {
			if err := compiler.BuildToTarget(string(source), target, baseName, opts); err != nil {
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
	target := "rust" // legacy default; phase 16 introduces --target intent
	var filePath string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--emit":
			emitFile = true
		case "--target":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "Error: --target requires an argument")
				os.Exit(1)
			}
			i++
			target = args[i]
			if target != "rust" && target != "intent" {
				fmt.Fprintf(os.Stderr, "Error: unknown test-gen target: %s (expected rust or intent)\n", target)
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

	// Phase 16 / ADR 0029: --target intent produces a sibling .intent file
	// of test blocks consumable by `intentc test`. The legacy --target rust
	// path remains for entity tests and other cases not yet covered by the
	// Intent emission.
	if target == "intent" {
		source, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file: %s\n", err)
			os.Exit(1)
		}
		// Generated tests live alongside the source file so the relative
		// import resolves cleanly. The import path is the source basename
		// (e.g. "fibonacci.intent").
		srcDir := filepath.Dir(filePath)
		srcBaseWithExt := filepath.Base(filePath)
		srcBase := strings.TrimSuffix(srcBaseWithExt, filepath.Ext(srcBaseWithExt))

		// When piping to stdout, omit the import so the user can decide where
		// the file will live; when writing alongside the source, embed it.
		importPath := ""
		if emitFile {
			importPath = srcBaseWithExt
		}

		intentSrc, err := compiler.GenerateIntentTests(string(source), importPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}
		if emitFile {
			outPath := filepath.Join(srcDir, srcBase+"_test.intent")
			if err := os.WriteFile(outPath, []byte(intentSrc), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing file: %s\n", err)
				os.Exit(1)
			}
			fmt.Printf("Wrote %s\n", outPath)
		} else {
			fmt.Print(intentSrc)
		}
		return
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

// Phase 16 / ADR 0029: `intentc test` runs in-language tests on one or more
// targets, reports per-test pass/fail, and (with --all-targets) flags
// cross-backend divergence. See ops/plans/phase-16-testing-framework.md.
func handleTest(args []string) {
	opts := compiler.TestRunOptions{}
	var filePath string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--target":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "Error: --target requires an argument")
				os.Exit(1)
			}
			i++
			t := args[i]
			if t != "rust" && t != "js" && t != "wasm" {
				fmt.Fprintf(os.Stderr, "Error: unknown target: %s (expected rust, js, wasm)\n", t)
				os.Exit(1)
			}
			opts.Targets = append(opts.Targets, t)
		case "--all-targets":
			opts.AllTargets = true
		// Phase 17 / 17.F: DX flags.
		case "--filter":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "Error: --filter requires a substring argument")
				os.Exit(1)
			}
			i++
			opts.Filter = args[i]
		case "--list":
			opts.List = true
		case "--quiet":
			opts.Quiet = true
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

	results, err := compiler.RunTests(filePath, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	switch {
	case opts.List:
		fmt.Print(compiler.FormatList(results))
	case opts.Quiet:
		fmt.Print(compiler.FormatResultsQuiet(results))
	default:
		fmt.Print(compiler.FormatResults(results))
	}
	if !opts.List && compiler.AnyFailures(results) {
		os.Exit(1)
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

	var output *compiler.VerifyOutput
	if isMulti {
		output, err = compiler.VerifyProjectWithReport(filePath)
	} else {
		source, readErr := os.ReadFile(filePath)
		if readErr != nil {
			fmt.Fprintf(os.Stderr, "Error reading file: %s\n", readErr)
			os.Exit(1)
		}
		output, err = compiler.VerifyWithReport(string(source))
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
	for _, result := range output.Results {
		name := result.QualifiedName()

		switch result.Status {
		case "verified":
			fmt.Printf("VERIFIED: %s: %s\n", name, result.ContractText)
			verified++
		case "unverified":
			fmt.Printf("UNVERIFIED: %s: %s\n", name, result.ContractText)
			fmt.Printf("  %s\n", result.Message)
			unverified++
			hasUnverified = true
		case "error":
			fmt.Printf("ERROR: %s\n", result.Message)
			errors++
			hasError = true
		case "timeout":
			fmt.Printf("TIMEOUT: %s: %s\n", name, result.ContractText)
			fmt.Printf("  %s\n", result.Message)
			timeouts++
			hasUnverified = true
		}
	}

	// Print summary
	fmt.Println()
	fmt.Printf("Verification summary: %d verified, %d unverified, %d timeouts, %d errors\n",
		verified, unverified, timeouts, errors)

	// Print intent verification report
	if len(output.IntentReports) > 0 {
		fmt.Println()
		fmt.Print(verify.FormatReport(output.IntentReports))
	}

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

const pkgUsage = `Usage:
  intentc pkg init                     Create intent.toml from module declarations
  intentc pkg add <name> <version>     Add dependency with version constraint
  intentc pkg add <name> --path <dir>  Add local path dependency
  intentc pkg remove <name>            Remove dependency from manifest
  intentc pkg install                  Resolve and cache all dependencies
  intentc pkg list                     Show dependency tree
`

// handleLsp starts the Intent LSP server on stdio. Phase 18 / ADR 0032.
// Takes no positional args; reads JSON-RPC framed messages from stdin and
// writes responses + notifications to stdout. Editor extensions spawn this
// as the language server.
func handleLsp(args []string) {
	if len(args) > 0 {
		fmt.Fprintf(os.Stderr, "Unknown argument to 'lsp': %s\n", args[0])
		fmt.Fprintln(os.Stderr, "Usage: intentc lsp")
		os.Exit(1)
	}
	srv := lsp.NewServer(os.Stdin, os.Stdout)
	if err := srv.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "lsp: %s\n", err)
		os.Exit(1)
	}
}

func handlePkg(args []string) {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, pkgUsage)
		os.Exit(1)
	}

	switch args[0] {
	case "init":
		handlePkgInit()
	case "add":
		handlePkgAdd(args[1:])
	case "remove":
		handlePkgRemove(args[1:])
	case "install":
		handlePkgInstall()
	case "list":
		handlePkgList()
	default:
		fmt.Fprintf(os.Stderr, "Unknown pkg subcommand: %s\n\n", args[0])
		fmt.Fprint(os.Stderr, pkgUsage)
		os.Exit(1)
	}
}

// modulePattern matches "module <name> version "<ver>";" declarations.
var modulePattern = regexp.MustCompile(`^\s*module\s+(\w+)\s+version\s+"[^"]*"\s*;`)

func handlePkgInit() {
	manifestPath := "intent.toml"
	if _, err := os.Stat(manifestPath); err == nil {
		fmt.Fprintln(os.Stderr, "Error: intent.toml already exists")
		os.Exit(1)
	}

	// Scan for .intent files in current directory to find module name
	pkgName := ""
	entries, err := os.ReadDir(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading directory: %s\n", err)
		os.Exit(1)
	}

	// Sort entries by name for deterministic iteration, then prefer main.intent.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	pkgSource := ""
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".intent") {
			continue
		}
		name, found := extractModuleName(entry.Name())
		if found {
			if entry.Name() == "main.intent" {
				pkgName = name
				pkgSource = "main.intent"
				break
			}
			if pkgName == "" {
				pkgName = name
				pkgSource = entry.Name()
			}
		}
	}
	if pkgSource != "" && pkgSource != "main.intent" {
		fmt.Fprintf(os.Stderr, "Note: Using module name from %s (no main.intent found)\n", pkgSource)
	}

	// Fall back to directory name
	if pkgName == "" {
		wd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting working directory: %s\n", err)
			os.Exit(1)
		}
		pkgName = filepath.Base(wd)
	}

	m := &compiler.Manifest{
		Package: compiler.PackageInfo{
			Name:    pkgName,
			Version: "0.1.0",
		},
		Dependencies: make(map[string]compiler.DependencySpec),
	}

	if err := compiler.WriteManifest(manifestPath, m); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing intent.toml: %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("Created intent.toml for package %q\n", pkgName)
}

// extractModuleName reads a file and returns the module name from a module declaration.
func extractModuleName(path string) (string, bool) {
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		matches := modulePattern.FindStringSubmatch(scanner.Text())
		if len(matches) >= 2 {
			return matches[1], true
		}
	}
	return "", false
}

func handlePkgAdd(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Error: missing package name")
		fmt.Fprintln(os.Stderr, "Usage: intentc pkg add <name> <version>")
		fmt.Fprintln(os.Stderr, "       intentc pkg add <name> --path <dir>")
		os.Exit(1)
	}

	name := args[0]
	var dep compiler.DependencySpec

	// Parse remaining args for version or --path
	i := 1
	for i < len(args) {
		switch args[i] {
		case "--path":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "Error: --path requires a directory argument")
				os.Exit(1)
			}
			i++ // advance past --path to the directory value before checking for duplicates
			if dep.Path != "" {
				fmt.Fprintf(os.Stderr, "Error: duplicate --path argument: %s\n", args[i])
				os.Exit(1)
			}
			dep.Path = args[i]
		default:
			if strings.HasPrefix(args[i], "-") {
				fmt.Fprintf(os.Stderr, "Error: unknown option: %s\n", args[i])
				os.Exit(1)
			}
			if dep.Version != "" {
				fmt.Fprintf(os.Stderr, "Error: duplicate version argument: %s\n", args[i])
				os.Exit(1)
			}
			dep.Version = args[i]
		}
		i++
	}

	if dep.Version == "" && dep.Path == "" {
		fmt.Fprintln(os.Stderr, "Error: must specify a version or --path")
		os.Exit(1)
	}

	if dep.Version != "" && dep.Path != "" {
		fmt.Fprintln(os.Stderr, "Error: cannot specify both a version and --path")
		os.Exit(1)
	}

	// Validate version constraint if provided
	if dep.Version != "" {
		if _, err := compiler.ParseConstraint(dep.Version); err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid version constraint %q: %s\n", dep.Version, err)
			os.Exit(1)
		}
	}

	// Resolve manifest directory so path deps are validated relative to it,
	// consistent with how handlePkgInstall resolves them.
	manifestDir, err := filepath.Abs(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Validate path exists if provided, resolving relative to manifest directory
	if dep.Path != "" {
		resolvedPath := dep.Path
		if !filepath.IsAbs(dep.Path) {
			resolvedPath = filepath.Join(manifestDir, dep.Path)
		}
		info, err := os.Stat(resolvedPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: path %q does not exist\n", dep.Path)
			os.Exit(1)
		}
		if !info.IsDir() {
			fmt.Fprintf(os.Stderr, "Error: path %q is not a directory\n", dep.Path)
			os.Exit(1)
		}
	}

	m, err := compiler.LoadManifest(manifestDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		fmt.Fprintln(os.Stderr, "Run 'intentc pkg init' to create an intent.toml first.")
		os.Exit(1)
	}

	m.Dependencies[name] = dep

	if err := compiler.WriteManifest("intent.toml", m); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing intent.toml: %s\n", err)
		os.Exit(1)
	}

	if dep.Path != "" {
		fmt.Printf("Added dependency %s (path: %s)\n", name, dep.Path)
	} else {
		fmt.Printf("Added dependency %s %s\n", name, dep.Version)
	}
}

func handlePkgRemove(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Error: missing package name")
		fmt.Fprintln(os.Stderr, "Usage: intentc pkg remove <name>")
		os.Exit(1)
	}

	name := args[0]

	m, err := compiler.LoadManifest(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	if _, ok := m.Dependencies[name]; !ok {
		fmt.Fprintf(os.Stderr, "Error: dependency %q not found in intent.toml\n", name)
		os.Exit(1)
	}

	delete(m.Dependencies, name)

	if err := compiler.WriteManifest("intent.toml", m); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing intent.toml: %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("Removed dependency %s\n", name)
}

func handlePkgInstall() {
	manifestDir, err := filepath.Abs(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	m, err := compiler.LoadManifest(manifestDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		fmt.Fprintln(os.Stderr, "Run 'intentc pkg init' to create an intent.toml first.")
		os.Exit(1)
	}

	if len(m.Dependencies) == 0 {
		fmt.Println("No dependencies to install.")
		return
	}

	cache, err := compiler.NewPackageCache()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	// Check if any versioned dependencies exist and warn early.
	hasVersioned := false
	for _, dep := range m.Dependencies {
		if dep.Path == "" && dep.Version != "" {
			hasVersioned = true
			break
		}
	}
	if hasVersioned {
		fmt.Fprintln(os.Stderr, "WARNING: no package registry available. Versioned dependencies can only be resolved from the local cache.")
	}

	hasErrors := false
	hasUnresolved := false
	names := sortedKeys(m.Dependencies)
	for _, name := range names {
		dep := m.Dependencies[name]

		if dep.Path != "" {
			// Local path dependency - resolve relative to manifest directory, not CWD
			resolvedPath := dep.Path
			if !filepath.IsAbs(dep.Path) {
				resolvedPath = filepath.Join(manifestDir, dep.Path)
			}
			info, err := os.Stat(resolvedPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  %s: ERROR path %q not found\n", name, dep.Path)
				hasErrors = true
				continue
			}
			if !info.IsDir() {
				fmt.Fprintf(os.Stderr, "  %s: ERROR path %q is not a directory\n", name, dep.Path)
				hasErrors = true
				continue
			}
			fmt.Printf("  %s: ok (local: %s)\n", name, dep.Path)
			continue
		}

		// Versioned dependency - check cache
		if dep.Version == "" {
			fmt.Fprintf(os.Stderr, "  %s: ERROR no version specified\n", name)
			hasErrors = true
			continue
		}

		// Resolve the constraint to a concrete version for cache lookup.
		// ConstraintBaseVersion extracts the base version from constraints like
		// "^1.0.0" -> "1.0.0", ensuring consistent cache keys.
		//
		// Limitation: For caret/tilde constraints (e.g. ^1.0.0, ~1.2.0) the
		// resolved version may differ from the constraint's base version. When
		// a real package registry is added, the resolved version (not the
		// constraint base) should be used as the cache key.
		version, err := compiler.ConstraintBaseVersion(dep.Version)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  %s: ERROR invalid version %q: %s\n", name, dep.Version, err)
			hasErrors = true
			continue
		}

		if cache.Has(name, version) {
			fmt.Printf("  %s@%s: cached\n", name, version)
		} else if found, foundVer := cache.FindMatchingVersion(name, dep.Version); found {
			fmt.Printf("  %s@%s: cached (constraint %s matched cached version)\n", name, foundVer, dep.Version)
		} else {
			// TODO: Cache population will happen once a package registry is available. Store/StoreWithChecksum APIs are ready for integration.
			fmt.Fprintf(os.Stderr, "  %s@%s: not installed — no registry available to fetch this version\n", name, version)
			hasUnresolved = true
		}
	}

	if hasErrors {
		os.Exit(1)
	}

	if hasUnresolved {
		fmt.Fprintln(os.Stderr, "\nSome packages could not be installed (no registry available).")
		fmt.Fprintln(os.Stderr, "To use local packages instead, add them with: intentc pkg add <name> --path <dir>")
	} else {
		fmt.Println("Install complete.")
	}
}

func handlePkgList() {
	m, err := compiler.LoadManifest(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("%s %s\n", m.Package.Name, m.Package.Version)

	if len(m.Dependencies) == 0 {
		fmt.Println("  (no dependencies)")
		return
	}

	visited := map[string]bool{m.Package.Name: true}
	printDeps(m.Dependencies, "", ".", visited)
}

func printDeps(deps map[string]compiler.DependencySpec, indent string, baseDir string, visited map[string]bool) {
	names := sortedKeys(deps)
	for i, name := range names {
		dep := deps[name]
		connector := "├── "
		childIndent := indent + "│   "
		if i == len(names)-1 {
			connector = "└── "
			childIndent = indent + "    "
		}
		if dep.Path != "" {
			fmt.Printf("%s%s%s (local)\n", indent, connector, name)
			if visited[name] {
				fmt.Printf("%s(already listed)\n", childIndent)
				continue
			}
			visited[name] = true
			resolvedPath := dep.Path
			if !filepath.IsAbs(resolvedPath) {
				resolvedPath = filepath.Join(baseDir, resolvedPath)
			}
			sub, err := compiler.LoadManifest(resolvedPath)
			if err == nil && len(sub.Dependencies) > 0 {
				printDeps(sub.Dependencies, childIndent, resolvedPath, visited)
			}
		} else {
			fmt.Printf("%s%s%s %s\n", indent, connector, name, dep.Version)
		}
	}
}

func sortedKeys(m map[string]compiler.DependencySpec) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
