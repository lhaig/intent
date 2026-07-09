package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

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
  intentc build [--target <target>] [--emit] [--self-hosted] <file.intent>
                                                               Compile to binary or source
  intentc check [--self-hosted] <file.intent>                  Parse and type-check only
  intentc verify <file.intent>                                 Verify contracts using Z3 SMT solver
  intentc test-gen [--emit] <file.intent>                      Generate Intent test blocks from contracts
  intentc test [--target <t>] [--all-targets] [--filter <s>] [--list] [--quiet] <file.intent>
                                                               Run in-language tests on one or more targets
  intentc fmt [--check] [--self-hosted] <file.intent>          Format source to canonical style
  intentc lint [--self-hosted] <file.intent>                   Run lint checks for style/best practices
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
  --self-hosted       With --emit: route through the stage2 (Intent) compiler
                      (ADR 0059). Rust target only; single-file for now.
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
  intentc check --self-hosted hello.intent      Check using stage2 (Intent) checker
  intentc verify hello.intent                   Verify contracts with Z3 (requires z3 on PATH)
  intentc test-gen fibonacci.intent             Generate Intent test blocks to stdout
  intentc test-gen --emit fibonacci.intent      Write to fibonacci_test.intent
  intentc test hello.intent                     Run all tests in hello.intent on the rust target
  intentc test --target js hello.intent         Run tests on the js target (requires node)
  intentc test --all-targets hello.intent       Run on rust + js + wasm; flag cross-target divergence
  intentc fmt hello.intent                      Format hello.intent in-place
  intentc fmt --check hello.intent              Check if already formatted (exit 1 if not)
  intentc fmt --self-hosted hello.intent        Format using stage2 (Intent) formatter
  intentc fmt --self-hosted --check hello.intent  Check using stage2 formatter
  intentc lint hello.intent                     Lint for style/best practice issues
  intentc lint --self-hosted hello.intent       Lint using the stage2 (Intent) linter
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
	selfHosted := false
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
		case "--self-hosted":
			selfHosted = true
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

	// Phase 55 / ADR 0059: --emit --self-hosted routes through the stage2
	// (Intent) compiler instead of the Go backend. The stage2 binary prints the
	// generated source to stdout; strip the single trailing newline print()
	// appends and write <base>.rs, byte-equal with stage1 `intentc build --emit`.
	if emit && selfHosted {
		if target != "rust" {
			fmt.Fprintln(os.Stderr, "Error: --self-hosted emit currently supports only --target rust")
			os.Exit(1)
		}
		binPath, err := stage2CompilerBinary()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}
		stdout, exitCode, err := runStage2Checker(binPath, stage2CompilePaths(filePath))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}
		if exitCode != 0 {
			// compile_main reports usage / read / parse errors on stdout, exit 1.
			fmt.Fprint(os.Stderr, stdout)
			os.Exit(1)
		}
		rust := strings.TrimSuffix(stdout, "\n")
		outPath := baseName + ".rs"
		if werr := os.WriteFile(outPath, []byte(rust), 0644); werr != nil {
			fmt.Fprintf(os.Stderr, "Error writing %s: %s\n", outPath, werr)
			os.Exit(1)
		}
		fmt.Printf("Wrote %s\n", outPath)
		return
	}

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

func parseCheckFlags(args []string) (selfHosted bool, filePath, errMsg string) {
	for _, arg := range args {
		switch arg {
		case "--self-hosted":
			selfHosted = true
		default:
			if strings.HasPrefix(arg, "-") {
				errMsg = "Unknown option: " + arg
				return
			}
			filePath = arg
		}
	}
	return
}

