package rustbe

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/lhaig/intent/internal/checker"
	"github.com/lhaig/intent/internal/ir"
	"github.com/lhaig/intent/internal/lexer"
)

// Options carries codegen flags that aren't part of the IR. Phase 22 /
// ADR 0033: StripContracts swaps `assert!` for `debug_assert!` on all
// contract checks (preconditions, postconditions, invariants, loop
// invariants, decreases/termination metrics). User-written
// `assert(...)` / `assert_eq(...)` calls from test bodies are
// unaffected — they're the runtime assertion API, not contracts.
type Options struct {
	StripContracts bool
}

// Generate produces Rust source code from a single IR Module.
func Generate(mod *ir.Module, opts Options) string {
	g := &generator{
		entities:       make(map[string]*ir.Entity),
		enums:          make(map[string]*ir.Enum),
		functions:      make(map[string]*ir.Function),
		externs:        make(map[string]*ir.ExternFunction),
		stripContracts: opts.StripContracts,
	}

	for _, e := range mod.Entities {
		g.entities[e.Name] = e
	}
	for _, e := range mod.Enums {
		g.enums[e.Name] = e
	}
	for _, f := range mod.Functions {
		g.functions[f.Name] = f
	}
	for _, ext := range mod.ExternFunctions {
		g.externs[ext.Name] = ext
	}

	g.emitLine("// Generated Rust code from Intent")
	g.emitLine("#![allow(unused_parens, unused_variables, unused_mut, dead_code, private_interfaces)]")
	g.emitLine("")

	for _, e := range mod.Entities {
		g.generateEntity(e)
		g.emitLine("")
	}
	for _, e := range mod.Enums {
		g.generateEnumDecl(e)
		g.emitLine("")
	}
	for _, t := range mod.Traits {
		g.generateTrait(t)
		g.emitLine("")
	}
	for _, ib := range mod.ImplBlocks {
		g.generateImplBlock(ib)
		g.emitLine("")
	}
	for _, f := range mod.Functions {
		g.generateFunction(f)
		g.emitLine("")
	}
	for _, i := range mod.Intents {
		g.generateIntent(i)
		g.emitLine("")
	}
	for _, t := range mod.Tests {
		g.generateTest(t)
		g.emitLine("")
	}

	result := g.sb.String()
	if g.needsHashMap {
		// Insert use statement after #![allow(...)] line
		marker := "#![allow(unused_parens, unused_variables, unused_mut, dead_code, private_interfaces)]\n"
		result = strings.Replace(result, marker, marker+"use std::collections::HashMap;\n", 1)
	}
	if g.needsReqwest {
		marker := "#![allow(unused_parens, unused_variables, unused_mut, dead_code, private_interfaces)]\n"
		if strings.Contains(result, "use std::collections::HashMap;\n") {
			afterHashMap := "use std::collections::HashMap;\n"
			result = strings.Replace(result, afterHashMap, afterHashMap+"use reqwest;\nuse serde_json;\n", 1)
		} else {
			result = strings.Replace(result, marker, marker+"use reqwest;\nuse serde_json;\n", 1)
		}
		// Inject helper functions before the first fn declaration
		helpers := g.emitHttpHelpers()
		fnIdx := strings.Index(result, "\nfn ")
		if fnIdx >= 0 {
			result = result[:fnIdx] + helpers + result[fnIdx:]
		}
	} else if g.needsSerdeJson {
		marker := "#![allow(unused_parens, unused_variables, unused_mut, dead_code, private_interfaces)]\n"
		if strings.Contains(result, "use std::collections::HashMap;\n") {
			afterHashMap := "use std::collections::HashMap;\n"
			result = strings.Replace(result, afterHashMap, afterHashMap+"use serde_json;\n", 1)
		} else {
			result = strings.Replace(result, marker, marker+"use serde_json;\n", 1)
		}
	}
	result = injectAsyncUseStatements(result, g.needsTokio, g.needsFutures)
	return result
}

// injectAsyncUseStatements adds tokio/futures use statements if needed.
func injectAsyncUseStatements(result string, needsTokio, needsFutures bool) string {
	if !needsTokio && !needsFutures {
		return result
	}
	// Find insertion point: after the last existing use statement, or after #![allow(...)]
	marker := "#![allow(unused_parens, unused_variables, unused_mut, dead_code, private_interfaces)]\n"
	insertAfter := marker
	// Find the last use statement to insert after
	lines := strings.Split(result, "\n")
	lastUseIdx := -1
	for i, line := range lines {
		if strings.HasPrefix(line, "use ") {
			lastUseIdx = i
		}
	}
	var useStmts string
	if needsFutures {
		useStmts += "use futures;\n"
	}
	if lastUseIdx >= 0 {
		// Insert after the last use statement
		lines = append(lines[:lastUseIdx+1], append([]string{strings.TrimSuffix(useStmts, "\n")}, lines[lastUseIdx+1:]...)...)
		result = strings.Join(lines, "\n")
		// Clean up empty lines from insertion
		result = strings.ReplaceAll(result, "\n\n\n\n", "\n\n")
	} else if strings.Contains(result, insertAfter) {
		result = strings.Replace(result, insertAfter, insertAfter+useStmts, 1)
	}
	return result
}

// GenerateAll produces Rust source from a multi-file IR Program.
func GenerateAll(prog *ir.Program, opts Options) string {
	if len(prog.Modules) == 0 {
		return ""
	}

	// Build module manglings map.
	// Keys are the qualifier names users write in source (module name, decl name,
	// or package name).  Module-name entries are authoritative; decl-name and
	// package-name entries are only added when they don't collide with an
	// existing entry, preventing a package name from shadowing another module's
	// name entry.
	moduleManglings := make(map[string]string)
	// First pass: module names (highest priority – always set).
	for _, mod := range prog.Modules {
		if !mod.IsEntry {
			moduleManglings[mod.Name] = mod.Name + "_"
		}
	}
	// Second pass: decl names and package names (only if key is free).
	for _, mod := range prog.Modules {
		if !mod.IsEntry {
			if mod.DeclName != "" && mod.DeclName != mod.Name {
				if _, exists := moduleManglings[mod.DeclName]; !exists {
					moduleManglings[mod.DeclName] = mod.Name + "_"
				}
			}
			if mod.PackageName != "" && mod.PackageName != mod.Name {
				if _, exists := moduleManglings[mod.PackageName]; !exists {
					moduleManglings[mod.PackageName] = mod.Name + "_"
				}
			}
		}
	}

	// Build typeOrigins: map entity/enum name -> defining module's struct prefix
	typeOrigins := make(map[string]string)
	for _, mod := range prog.Modules {
		prefix := ""
		if !mod.IsEntry {
			prefix = strings.ToUpper(mod.Name[:1]) + mod.Name[1:]
		}
		for _, e := range mod.Entities {
			typeOrigins[e.Name] = prefix
		}
		for _, e := range mod.Enums {
			typeOrigins[e.Name] = prefix
		}
	}

	// Build global functions map for cross-module lookups
	allFunctions := make(map[string]*ir.Function)
	for _, mod := range prog.Modules {
		for _, f := range mod.Functions {
			allFunctions[f.Name] = f
		}
	}

	// Build global entities map for cross-module constructor lookups
	allEntities := make(map[string]*ir.Entity)
	for _, mod := range prog.Modules {
		for _, e := range mod.Entities {
			allEntities[e.Name] = e
		}
	}

	// Build global extern (FFI) functions map. Phase 15.
	allExterns := make(map[string]*ir.ExternFunction)
	for _, mod := range prog.Modules {
		for _, ext := range mod.ExternFunctions {
			allExterns[ext.Name] = ext
		}
	}

	var sb strings.Builder
	sb.WriteString("// Generated Rust code from Intent (multi-file)\n")
	sb.WriteString("#![allow(unused_parens, unused_variables, unused_mut, dead_code, private_interfaces)]\n\n")

	needsHashMapGlobal := false
	needsReqwestGlobal := false
	needsSerdeJsonGlobal := false
	needsTokioGlobal := false
	needsFuturesGlobal := false
	for _, mod := range prog.Modules {
		g := &generator{
			entities:        make(map[string]*ir.Entity),
			enums:           make(map[string]*ir.Enum),
			functions:       make(map[string]*ir.Function),
			externs:         make(map[string]*ir.ExternFunction),
			isEntryFile:     mod.IsEntry,
			moduleManglings: moduleManglings,
			typeOrigins:     typeOrigins,
			allFunctions:    allFunctions,
			allEntities:     allEntities,
			allExterns:      allExterns,
			stripContracts:  opts.StripContracts,
		}

		if !mod.IsEntry {
			g.namePrefix = mod.Name + "_"
			g.structPrefix = strings.ToUpper(mod.Name[:1]) + mod.Name[1:]
		}

		for _, e := range mod.Entities {
			g.entities[e.Name] = e
		}
		for _, e := range mod.Enums {
			g.enums[e.Name] = e
		}
		for _, f := range mod.Functions {
			g.functions[f.Name] = f
		}
		for _, ext := range mod.ExternFunctions {
			g.externs[ext.Name] = ext
		}

		for _, e := range mod.Entities {
			g.generateEntity(e)
			g.emitLine("")
		}
		for _, e := range mod.Enums {
			g.generateEnumDecl(e)
			g.emitLine("")
		}
		for _, t := range mod.Traits {
			g.generateTrait(t)
			g.emitLine("")
		}
		for _, ib := range mod.ImplBlocks {
			g.generateImplBlock(ib)
			g.emitLine("")
		}
		for _, f := range mod.Functions {
			g.generateFunction(f)
			g.emitLine("")
		}
		for _, t := range mod.Tests {
			g.generateTest(t)
			g.emitLine("")
		}

		sb.WriteString(g.sb.String())
		if g.needsHashMap {
			needsHashMapGlobal = true
		}
		if g.needsReqwest {
			needsReqwestGlobal = true
		}
		if g.needsSerdeJson {
			needsSerdeJsonGlobal = true
		}
		if g.needsTokio {
			needsTokioGlobal = true
		}
		if g.needsFutures {
			needsFuturesGlobal = true
		}
	}

	output := sb.String()
	if needsHashMapGlobal {
		marker := "#![allow(unused_parens, unused_variables, unused_mut, dead_code, private_interfaces)]\n"
		output = strings.Replace(output, marker, marker+"use std::collections::HashMap;\n", 1)
	}
	if needsReqwestGlobal {
		marker := "#![allow(unused_parens, unused_variables, unused_mut, dead_code, private_interfaces)]\n"
		if strings.Contains(output, "use std::collections::HashMap;\n") {
			afterHashMap := "use std::collections::HashMap;\n"
			output = strings.Replace(output, afterHashMap, afterHashMap+"use reqwest;\nuse serde_json;\n", 1)
		} else {
			output = strings.Replace(output, marker, marker+"use reqwest;\nuse serde_json;\n", 1)
		}
		// Use a temporary generator just to call emitHttpHelpers
		tmpG := &generator{}
		helpers := tmpG.emitHttpHelpers()
		fnIdx := strings.Index(output, "\nfn ")
		if fnIdx >= 0 {
			output = output[:fnIdx] + helpers + output[fnIdx:]
		}
	} else if needsSerdeJsonGlobal {
		marker := "#![allow(unused_parens, unused_variables, unused_mut, dead_code, private_interfaces)]\n"
		if strings.Contains(output, "use std::collections::HashMap;\n") {
			afterHashMap := "use std::collections::HashMap;\n"
			output = strings.Replace(output, afterHashMap, afterHashMap+"use serde_json;\n", 1)
		} else {
			output = strings.Replace(output, marker, marker+"use serde_json;\n", 1)
		}
	}
	output = injectAsyncUseStatements(output, needsTokioGlobal, needsFuturesGlobal)
	return output
}

