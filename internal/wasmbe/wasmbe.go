// Package wasmbe generates WASM binary format directly from IR,
// without going through an intermediate Rust representation.
package wasmbe

import (
	"strconv"

	"github.com/lhaig/intent/internal/checker"
	"github.com/lhaig/intent/internal/ir"
	"github.com/lhaig/intent/internal/lexer"
)

// Generate produces a WASM binary module from a single IR module.
func Generate(mod *ir.Module) []byte {
	g := newGenerator()
	g.addModule(mod)
	return g.emit()
}

// GenerateAll produces a WASM binary module from a multi-module program.
func GenerateAll(prog *ir.Program) []byte {
	g := newGenerator()
	for _, mod := range prog.Modules {
		g.addModule(mod)
	}
	return g.emit()
}

// funcSig represents a WASM function type signature.
type funcSig struct {
	params  []byte // value types
	results []byte // value types
}

// localVar tracks a local variable in a WASM function.
type localVar struct {
	name  string
	vtype byte
}

// generator builds a WASM binary module.
type generator struct {
	types     []funcSig       // type section entries
	typeCache map[string]int  // sig string -> type index
	funcs     []int           // function section: type index per function
	exports   []wasmExport    // export section entries
	codes     [][]byte        // code section: encoded function bodies
	funcIndex map[string]int  // function name -> function index
	dataSegs  []dataSeg       // data section entries
	dataOff   int             // next free offset in linear memory
	entryFunc string          // entry function name
	isEntry   bool            // whether this module has an entry point
	mangledFn map[string]bool // track mangled function names for multi-module
}

type wasmExport struct {
	name  string
	kind  byte
	index int
}

type dataSeg struct {
	offset int
	data   []byte
}

func newGenerator() *generator {
	return &generator{
		typeCache: make(map[string]int),
		funcIndex: make(map[string]int),
		mangledFn: make(map[string]bool),
		dataOff:   1024, // start data after a 1KB stack area
	}
}

// typeIndex returns the type section index for a given signature, adding it if new.
func (g *generator) typeIndex(params, results []byte) int {
	key := sigKey(params, results)
	if idx, ok := g.typeCache[key]; ok {
		return idx
	}
	idx := len(g.types)
	g.types = append(g.types, funcSig{params: params, results: results})
	g.typeCache[key] = idx
	return idx
}

func sigKey(params, results []byte) string {
	return string(params) + "|" + string(results)
}

// typeForIR maps an IR type to a WASM value type.
func typeForIR(t *checker.Type) byte {
	if t == nil {
		return valI64
	}
	switch t.Name {
	case "Int":
		return valI64
	case "Float":
		return valF64
	case "Bool":
		return valI32
	case "String":
		// Strings are represented as i32 pointers into linear memory
		return valI32
	default:
		// Entities, enums, arrays -> i32 pointer
		return valI32
	}
}

// addModule adds all functions from a module to the generator.
func (g *generator) addModule(mod *ir.Module) {
	if mod.IsEntry {
		g.isEntry = true
	}

	prefix := ""
	if !mod.IsEntry {
		prefix = mod.Name + "_"
	}

	for _, fn := range mod.Functions {
		name := prefix + fn.Name
		if fn.IsEntry {
			name = fn.Name
			g.entryFunc = name
		}
		g.addFunction(name, fn)
	}
}

// addFunction compiles a single IR function to WASM.
func (g *generator) addFunction(name string, fn *ir.Function) {
	// Build parameter types
	var paramTypes []byte
	for _, p := range fn.Params {
		paramTypes = append(paramTypes, typeForIR(p.Type))
	}

	// Build result types
	var resultTypes []byte
	if fn.ReturnType != nil && fn.ReturnType.Name != "Void" {
		resultTypes = append(resultTypes, typeForIR(fn.ReturnType))
	}

	// Get or create type index
	tidx := g.typeIndex(paramTypes, resultTypes)

	// Record function index
	fidx := len(g.funcs)
	g.funcIndex[name] = fidx
	g.funcs = append(g.funcs, tidx)

	// Export the function
	g.exports = append(g.exports, wasmExport{name: name, kind: exportFunc, index: fidx})

	// Compile function body
	fc := &funcCompiler{
		gen:        g,
		fn:         fn,
		localCount: len(fn.Params),
		localMap:   make(map[string]int),
		blockDepth: 0,
	}

	// Map parameters to local indices
	for i, p := range fn.Params {
		fc.localMap[p.Name] = i
	}

	body := fc.compileBody()
	g.codes = append(g.codes, body)
}