func handleCheck(args []string) {
	selfHosted, filePath, errMsg := parseCheckFlags(args)
	if errMsg != "" {
		fmt.Fprintln(os.Stderr, errMsg)
		os.Exit(1)
	}

	if filePath == "" {
		fmt.Fprintln(os.Stderr, "Error: no input file specified")
		os.Exit(1)
	}

	// --self-hosted delegates to the stage2 (Intent) checker binary, which emits
	// output byte-identical to the stage1 (Go) checker below. No silent fallback:
	// a stage2 build/parse failure is surfaced and exits non-zero.
	if selfHosted {
		binPath, err := stage2CheckerBinary()
		if err != nil {
			fmt.Fprintf(os.Stderr, "stage2 checker: %s\n", err)
			os.Exit(1)
		}
		stdout, exitCode, err := runStage2Checker(binPath, stage2CheckPaths(filePath))
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			os.Exit(1)
		}
		if exitCode == 0 {
			// Clean: "No errors found.\n" — emit verbatim to stdout.
			fmt.Print(stdout)
			return
		}
		// Errors found: stage2 stdout holds the diagnostic block with no trailing
		// newline (format_diags has no trailing newline; print() adds one). Stage1
		// writes diag.Format() to stderr via Fprintf with no trailing newline, so
		// we strip the one that print() added before forwarding to stderr.
		fmt.Fprintf(os.Stderr, "%s", strings.TrimRight(stdout, "\n"))
		os.Exit(1)
	}

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
	target := "intent"
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
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(os.Stderr, "Unknown option: %s\n", arg)
				os.Exit(1)
			}
			filePath = arg
		}
	}

	// Phase 29 / ADR 0038: the legacy `--target rust` testgen path was retired
	// once the Intent emitter covered entities (Phase 27) and multi-parameter
	// iteration (Phase 28). Generated `.intent` test files run unchanged on
	// every backend (rust / js / wasm) via `intentc test`.
	if target == "rust" {
		fmt.Fprintln(os.Stderr, "Error: `intentc test-gen --target rust` was removed in Phase 29 (ADR 0038).")
		fmt.Fprintln(os.Stderr, "Use `intentc test-gen --emit <file.intent>` to write a sibling _test.intent file,")
		fmt.Fprintln(os.Stderr, "then run `intentc test --target rust <file.intent>` to execute it on the Rust backend.")
		os.Exit(1)
	}
	if target != "intent" {
		fmt.Fprintf(os.Stderr, "Error: unknown test-gen target: %s (expected intent)\n", target)
		os.Exit(1)
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

	// Generated tests live alongside the source file so the relative import
	// resolves cleanly. When piping to stdout, omit the import so the user
	// can decide where the file will live.
	srcDir := filepath.Dir(filePath)
	srcBaseWithExt := filepath.Base(filePath)
	srcBase := strings.TrimSuffix(srcBaseWithExt, filepath.Ext(srcBaseWithExt))
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
}

// Phase 16 / ADR 0029: `intentc test` runs in-language tests on one or more
// targets, reports per-test pass/fail, and (with --all-targets) flags
// cross-backend divergence. See prds/done/phase-16-testing-framework.md.
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

// parseFmtFlags parses the flags for the fmt subcommand and returns
// (checkOnly, selfHosted, filePath, errorMsg). errorMsg is non-empty on
// any flag-parsing problem; callers should print it to stderr and exit 1.
func parseFmtFlags(args []string) (checkOnly, selfHosted bool, filePath, errMsg string) {
	for _, arg := range args {
		switch arg {
		case "--check":
			checkOnly = true
		case "--self-hosted":
			selfHosted = true
		default:
			if strings.HasPrefix(arg, "-") {
				errMsg = "Unknown option: " + arg
				return
			}
			filePath = arg
		}
	}
	return
}

func handleFmt(args []string) {
	checkOnly, selfHosted, filePath, errMsg := parseFmtFlags(args)
	if errMsg != "" {
		fmt.Fprintln(os.Stderr, errMsg)
		os.Exit(1)
	}

	if filePath == "" {
		fmt.Fprintln(os.Stderr, "Error: no input file specified")
		os.Exit(1)
	}

	var formatted string

	if selfHosted {
		binPath, err := stage2FormatterBinary()
		if err != nil {
			fmt.Fprintf(os.Stderr, "stage2 formatter: %s\n", err)
			os.Exit(1)
		}
		out, err := runStage2Formatter(binPath, filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			os.Exit(1)
		}
		formatted = out
	} else {
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

		formatted = formatter.Format(prog)
	}

	if checkOnly {
		source, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file: %s\n", err)
			os.Exit(1)
		}
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

// runStage2Formatter runs the prebuilt stage2 formatter binary on filePath and
// returns the canonical formatted source (stdout with one trailing newline
// trimmed). A non-zero exit is returned as an error including the binary output.
func runStage2Formatter(binaryPath, filePath string) (string, error) {
	cmd := exec.Command(binaryPath, filePath)
	out, err := cmd.Output()
	if err != nil {
		// Collect stderr too when available.
		var stderr []byte
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = exitErr.Stderr
		}
		combined := strings.TrimSpace(string(out) + string(stderr))
		if combined == "" {
			return "", fmt.Errorf("stage2 formatter exited with error: %w", err)
		}
		return "", fmt.Errorf("%s", combined)
	}
	// The stage2 formatter emits one extra trailing newline (print() lowers to
	// println!/console.log). Strip exactly one trailing newline.
	result := strings.TrimSuffix(string(out), "\n")
	return result, nil
}