type generator struct {
	sb             strings.Builder
	indent         int
	entities       map[string]*ir.Entity
	enums          map[string]*ir.Enum
	functions      map[string]*ir.Function
	externs        map[string]*ir.ExternFunction // Phase 15: FFI declarations
	inConstructor  bool
	inLabeledBlock bool
	inImplBlock    bool
	ensuresContext bool
	needsHashMap   bool
	needsReqwest   bool
	needsSerdeJson bool
	needsTokio     bool
	needsFutures   bool

	// Multi-file fields
	namePrefix      string
	structPrefix    string
	isEntryFile     bool
	moduleManglings map[string]string
	typeOrigins     map[string]string             // entity/enum name -> defining module's struct prefix
	allFunctions    map[string]*ir.Function       // all functions across all modules (for cross-module ref lookups)
	allEntities     map[string]*ir.Entity         // all entities across all modules (for cross-module constructor lookups)
	allExterns      map[string]*ir.ExternFunction // all extern (FFI) functions across all modules
	mutatedVars     map[string]bool               // variables assigned to in current function body

	// Phase 22 / ADR 0033: when true, contract checks emit
	// `debug_assert!(...)` instead of `assert!(...)`. cargo's --release
	// profile then compiles the calls out. User-written assertion
	// builtins (assert/assert_eq/assert_close/assert_panics) are
	// unaffected.
	stripContracts bool
}

// contractAssertMacro returns the macro name to use for contract-check
// emissions. Phase 22: returns `debug_assert!` under --strip-contracts,
// `assert!` otherwise. This is the single point of policy — every
// contract-check `emitLinef` uses this helper rather than a literal
// `assert!`.
func (g *generator) contractAssertMacro() string {
	if g.stripContracts {
		return "debug_assert!"
	}
	return "assert!"
}

func (g *generator) emit(s string) {
	g.sb.WriteString(s)
}

func (g *generator) emitf(format string, args ...any) {
	g.sb.WriteString(fmt.Sprintf(format, args...))
}

func (g *generator) emitLinef(format string, args ...any) {
	g.sb.WriteString(g.indentStr())
	g.sb.WriteString(fmt.Sprintf(format, args...))
}

func (g *generator) emitLine(s string) {
	if s == "" {
		g.sb.WriteString("\n")
	} else {
		g.sb.WriteString(g.indentStr())
		g.sb.WriteString(s)
		g.sb.WriteString("\n")
	}
}

func (g *generator) incIndent() { g.indent++ }
func (g *generator) decIndent() { g.indent-- }

func (g *generator) indentStr() string {
	return strings.Repeat("    ", g.indent)
}

// --- Type mapping ---

// fnReturnType maps an Intent function's declared return type to its Rust
// signature type. For async functions whose Intent signature declares
// `returns Future<T>`, the Rust signature drops the Future wrapper because
// `async fn ... -> X` already produces an `impl Future<Output = X>`. Without
// this peel, the emitted signature `async fn f() -> JoinHandle<X>` does not
// match the body which returns X.
func (g *generator) fnReturnType(f *ir.Function) string {
	t := f.ReturnType
	if f.IsAsync && t != nil && t.Name == "Future" && t.IsGeneric && len(t.TypeParams) == 1 {
		return g.mapType(t.TypeParams[0])
	}
	return g.mapType(t)
}

// fnResultType is the type assigned to __result inside a labeled-block body.
// Same Future-peeling rule as fnReturnType because __result is what gets
// returned from the function.
func (g *generator) fnResultType(f *ir.Function) string {
	return g.fnReturnType(f)
}

func (g *generator) mapType(t *checker.Type) string {
	if t == nil {
		return "()"
	}
	switch t.Name {
	case "Int":
		return "i64"
	case "Float":
		return "f64"
	case "String":
		return "String"
	case "Bool":
		return "bool"
	case "Void":
		return "()"
	case "Char":
		return "char"
	case "Array":
		if t.IsGeneric && len(t.TypeParams) == 1 {
			return "Vec<" + g.mapType(t.TypeParams[0]) + ">"
		}
		return "Vec<_>"
	case "Result":
		if t.IsGeneric && len(t.TypeParams) == 2 {
			return "Result<" + g.mapType(t.TypeParams[0]) + ", " + g.mapType(t.TypeParams[1]) + ">"
		}
		return "Result<_, _>"
	case "Option":
		if t.IsGeneric && len(t.TypeParams) == 1 {
			return "Option<" + g.mapType(t.TypeParams[0]) + ">"
		}
		return "Option<_>"
	case "Map":
		if t.IsGeneric && len(t.TypeParams) == 2 {
			g.needsHashMap = true
			return "HashMap<" + g.mapType(t.TypeParams[0]) + ", " + g.mapType(t.TypeParams[1]) + ">"
		}
		return "HashMap<_, _>"
	case "Future":
		if t.IsGeneric && len(t.TypeParams) == 1 {
			inner := g.mapType(t.TypeParams[0])
			return "tokio::task::JoinHandle<" + inner + ">"
		}
		return "tokio::task::JoinHandle<()>"
	case "Fn":
		if t.IsFunction {
			params := make([]string, len(t.FnParams))
			for i, p := range t.FnParams {
				params[i] = g.mapType(p)
			}
			return "impl Fn(" + strings.Join(params, ", ") + ") -> " + g.mapType(t.FnReturn)
		}
		return "impl Fn()"
	default:
		if t.IsEntity {
			return g.mangledEntityName(t.Name)
		}
		if t.IsEnum {
			return g.mangledEnumName(t.Name)
		}
		return t.Name
	}
}

func (g *generator) defaultValue(t *checker.Type) string {
	if t == nil {
		return "()"
	}
	switch t.Name {
	case "Int":
		return "0i64"
	case "Float":
		return "0.0"
	case "String":
		return "String::new()"
	case "Bool":
		return "false"
	case "Char":
		return "'\\0'"
	case "Array":
		return "Vec::new()"
	case "Map":
		g.needsHashMap = true
		return "HashMap::new()"
	default:
		if t.IsEnum && t.EnumInfo != nil {
			// Use the first unit variant as default
			for _, v := range t.EnumInfo.Variants {
				if len(v.Fields) == 0 {
					return fmt.Sprintf("%s::%s", g.mangledEnumName(t.Name), v.Name)
				}
			}
		}
		if t.IsEntity {
			return fmt.Sprintf("%s { /* default fields */ }", g.mangledEntityName(t.Name))
		}
		return fmt.Sprintf("%s { /* default fields */ }", t.Name)
	}
}

func (g *generator) mangledEntityName(name string) string {
	if g.typeOrigins != nil {
		if prefix, ok := g.typeOrigins[name]; ok {
			return prefix + name
		}
	}
	if g.structPrefix != "" {
		return g.structPrefix + name
	}
	return name
}

func (g *generator) mangledEnumName(name string) string {
	if g.typeOrigins != nil {
		if prefix, ok := g.typeOrigins[name]; ok {
			return prefix + name
		}
	}
	if g.structPrefix != "" {
		return g.structPrefix + name
	}
	return name
}

// --- Function generation ---

func (g *generator) generateFunction(f *ir.Function) {
	g.mutatedVars = collectMutatedVars(f.Body)
	if f.IsEntry {
		if f.IsAsync {
			g.needsTokio = true
			g.emitLine("async fn __intent_main() -> i64 {")
			g.incIndent()
			g.generateStmts(f.Body)
			g.decIndent()
			g.emitLine("}")
			g.emitLine("")
			g.emitLine("#[tokio::main]")
			g.emitLine("async fn main() {")
			g.incIndent()
			g.emitLine("let __exit_code = __intent_main().await;")
			g.emitLine("std::process::exit(__exit_code as i32);")
			g.decIndent()
			g.emitLine("}")
		} else {
			g.emitLine("fn __intent_main() -> i64 {")
			g.incIndent()
			g.generateStmts(f.Body)
			g.decIndent()
			g.emitLine("}")
			g.emitLine("")
			g.emitLine("fn main() {")
			g.incIndent()
			g.emitLine("let __exit_code = __intent_main();")
			g.emitLine("std::process::exit(__exit_code as i32);")
			g.decIndent()
			g.emitLine("}")
		}
	} else {
		// arrayRefParams tracks Array and Map params that should be passed by reference
		arrayRefParams := make(map[string]bool)
		for _, p := range f.Params {
			if p.Type != nil && (p.Type.Name == "Array" || p.Type.Name == "Map") {
				arrayRefParams[p.Name] = true
			}
		}

		fnName := f.Name
		if g.namePrefix != "" {
			fnName = g.namePrefix + f.Name
		}

		if f.IsAsync {
			g.needsTokio = true
			g.emitLinef("async fn %s(", fnName)
		} else {
			g.emitLinef("fn %s(", fnName)
		}
		for i, p := range f.Params {
			if i > 0 {
				g.emit(", ")
			}
			paramType := g.mapType(p.Type)
			if p.Type != nil && (p.Type.Name == "Array" || p.Type.Name == "Map") {
				paramType = "&" + paramType
			}
			g.emitf("%s: %s", p.Name, paramType)
		}
		g.emitf(") -> %s {\n", g.fnReturnType(f))
		g.incIndent()

		// Requires
		for _, req := range f.Requires {
			g.emitLinef("%s(%s, \"Precondition failed: %s\");\n", g.contractAssertMacro(),
				g.generateExpr(req.Expr, arrayRefParams), escapeRustString(req.RawText))
		}

		// Ensures with labeled block
		needsLabeledBlock := len(f.Ensures) > 0 && f.ReturnType != nil && f.ReturnType.Name != "Void"

		if needsLabeledBlock {
			g.emitLinef("let __result: %s = 'body: {\n", g.fnResultType(f))
			g.incIndent()
			g.inLabeledBlock = true
			g.generateStmtsWithArrayRef(f.Body, arrayRefParams)
			g.inLabeledBlock = false
			g.decIndent()
			g.emitLine("};")

			g.ensuresContext = true
			for _, ens := range f.Ensures {
				g.emitLinef("%s(%s, \"Postcondition failed: %s\");\n", g.contractAssertMacro(),
					g.generateExpr(ens.Expr, arrayRefParams), escapeRustString(ens.RawText))
			}
			g.ensuresContext = false
			g.emitLine("__result")
		} else {
			g.generateStmtsWithArrayRef(f.Body, arrayRefParams)

			if len(f.Ensures) > 0 {
				g.ensuresContext = true
				for _, ens := range f.Ensures {
					g.emitLinef("%s(%s, \"Postcondition failed: %s\");\n", g.contractAssertMacro(),
						g.generateExpr(ens.Expr, arrayRefParams), escapeRustString(ens.RawText))
				}
				g.ensuresContext = false
			}
		}

		g.decIndent()
		g.emitLine("}")
	}
}