// emit produces the complete WASM binary.
func (g *generator) emit() []byte {
	var wasm []byte
	wasm = append(wasm, wasmMagic...)
	wasm = append(wasm, wasmVersion...)

	// Type section
	wasm = append(wasm, g.emitTypeSection()...)

	// Function section
	wasm = append(wasm, g.emitFunctionSection()...)

	// Memory section (1 page = 64KB)
	wasm = append(wasm, g.emitMemorySection()...)

	// Export section
	wasm = append(wasm, g.emitExportSection()...)

	// Code section
	wasm = append(wasm, g.emitCodeSection()...)

	// Data section (string literals etc.)
	if len(g.dataSegs) > 0 {
		wasm = append(wasm, g.emitDataSection()...)
	}

	return wasm
}

func (g *generator) emitTypeSection() []byte {
	var contents []byte
	for _, sig := range g.types {
		// Function type tag
		contents = append(contents, 0x60)
		// Parameter types
		contents = append(contents, encodeLEB128U(uint64(len(sig.params)))...)
		contents = append(contents, sig.params...)
		// Result types
		contents = append(contents, encodeLEB128U(uint64(len(sig.results)))...)
		contents = append(contents, sig.results...)
	}
	body := encodeVector(len(g.types), contents)
	return encodeSection(sectionType, body)
}

func (g *generator) emitFunctionSection() []byte {
	var contents []byte
	for _, tidx := range g.funcs {
		contents = append(contents, encodeLEB128U(uint64(tidx))...)
	}
	body := encodeVector(len(g.funcs), contents)
	return encodeSection(sectionFunction, body)
}

func (g *generator) emitMemorySection() []byte {
	// One memory with initial size of 1 page (64KB), no max
	var contents []byte
	// limits: flags=0 (no max), min=1
	contents = append(contents, 0x00) // no max flag
	contents = append(contents, encodeLEB128U(1)...)
	body := encodeVector(1, contents)
	return encodeSection(sectionMemory, body)
}

func (g *generator) emitExportSection() []byte {
	var contents []byte
	for _, exp := range g.exports {
		contents = append(contents, encodeString(exp.name)...)
		contents = append(contents, exp.kind)
		contents = append(contents, encodeLEB128U(uint64(exp.index))...)
	}

	// Also export memory
	contents = append(contents, encodeString("memory")...)
	contents = append(contents, exportMemory)
	contents = append(contents, encodeLEB128U(0)...) // memory index 0

	body := encodeVector(len(g.exports)+1, contents)
	return encodeSection(sectionExport, body)
}

func (g *generator) emitCodeSection() []byte {
	var contents []byte
	for _, code := range g.codes {
		// Each function body is prefixed with its byte length
		contents = append(contents, encodeLEB128U(uint64(len(code)))...)
		contents = append(contents, code...)
	}
	body := encodeVector(len(g.codes), contents)
	return encodeSection(sectionCode, body)
}

func (g *generator) emitDataSection() []byte {
	var contents []byte
	for _, seg := range g.dataSegs {
		contents = append(contents, 0x00) // active segment, memory 0
		// offset expression: i32.const <offset>
		contents = append(contents, opI32Const)
		contents = append(contents, encodeLEB128S(int64(seg.offset))...)
		contents = append(contents, opEnd)
		// data bytes
		contents = append(contents, encodeLEB128U(uint64(len(seg.data)))...)
		contents = append(contents, seg.data...)
	}
	body := encodeVector(len(g.dataSegs), contents)
	return encodeSection(sectionData, body)
}

// addStringData stores a string in linear memory and returns its (offset, length).
func (g *generator) addStringData(s string) (int, int) {
	offset := g.dataOff
	data := []byte(s)
	g.dataSegs = append(g.dataSegs, dataSeg{offset: offset, data: data})
	g.dataOff += len(data)
	return offset, len(data)
}

// --- Function compiler ---

type funcCompiler struct {
	gen        *generator
	fn         *ir.Function
	localCount int
	localMap   map[string]int
	extraTypes []byte // additional local types beyond parameters
	body       []byte
	blockDepth int
	// Track break/continue labels (block depth at loop entry)
	loopBreakDepth    int
	loopContinueDepth int
}