// stage2FormatterBinary returns the path to the stage2 formatter binary,
// either from the INTENT_STAGE2_FMT env override or by auto-building from
// selfhost/formatter/main.intent (with caching in os.TempDir()).
func stage2FormatterBinary() (string, error) {
	// Env override.
	if envPath := os.Getenv("INTENT_STAGE2_FMT"); envPath != "" {
		info, err := os.Stat(envPath)
		if err != nil {
			return "", fmt.Errorf("INTENT_STAGE2_FMT=%q: %w", envPath, err)
		}
		if info.IsDir() || info.Mode()&0111 == 0 {
			return "", fmt.Errorf("INTENT_STAGE2_FMT=%q is not an executable file", envPath)
		}
		return envPath, nil
	}

	// Source location: relative to cwd.
	srcDir := filepath.Join("selfhost", "formatter")
	mainSrc := filepath.Join(srcDir, "main.intent")
	if _, err := os.Stat(mainSrc); err != nil {
		return "", fmt.Errorf(
			"stage2 formatter sources not found at selfhost/formatter/main.intent; run from the repo root or set INTENT_STAGE2_FMT to a prebuilt binary",
		)
	}

	cachePath := filepath.Join(os.TempDir(), "intent-stage2-fmt")

	// Staleness check: rebuild if cached binary is missing or any .intent file
	// in selfhost/formatter/ or selfhost/shared/ is newer than the cache.
	needBuild := false
	cacheInfo, err := os.Stat(cachePath)
	if err != nil {
		needBuild = true
	} else {
		sharedDir := filepath.Join("selfhost", "shared")
		for _, scanDir := range []string{srcDir, sharedDir} {
			if needBuild {
				break
			}
			entries, err := os.ReadDir(scanDir)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if !strings.HasSuffix(e.Name(), ".intent") {
					continue
				}
				fi, err := e.Info()
				if err != nil {
					continue
				}
				if fi.ModTime().After(cacheInfo.ModTime()) {
					needBuild = true
					break
				}
			}
		}
	}

	if !needBuild {
		return cachePath, nil
	}

	// Build the stage2 formatter binary.
	intentc, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("could not determine intentc path: %w", err)
	}

	absSrc, err := filepath.Abs(mainSrc)
	if err != nil {
		return "", fmt.Errorf("could not resolve main.intent path: %w", err)
	}

	buildDir, err := os.MkdirTemp("", "intent-stage2-build-*")
	if err != nil {
		return "", fmt.Errorf("could not create build temp dir: %w", err)
	}
	defer os.RemoveAll(buildDir)

	buildCmd := exec.Command(intentc, "build", "--target", "rust", absSrc)
	buildCmd.Dir = buildDir
	buildOut, buildErr := buildCmd.CombinedOutput()
	if buildErr != nil {
		return "", fmt.Errorf("stage2 formatter build failed: %w\n%s", buildErr, strings.TrimSpace(string(buildOut)))
	}

	builtBin := filepath.Join(buildDir, "main")
	if _, err := os.Stat(builtBin); err != nil {
		return "", fmt.Errorf("stage2 formatter build succeeded but binary not found at %s: %w", builtBin, err)
	}

	// Move to cache path (try rename first; fall back to copy for cross-device).
	if err := os.Rename(builtBin, cachePath); err != nil {
		if copyErr := copyFile(builtBin, cachePath, 0755); copyErr != nil {
			return "", fmt.Errorf("could not install stage2 formatter binary: %w", copyErr)
		}
	}

	return cachePath, nil
}

// copyFile copies src to dst with the given permissions.
func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
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

func parseLintFlags(args []string) (selfHosted bool, filePath, errMsg string) {
	for _, arg := range args {
		switch arg {
		case "--self-hosted":
			selfHosted = true
		default:
			if strings.HasPrefix(arg, "-") {
				errMsg = "Unknown option: " + arg
				return
			}
			filePath = arg
		}
	}
	return
}

func handleLint(args []string) {
	selfHosted, filePath, errMsg := parseLintFlags(args)
	if errMsg != "" {
		fmt.Fprintln(os.Stderr, errMsg)
		os.Exit(1)
	}

	if filePath == "" {
		fmt.Fprintln(os.Stderr, "Error: no input file specified")
		os.Exit(1)
	}

	// --self-hosted delegates to the stage2 (Intent) linter binary, which emits
	// output byte-identical to the stage1 (Go) linter below. No silent fallback:
	// a stage2 build/parse failure is surfaced and exits non-zero.
	if selfHosted {
		binPath, err := stage2LinterBinary()
		if err != nil {
			fmt.Fprintf(os.Stderr, "stage2 linter: %s\n", err)
			os.Exit(1)
		}
		out, err := runStage2Linter(binPath, filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			os.Exit(1)
		}
		fmt.Print(out)
		return
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

	diag := linter.Lint(prog)

	if diag.Count() == 0 {
		fmt.Println("No lint warnings.")
		return
	}

	fmt.Print(diag.Format(filePath))
	fmt.Println()
	fmt.Printf("%d warning(s) found.\n", diag.Count())
}

// runStage2Linter runs the prebuilt stage2 linter binary on filePath and returns
// its stdout verbatim. The stage2 linter already emits the full diagnostic block
// plus the "N warning(s) found." / "No lint warnings." summary with the same
// trailing newline as stage1, so no trimming is applied. A non-zero exit (e.g. a
// stage2 parse error) is returned as an error including the binary output.
func runStage2Linter(binaryPath, filePath string) (string, error) {
	cmd := exec.Command(binaryPath, filePath)
	out, err := cmd.Output()
	if err != nil {
		var stderr []byte
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = exitErr.Stderr
		}
		combined := strings.TrimSpace(string(out) + string(stderr))
		if combined == "" {
			return "", fmt.Errorf("stage2 linter exited with error: %w", err)
		}
		return "", fmt.Errorf("%s", combined)
	}
	return string(out), nil
}