// --- Entity generation ---

func (g *generator) generateEntity(e *ir.Entity) {
	mangledName := g.mangledEntityName(e.Name)

	g.emitLine("#[derive(Clone, Debug)]")
	g.emitLinef("struct %s {\n", mangledName)
	g.incIndent()
	for _, f := range e.Fields {
		g.emitLinef("%s: %s,\n", f.Name, g.mapType(f.Type))
	}
	g.decIndent()
	g.emitLine("}")
	g.emitLine("")

	g.emitLinef("impl %s {\n", mangledName)
	g.incIndent()

	if len(e.Invariants) > 0 {
		g.emitLine("fn __check_invariants(&self) {")
		g.incIndent()
		for _, inv := range e.Invariants {
			g.emitLinef("%s(%s, \"Invariant failed: %s\");\n", g.contractAssertMacro(),
				g.generateExpr(inv.Expr, nil), escapeRustString(inv.RawText))
		}
		g.decIndent()
		g.emitLine("}")
		g.emitLine("")
	}

	if e.Constructor != nil {
		g.generateConstructor(e)
		g.emitLine("")
	}

	for _, m := range e.Methods {
		g.generateMethod(e, m)
		g.emitLine("")
	}

	g.decIndent()
	g.emitLine("}")
}

func (g *generator) generateConstructor(e *ir.Entity) {
	mangledName := g.mangledEntityName(e.Name)
	ctor := e.Constructor
	g.mutatedVars = collectMutatedVars(ctor.Body)

	g.emitLinef("fn new(")
	for i, p := range ctor.Params {
		if i > 0 {
			g.emit(", ")
		}
		g.emitf("%s: %s", p.Name, g.mapType(p.Type))
	}
	g.emitf(") -> %s {\n", mangledName)
	g.incIndent()

	// Requires
	for _, req := range ctor.Requires {
		g.emitLinef("%s(%s, \"Precondition failed: %s\");\n", g.contractAssertMacro(),
			g.generateExpr(req.Expr, nil), escapeRustString(req.RawText))
	}

	// Initialize with defaults
	g.emitLinef("let mut __self = %s {\n", mangledName)
	g.incIndent()
	for _, f := range e.Fields {
		g.emitLinef("%s: %s,\n", f.Name, g.defaultValue(f.Type))
	}
	g.decIndent()
	g.emitLine("};")

	// Body
	g.inConstructor = true
	g.generateStmts(ctor.Body)

	// Ensures
	g.ensuresContext = true
	for _, ens := range ctor.Ensures {
		g.emitLinef("%s(%s, \"Postcondition failed: %s\");\n", g.contractAssertMacro(),
			g.generateExpr(ens.Expr, nil), escapeRustString(ens.RawText))
	}
	g.ensuresContext = false
	g.inConstructor = false

	// Invariant check
	if len(e.Invariants) > 0 {
		g.emitLine("__self.__check_invariants();")
	}

	g.emitLine("__self")
	g.decIndent()
	g.emitLine("}")
}

// methodMutatesSelf checks if a method's body mutates self (assigns to self fields,
// calls mutating methods on self like push/insert/remove/set, etc.).
func methodMutatesSelf(stmts []Stmt) bool {
	for _, stmt := range stmts {
		if stmtMutatesSelf(stmt) {
			return true
		}
	}
	return false
}

func stmtMutatesSelf(stmt Stmt) bool {
	switch s := stmt.(type) {
	case *ir.AssignStmt:
		if targetsMutSelf(s.Target) {
			return true
		}
	case *ir.ExprStmt:
		if exprMutatesSelf(s.Expr) {
			return true
		}
	case *ir.IfStmt:
		if methodMutatesSelf(s.Then) || methodMutatesSelf(s.Else) {
			return true
		}
	case *ir.WhileStmt:
		if methodMutatesSelf(s.Body) {
			return true
		}
	case *ir.ForInStmt:
		if methodMutatesSelf(s.Body) {
			return true
		}
	case *ir.ReturnStmt:
		// return doesn't mutate self
	}
	return false
}

// targetsMutSelf checks if an assignment target is a field on self.
func targetsMutSelf(expr ir.Expr) bool {
	switch e := expr.(type) {
	case *ir.FieldAccessExpr:
		if _, ok := e.Object.(*ir.SelfRef); ok {
			return true
		}
		return targetsMutSelf(e.Object)
	case *ir.IndexExpr:
		return targetsMutSelf(e.Object)
	}
	return false
}

// exprMutatesSelf checks if an expression statement mutates self (e.g., self.list.push(x)).
func exprMutatesSelf(expr ir.Expr) bool {
	switch e := expr.(type) {
	case *ir.MethodCallExpr:
		// Check if object is self or self.field and method is mutating
		if isSelfOrSelfField(e.Object) {
			mutatingMethods := map[string]bool{
				"push": true, "pop": true, "insert": true, "remove": true,
				"set": true, "clear": true, "sort": true, "reverse": true,
			}
			if mutatingMethods[e.Method] {
				return true
			}
		}
	}
	return false
}

func isSelfOrSelfField(expr ir.Expr) bool {
	switch e := expr.(type) {
	case *ir.SelfRef:
		return true
	case *ir.FieldAccessExpr:
		return isSelfOrSelfField(e.Object)
	case *ir.IndexExpr:
		return isSelfOrSelfField(e.Object)
	}
	return false
}

type Stmt = ir.Stmt

func (g *generator) generateMethod(e *ir.Entity, m *ir.Method) {
	g.mutatedVars = collectMutatedVars(m.Body)
	selfReceiver := "&mut self"
	// Use &self for non-mutating methods, but always use &mut self in impl blocks
	// to match trait declarations which always declare &mut self.
	if !g.inImplBlock && !methodMutatesSelf(m.Body) {
		selfReceiver = "&self"
	}
	g.emitLinef("fn %s(%s", m.Name, selfReceiver)
	for _, p := range m.Params {
		g.emitf(", %s: %s", p.Name, g.mapType(p.Type))
	}
	if m.ReturnType == nil || m.ReturnType.Name == "Void" {
		g.emit(") {\n")
	} else {
		g.emitf(") -> %s {\n", g.mapType(m.ReturnType))
	}
	g.incIndent()

	// Old captures
	for _, cap := range m.OldCaptures {
		g.emitLinef("let %s = %s;\n", cap.Name, g.generateExpr(cap.Expr, nil))
	}

	// Requires
	for _, req := range m.Requires {
		g.emitLinef("%s(%s, \"Precondition failed: %s\");\n", g.contractAssertMacro(),
			g.generateExpr(req.Expr, nil), escapeRustString(req.RawText))
	}

	// Labeled block for non-Void methods with ensures/invariants
	hasInvariants := len(e.Invariants) > 0
	needsLabeledBlock := (m.ReturnType != nil && m.ReturnType.Name != "Void") && (len(m.Ensures) > 0 || hasInvariants)

	if needsLabeledBlock {
		g.emitLinef("let __result: %s = 'body: {\n", g.mapType(m.ReturnType))
		g.incIndent()
		g.inLabeledBlock = true
		g.generateStmts(m.Body)
		g.inLabeledBlock = false
		g.decIndent()
		g.emitLine("};")

		g.ensuresContext = true
		for _, ens := range m.Ensures {
			g.emitLinef("%s(%s, \"Postcondition failed: %s\");\n", g.contractAssertMacro(),
				g.generateExpr(ens.Expr, nil), escapeRustString(ens.RawText))
		}
		g.ensuresContext = false

		if hasInvariants {
			g.emitLine("self.__check_invariants();")
		}

		g.emitLine("__result")
	} else {
		g.generateStmts(m.Body)

		if len(m.Ensures) > 0 {
			g.ensuresContext = true
			for _, ens := range m.Ensures {
				g.emitLinef("%s(%s, \"Postcondition failed: %s\");\n", g.contractAssertMacro(),
					g.generateExpr(ens.Expr, nil), escapeRustString(ens.RawText))
			}
			g.ensuresContext = false
		}

		if hasInvariants {
			g.emitLine("self.__check_invariants();")
		}
	}

	g.decIndent()
	g.emitLine("}")
}

// --- Enum generation ---

func (g *generator) generateEnumDecl(e *ir.Enum) {
	mangledName := g.mangledEnumName(e.Name)

	g.emitLine("#[derive(Clone, Debug)]")
	g.emitLinef("enum %s {\n", mangledName)
	g.incIndent()
	for _, v := range e.Variants {
		if len(v.Fields) == 0 {
			g.emitLinef("%s,\n", v.Name)
		} else {
			g.emitf("%s%s { ", g.indentStr(), v.Name)
			for i, f := range v.Fields {
				if i > 0 {
					g.emit(", ")
				}
				g.emitf("%s: %s", f.Name, g.mapType(f.Type))
			}
			g.emit(" },\n")
		}
	}
	g.decIndent()
	g.emitLine("}")
}

// --- Trait generation ---