// allocLocal allocates a new local variable and returns its index.
func (fc *funcCompiler) allocLocal(name string, vtype byte) int {
	idx := fc.localCount
	fc.localCount++
	fc.localMap[name] = idx
	fc.extraTypes = append(fc.extraTypes, vtype)
	return idx
}

// allocAnon allocates an anonymous local variable.
func (fc *funcCompiler) allocAnon(vtype byte) int {
	idx := fc.localCount
	fc.localCount++
	fc.extraTypes = append(fc.extraTypes, vtype)
	return idx
}

// compileBody compiles the function body and returns the encoded function body bytes.
func (fc *funcCompiler) compileBody() []byte {
	// Compile statements
	for _, stmt := range fc.fn.Body {
		fc.compileStmt(stmt)
	}

	// Ensure body ends with end opcode
	fc.body = append(fc.body, opEnd)

	// Encode locals declaration
	var locals []byte
	if len(fc.extraTypes) > 0 {
		// Group consecutive same-type locals for compact encoding
		groups := compactLocals(fc.extraTypes)
		locals = encodeLEB128U(uint64(len(groups)))
		for _, g := range groups {
			locals = append(locals, encodeLEB128U(uint64(g.count))...)
			locals = append(locals, g.vtype)
		}
	} else {
		locals = encodeLEB128U(0)
	}

	var result []byte
	result = append(result, locals...)
	result = append(result, fc.body...)
	return result
}

type localGroup struct {
	count int
	vtype byte
}

func compactLocals(types []byte) []localGroup {
	if len(types) == 0 {
		return nil
	}
	var groups []localGroup
	current := localGroup{count: 1, vtype: types[0]}
	for i := 1; i < len(types); i++ {
		if types[i] == current.vtype {
			current.count++
		} else {
			groups = append(groups, current)
			current = localGroup{count: 1, vtype: types[i]}
		}
	}
	groups = append(groups, current)
	return groups
}

// --- Statement compilation ---

func (fc *funcCompiler) compileStmt(stmt ir.Stmt) {
	switch s := stmt.(type) {
	case *ir.LetStmt:
		fc.compileLetStmt(s)
	case *ir.AssignStmt:
		fc.compileAssignStmt(s)
	case *ir.ReturnStmt:
		fc.compileReturnStmt(s)
	case *ir.IfStmt:
		fc.compileIfStmt(s)
	case *ir.WhileStmt:
		fc.compileWhileStmt(s)
	case *ir.ForInStmt:
		fc.compileForInStmt(s)
	case *ir.ExprStmt:
		fc.compileExpr(s.Expr)
		// Drop the result if the expression produces one
		if s.Expr.ExprType() != nil && s.Expr.ExprType().Name != "Void" {
			fc.body = append(fc.body, opDrop)
		}
	case *ir.BreakStmt:
		// Branch to break label (outer block)
		depth := fc.blockDepth - fc.loopBreakDepth
		fc.body = append(fc.body, opBr)
		fc.body = append(fc.body, encodeLEB128U(uint64(depth))...)
	case *ir.ContinueStmt:
		// Branch to continue label (loop start)
		depth := fc.blockDepth - fc.loopContinueDepth
		fc.body = append(fc.body, opBr)
		fc.body = append(fc.body, encodeLEB128U(uint64(depth))...)
	}
}

func (fc *funcCompiler) compileLetStmt(s *ir.LetStmt) {
	vtype := typeForIR(s.Type)
	idx := fc.allocLocal(s.Name, vtype)
	fc.compileExpr(s.Value)
	fc.body = append(fc.body, opLocalSet)
	fc.body = append(fc.body, encodeLEB128U(uint64(idx))...)
}

func (fc *funcCompiler) compileAssignStmt(s *ir.AssignStmt) {
	switch target := s.Target.(type) {
	case *ir.VarRef:
		fc.compileExpr(s.Value)
		if idx, ok := fc.localMap[target.Name]; ok {
			fc.body = append(fc.body, opLocalSet)
			fc.body = append(fc.body, encodeLEB128U(uint64(idx))...)
		}
	case *ir.FieldAccessExpr:
		// For entity field assignment via memory store
		// Simplified: just compile target and value for now
		fc.compileExpr(s.Value)
		fc.body = append(fc.body, opDrop)
	case *ir.IndexExpr:
		// Array index assignment
		fc.compileExpr(s.Value)
		fc.body = append(fc.body, opDrop)
	}
}