// stage2LinterBinary returns the path to the stage2 linter binary, either from the
// INTENT_STAGE2_LINT env override or by auto-building from
// selfhost/linter/lint_main.intent (cached in os.TempDir(), rebuilt when any
// selfhost/linter/*.intent or selfhost/shared/*.intent file is newer than the cache).
func stage2LinterBinary() (string, error) {
	if envPath := os.Getenv("INTENT_STAGE2_LINT"); envPath != "" {
		info, err := os.Stat(envPath)
		if err != nil {
			return "", fmt.Errorf("INTENT_STAGE2_LINT=%q: %w", envPath, err)
		}
		if info.IsDir() || info.Mode()&0111 == 0 {
			return "", fmt.Errorf("INTENT_STAGE2_LINT=%q is not an executable file", envPath)
		}
		return envPath, nil
	}

	srcDir := filepath.Join("selfhost", "linter")
	mainSrc := filepath.Join(srcDir, "lint_main.intent")
	if _, err := os.Stat(mainSrc); err != nil {
		return "", fmt.Errorf(
			"stage2 linter sources not found at selfhost/linter/lint_main.intent; run from the repo root or set INTENT_STAGE2_LINT to a prebuilt binary",
		)
	}

	cachePath := filepath.Join(os.TempDir(), "intent-stage2-lint")

	// Staleness check: rebuild if cached binary is missing or any .intent file
	// in selfhost/linter/ or selfhost/shared/ is newer than the cache.
	needBuild := false
	cacheInfo, err := os.Stat(cachePath)
	if err != nil {
		needBuild = true
	} else {
		sharedDir := filepath.Join("selfhost", "shared")
		for _, scanDir := range []string{srcDir, sharedDir} {
			if needBuild {
				break
			}
			entries, err := os.ReadDir(scanDir)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if !strings.HasSuffix(e.Name(), ".intent") {
					continue
				}
				fi, err := e.Info()
				if err != nil {
					continue
				}
				if fi.ModTime().After(cacheInfo.ModTime()) {
					needBuild = true
					break
				}
			}
		}
	}

	if !needBuild {
		return cachePath, nil
	}

	intentc, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("could not determine intentc path: %w", err)
	}

	absSrc, err := filepath.Abs(mainSrc)
	if err != nil {
		return "", fmt.Errorf("could not resolve lint_main.intent path: %w", err)
	}

	buildDir, err := os.MkdirTemp("", "intent-stage2-lint-build-*")
	if err != nil {
		return "", fmt.Errorf("could not create build temp dir: %w", err)
	}
	defer os.RemoveAll(buildDir)

	buildCmd := exec.Command(intentc, "build", "--target", "rust", absSrc)
	buildCmd.Dir = buildDir
	buildOut, buildErr := buildCmd.CombinedOutput()
	if buildErr != nil {
		return "", fmt.Errorf("stage2 linter build failed: %w\n%s", buildErr, strings.TrimSpace(string(buildOut)))
	}

	builtBin := filepath.Join(buildDir, "lint_main")
	if _, err := os.Stat(builtBin); err != nil {
		return "", fmt.Errorf("stage2 linter build succeeded but binary not found at %s: %w", builtBin, err)
	}

	if err := os.Rename(builtBin, cachePath); err != nil {
		if copyErr := copyFile(builtBin, cachePath, 0755); copyErr != nil {
			return "", fmt.Errorf("could not install stage2 linter binary: %w", copyErr)
		}
	}

	return cachePath, nil
}