func (g *generator) generateTrait(t *ir.Trait) {
	if t.IsPublic {
		g.emitLinef("pub trait %s {\n", t.Name)
	} else {
		g.emitLinef("trait %s {\n", t.Name)
	}
	g.incIndent()
	for _, m := range t.Methods {
		g.emitLinef("fn %s(&mut self", m.Name)
		for _, p := range m.Params {
			g.emitf(", %s: %s", p.Name, g.mapType(p.Type))
		}
		if m.ReturnType == nil || m.ReturnType.Name == "Void" {
			g.emit(");\n")
		} else {
			g.emitf(") -> %s;\n", g.mapType(m.ReturnType))
		}
	}
	g.decIndent()
	g.emitLine("}")
}

// --- Impl block generation ---

func (g *generator) generateImplBlock(ib *ir.ImplBlock) {
	g.emitLinef("impl %s for %s {\n", ib.TraitName, g.mangledEntityName(ib.EntityName))
	g.incIndent()

	// Look up the entity so generateMethod can emit invariant checks.
	entity := g.entities[ib.EntityName]
	if entity == nil {
		// Synthesize a minimal entity with no invariants to avoid nil dereference.
		entity = &ir.Entity{Name: ib.EntityName}
	}

	g.inImplBlock = true
	for _, m := range ib.Methods {
		g.generateMethod(entity, m)
		g.emitLine("")
	}
	g.inImplBlock = false

	g.decIndent()
	g.emitLine("}")
}

// --- Intent generation ---

// SanitiseTestNameExternal is the exported form of sanitiseTestName for use
// by other packages (notably internal/compiler/test_runner.go which needs to
// map declared test names back to the sanitised Rust identifiers in cargo
// test output).
func SanitiseTestNameExternal(name string) string { return sanitiseTestName(name) }

// sanitiseTestName converts a human-readable test name to a Rust-legal
// identifier: lowercase ASCII letters/digits with non-alphanumerics replaced
// by underscore, runs of underscores collapsed. Empty inputs yield "unnamed".
func sanitiseTestName(name string) string {
	var b strings.Builder
	prevUnderscore := true // suppress leading underscores
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			prevUnderscore = false
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r - 'A' + 'a')
			prevUnderscore = false
		case r >= '0' && r <= '9':
			if b.Len() == 0 {
				b.WriteByte('_')
			}
			b.WriteRune(r)
			prevUnderscore = false
		default:
			if !prevUnderscore {
				b.WriteByte('_')
				prevUnderscore = true
			}
		}
	}
	// Trim trailing underscore
	s := b.String()
	s = strings.TrimRight(s, "_")
	if s == "" {
		return "unnamed"
	}
	return s
}

// generateTest emits a Rust #[test] function for an ir.Test. Body statements
// reuse the standard statement-emission path so the four assertion builtins
// (assert / assert_eq / assert_close / assert_panics) and any other Intent
// code in the test work identically to the same code in a function body.
func (g *generator) generateTest(t *ir.Test) {
	fnName := "__test_" + sanitiseTestName(t.Name)
	if t.IsAsync {
		g.needsTokio = true
		g.emitLinef("#[tokio::test]\n")
		g.emitLinef("async fn %s() {\n", fnName)
	} else {
		g.emitLinef("#[test]\n")
		g.emitLinef("fn %s() {\n", fnName)
	}
	g.incIndent()
	g.generateStmts(t.Body)
	g.decIndent()
	g.emitLine("}")
}

func (g *generator) generateIntent(i *ir.Intent) {
	g.emitLinef("// Intent: %s\n", i.Description)
	for _, goal := range i.Goals {
		g.emitLinef("// Goal: %s\n", goal)
	}
	for _, constraint := range i.Constraints {
		g.emitLinef("// Constraint: %s\n", constraint)
	}
	for _, guarantee := range i.Guarantees {
		g.emitLinef("// Guarantee: %s\n", guarantee)
	}
	for _, vb := range i.VerifiedBy {
		g.emitLinef("// Verified by: %s\n", strings.Join(vb, "."))
	}
	g.emitLine("")

	testName := g.mangleIntentName(i.Description)
	g.emitLine("#[cfg(test)]")
	g.emitLinef("mod %s {\n", testName)
	g.incIndent()
	g.emitLine("// Intent verification completed at compile time.")
	g.decIndent()
	g.emitLine("}")
}

// --- Statement generation ---

func (g *generator) generateStmts(stmts []ir.Stmt) {
	g.generateStmtsWithArrayRef(stmts, nil)
}

func (g *generator) generateStmtsWithArrayRef(stmts []ir.Stmt, arrayRefParams map[string]bool) {
	for _, stmt := range stmts {
		g.generateStmt(stmt, arrayRefParams)
	}
}

func (g *generator) generateStmt(s ir.Stmt, arrayRefParams map[string]bool) {
	switch stmt := s.(type) {
	case *ir.LetStmt:
		isMut := stmt.Mutable || g.mutatedVars[stmt.Name]
		valueExpr := g.generateExpr(stmt.Value, arrayRefParams)

		// Clone array/map ref params when binding
		if stmt.Type != nil && (stmt.Type.Name == "Array" || stmt.Type.Name == "Map") {
			if vr, ok := stmt.Value.(*ir.VarRef); ok {
				if arrayRefParams != nil && arrayRefParams[vr.Name] {
					valueExpr = valueExpr + ".clone()"
				}
			}
		}

		// Clone index expressions for non-Copy types (e.g., let x: String = arr[i])
		if _, isIdx := stmt.Value.(*ir.IndexExpr); isIdx && !isCopyType(stmt.Type) {
			if !strings.HasSuffix(valueExpr, ".clone()") {
				valueExpr = valueExpr + ".clone()"
			}
		}

		// Fn types use type inference in let bindings (impl Fn not allowed in variable position)
		if stmt.Type != nil && stmt.Type.IsFunction {
			if isMut {
				g.emitLinef("let mut %s = %s;\n", stmt.Name, valueExpr)
			} else {
				g.emitLinef("let %s = %s;\n", stmt.Name, valueExpr)
			}
		} else if isMut {
			g.emitLinef("let mut %s: %s = %s;\n",
				stmt.Name, g.mapType(stmt.Type), valueExpr)
		} else {
			g.emitLinef("let %s: %s = %s;\n",
				stmt.Name, g.mapType(stmt.Type), valueExpr)
		}

	case *ir.AssignStmt:
		valueStr := g.generateExpr(stmt.Value, arrayRefParams)
		// Clone non-Copy field access and index access to avoid partial moves
		if fa, ok := stmt.Value.(*ir.FieldAccessExpr); ok && !isCopyType(fa.Type) {
			if !strings.HasSuffix(valueStr, ".clone()") {
				valueStr += ".clone()"
			}
		}
		if idx, ok := stmt.Value.(*ir.IndexExpr); ok && !isCopyType(idx.Type) {
			if !strings.HasSuffix(valueStr, ".clone()") {
				valueStr += ".clone()"
			}
		}
		g.emitLinef("%s = %s;\n",
			g.generateExpr(stmt.Target, arrayRefParams),
			valueStr)

	case *ir.ReturnStmt:
		if g.inLabeledBlock {
			if stmt.Value != nil {
				g.emitLinef("break 'body %s;\n", g.generateExpr(stmt.Value, arrayRefParams))
			} else {
				g.emitLine("break 'body;")
			}
		} else {
			if stmt.Value != nil {
				g.emitLinef("return %s;\n", g.generateExpr(stmt.Value, arrayRefParams))
			} else {
				g.emitLine("return;")
			}
		}

	case *ir.WhileStmt:
		g.generateWhileStmt(stmt, arrayRefParams)

	case *ir.ForInStmt:
		g.generateForInStmt(stmt, arrayRefParams)

	case *ir.BreakStmt:
		g.emitLine("break;")

	case *ir.ContinueStmt:
		g.emitLine("continue;")

	case *ir.IfStmt:
		g.generateIfStmt(stmt, arrayRefParams)

	case *ir.ExprStmt:
		g.emitLinef("%s;\n", g.generateExpr(stmt.Expr, arrayRefParams))
	}
}

func (g *generator) generateIfStmt(stmt *ir.IfStmt, arrayRefParams map[string]bool) {
	g.emitLinef("if %s {\n", g.generateExpr(stmt.Condition, arrayRefParams))
	g.incIndent()
	g.generateStmtsWithArrayRef(stmt.Then, arrayRefParams)
	g.decIndent()
	g.emitLinef("}")

	if stmt.Else != nil {
		if len(stmt.Else) == 1 {
			if elseIf, ok := stmt.Else[0].(*ir.IfStmt); ok {
				g.emit(" else ")
				g.emitf("if %s {\n", g.generateExpr(elseIf.Condition, arrayRefParams))
				g.incIndent()
				g.generateStmtsWithArrayRef(elseIf.Then, arrayRefParams)
				g.decIndent()
				g.emitLinef("}")
				if elseIf.Else != nil {
					g.emit(" else {\n")
					g.incIndent()
					g.generateStmtsWithArrayRef(elseIf.Else, arrayRefParams)
					g.decIndent()
					g.emitLinef("}\n")
				} else {
					g.emit("\n")
				}
				return
			}
		}
		g.emit(" else {\n")
		g.incIndent()
		g.generateStmtsWithArrayRef(stmt.Else, arrayRefParams)
		g.decIndent()
		g.emitLine("}")
	} else {
		g.emit("\n")
	}
}