func (fc *funcCompiler) compileReturnStmt(s *ir.ReturnStmt) {
	if s.Value != nil {
		fc.compileExpr(s.Value)
	}
	fc.body = append(fc.body, opReturn)
}

func (fc *funcCompiler) compileIfStmt(s *ir.IfStmt) {
	fc.compileExpr(s.Condition)
	// Ensure condition is i32 (WASM if expects i32)
	fc.ensureI32(s.Condition)

	fc.body = append(fc.body, opIf)
	fc.body = append(fc.body, blockVoid)
	fc.blockDepth++

	for _, stmt := range s.Then {
		fc.compileStmt(stmt)
	}

	if len(s.Else) > 0 {
		fc.body = append(fc.body, opElse)
		for _, stmt := range s.Else {
			fc.compileStmt(stmt)
		}
	}

	fc.body = append(fc.body, opEnd)
	fc.blockDepth--
}

func (fc *funcCompiler) compileWhileStmt(s *ir.WhileStmt) {
	// WASM pattern for while:
	//   block $break
	//     loop $continue
	//       br_if (not condition) $break
	//       ...body...
	//       br $continue
	//     end
	//   end

	savedBreak := fc.loopBreakDepth
	savedContinue := fc.loopContinueDepth

	fc.body = append(fc.body, opBlock)
	fc.body = append(fc.body, blockVoid)
	fc.blockDepth++
	fc.loopBreakDepth = fc.blockDepth

	fc.body = append(fc.body, opLoop)
	fc.body = append(fc.body, blockVoid)
	fc.blockDepth++
	fc.loopContinueDepth = fc.blockDepth

	// Check condition, branch to break block if false
	fc.compileExpr(s.Condition)
	fc.ensureI32(s.Condition)
	fc.body = append(fc.body, opI32Eqz) // negate condition
	fc.body = append(fc.body, opBrIf)
	// break label is 1 level up (the block)
	fc.body = append(fc.body, encodeLEB128U(1)...)

	// Body
	for _, stmt := range s.Body {
		fc.compileStmt(stmt)
	}

	// Branch back to loop start
	fc.body = append(fc.body, opBr)
	fc.body = append(fc.body, encodeLEB128U(0)...) // continue = loop label

	fc.body = append(fc.body, opEnd) // end loop
	fc.blockDepth--
	fc.body = append(fc.body, opEnd) // end block
	fc.blockDepth--

	fc.loopBreakDepth = savedBreak
	fc.loopContinueDepth = savedContinue
}

func (fc *funcCompiler) compileForInStmt(s *ir.ForInStmt) {
	// For-in over a range: for x in start..end { body }
	if rangeExpr, ok := s.Iterable.(*ir.RangeExpr); ok {
		fc.compileForRange(s.Variable, rangeExpr, s.Body)
		return
	}
	// For-in over array: not yet implemented in WASM backend
}

func (fc *funcCompiler) compileForRange(varName string, r *ir.RangeExpr, body []ir.Stmt) {
	// Allocate loop variable
	iterIdx := fc.allocLocal(varName, valI64)

	// Initialize: iter = start
	fc.compileExpr(r.Start)
	fc.body = append(fc.body, opLocalSet)
	fc.body = append(fc.body, encodeLEB128U(uint64(iterIdx))...)

	// Compile end value and store in temp
	endIdx := fc.allocAnon(valI64)
	fc.compileExpr(r.End)
	fc.body = append(fc.body, opLocalSet)
	fc.body = append(fc.body, encodeLEB128U(uint64(endIdx))...)

	savedBreak := fc.loopBreakDepth
	savedContinue := fc.loopContinueDepth

	// block $break
	fc.body = append(fc.body, opBlock)
	fc.body = append(fc.body, blockVoid)
	fc.blockDepth++
	fc.loopBreakDepth = fc.blockDepth

	// loop $continue
	fc.body = append(fc.body, opLoop)
	fc.body = append(fc.body, blockVoid)
	fc.blockDepth++
	fc.loopContinueDepth = fc.blockDepth

	// Check: iter < end
	fc.body = append(fc.body, opLocalGet)
	fc.body = append(fc.body, encodeLEB128U(uint64(iterIdx))...)
	fc.body = append(fc.body, opLocalGet)
	fc.body = append(fc.body, encodeLEB128U(uint64(endIdx))...)
	fc.body = append(fc.body, opI64GeS) // iter >= end => exit
	fc.body = append(fc.body, opBrIf)
	fc.body = append(fc.body, encodeLEB128U(1)...) // break

	// Body
	for _, stmt := range body {
		fc.compileStmt(stmt)
	}

	// Increment: iter = iter + 1
	fc.body = append(fc.body, opLocalGet)
	fc.body = append(fc.body, encodeLEB128U(uint64(iterIdx))...)
	fc.body = append(fc.body, opI64Const)
	fc.body = append(fc.body, encodeLEB128S(1)...)
	fc.body = append(fc.body, opI64Add)
	fc.body = append(fc.body, opLocalSet)
	fc.body = append(fc.body, encodeLEB128U(uint64(iterIdx))...)

	// Branch back to loop
	fc.body = append(fc.body, opBr)
	fc.body = append(fc.body, encodeLEB128U(0)...)

	fc.body = append(fc.body, opEnd) // end loop
	fc.blockDepth--
	fc.body = append(fc.body, opEnd) // end block
	fc.blockDepth--

	fc.loopBreakDepth = savedBreak
	fc.loopContinueDepth = savedContinue
}