// runStage2Checker runs the prebuilt stage2 checker binary on filePath and
// returns its stdout and exit code. Unlike runStage2Linter, a non-zero exit
// does NOT mean an error in running the binary — it means the checker found
// semantic errors. The caller distinguishes exit 0 (clean) from exit 1 (diags).
// stage2CheckPaths returns the argument list for the stage2 checker binary: the
// entry file first (its path is used for diagnostic output), followed by its
// transitive import closure for multi-file programs. Stage1 resolves imports via
// CheckProject/CheckAll; the stage2 checker is a single-Program checker, so the
// harness discovers the closure here (reusing the module registry) and passes
// every module's path — check_main merges their declarations so cross-module
// types and call qualifiers resolve. Single-file programs (or any discovery
// failure) fall back to the entry alone, preserving the original behaviour.
func stage2CheckPaths(entryPath string) []string {
	paths := []string{entryPath}
	isMulti, err := compiler.IsMultiFile(entryPath)
	if err != nil || !isMulti {
		return paths
	}
	registry, err := compiler.NewModuleRegistry(entryPath)
	if err != nil {
		return paths
	}
	depDiag, err := registry.DiscoverDependencies()
	if err != nil || (depDiag != nil && depDiag.HasErrors()) {
		return paths
	}
	sorted, err := registry.TopologicalSort()
	if err != nil {
		return paths
	}
	entryAbs, _ := filepath.Abs(entryPath)
	for _, p := range sorted {
		if pAbs, aerr := filepath.Abs(p); aerr == nil && pAbs == entryAbs {
			continue // entry is passed first, above; don't parse it twice
		}
		paths = append(paths, p)
	}
	return paths
}

// stage2CompilePaths returns the argument list for the stage2 compiler binary:
// the entry file's transitive import closure in TOPOLOGICAL order (dependencies
// first, entry LAST). This mirrors stage1's ir.LowerAll, which treats the last
// sorted path as the program entry and emits modules in that order. Contrast
// stage2CheckPaths, which passes the entry FIRST (its path drives diagnostics);
// emit ordering is output-significant, so the order differs deliberately.
//
// When the entry is multi-file (has imports OR is a package member with intent.toml —
// stage1's IsMultiFile), the args are prefixed with a "--multi" sentinel so the stage2
// binary uses the multi-file path (GenerateAll: "(multi-file)" header, intent blocks
// omitted) even when the closure is a SINGLE module (a lone package/project member).
// This matches stage1's EmitProjectToTarget, which runs for any IsMultiFile entry.
// A true single-file program (or any discovery failure) falls back to the entry alone.
func stage2CompilePaths(entryPath string) []string {
	isMulti, err := compiler.IsMultiFile(entryPath)
	if err != nil || !isMulti {
		return []string{entryPath}
	}
	registry, err := compiler.NewModuleRegistry(entryPath)
	if err != nil {
		return []string{entryPath}
	}
	depDiag, err := registry.DiscoverDependencies()
	if err != nil || (depDiag != nil && depDiag.HasErrors()) {
		return []string{entryPath}
	}
	sorted, err := registry.TopologicalSort()
	if err != nil || len(sorted) == 0 {
		return []string{entryPath}
	}
	return append([]string{"--multi"}, sorted...)
}

func runStage2Checker(binaryPath string, filePaths []string) (stdout string, exitCode int, err error) {
	cmd := exec.Command(binaryPath, filePaths...)
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	runErr := cmd.Run()
	out := outBuf.String()
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			return out, exitErr.ExitCode(), nil
		}
		// Binary failed to run (not just non-zero exit) — surface as error.
		combined := strings.TrimSpace(out + errBuf.String())
		if combined == "" {
			return "", -1, fmt.Errorf("stage2 checker failed: %w", runErr)
		}
		return "", -1, fmt.Errorf("%s", combined)
	}
	return out, 0, nil
}