func (g *generator) generateWhileStmt(stmt *ir.WhileStmt, arrayRefParams map[string]bool) {
	hasContracts := len(stmt.Invariants) > 0 || stmt.Decreases != nil

	if hasContracts {
		g.emitLine("{")
		g.incIndent()

		// Old captures from invariants
		for _, cap := range stmt.OldCaptures {
			g.emitLinef("let %s = %s;\n", cap.Name, g.generateExpr(cap.Expr, arrayRefParams))
		}

		// Check invariants at entry
		savedEnsures := g.ensuresContext
		g.ensuresContext = true
		for _, inv := range stmt.Invariants {
			g.emitLinef("%s(%s, \"Loop invariant failed at entry: %s\");\n", g.contractAssertMacro(),
				g.generateExpr(inv.Expr, arrayRefParams), escapeRustString(inv.RawText))
		}
		g.ensuresContext = savedEnsures

		// Decreases initialization
		if stmt.Decreases != nil {
			metricExpr := g.generateExpr(stmt.Decreases.Expr, arrayRefParams)
			g.emitLinef("let mut __decreases_prev: i64 = %s;\n", metricExpr)
			g.emitLinef("%s(__decreases_prev >= 0, \"Decreases metric must be non-negative at entry: %s\");\n", g.contractAssertMacro(),
				escapeRustString(stmt.Decreases.RawText))
		}

		g.emitLinef("while %s {\n", g.generateExpr(stmt.Condition, arrayRefParams))
		g.incIndent()

		g.generateStmtsWithArrayRef(stmt.Body, arrayRefParams)

		// Check invariants after iteration
		g.ensuresContext = true
		for _, inv := range stmt.Invariants {
			g.emitLinef("%s(%s, \"Loop invariant failed after iteration: %s\");\n", g.contractAssertMacro(),
				g.generateExpr(inv.Expr, arrayRefParams), escapeRustString(inv.RawText))
		}
		g.ensuresContext = savedEnsures

		// Check decreases
		if stmt.Decreases != nil {
			metricExpr := g.generateExpr(stmt.Decreases.Expr, arrayRefParams)
			g.emitLinef("let __decreases_next: i64 = %s;\n", metricExpr)
			g.emitLinef("%s(__decreases_next < __decreases_prev, \"Termination metric did not decrease: %s\");\n", g.contractAssertMacro(),
				escapeRustString(stmt.Decreases.RawText))
			g.emitLinef("%s(__decreases_next >= 0, \"Termination metric became negative: %s\");\n", g.contractAssertMacro(),
				escapeRustString(stmt.Decreases.RawText))
			g.emitLine("__decreases_prev = __decreases_next;")
		}

		g.decIndent()
		g.emitLine("}")

		g.decIndent()
		g.emitLine("}")
	} else {
		g.emitLinef("while %s {\n", g.generateExpr(stmt.Condition, arrayRefParams))
		g.incIndent()
		g.generateStmtsWithArrayRef(stmt.Body, arrayRefParams)
		g.decIndent()
		g.emitLine("}")
	}
}

func (g *generator) generateForInStmt(stmt *ir.ForInStmt, arrayRefParams map[string]bool) {
	g.emit(g.indentStr())
	g.emitf("for %s in ", stmt.Variable)

	if rangeExpr, ok := stmt.Iterable.(*ir.RangeExpr); ok {
		g.emitf("(%s..%s)",
			g.generateExpr(rangeExpr.Start, arrayRefParams),
			g.generateExpr(rangeExpr.End, arrayRefParams))
	} else {
		g.emitf("%s.iter()", g.generateExpr(stmt.Iterable, arrayRefParams))
	}

	g.emit(" {\n")
	g.incIndent()
	g.generateStmtsWithArrayRef(stmt.Body, arrayRefParams)
	g.decIndent()
	g.emitLine("}")
}

// --- Expression generation ---

func (g *generator) generateExpr(e ir.Expr, arrayRefParams map[string]bool) string {
	if e == nil {
		return "<nil>"
	}
	switch expr := e.(type) {
	case *ir.BinaryExpr:
		left := g.generateExpr(expr.Left, arrayRefParams)
		right := g.generateExpr(expr.Right, arrayRefParams)
		op := g.mapOperator(expr.Op)

		// String concatenation detection by AST node type (fallback - should be StringConcat in IR)
		if expr.Op == lexer.PLUS {
			if _, ok := expr.Left.(*ir.StringLit); ok {
				return fmt.Sprintf("format!(\"{}{}\", %s, %s)", left, right)
			}
			if _, ok := expr.Right.(*ir.StringLit); ok {
				return fmt.Sprintf("format!(\"{}{}\", %s, %s)", left, right)
			}
		}

		if expr.Op == lexer.IMPLIES {
			return fmt.Sprintf("(!%s || %s)", left, right)
		}

		return fmt.Sprintf("(%s %s %s)", left, op, right)

	case *ir.StringConcat:
		left := g.generateExpr(expr.Left, arrayRefParams)
		right := g.generateExpr(expr.Right, arrayRefParams)
		return fmt.Sprintf("format!(\"{}{}\", %s, %s)", left, right)

	case *ir.UnaryExpr:
		operand := g.generateExpr(expr.Operand, arrayRefParams)
		if expr.Op == lexer.NOT {
			return fmt.Sprintf("!%s", operand)
		}
		return fmt.Sprintf("-%s", operand)

	case *ir.CallExpr:
		return g.generateCallExpr(expr, arrayRefParams)

	case *ir.MethodCallExpr:
		return g.generateMethodCallExpr(expr, arrayRefParams)

	case *ir.FieldAccessExpr:
		obj := g.generateExpr(expr.Object, arrayRefParams)
		result := fmt.Sprintf("%s.%s", obj, expr.Field)
		// Clone field access from indexed Vec elements to avoid moving out of borrowed content
		if _, isIndex := expr.Object.(*ir.IndexExpr); isIndex {
			result += ".clone()"
		}
		return result

	case *ir.OldRef:
		return expr.Name

	case *ir.VarRef:
		// Unit enum variants need to be qualified with their enum name
		if expr.Type != nil && expr.Type.IsEnum && expr.Type.EnumInfo != nil {
			for _, v := range expr.Type.EnumInfo.Variants {
				if v.Name == expr.Name {
					return fmt.Sprintf("%s::%s", g.mangledEnumName(expr.Type.Name), expr.Name)
				}
			}
		}
		return expr.Name

	case *ir.SelfRef:
		if g.inConstructor {
			return "__self"
		}
		return "self"

	case *ir.ResultRef:
		return "__result"

	case *ir.IntLit:
		return fmt.Sprintf("%di64", expr.Value)

	case *ir.FloatLit:
		return expr.Value

	case *ir.StringLit:
		return expr.Value + ".to_string()"

	case *ir.StringInterp:
		return g.generateStringInterp(expr)

	case *ir.BoolLit:
		if expr.Value {
			return "true"
		}
		return "false"

	case *ir.CharLit:
		return fmt.Sprintf("'\\u{%X}'", expr.Value)

	case *ir.ArrayLit:
		if len(expr.Elements) == 0 {
			if expr.Type != nil && expr.Type.Name == "Map" {
				g.needsHashMap = true
				return "HashMap::new()"
			}
			return "Vec::new()"
		}
		elems := make([]string, len(expr.Elements))
		for i, el := range expr.Elements {
			elems[i] = g.generateExpr(el, arrayRefParams)
		}
		return fmt.Sprintf("vec![%s]", strings.Join(elems, ", "))

	case *ir.IndexExpr:
		// Phase 31 / ADR 0041: String indexing/slicing dispatches to .chars().
		if objType := expr.Object.ExprType(); objType != nil && objType.Name == "String" {
			obj := g.generateExpr(expr.Object, arrayRefParams)
			if rng, ok := expr.Index.(*ir.RangeExpr); ok {
				start := g.generateExpr(rng.Start, arrayRefParams)
				end := g.generateExpr(rng.End, arrayRefParams)
				return fmt.Sprintf("((%s).chars().skip(%s as usize).take((%s - %s) as usize).collect::<String>())", obj, start, end, start)
			}
			idx := g.generateExpr(expr.Index, arrayRefParams)
			return fmt.Sprintf("((%s).chars().nth(%s as usize).expect(\"string index out of bounds\"))", obj, idx)
		}
		return fmt.Sprintf("%s[%s as usize]",
			g.generateExpr(expr.Object, arrayRefParams),
			g.generateExpr(expr.Index, arrayRefParams))

	case *ir.RangeExpr:
		return fmt.Sprintf("(%s..%s)",
			g.generateExpr(expr.Start, arrayRefParams),
			g.generateExpr(expr.End, arrayRefParams))

	case *ir.ForallExpr:
		return g.generateForallExpr(expr, arrayRefParams)

	case *ir.ExistsExpr:
		return g.generateExistsExpr(expr, arrayRefParams)

	case *ir.MatchExpr:
		return g.generateMatchExpr(expr, arrayRefParams)

	case *ir.AwaitExpr:
		inner := g.generateExpr(expr.Expr, arrayRefParams)
		if expr.IsJoinHandle {
			// JoinHandle<T>.await yields Result<T, JoinError>; unwrap it.
			return "(" + inner + ").await.expect(\"spawned task panicked\")"
		}
		// Direct async-fn call yields impl Future<Output = T>; plain .await
		// gives T.
		return "(" + inner + ").await"

	case *ir.SpawnExpr:
		g.needsTokio = true
		// async fn signatures emit `-> T` (see fnReturnType), so a direct call
		// to an async fn returns `impl Future<Output = T>`. Pass that future
		// straight to tokio::spawn — wrapping in `async move { ... }` would
		// force a by-move capture of every argument variable in the enclosing
		// scope, breaking ownership when the same variable is used in
		// multiple spawn sites.
		return "tokio::spawn(" + g.generateExpr(expr.Expr, arrayRefParams) + ")"

	case *ir.TryExpr:
		return g.generateExpr(expr.Expr, arrayRefParams) + "?"

	case *ir.LambdaExpr:
		return g.generateLambdaExpr(expr, arrayRefParams)

	default:
		return "<unknown>"
	}
}

// isCopyType returns true for types that implement Copy in Rust (no clone needed).
func isCopyType(t *checker.Type) bool {
	if t == nil {
		return true // unknown type, assume copy to be safe
	}
	if t.IsFunction {
		return true // closures are passed by value, not cloned
	}
	switch t.Name {
	case "Int", "Float", "Bool", "Void":
		return true
	}
	return false
}

// isLiteralExpr returns true if the expression is a literal that doesn't need cloning.
func isLiteralExpr(expr ir.Expr) bool {
	switch expr.(type) {
	case *ir.IntLit, *ir.FloatLit, *ir.StringLit, *ir.BoolLit, *ir.ArrayLit, *ir.StringInterp:
		return true
	}
	return false
}

// cloneIfNeeded appends .clone() to an argument string if the expression is a non-Copy
// variable/field reference that would be moved by Rust's ownership system.
func (g *generator) cloneIfNeeded(argStr string, arg ir.Expr) string {
	if isLiteralExpr(arg) {
		return argStr
	}
	// Already a reference, no clone needed
	if strings.HasPrefix(argStr, "&") {
		return argStr
	}
	// Already cloned, don't double-clone
	if strings.HasSuffix(argStr, ".clone()") {
		return argStr
	}
	switch e := arg.(type) {
	case *ir.VarRef:
		if !isCopyType(e.Type) && !isFutureType(e.Type) {
			return argStr + ".clone()"
		}
	case *ir.FieldAccessExpr:
		if !isCopyType(e.Type) && !isFutureType(e.Type) {
			return argStr + ".clone()"
		}
	case *ir.IndexExpr:
		// vec[i] moves the element -- clone if non-Copy
		if !isCopyType(e.Type) && !isFutureType(e.Type) {
			return argStr + ".clone()"
		}
	}
	return argStr
}