// --- Expression compilation ---

func (fc *funcCompiler) compileExpr(expr ir.Expr) {
	switch e := expr.(type) {
	case *ir.IntLit:
		fc.body = append(fc.body, opI64Const)
		fc.body = append(fc.body, encodeLEB128S(e.Value)...)

	case *ir.FloatLit:
		val, err := strconv.ParseFloat(e.Value, 64)
		if err != nil {
			val = 0.0
		}
		fc.body = append(fc.body, opF64Const)
		fc.body = append(fc.body, encodeF64(val)...)

	case *ir.BoolLit:
		fc.body = append(fc.body, opI32Const)
		if e.Value {
			fc.body = append(fc.body, encodeLEB128S(1)...)
		} else {
			fc.body = append(fc.body, encodeLEB128S(0)...)
		}

	case *ir.StringLit:
		// Store string in data segment, push pointer as i32
		s := e.Value
		// Strip quotes if present
		if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
			s = s[1 : len(s)-1]
		}
		offset, _ := fc.gen.addStringData(s)
		fc.body = append(fc.body, opI32Const)
		fc.body = append(fc.body, encodeLEB128S(int64(offset))...)

	case *ir.VarRef:
		if idx, ok := fc.localMap[e.Name]; ok {
			fc.body = append(fc.body, opLocalGet)
			fc.body = append(fc.body, encodeLEB128U(uint64(idx))...)
		} else {
			// Unknown variable - push 0
			fc.body = append(fc.body, opI64Const)
			fc.body = append(fc.body, encodeLEB128S(0)...)
		}

	case *ir.BinaryExpr:
		fc.compileBinaryExpr(e)

	case *ir.UnaryExpr:
		fc.compileUnaryExpr(e)

	case *ir.CallExpr:
		fc.compileCallExpr(e)

	case *ir.MethodCallExpr:
		fc.compileMethodCallExpr(e)

	case *ir.FieldAccessExpr:
		// Simplified: just push 0 for now
		fc.body = append(fc.body, opI32Const)
		fc.body = append(fc.body, encodeLEB128S(0)...)

	case *ir.IndexExpr:
		// Simplified: just push 0 for now
		fc.body = append(fc.body, opI64Const)
		fc.body = append(fc.body, encodeLEB128S(0)...)

	case *ir.ArrayLit:
		// Simplified: push 0 (null pointer)
		fc.body = append(fc.body, opI32Const)
		fc.body = append(fc.body, encodeLEB128S(0)...)

	case *ir.RangeExpr:
		// Not directly compiled; used by ForInStmt
		fc.body = append(fc.body, opI32Const)
		fc.body = append(fc.body, encodeLEB128S(0)...)

	case *ir.MatchExpr:
		// Simplified match: evaluate scrutinee and push default
		fc.compileExpr(e.Scrutinee)
		fc.body = append(fc.body, opDrop)
		fc.body = append(fc.body, opI64Const)
		fc.body = append(fc.body, encodeLEB128S(0)...)

	case *ir.SelfRef:
		// In WASM, self would be a pointer to entity memory
		fc.body = append(fc.body, opI32Const)
		fc.body = append(fc.body, encodeLEB128S(0)...)

	case *ir.ResultRef:
		// Result reference in ensures (not compiled to WASM)
		fc.body = append(fc.body, opI64Const)
		fc.body = append(fc.body, encodeLEB128S(0)...)

	case *ir.OldRef:
		if idx, ok := fc.localMap[e.Name]; ok {
			fc.body = append(fc.body, opLocalGet)
			fc.body = append(fc.body, encodeLEB128U(uint64(idx))...)
		}

	case *ir.StringInterp:
		// Simplified: store concatenated static parts
		var s string
		for _, part := range e.Parts {
			if !part.IsExpr {
				s += part.Static
			}
		}
		offset, _ := fc.gen.addStringData(s)
		fc.body = append(fc.body, opI32Const)
		fc.body = append(fc.body, encodeLEB128S(int64(offset))...)

	case *ir.StringConcat:
		// Simplified: compile left side only
		fc.compileExpr(e.Left)

	case *ir.TryExpr:
		fc.compileExpr(e.Expr)

	case *ir.ForallExpr, *ir.ExistsExpr:
		// Quantifiers are verification-only, push true
		fc.body = append(fc.body, opI32Const)
		fc.body = append(fc.body, encodeLEB128S(1)...)

	default:
		// Unknown expression type, push 0
		fc.body = append(fc.body, opI64Const)
		fc.body = append(fc.body, encodeLEB128S(0)...)
	}
}