// stage2CheckerBinary returns the path to the stage2 checker binary, either
// from the INTENT_STAGE2_CHECK env override or by auto-building from
// selfhost/checker/check_main.intent (cached in os.TempDir(), rebuilt when any
// selfhost/checker/*.intent or selfhost/shared/*.intent file is newer than the cache).
func stage2CheckerBinary() (string, error) {
	if envPath := os.Getenv("INTENT_STAGE2_CHECK"); envPath != "" {
		info, err := os.Stat(envPath)
		if err != nil {
			return "", fmt.Errorf("INTENT_STAGE2_CHECK=%q: %w", envPath, err)
		}
		if info.IsDir() || info.Mode()&0111 == 0 {
			return "", fmt.Errorf("INTENT_STAGE2_CHECK=%q is not an executable file", envPath)
		}
		return envPath, nil
	}

	srcDir := filepath.Join("selfhost", "checker")
	mainSrc := filepath.Join(srcDir, "check_main.intent")
	if _, err := os.Stat(mainSrc); err != nil {
		return "", fmt.Errorf(
			"stage2 checker sources not found at selfhost/checker/check_main.intent; run from the repo root or set INTENT_STAGE2_CHECK to a prebuilt binary",
		)
	}

	cachePath := filepath.Join(os.TempDir(), "intent-stage2-check")

	// Staleness check: rebuild if cached binary is missing or any .intent file
	// in selfhost/checker/ or selfhost/shared/ is newer than the cache.
	needBuild := false
	cacheInfo, err := os.Stat(cachePath)
	if err != nil {
		needBuild = true
	} else {
		sharedDir := filepath.Join("selfhost", "shared")
		for _, scanDir := range []string{srcDir, sharedDir} {
			if needBuild {
				break
			}
			entries, err := os.ReadDir(scanDir)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if !strings.HasSuffix(e.Name(), ".intent") {
					continue
				}
				fi, err := e.Info()
				if err != nil {
					continue
				}
				if fi.ModTime().After(cacheInfo.ModTime()) {
					needBuild = true
					break
				}
			}
		}
	}

	if !needBuild {
		return cachePath, nil
	}

	intentc, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("could not determine intentc path: %w", err)
	}

	absSrc, err := filepath.Abs(mainSrc)
	if err != nil {
		return "", fmt.Errorf("could not resolve check_main.intent path: %w", err)
	}

	buildDir, err := os.MkdirTemp("", "intent-stage2-check-build-*")
	if err != nil {
		return "", fmt.Errorf("could not create build temp dir: %w", err)
	}
	defer os.RemoveAll(buildDir)

	buildCmd := exec.Command(intentc, "build", "--target", "rust", absSrc)
	buildCmd.Dir = buildDir
	buildOut, buildErr := buildCmd.CombinedOutput()
	if buildErr != nil {
		return "", fmt.Errorf("stage2 checker build failed: %w\n%s", buildErr, strings.TrimSpace(string(buildOut)))
	}

	builtBin := filepath.Join(buildDir, "check_main")
	if _, err := os.Stat(builtBin); err != nil {
		return "", fmt.Errorf("stage2 checker build succeeded but binary not found at %s: %w", builtBin, err)
	}

	if err := os.Rename(builtBin, cachePath); err != nil {
		if copyErr := copyFile(builtBin, cachePath, 0755); copyErr != nil {
			return "", fmt.Errorf("could not install stage2 checker binary: %w", copyErr)
		}
	}

	return cachePath, nil
}

// stage2CompilerBinary returns the path to the stage2 (Intent) compiler binary
// that emits Rust, either from the INTENT_STAGE2_COMPILE env override or by
// auto-building from selfhost/compiler/compile_main.intent (cached in
// os.TempDir(), rebuilt when any selfhost/compiler/*.intent or
// selfhost/shared/*.intent file is newer than the cache). Mirrors
// stage2CheckerBinary (Phase 54); used by `intentc build --emit --self-hosted`.
func stage2CompilerBinary() (string, error) {
	if envPath := os.Getenv("INTENT_STAGE2_COMPILE"); envPath != "" {
		info, err := os.Stat(envPath)
		if err != nil {
			return "", fmt.Errorf("INTENT_STAGE2_COMPILE=%q: %w", envPath, err)
		}
		if info.IsDir() || info.Mode()&0111 == 0 {
			return "", fmt.Errorf("INTENT_STAGE2_COMPILE=%q is not an executable file", envPath)
		}
		return envPath, nil
	}

	srcDir := filepath.Join("selfhost", "compiler")
	mainSrc := filepath.Join(srcDir, "compile_main.intent")
	if _, err := os.Stat(mainSrc); err != nil {
		return "", fmt.Errorf(
			"stage2 compiler sources not found at selfhost/compiler/compile_main.intent; run from the repo root or set INTENT_STAGE2_COMPILE to a prebuilt binary",
		)
	}

	cachePath := filepath.Join(os.TempDir(), "intent-stage2-compile")

	// Staleness check: rebuild if cached binary is missing or any .intent file
	// in selfhost/compiler/ or selfhost/shared/ is newer than the cache.
	needBuild := false
	cacheInfo, err := os.Stat(cachePath)
	if err != nil {
		needBuild = true
	} else {
		sharedDir := filepath.Join("selfhost", "shared")
		for _, scanDir := range []string{srcDir, sharedDir} {
			if needBuild {
				break
			}
			entries, err := os.ReadDir(scanDir)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if !strings.HasSuffix(e.Name(), ".intent") {
					continue
				}
				fi, err := e.Info()
				if err != nil {
					continue
				}
				if fi.ModTime().After(cacheInfo.ModTime()) {
					needBuild = true
					break
				}
			}
		}
	}

	if !needBuild {
		return cachePath, nil
	}

	intentc, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("could not determine intentc path: %w", err)
	}

	absSrc, err := filepath.Abs(mainSrc)
	if err != nil {
		return "", fmt.Errorf("could not resolve compile_main.intent path: %w", err)
	}

	buildDir, err := os.MkdirTemp("", "intent-stage2-compile-build-*")
	if err != nil {
		return "", fmt.Errorf("could not create build temp dir: %w", err)
	}
	defer os.RemoveAll(buildDir)

	buildCmd := exec.Command(intentc, "build", "--target", "rust", absSrc)
	buildCmd.Dir = buildDir
	buildOut, buildErr := buildCmd.CombinedOutput()
	if buildErr != nil {
		return "", fmt.Errorf("stage2 compiler build failed: %w\n%s", buildErr, strings.TrimSpace(string(buildOut)))
	}

	builtBin := filepath.Join(buildDir, "compile_main")
	if _, err := os.Stat(builtBin); err != nil {
		return "", fmt.Errorf("stage2 compiler build succeeded but binary not found at %s: %w", builtBin, err)
	}

	if err := os.Rename(builtBin, cachePath); err != nil {
		if copyErr := copyFile(builtBin, cachePath, 0755); copyErr != nil {
			return "", fmt.Errorf("could not install stage2 compiler binary: %w", copyErr)
		}
	}

	return cachePath, nil
}