// isFutureType reports whether the type is Future<T>, which maps to
// tokio::task::JoinHandle<T> in Rust. JoinHandle is non-Clone and represents
// unique ownership of a spawned task — values should be moved, never cloned.
func isFutureType(t *checker.Type) bool {
	return t != nil && t.Name == "Future" && t.IsGeneric
}

func (g *generator) generateCallExpr(expr *ir.CallExpr, arrayRefParams map[string]bool) string {
	switch expr.Kind {
	case ir.CallBuiltin:
		return g.generateBuiltinCall(expr, arrayRefParams)

	case ir.CallVariant:
		return g.generateVariantConstructor(expr, arrayRefParams)

	case ir.CallConstructor:
		args := make([]string, len(expr.Args))
		entity := g.entities[expr.Function]
		if entity == nil && g.allEntities != nil {
			entity = g.allEntities[expr.Function]
		}
		for i, arg := range expr.Args {
			argStr := g.generateExpr(arg, arrayRefParams)
			// Fix: empty ArrayLiteral for Map-typed field should emit HashMap::new()
			if arrLit, ok := arg.(*ir.ArrayLit); ok && len(arrLit.Elements) == 0 {
				if entity != nil && i < len(entity.Fields) && entity.Fields[i].Type != nil && entity.Fields[i].Type.Name == "Map" {
					g.needsHashMap = true
					argStr = "HashMap::new()"
				}
			}
			argStr = g.cloneIfNeeded(argStr, arg)
			args[i] = argStr
		}
		return fmt.Sprintf("%s::new(%s)", g.mangledEntityName(expr.Function), strings.Join(args, ", "))

	default: // CallFunction
		// Handle async built-in functions
		switch expr.Function {
		case "await_all":
			if len(expr.Args) == 1 {
				g.needsFutures = true
				arg := g.generateExpr(expr.Args[0], arrayRefParams)
				// join_all over a Vec<JoinHandle<T>> yields Vec<Result<T, JoinError>>.
				// Per Phase 14 ADR 0026, Future<T> = JoinHandle<T> uniformly and
				// awaiting a JoinHandle panics on JoinError, so unwrap each
				// element to give the caller the Array<T> shape Intent declares.
				return fmt.Sprintf("futures::future::join_all(%s).await.into_iter().map(|r| r.expect(\"spawned task panicked\")).collect::<Vec<_>>()", arg)
			}
		case "sleep":
			if len(expr.Args) == 1 {
				g.needsTokio = true
				arg := g.generateExpr(expr.Args[0], arrayRefParams)
				// sleep : Int -> Future<Void>. Returns the unawaited Sleep
				// future; the source-level `await` adds the `.await`. Since
				// `await sleep(ms)` is structurally not a JoinHandle await,
				// AwaitExpr's IsJoinHandle is false and the emit is bare
				// `.await` rather than `.await.expect(...)`.
				return fmt.Sprintf("tokio::time::sleep(std::time::Duration::from_millis(%s as u64))", arg)
			}
		case "timeout":
			if len(expr.Args) == 2 {
				g.needsTokio = true
				future := g.generateExpr(expr.Args[0], arrayRefParams)
				ms := g.generateExpr(expr.Args[1], arrayRefParams)
				return fmt.Sprintf("tokio::time::timeout(std::time::Duration::from_millis(%s as u64), %s).await.map_err(|_| \"timeout\".to_string())", ms, future)
			}
		case "await_any":
			if len(expr.Args) == 1 {
				g.needsFutures = true
				arg := g.generateExpr(expr.Args[0], arrayRefParams)
				// select_all(handles).await -> (Result<T, JoinError>, idx, rest);
				// take the result and unwrap the JoinError as per Phase 14 rule.
				return fmt.Sprintf("futures::future::select_all(%s).await.0.expect(\"spawned task panicked\")", arg)
			}
		}
		// Phase 15 / ADR 0028: extern (FFI) functions emit a call to the
		// Rust crate function named in `from "..."`. No namePrefix mangling
		// — the Rust path is authoritative.
		if ext := g.lookupExtern(expr.Function); ext != nil {
			args := make([]string, len(expr.Args))
			for i, arg := range expr.Args {
				argStr := g.generateExpr(arg, arrayRefParams)
				if i < len(ext.Params) && ext.Params[i].Type != nil {
					tn := ext.Params[i].Type.Name
					if tn == "Array" || tn == "Map" {
						if _, ok := arg.(*ir.VarRef); ok {
							argStr = "&" + argStr
						}
					}
				}
				argStr = g.cloneIfNeeded(argStr, arg)
				args[i] = argStr
			}
			return fmt.Sprintf("%s(%s)", ext.RustPath, strings.Join(args, ", "))
		}

		args := make([]string, len(expr.Args))
		funcDef := g.functions[expr.Function]
		if funcDef == nil && g.allFunctions != nil {
			funcDef = g.allFunctions[expr.Function]
		}
		for i, arg := range expr.Args {
			argStr := g.generateExpr(arg, arrayRefParams)
			// Pass arrays by reference
			if funcDef != nil && i < len(funcDef.Params) {
				if funcDef.Params[i].Type != nil && (funcDef.Params[i].Type.Name == "Array" || funcDef.Params[i].Type.Name == "Map") {
					if _, ok := arg.(*ir.VarRef); ok {
						argStr = "&" + argStr
					}
				}
			}
			// Clone non-Copy args to avoid ownership moves
			argStr = g.cloneIfNeeded(argStr, arg)
			args[i] = argStr
		}
		return fmt.Sprintf("%s(%s)", g.namePrefix+expr.Function, strings.Join(args, ", "))
	}
}

// lookupExtern resolves an FFI declaration by name, consulting the current
// module first and falling back to the cross-module map.
func (g *generator) lookupExtern(name string) *ir.ExternFunction {
	if ext, ok := g.externs[name]; ok {
		return ext
	}
	if g.allExterns != nil {
		if ext, ok := g.allExterns[name]; ok {
			return ext
		}
	}
	return nil
}

func (g *generator) generateBuiltinCall(expr *ir.CallExpr, arrayRefParams map[string]bool) string {
	switch expr.Function {
	case "print":
		if len(expr.Args) == 1 {
			arg := g.generateExpr(expr.Args[0], arrayRefParams)
			return fmt.Sprintf("println!(\"{}\", %s)", arg)
		}
	case "len":
		if len(expr.Args) == 1 {
			arg := g.generateExpr(expr.Args[0], arrayRefParams)
			// Phase 31 / ADR 0041: String len is codepoint count, not byte count.
			if argType := expr.Args[0].ExprType(); argType != nil && argType.Name == "String" {
				return fmt.Sprintf("((%s).chars().count() as i64)", arg)
			}
			return fmt.Sprintf("(%s.len() as i64)", arg)
		}
	case "char_from_codepoint":
		// Phase 31 / ADR 0041.
		if len(expr.Args) == 1 {
			n := g.generateExpr(expr.Args[0], arrayRefParams)
			return fmt.Sprintf("(match u32::try_from(%s).ok().and_then(std::char::from_u32) { Some(c) => Ok::<char, String>(c), None => Err::<char, String>(\"codepoint out of range or surrogate\".to_string()) })", n)
		}
	// Phase 16 / ADR 0029: assertion builtins.
	case "assert":
		if len(expr.Args) == 1 {
			cond := g.generateExpr(expr.Args[0], arrayRefParams)
			return fmt.Sprintf("assert!(%s, \"assertion failed\")", cond)
		}
	case "assert_eq":
		if len(expr.Args) == 2 {
			actual := g.cloneIfNeeded(g.generateExpr(expr.Args[0], arrayRefParams), expr.Args[0])
			expected := g.cloneIfNeeded(g.generateExpr(expr.Args[1], arrayRefParams), expr.Args[1])
			// Entity equality compiles to a call into the user-defined eq method
			// so that user semantics determine equality (ADR 0029).
			if t := expr.Args[0].ExprType(); t != nil && t.IsEntity {
				return fmt.Sprintf("assert!((%s).eq(&%s), \"assertion failed: entity equality\")", actual, expected)
			}
			return fmt.Sprintf("assert_eq!(%s, %s, \"assertion failed\")", actual, expected)
		}
	case "assert_close":
		if len(expr.Args) == 3 {
			actual := g.generateExpr(expr.Args[0], arrayRefParams)
			expected := g.generateExpr(expr.Args[1], arrayRefParams)
			epsilon := g.generateExpr(expr.Args[2], arrayRefParams)
			return fmt.Sprintf("assert!(((%s) - (%s)).abs() <= (%s), \"assertion failed: floats not within epsilon\")", actual, expected, epsilon)
		}
	case "assert_panics":
		if len(expr.Args) == 1 {
			arg := g.generateExpr(expr.Args[0], arrayRefParams)
			return fmt.Sprintf("std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| { (%s)(); })).expect_err(\"assertion failed: expected panic, none occurred\")", arg)
		}
	case "Ok", "Err", "Some":
		if len(expr.Args) == 1 {
			arg := g.generateExpr(expr.Args[0], arrayRefParams)
			arg = g.cloneIfNeeded(arg, expr.Args[0])
			return fmt.Sprintf("%s(%s)", expr.Function, arg)
		}
	case "None":
		return "None"
	case "read_file":
		if len(expr.Args) == 1 {
			arg := g.generateExpr(expr.Args[0], arrayRefParams)
			return fmt.Sprintf("std::fs::read_to_string(%s).map_err(|e| e.to_string())", arg)
		}
	case "write_file":
		if len(expr.Args) == 2 {
			path := g.generateExpr(expr.Args[0], arrayRefParams)
			content := g.generateExpr(expr.Args[1], arrayRefParams)
			return fmt.Sprintf("std::fs::write(%s, %s).map_err(|e| e.to_string())", path, content)
		}
	case "create_dir":
		if len(expr.Args) == 1 {
			arg := g.generateExpr(expr.Args[0], arrayRefParams)
			return fmt.Sprintf("std::fs::create_dir_all(%s).map_err(|e| e.to_string())", arg)
		}
	case "file_exists":
		if len(expr.Args) == 1 {
			arg := g.generateExpr(expr.Args[0], arrayRefParams)
			return fmt.Sprintf("std::path::Path::new(&%s).exists()", arg)
		}
	case "env_get":
		if len(expr.Args) == 1 {
			arg := g.generateExpr(expr.Args[0], arrayRefParams)
			return fmt.Sprintf("std::env::var(%s).ok()", arg)
		}
	case "http_post":
		if len(expr.Args) == 3 {
			g.needsReqwest = true
			g.needsSerdeJson = true
			url := g.generateExpr(expr.Args[0], arrayRefParams)
			headers := g.generateExpr(expr.Args[1], arrayRefParams)
			body := g.generateExpr(expr.Args[2], arrayRefParams)
			return fmt.Sprintf("__intent_http_post(&%s, &%s, &%s)", url, headers, body)
		}
	case "http_get":
		if len(expr.Args) == 2 {
			g.needsReqwest = true
			g.needsSerdeJson = true
			url := g.generateExpr(expr.Args[0], arrayRefParams)
			headers := g.generateExpr(expr.Args[1], arrayRefParams)
			return fmt.Sprintf("__intent_http_get(&%s, &%s)", url, headers)
		}
	case "json_get":
		if len(expr.Args) == 2 {
			g.needsSerdeJson = true
			json := g.generateExpr(expr.Args[0], arrayRefParams)
			key := g.generateExpr(expr.Args[1], arrayRefParams)
			return fmt.Sprintf("serde_json::from_str::<serde_json::Value>(&%s).ok().and_then(|v| v.get(&*%s).and_then(|v| v.as_str().map(|s| s.to_string())))", json, key)
		}
	case "json_path":
		if len(expr.Args) == 2 {
			g.needsSerdeJson = true
			jsonStr := g.generateExpr(expr.Args[0], arrayRefParams)
			path := g.generateExpr(expr.Args[1], arrayRefParams)
			return fmt.Sprintf(`{
    let __jp_json: &str = &%s;
    let __jp_path: &str = &%s;
    (|| -> Option<String> {
        let val: serde_json::Value = serde_json::from_str(__jp_json).ok()?;
        let mut current = &val;
        for component in __jp_path.split('.') {
            if let Ok(idx) = component.parse::<usize>() {
                current = current.get(idx)?;
            } else {
                current = current.get(component)?;
            }
        }
        Some(match current {
            serde_json::Value::String(s) => s.clone(),
            other => other.to_string(),
        })
    })()
}`, jsonStr, path)
		}
	case "emit_event":
		if len(expr.Args) == 2 {
			eventType := g.generateExpr(expr.Args[0], arrayRefParams)
			payload := g.generateExpr(expr.Args[1], arrayRefParams)
			return fmt.Sprintf("eprintln!(\"[EVENT] {}: {}\", %s, %s)", eventType, payload)
		}
	case "timestamp_ms":
		return "{ use std::time::{SystemTime, UNIX_EPOCH}; SystemTime::now().duration_since(UNIX_EPOCH).unwrap().as_millis() as i64 }"
	}
	// Fallback
	args := make([]string, len(expr.Args))
	for i, a := range expr.Args {
		args[i] = g.generateExpr(a, arrayRefParams)
	}
	return fmt.Sprintf("%s(%s)", expr.Function, strings.Join(args, ", "))
}