func (fc *funcCompiler) compileBinaryExpr(e *ir.BinaryExpr) {
	fc.compileExpr(e.Left)
	fc.compileExpr(e.Right)

	leftType := e.Left.ExprType()
	isFloat := leftType != nil && leftType.Name == "Float"
	isBool := leftType != nil && leftType.Name == "Bool"

	switch e.Op {
	case lexer.PLUS:
		if isFloat {
			fc.body = append(fc.body, opF64Add)
		} else {
			fc.body = append(fc.body, opI64Add)
		}
	case lexer.MINUS:
		if isFloat {
			fc.body = append(fc.body, opF64Sub)
		} else {
			fc.body = append(fc.body, opI64Sub)
		}
	case lexer.STAR:
		if isFloat {
			fc.body = append(fc.body, opF64Mul)
		} else {
			fc.body = append(fc.body, opI64Mul)
		}
	case lexer.SLASH:
		if isFloat {
			fc.body = append(fc.body, opF64Div)
		} else {
			fc.body = append(fc.body, opI64DivS)
		}
	case lexer.PERCENT:
		fc.body = append(fc.body, opI64RemS)
	case lexer.EQ:
		if isFloat {
			fc.body = append(fc.body, opF64Eq)
		} else if isBool {
			fc.body = append(fc.body, opI32Eq)
		} else {
			fc.body = append(fc.body, opI64Eq)
		}
	case lexer.NEQ:
		if isFloat {
			fc.body = append(fc.body, opF64Ne)
		} else if isBool {
			fc.body = append(fc.body, opI32Ne)
		} else {
			fc.body = append(fc.body, opI64Ne)
		}
	case lexer.LT:
		if isFloat {
			fc.body = append(fc.body, opF64Lt)
		} else {
			fc.body = append(fc.body, opI64LtS)
		}
	case lexer.GT:
		if isFloat {
			fc.body = append(fc.body, opF64Gt)
		} else {
			fc.body = append(fc.body, opI64GtS)
		}
	case lexer.LEQ:
		if isFloat {
			fc.body = append(fc.body, opF64Le)
		} else {
			fc.body = append(fc.body, opI64LeS)
		}
	case lexer.GEQ:
		if isFloat {
			fc.body = append(fc.body, opF64Ge)
		} else {
			fc.body = append(fc.body, opI64GeS)
		}
	case lexer.AND:
		fc.body = append(fc.body, opI32And)
	case lexer.OR:
		fc.body = append(fc.body, opI32Or)
	}
}