const pkgUsage = `Usage:
  intentc pkg init                            Create intent.toml from module declarations
  intentc pkg add <name> <version>            Add dependency with version constraint
  intentc pkg add <name> --git <url> <ver>    Add git-source dependency (ADR 0039)
  intentc pkg add <name> --path <dir>         Add local path dependency
  intentc pkg remove <name>                   Remove dependency from manifest
  intentc pkg install [--refresh]             Resolve, fetch, and write intent.lock
  intentc pkg upgrade <name> [--major]        Bump a dep to the latest compatible version
  intentc pkg vendor                          Copy resolved deps into ./vendor/ for offline builds
  intentc pkg list                            Show dependency tree
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
		handlePkgInstall(args[1:])
	case "upgrade":
		handlePkgUpgrade(args[1:])
	case "vendor":
		handlePkgVendor(args[1:])
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

func handlePkgInstall(args []string) {
	refresh := false
	for _, a := range args {
		switch a {
		case "--refresh":
			refresh = true
		default:
			fmt.Fprintf(os.Stderr, "Unknown pkg install flag: %s\n", a)
			os.Exit(1)
		}
	}

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
	for _, w := range m.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
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
	if refresh {
		if err := cache.RefreshGit(); err != nil {
			fmt.Fprintf(os.Stderr, "Error refreshing cache: %s\n", err)
			os.Exit(1)
		}
	}

	loader := &compiler.GitFsLoader{
		Fetcher: compiler.GitFetcher{},
		Cache:   cache,
		Root:    manifestDir,
	}
	rs, err := (&compiler.Resolver{Loader: loader}).Resolve(m, manifestDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving dependencies: %s\n", err)
		os.Exit(1)
	}

	// Checksum each git-source package from its cached tree.
	checksumOf := func(p compiler.LockedPackage) (string, error) {
		if !strings.HasPrefix(p.Source, "git+") {
			return "", nil
		}
		// Locate the resolved package to obtain its (host, owner, repo, rev).
		for _, rp := range rs.Packages {
			if rp.Name != p.Name {
				continue
			}
			if !rp.Source.IsGit() {
				return "", nil
			}
			host, owner, repo, err := compiler.ParseGitURL(rp.Source.Git)
			if err != nil {
				return "", err
			}
			return cache.GitTreeChecksum(host, owner, repo, rp.Rev)
		}
		return "", fmt.Errorf("no resolved package matches lockfile entry %s", p.Name)
	}

	lock, err := compiler.FromResolvedSet(rs, checksumOf, time.Now())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error computing lockfile: %s\n", err)
		os.Exit(1)
	}
	lockPath := filepath.Join(manifestDir, "intent.lock")
	if err := compiler.WriteLockfile(lockPath, lock); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing %s: %s\n", lockPath, err)
		os.Exit(1)
	}

	fmt.Printf("Resolved %d package(s); lockfile written to %s.\n", len(lock.Packages), lockPath)
	for _, p := range lock.Packages {
		fmt.Printf("  %s@%s\n", p.Name, p.Version.String())
	}
}

// handlePkgUpgrade bumps a single dependency's minimum version in intent.toml
// and re-runs install. Without --major, the bump is constrained to the same
// major version. With --major, the highest available major is selected.
func handlePkgUpgrade(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: intentc pkg upgrade <name> [--major]")
		os.Exit(1)
	}
	allowMajor := false
	var name string
	for _, a := range args {
		switch {
		case a == "--major":
			allowMajor = true
		case !strings.HasPrefix(a, "-"):
			if name != "" {
				fmt.Fprintln(os.Stderr, "Error: upgrade takes exactly one package name")
				os.Exit(1)
			}
			name = a
		default:
			fmt.Fprintf(os.Stderr, "Unknown pkg upgrade flag: %s\n", a)
			os.Exit(1)
		}
	}
	if name == "" {
		fmt.Fprintln(os.Stderr, "Error: missing package name")
		os.Exit(1)
	}

	manifestDir, err := filepath.Abs(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	m, err := compiler.LoadManifest(manifestDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
	dep, ok := m.Dependencies[name]
	if !ok {
		fmt.Fprintf(os.Stderr, "Error: %s is not a declared dependency\n", name)
		os.Exit(1)
	}
	if !dep.IsGit() {
		fmt.Fprintf(os.Stderr, "Error: only git dependencies can be upgraded; %s is %s\n", name, dependencyKind(dep))
		os.Exit(1)
	}

	curr, err := compiler.ParseVersion(strings.TrimLeft(dep.Version, "^~>= "))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: current version %q is not a valid semver: %s\n", dep.Version, err)
		os.Exit(1)
	}
	tags, err := compiler.GitFetcher{}.ListTags(dep.Git)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing tags for %s: %s\n", dep.Git, err)
		os.Exit(1)
	}
	if len(tags) == 0 {
		fmt.Fprintf(os.Stderr, "Error: %s has no semver tags\n", dep.Git)
		os.Exit(1)
	}
	var picked compiler.Tag
	found := false
	for _, t := range tags {
		if !allowMajor && t.Version.Major != curr.Major {
			continue
		}
		if t.Version.Compare(curr) <= 0 {
			continue
		}
		if !found || t.Version.Compare(picked.Version) > 0 {
			picked = t
			found = true
		}
	}
	if !found {
		fmt.Fprintf(os.Stderr, "Nothing to upgrade — %s is already at the latest %s tag.\n", name, upgradeScope(allowMajor))
		return
	}

	dep.Version = picked.Version.String()
	m.Dependencies[name] = dep
	if err := compiler.WriteManifest(filepath.Join(manifestDir, "intent.toml"), m); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing intent.toml: %s\n", err)
		os.Exit(1)
	}
	fmt.Printf("Bumped %s to %s. Run `intentc pkg install` to refresh the lockfile.\n", name, picked.Version.String())
}

func dependencyKind(d compiler.DependencySpec) string {
	switch {
	case d.IsPath():
		return "a path dependency"
	case d.IsGit():
		return "a git dependency"
	default:
		return "a bare-version dependency"
	}
}

func upgradeScope(allowMajor bool) string {
	if allowMajor {
		return "version"
	}
	return "same-major version"
}

// handlePkgVendor reads intent.lock and copies each resolved package's cached
// tree into ./vendor/<name>-<version>/. When ./vendor/ exists, future builds
// read from it instead of the cache — ADR 0039 §8.
func handlePkgVendor(args []string) {
	if len(args) > 0 {
		fmt.Fprintln(os.Stderr, "Usage: intentc pkg vendor")
		os.Exit(1)
	}
	manifestDir, err := filepath.Abs(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	lockPath := filepath.Join(manifestDir, "intent.lock")
	lock, err := compiler.ReadLockfile(lockPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		fmt.Fprintln(os.Stderr, "Run 'intentc pkg install' first to create intent.lock.")
		os.Exit(1)
	}

	cache, err := compiler.NewPackageCache()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	vendorDir := filepath.Join(manifestDir, "vendor")
	if err := os.RemoveAll(vendorDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error wiping vendor dir: %s\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(vendorDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating vendor dir: %s\n", err)
		os.Exit(1)
	}

	for _, p := range lock.Packages {
		if !strings.HasPrefix(p.Source, "git+") {
			continue // skip path deps; they already live where they live
		}
		gitURL := strings.TrimPrefix(p.Source, "git+")
		host, owner, repo, err := compiler.ParseGitURL(gitURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s: %s\n", p.Name, err)
			os.Exit(1)
		}
		src, err := cache.GitCachePath(host, owner, repo, p.Rev)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s: %s\n", p.Name, err)
			os.Exit(1)
		}
		if _, err := os.Stat(src); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s@%s not in cache (run `intentc pkg install`).\n", p.Name, p.Version.String())
			os.Exit(1)
		}
		dest := filepath.Join(vendorDir, fmt.Sprintf("%s-%s", p.Name, p.Version.String()))
		if err := copyTree(src, dest); err != nil {
			fmt.Fprintf(os.Stderr, "Error copying %s: %s\n", p.Name, err)
			os.Exit(1)
		}
		fmt.Printf("  vendored %s@%s\n", p.Name, p.Version.String())
	}
	fmt.Printf("Vendored %d package(s) to %s\n", len(lock.Packages), vendorDir)
}

func copyTree(src, dest string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		out := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(out, 0755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(out, data, 0644)
	})
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