func (g *generator) generateVariantConstructor(expr *ir.CallExpr, arrayRefParams map[string]bool) string {
	enumName := g.mangledEnumName(expr.EnumName)

	// Find variant declaration from IR enums
	var variant *ir.EnumVariant
	if e, ok := g.enums[enumName]; ok {
		for _, v := range e.Variants {
			if v.Name == expr.Function {
				variant = v
				break
			}
		}
	}

	// Unit variant
	if variant == nil || len(variant.Fields) == 0 {
		return fmt.Sprintf("%s::%s", enumName, expr.Function)
	}

	// Data variant
	var sb strings.Builder
	sb.WriteString(enumName)
	sb.WriteString("::")
	sb.WriteString(expr.Function)
	sb.WriteString(" { ")
	for i, f := range variant.Fields {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(f.Name)
		sb.WriteString(": ")
		sb.WriteString(g.generateExpr(expr.Args[i], arrayRefParams))
	}
	sb.WriteString(" }")
	return sb.String()
}

func (g *generator) generateMethodCallExpr(expr *ir.MethodCallExpr, arrayRefParams map[string]bool) string {
	// Module-qualified calls
	if expr.IsModuleCall && g.moduleManglings != nil {
		args := make([]string, len(expr.Args))
		funcDecl := g.functions[expr.Method]
		if funcDecl == nil && g.allFunctions != nil {
			funcDecl = g.allFunctions[expr.Method]
		}
		// Look up entity for constructor calls to resolve Map-typed fields
		var moduleEntity *ir.Entity
		if expr.CallKind == ir.CallConstructor {
			moduleEntity = g.entities[expr.Method]
			if moduleEntity == nil && g.allEntities != nil {
				moduleEntity = g.allEntities[expr.Method]
			}
		}
		for i, arg := range expr.Args {
			argStr := g.generateExpr(arg, arrayRefParams)
			if funcDecl != nil && i < len(funcDecl.Params) {
				if funcDecl.Params[i].Type != nil && (funcDecl.Params[i].Type.Name == "Array" || funcDecl.Params[i].Type.Name == "Map") {
					if _, ok := arg.(*ir.VarRef); ok {
						argStr = "&" + argStr
					}
				}
			}
			// Fix: empty ArrayLiteral for Map-typed constructor field should emit HashMap::new()
			if arrLit, ok := arg.(*ir.ArrayLit); ok && len(arrLit.Elements) == 0 {
				if moduleEntity != nil && i < len(moduleEntity.Fields) && moduleEntity.Fields[i].Type != nil && moduleEntity.Fields[i].Type.Name == "Map" {
					g.needsHashMap = true
					argStr = "HashMap::new()"
				}
			}
			argStr = g.cloneIfNeeded(argStr, arg)
			args[i] = argStr
		}

		if expr.CallKind == ir.CallConstructor {
			return fmt.Sprintf("%s::new(%s)", g.mangledEntityName(expr.Method), strings.Join(args, ", "))
		}

		// Look up the function name prefix from moduleManglings
		fnPrefix := expr.ModuleName + "_"
		if p, ok := g.moduleManglings[expr.ModuleName]; ok {
			fnPrefix = p
		}
		mangledFnName := fnPrefix + expr.Method
		return fmt.Sprintf("%s(%s)", mangledFnName, strings.Join(args, ", "))
	}

	obj := g.generateExpr(expr.Object, arrayRefParams)

	// Result/Option predicate methods
	if expr.Method == "is_ok" || expr.Method == "is_err" || expr.Method == "is_some" || expr.Method == "is_none" {
		return fmt.Sprintf("%s.%s()", obj, expr.Method)
	}

	// to_string() method on Int, Float, Bool
	if expr.Method == "to_string" && len(expr.Args) == 0 {
		return fmt.Sprintf("%s.to_string()", obj)
	}

	// Phase 31 / ADR 0041: Char methods.
	if objType := expr.Object.ExprType(); objType != nil && objType.Name == "Char" {
		switch expr.Method {
		case "to_codepoint":
			return fmt.Sprintf("((%s) as u32 as i64)", obj)
		case "is_digit":
			return fmt.Sprintf("(%s).is_ascii_digit()", obj)
		case "is_alpha":
			return fmt.Sprintf("(%s).is_ascii_alphabetic()", obj)
		case "is_alphanumeric":
			return fmt.Sprintf("(%s).is_ascii_alphanumeric()", obj)
		case "is_whitespace":
			return fmt.Sprintf("{ let __c = %s; __c == ' ' || __c == '\\t' || __c == '\\n' || __c == '\\r' }", obj)
		case "is_lowercase":
			return fmt.Sprintf("(%s).is_ascii_lowercase()", obj)
		case "is_uppercase":
			return fmt.Sprintf("(%s).is_ascii_uppercase()", obj)
		}
	}

	// String methods
	if objType := expr.Object.ExprType(); objType != nil && objType.Name == "String" {
		switch expr.Method {
		case "len":
			// Phase 31 / ADR 0041: char count, not byte count.
			return fmt.Sprintf("(%s.chars().count() as i64)", obj)
		case "to_lowercase":
			return fmt.Sprintf("%s.to_lowercase()", obj)
		case "trim":
			return fmt.Sprintf("%s.trim().to_string()", obj)
		case "starts_with":
			arg := g.generateExpr(expr.Args[0], arrayRefParams)
			return fmt.Sprintf("%s.starts_with(%s.as_str())", obj, arg)
		case "contains":
			arg := g.generateExpr(expr.Args[0], arrayRefParams)
			return fmt.Sprintf("%s.contains(%s.as_str())", obj, arg)
		case "split":
			arg := g.generateExpr(expr.Args[0], arrayRefParams)
			return fmt.Sprintf("%s.split(%s.as_str()).map(|s| s.to_string()).collect::<Vec<String>>()", obj, arg)
		}
	}

	// Map methods
	if objType := expr.Object.ExprType(); objType != nil && objType.Name == "Map" {
		switch expr.Method {
		case "get":
			key := g.generateExpr(expr.Args[0], arrayRefParams)
			def := g.generateExpr(expr.Args[1], arrayRefParams)
			return fmt.Sprintf("%s.get(&%s).cloned().unwrap_or(%s)", obj, key, def)
		case "set":
			key := g.generateExpr(expr.Args[0], arrayRefParams)
			val := g.generateExpr(expr.Args[1], arrayRefParams)
			return fmt.Sprintf("%s.insert(%s, %s)", obj, key, val)
		case "contains":
			key := g.generateExpr(expr.Args[0], arrayRefParams)
			return fmt.Sprintf("%s.contains_key(&%s)", obj, key)
		case "keys":
			return fmt.Sprintf("%s.keys().cloned().collect::<Vec<_>>()", obj)
		case "remove":
			key := g.generateExpr(expr.Args[0], arrayRefParams)
			return fmt.Sprintf("%s.remove(&%s)", obj, key)
		}
	}

	args := make([]string, len(expr.Args))
	for i, arg := range expr.Args {
		argStr := g.generateExpr(arg, arrayRefParams)
		argStr = g.cloneIfNeeded(argStr, arg)
		args[i] = argStr
	}
	return fmt.Sprintf("%s.%s(%s)", obj, expr.Method, strings.Join(args, ", "))
}