func (fc *funcCompiler) compileUnaryExpr(e *ir.UnaryExpr) {
	fc.compileExpr(e.Operand)
	switch e.Op {
	case lexer.MINUS:
		if e.Operand.ExprType() != nil && e.Operand.ExprType().Name == "Float" {
			// f64.neg doesn't exist as single opcode; use 0.0 - x
			// Actually: reorder -> push 0, push operand, sub
			// We already pushed operand, so store it, push 0, get it back, sub
			tmp := fc.allocAnon(valF64)
			fc.body = append(fc.body, opLocalSet)
			fc.body = append(fc.body, encodeLEB128U(uint64(tmp))...)
			fc.body = append(fc.body, opF64Const)
			fc.body = append(fc.body, encodeF64(0.0)...)
			fc.body = append(fc.body, opLocalGet)
			fc.body = append(fc.body, encodeLEB128U(uint64(tmp))...)
			fc.body = append(fc.body, opF64Sub)
		} else {
			// 0 - x
			tmp := fc.allocAnon(valI64)
			fc.body = append(fc.body, opLocalSet)
			fc.body = append(fc.body, encodeLEB128U(uint64(tmp))...)
			fc.body = append(fc.body, opI64Const)
			fc.body = append(fc.body, encodeLEB128S(0)...)
			fc.body = append(fc.body, opLocalGet)
			fc.body = append(fc.body, encodeLEB128U(uint64(tmp))...)
			fc.body = append(fc.body, opI64Sub)
		}
	case lexer.NOT:
		fc.body = append(fc.body, opI32Eqz)
	}
}

func (fc *funcCompiler) compileCallExpr(e *ir.CallExpr) {
	switch e.Kind {
	case ir.CallBuiltin:
		fc.compileBuiltinCall(e)
	case ir.CallFunction:
		// Compile arguments
		for _, arg := range e.Args {
			fc.compileExpr(arg)
		}
		// Look up function index
		if idx, ok := fc.gen.funcIndex[e.Function]; ok {
			fc.body = append(fc.body, opCall)
			fc.body = append(fc.body, encodeLEB128U(uint64(idx))...)
		} else {
			// Unknown function; push default
			fc.body = append(fc.body, opI64Const)
			fc.body = append(fc.body, encodeLEB128S(0)...)
		}
	case ir.CallConstructor:
		// Simplified: push 0 pointer
		fc.body = append(fc.body, opI32Const)
		fc.body = append(fc.body, encodeLEB128S(0)...)
	case ir.CallVariant:
		// Simplified: push tag value
		fc.body = append(fc.body, opI32Const)
		fc.body = append(fc.body, encodeLEB128S(0)...)
	}
}

func (fc *funcCompiler) compileBuiltinCall(e *ir.CallExpr) {
	switch e.Function {
	case "print":
		// In pure WASM there's no stdout. We compile the arg but can't print.
		// The host environment would provide an imported print function.
		if len(e.Args) > 0 {
			fc.compileExpr(e.Args[0])
			fc.body = append(fc.body, opDrop)
		}
	case "len":
		// Simplified: push 0
		fc.body = append(fc.body, opI64Const)
		fc.body = append(fc.body, encodeLEB128S(0)...)
	case "push", "pop":
		// Array operations - simplified
		fc.body = append(fc.body, opI64Const)
		fc.body = append(fc.body, encodeLEB128S(0)...)
	default:
		// Unknown builtin
		fc.body = append(fc.body, opI64Const)
		fc.body = append(fc.body, encodeLEB128S(0)...)
	}
}

func (fc *funcCompiler) compileMethodCallExpr(e *ir.MethodCallExpr) {
	if e.IsModuleCall {
		// Cross-module function call: look up mangled name
		mangledName := e.ModuleName + "_" + e.Method
		for _, arg := range e.Args {
			fc.compileExpr(arg)
		}
		if idx, ok := fc.gen.funcIndex[mangledName]; ok {
			fc.body = append(fc.body, opCall)
			fc.body = append(fc.body, encodeLEB128U(uint64(idx))...)
		} else {
			fc.body = append(fc.body, opI64Const)
			fc.body = append(fc.body, encodeLEB128S(0)...)
		}
		return
	}

	// Regular method call - simplified
	fc.body = append(fc.body, opI64Const)
	fc.body = append(fc.body, encodeLEB128S(0)...)
}

// ensureI32 adds a conversion to i32 if the expression type is not already i32-compatible.
func (fc *funcCompiler) ensureI32(expr ir.Expr) {
	t := expr.ExprType()
	if t == nil {
		return
	}
	switch t.Name {
	case "Int":
		fc.body = append(fc.body, opI32WrapI64)
	case "Bool":
		// Already i32
	}
}