func (g *generator) generateForallExpr(expr *ir.ForallExpr, arrayRefParams map[string]bool) string {
	rangeStart := g.generateExpr(expr.Domain.Start, arrayRefParams)
	rangeEnd := g.generateExpr(expr.Domain.End, arrayRefParams)
	body := g.generateExpr(expr.Body, arrayRefParams)

	return fmt.Sprintf("{\n"+
		"    let mut __forall_holds = true;\n"+
		"    for %s in (%s..%s) {\n"+
		"        if !(%s) {\n"+
		"            __forall_holds = false;\n"+
		"            break;\n"+
		"        }\n"+
		"    }\n"+
		"    __forall_holds\n"+
		"}", expr.Variable, rangeStart, rangeEnd, body)
}

func (g *generator) generateExistsExpr(expr *ir.ExistsExpr, arrayRefParams map[string]bool) string {
	rangeStart := g.generateExpr(expr.Domain.Start, arrayRefParams)
	rangeEnd := g.generateExpr(expr.Domain.End, arrayRefParams)
	body := g.generateExpr(expr.Body, arrayRefParams)

	return fmt.Sprintf("{\n"+
		"    let mut __exists_found = false;\n"+
		"    for %s in (%s..%s) {\n"+
		"        if %s {\n"+
		"            __exists_found = true;\n"+
		"            break;\n"+
		"        }\n"+
		"    }\n"+
		"    __exists_found\n"+
		"}", expr.Variable, rangeStart, rangeEnd, body)
}

func (g *generator) generateMatchExpr(expr *ir.MatchExpr, arrayRefParams map[string]bool) string {
	var buf strings.Builder

	// For Result/Option scrutinees, clone the scrutinee to avoid moving.
	// This is simpler than borrowing since match arm bindings remain owned values.
	scrutineeType := expr.Scrutinee.ExprType()
	isResultOrOption := scrutineeType != nil && (scrutineeType.Name == "Result" || scrutineeType.Name == "Option")

	buf.WriteString("match ")
	scrutineeStr := g.generateExpr(expr.Scrutinee, arrayRefParams)
	if isResultOrOption {
		if _, isVar := expr.Scrutinee.(*ir.VarRef); isVar {
			scrutineeStr += ".clone()"
		}
	}
	buf.WriteString(scrutineeStr)
	buf.WriteString(" {\n")

	g.incIndent()
	for _, arm := range expr.Arms {
		buf.WriteString(g.indentStr())
		buf.WriteString(g.generateMatchPattern(arm.Pattern))
		buf.WriteString(" => ")
		buf.WriteString(g.generateExpr(arm.Body, arrayRefParams))
		buf.WriteString(",\n")
	}
	g.decIndent()

	buf.WriteString(g.indentStr())
	buf.WriteString("}")
	return buf.String()
}

func (g *generator) generateMatchPattern(pattern *ir.MatchPattern) string {
	if pattern.IsWildcard {
		return "_"
	}

	// Builtin patterns (Ok, Err, Some, None) use tuple syntax
	if pattern.IsBuiltin {
		if pattern.VariantName == "None" {
			return "None"
		}
		if len(pattern.Bindings) == 1 {
			return fmt.Sprintf("%s(%s)", pattern.VariantName, pattern.Bindings[0])
		}
		return pattern.VariantName
	}

	enumName := g.mangledEnumName(pattern.EnumName)

	// Unit variant
	if len(pattern.Bindings) == 0 {
		return fmt.Sprintf("%s::%s", enumName, pattern.VariantName)
	}

	// Data variant with field names
	var fields []string
	for i, binding := range pattern.Bindings {
		if i < len(pattern.FieldNames) {
			fields = append(fields, fmt.Sprintf("%s: %s", pattern.FieldNames[i], binding))
		}
	}

	return fmt.Sprintf("%s::%s { %s }", enumName, pattern.VariantName, strings.Join(fields, ", "))
}

// --- Helpers ---

func (g *generator) mapOperator(op lexer.TokenType) string {
	switch op {
	case lexer.PLUS:
		return "+"
	case lexer.MINUS:
		return "-"
	case lexer.STAR:
		return "*"
	case lexer.SLASH:
		return "/"
	case lexer.PERCENT:
		return "%"
	case lexer.EQ:
		return "=="
	case lexer.NEQ:
		return "!="
	case lexer.LT:
		return "<"
	case lexer.GT:
		return ">"
	case lexer.LEQ:
		return "<="
	case lexer.GEQ:
		return ">="
	case lexer.AND:
		return "&&"
	case lexer.OR:
		return "||"
	default:
		return "?"
	}
}

func (g *generator) generateLambdaExpr(expr *ir.LambdaExpr, arrayRefParams map[string]bool) string {
	var sb strings.Builder
	sb.WriteString("|")
	for i, p := range expr.Params {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("%s: %s", p.Name, g.mapType(p.Type)))
	}
	sb.WriteString("| -> ")
	sb.WriteString(g.mapType(expr.Type.FnReturn))
	sb.WriteString(" { ")
	sb.WriteString(g.generateExpr(expr.Body, arrayRefParams))
	sb.WriteString(" }")
	return sb.String()
}

func (g *generator) isEntityType(t *checker.Type) bool {
	if t == nil {
		return false
	}
	if _, ok := g.entities[t.Name]; ok {
		return true
	}
	if g.allEntities != nil {
		if _, ok := g.allEntities[t.Name]; ok {
			return true
		}
	}
	return t.IsEntity
}

// collectMutatedVars scans a list of IR statements and returns the set of
// variable names that are targets of assignments, field mutations, or
// direct method calls (which may require &mut self).
func collectMutatedVars(stmts []ir.Stmt) map[string]bool {
	mutated := make(map[string]bool)

	// collectMethodReceivers finds top-level method call receivers in an expression
	var collectMethodReceivers func(ir.Expr)
	collectMethodReceivers = func(e ir.Expr) {
		if e == nil {
			return
		}
		if mc, ok := e.(*ir.MethodCallExpr); ok && !mc.IsModuleCall {
			if v, ok := mc.Object.(*ir.VarRef); ok {
				mutated[v.Name] = true
			}
		}
	}

	var scan func([]ir.Stmt)
	scan = func(stmts []ir.Stmt) {
		for _, s := range stmts {
			switch stmt := s.(type) {
			case *ir.AssignStmt:
				switch target := stmt.Target.(type) {
				case *ir.VarRef:
					mutated[target.Name] = true
				case *ir.FieldAccessExpr:
					root := target.Object
					for {
						if fa, ok := root.(*ir.FieldAccessExpr); ok {
							root = fa.Object
						} else {
							break
						}
					}
					if v, ok := root.(*ir.VarRef); ok {
						mutated[v.Name] = true
					}
				case *ir.IndexExpr:
					if v, ok := target.Object.(*ir.VarRef); ok {
						mutated[v.Name] = true
					}
				}
			case *ir.ExprStmt:
				collectMethodReceivers(stmt.Expr)
			case *ir.LetStmt:
				collectMethodReceivers(stmt.Value)
			case *ir.ReturnStmt:
				collectMethodReceivers(stmt.Value)
			case *ir.IfStmt:
				scan(stmt.Then)
				scan(stmt.Else)
			case *ir.WhileStmt:
				scan(stmt.Body)
			case *ir.ForInStmt:
				scan(stmt.Body)
			}
		}
	}
	scan(stmts)
	return mutated
}

func (g *generator) mangleIntentName(desc string) string {
	desc = strings.ToLower(desc)
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	desc = reg.ReplaceAllString(desc, "_")
	desc = strings.Trim(desc, "_")
	if len(desc) > 0 && desc[0] >= '0' && desc[0] <= '9' {
		desc = "_" + desc
	}
	if desc == "" {
		desc = "__intent"
	}
	return "__intent_" + desc
}

func escapeRustString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}

func (g *generator) emitHttpHelpers() string {
	var sb strings.Builder
	sb.WriteString("\nfn __intent_http_post(url: &str, headers_json: &str, body: &str) -> Result<String, String> {\n")
	sb.WriteString("    let client = reqwest::blocking::Client::new();\n")
	sb.WriteString("    let mut req = client.post(url);\n")
	sb.WriteString("    if let Ok(hdrs) = serde_json::from_str::<serde_json::Value>(headers_json) {\n")
	sb.WriteString("        if let Some(obj) = hdrs.as_object() {\n")
	sb.WriteString("            for (k, v) in obj {\n")
	sb.WriteString("                if let Some(s) = v.as_str() {\n")
	sb.WriteString("                    req = req.header(k.as_str(), s);\n")
	sb.WriteString("                }\n")
	sb.WriteString("            }\n")
	sb.WriteString("        }\n")
	sb.WriteString("    }\n")
	sb.WriteString("    req.body(body.to_string()).send().and_then(|r| r.text()).map_err(|e| e.to_string())\n")
	sb.WriteString("}\n\n")
	sb.WriteString("fn __intent_http_get(url: &str, headers_json: &str) -> Result<String, String> {\n")
	sb.WriteString("    let client = reqwest::blocking::Client::new();\n")
	sb.WriteString("    let mut req = client.get(url);\n")
	sb.WriteString("    if let Ok(hdrs) = serde_json::from_str::<serde_json::Value>(headers_json) {\n")
	sb.WriteString("        if let Some(obj) = hdrs.as_object() {\n")
	sb.WriteString("            for (k, v) in obj {\n")
	sb.WriteString("                if let Some(s) = v.as_str() {\n")
	sb.WriteString("                    req = req.header(k.as_str(), s);\n")
	sb.WriteString("                }\n")
	sb.WriteString("            }\n")
	sb.WriteString("        }\n")
	sb.WriteString("    }\n")
	sb.WriteString("    req.send().and_then(|r| r.text()).map_err(|e| e.to_string())\n")
	sb.WriteString("}\n")
	return sb.String()
}

// generateStringInterp generates Rust format!() for string interpolation.
// "hello {expr} world" -> format!("hello {} world", expr)
func (g *generator) generateStringInterp(interp *ir.StringInterp) string {
	var fmtStr strings.Builder
	var args []string

	for _, part := range interp.Parts {
		if part.IsExpr {
			fmtStr.WriteString("{}")
			args = append(args, g.generateExpr(part.Expr, nil))
		} else {
			// Escape braces in static parts for Rust format!()
			escaped := strings.ReplaceAll(part.Static, "{", "{{")
			escaped = strings.ReplaceAll(escaped, "}", "}}")
			// Escape quotes and backslashes for Rust string literal
			escaped = strings.ReplaceAll(escaped, "\\", "\\\\")
			escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
			fmtStr.WriteString(escaped)
		}
	}

	if len(args) == 0 {
		return fmt.Sprintf("\"%s\".to_string()", fmtStr.String())
	}

	return fmt.Sprintf("format!(\"%s\", %s)", fmtStr.String(), strings.Join(args, ", "))
}
