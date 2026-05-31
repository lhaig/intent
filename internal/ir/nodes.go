package ir

import (
	"github.com/lhaig/intent/internal/checker"
	"github.com/lhaig/intent/internal/lexer"
)

// Program represents a multi-file Intent program after lowering.
type Program struct {
	Modules []*Module
}

// Module represents a single Intent source file after lowering.
type Module struct {
	Name            string
	DeclName        string // module declaration name (e.g., "attractor_validation")
	PackageName     string // package name from intent.toml (e.g., "types_pkg"); empty for same-package modules
	IsEntry         bool
	Path            string // original file path
	Functions       []*Function
	ExternFunctions []*ExternFunction
	Entities        []*Entity
	Enums           []*Enum
	Traits          []*Trait
	ImplBlocks      []*ImplBlock
	Intents         []*Intent
	Tests           []*Test
}

// ExternFunction represents a Rust FFI / crate-import declaration.
// See ADR 0028. No body; the call lowers to the named Rust function.
type ExternFunction struct {
	Name       string
	RustPath   string // e.g. "blake3::hash" — first segment is the crate
	Params     []*Param
	ReturnType *checker.Type
	Requires   []*Contract
	Ensures    []*Contract
}

// Test represents an in-language test declaration in the IR.
// See ADR 0029 / phase-16. Tests have no parameters, no return type
// (implicit Void), and their bodies may call any function or method in the
// module's import graph. Backends emit one runner-callable function per test
// and a target-specific dispatcher.
//
// ADR 0031: Annotations carries through from the AST so the runner can filter
// tests by target without re-parsing source.
type Test struct {
	Name        string // human-readable name from the test "..." string literal
	IsAsync     bool
	Annotations []*TestAnnotation
	Body        []Stmt
}

// TestAnnotation is the IR-level mirror of ast.TestAnnotation. See ADR 0031.
type TestAnnotation struct {
	Name string
	Args []string
}

// TargetSpecificTargets returns the union of target arguments across all
// @target_specific annotations on this test, in declaration order. Returns nil
// if the test has no @target_specific annotation (i.e. runs on every target).
func (t *Test) TargetSpecificTargets() []string {
	var targets []string
	seen := map[string]bool{}
	for _, ann := range t.Annotations {
		if ann.Name != "target_specific" {
			continue
		}
		for _, a := range ann.Args {
			if !seen[a] {
				seen[a] = true
				targets = append(targets, a)
			}
		}
	}
	return targets
}

// RunsOnTarget returns true if this test should execute on the given target.
// Tests without @target_specific run on every target. Tests with the
// annotation run only on the listed targets.
func (t *Test) RunsOnTarget(target string) bool {
	targets := t.TargetSpecificTargets()
	if len(targets) == 0 {
		return true
	}
	for _, a := range targets {
		if a == target {
			return true
		}
	}
	return false
}

// Function represents a function declaration in the IR.
type Function struct {
	Name       string
	IsEntry    bool
	IsPublic   bool
	IsAsync    bool
	Params     []*Param
	ReturnType *checker.Type
	Requires   []*Contract
	Ensures    []*Contract
	Body       []Stmt
}

// Param represents a function or method parameter.
type Param struct {
	Name string
	Type *checker.Type
}

// Entity represents an entity (class-like) declaration.
type Entity struct {
	Name        string
	IsPublic    bool
	Fields      []*Field
	Invariants  []*Contract
	Constructor *Constructor
	Methods     []*Method
}

// Field represents an entity field.
type Field struct {
	Name string
	Type *checker.Type
}

// Constructor represents an entity constructor.
type Constructor struct {
	Params      []*Param
	Requires    []*Contract
	Ensures     []*Contract
	OldCaptures []*OldCapture
	Body        []Stmt
}

// Method represents an entity method.
type Method struct {
	Name        string
	Params      []*Param
	ReturnType  *checker.Type
	Requires    []*Contract
	Ensures     []*Contract
	OldCaptures []*OldCapture
	Body        []Stmt
}

// OldCapture represents a pre-state capture for old() expressions.
// Before a method body executes, each OldCapture is evaluated and stored.
type OldCapture struct {
	Name string // generated name, e.g., "__old_0", "__old_1"
	Expr Expr   // expression to evaluate before body
}

// Contract represents a requires/ensures/invariant clause.
//
// Line / Column are 1-indexed source positions copied from the AST
// (ADR 0034). They point at the clause keyword (`requires` / `ensures` /
// `invariant`) and are consumed by the LSP to anchor verify diagnostics
// on the failing clause rather than the start of the file. A zero value
// for Line means "no source position" — used for synthetic clauses
// added by the IR (currently none, but reserved).
type Contract struct {
	Expr    Expr
	RawText string // original source text for error messages
	Line    int
	Column  int
}

// DecreasesClause represents a termination metric. Line / Column carry
// the position of the `decreases` keyword for editor diagnostics on
// failing termination checks (ADR 0034).
type DecreasesClause struct {
	Expr    Expr
	RawText string
	Line    int
	Column  int
}

// Enum represents an enum declaration.
type Enum struct {
	Name     string
	IsPublic bool
	Variants []*EnumVariant
}

// EnumVariant represents a variant in an enum.
type EnumVariant struct {
	Name   string
	Fields []*Field
}

// Trait represents a trait declaration.
type Trait struct {
	Name     string
	IsPublic bool
	Methods  []*TraitMethod
}

// TraitMethod represents a method signature in a trait.
type TraitMethod struct {
	Name       string
	Params     []*Param
	ReturnType *checker.Type
	Requires   []*Contract
	Ensures    []*Contract
}

// ImplBlock represents a trait implementation for an entity.
type ImplBlock struct {
	TraitName  string
	EntityName string
	Methods    []*Method
}

// Intent represents an intent declaration (documentation/verification).
type Intent struct {
	Description string
	Goals       []string
	Constraints []string
	Guarantees  []string
	VerifiedBy  [][]string // each element is Parts []string
}

// --- Statements ---

// Stmt is the interface for all IR statement nodes.
type Stmt interface {
	stmtNode()
}

// LetStmt represents a variable binding.
type LetStmt struct {
	Name    string
	Mutable bool
	Type    *checker.Type
	Value   Expr
}

func (*LetStmt) stmtNode() {}

// AssignStmt represents an assignment.
type AssignStmt struct {
	Target Expr
	Value  Expr
}

func (*AssignStmt) stmtNode() {}

// ReturnStmt represents a return statement.
type ReturnStmt struct {
	Value Expr // nil for bare return
}

func (*ReturnStmt) stmtNode() {}

// IfStmt represents an if/else statement.
type IfStmt struct {
	Condition Expr
	Then      []Stmt
	Else      []Stmt // nil if no else branch
}

func (*IfStmt) stmtNode() {}

// WhileStmt represents a while loop.
type WhileStmt struct {
	Condition   Expr
	Invariants  []*Contract
	Decreases   *DecreasesClause
	OldCaptures []*OldCapture // old() captures from loop invariants
	Body        []Stmt
}

func (*WhileStmt) stmtNode() {}

// ForInStmt represents a for-in loop.
type ForInStmt struct {
	Variable string
	Iterable Expr // could be RangeExpr or array expression
	Body     []Stmt
}

func (*ForInStmt) stmtNode() {}

// BreakStmt represents a break statement.
type BreakStmt struct{}

func (*BreakStmt) stmtNode() {}

// ContinueStmt represents a continue statement.
type ContinueStmt struct{}

func (*ContinueStmt) stmtNode() {}

// ExprStmt wraps an expression used as a statement.
type ExprStmt struct {
	Expr Expr
}

func (*ExprStmt) stmtNode() {}

// --- Expressions ---

// Expr is the interface for all IR expression nodes.
type Expr interface {
	ExprType() *checker.Type
	exprNode()
}

// BinaryExpr represents a binary operation.
type BinaryExpr struct {
	Left  Expr
	Op    lexer.TokenType
	Right Expr
	Type  *checker.Type
}

func (e *BinaryExpr) ExprType() *checker.Type { return e.Type }
func (*BinaryExpr) exprNode()                 {}

// UnaryExpr represents a unary operation.
type UnaryExpr struct {
	Op      lexer.TokenType
	Operand Expr
	Type    *checker.Type
}

func (e *UnaryExpr) ExprType() *checker.Type { return e.Type }
func (*UnaryExpr) exprNode()                 {}

// CallKind identifies how a call should be generated.
type CallKind int

const (
	CallFunction    CallKind = iota // regular function call
	CallConstructor                 // entity constructor (Entity::new)
	CallVariant                     // enum variant constructor
	CallBuiltin                     // print, len, Ok, Err, Some
	CallClosure                     // calling a Fn-typed variable
	CallMethod                      // reserved for future use
)

// CallExpr represents a function or constructor call.
type CallExpr struct {
	Function string
	Args     []Expr
	Kind     CallKind
	EnumName string // for CallVariant: the parent enum name
	Type     *checker.Type
}

func (e *CallExpr) ExprType() *checker.Type { return e.Type }
func (*CallExpr) exprNode()                 {}

// MethodCallExpr represents a method call on an object.
type MethodCallExpr struct {
	Object       Expr
	Method       string
	Args         []Expr
	IsModuleCall bool     // true if Object is a module name
	ModuleName   string   // set when IsModuleCall is true
	CallKind     CallKind // for module calls: function vs constructor
	EnumName     string   // for module entity constructor, the mangled name
	Type         *checker.Type
}

func (e *MethodCallExpr) ExprType() *checker.Type { return e.Type }
func (*MethodCallExpr) exprNode()                 {}

// FieldAccessExpr represents a field access on an object.
type FieldAccessExpr struct {
	Object Expr
	Field  string
	Type   *checker.Type
}

func (e *FieldAccessExpr) ExprType() *checker.Type { return e.Type }
func (*FieldAccessExpr) exprNode()                 {}

// IndexExpr represents an array index access.
type IndexExpr struct {
	Object Expr
	Index  Expr
	Type   *checker.Type
}

func (e *IndexExpr) ExprType() *checker.Type { return e.Type }
func (*IndexExpr) exprNode()                 {}

// OldRef references a previously captured old() value.
type OldRef struct {
	Name string // matches OldCapture.Name
	Type *checker.Type
}

func (e *OldRef) ExprType() *checker.Type { return e.Type }
func (*OldRef) exprNode()                 {}

// VarRef references a variable.
type VarRef struct {
	Name string
	Type *checker.Type
}

func (e *VarRef) ExprType() *checker.Type { return e.Type }
func (*VarRef) exprNode()                 {}

// SelfRef represents the self keyword.
type SelfRef struct {
	Type *checker.Type
}

func (e *SelfRef) ExprType() *checker.Type { return e.Type }
func (*SelfRef) exprNode()                 {}

// ResultRef represents the result keyword in ensures clauses.
type ResultRef struct {
	Type *checker.Type
}

func (e *ResultRef) ExprType() *checker.Type { return e.Type }
func (*ResultRef) exprNode()                 {}

// IntLit represents an integer literal.
type IntLit struct {
	Value int64
	Type  *checker.Type
}

func (e *IntLit) ExprType() *checker.Type { return e.Type }
func (*IntLit) exprNode()                 {}

// FloatLit represents a float literal.
type FloatLit struct {
	Value string // keep original string representation for fidelity
	Type  *checker.Type
}

func (e *FloatLit) ExprType() *checker.Type { return e.Type }
func (*FloatLit) exprNode()                 {}

// StringLit represents a string literal.
type StringLit struct {
	Value string
	Type  *checker.Type
}

func (e *StringLit) ExprType() *checker.Type { return e.Type }
func (*StringLit) exprNode()                 {}

// BoolLit represents a boolean literal.
type BoolLit struct {
	Value bool
	Type  *checker.Type
}

func (e *BoolLit) ExprType() *checker.Type { return e.Type }
func (*BoolLit) exprNode()                 {}

// ArrayLit represents an array literal.
type ArrayLit struct {
	Elements []Expr
	Type     *checker.Type
}

func (e *ArrayLit) ExprType() *checker.Type { return e.Type }
func (*ArrayLit) exprNode()                 {}

// RangeExpr represents a range expression (start..end).
type RangeExpr struct {
	Start Expr
	End   Expr
	Type  *checker.Type
}

func (e *RangeExpr) ExprType() *checker.Type { return e.Type }
func (*RangeExpr) exprNode()                 {}

// ForallExpr represents a universal quantifier.
type ForallExpr struct {
	Variable string
	Domain   *RangeExpr
	Body     Expr
	Type     *checker.Type
}

func (e *ForallExpr) ExprType() *checker.Type { return e.Type }
func (*ForallExpr) exprNode()                 {}

// ExistsExpr represents an existential quantifier.
type ExistsExpr struct {
	Variable string
	Domain   *RangeExpr
	Body     Expr
	Type     *checker.Type
}

func (e *ExistsExpr) ExprType() *checker.Type { return e.Type }
func (*ExistsExpr) exprNode()                 {}

// MatchExpr represents a match expression.
type MatchExpr struct {
	Scrutinee Expr
	Arms      []*MatchArm
	Type      *checker.Type
}

func (e *MatchExpr) ExprType() *checker.Type { return e.Type }
func (*MatchExpr) exprNode()                 {}

// MatchArm represents a single arm in a match expression.
type MatchArm struct {
	Pattern *MatchPattern
	Body    Expr
}

// MatchPattern represents a pattern in a match arm.
type MatchPattern struct {
	EnumName    string // resolved enum name (e.g., "Color", "Result")
	VariantName string
	Bindings    []string
	FieldNames  []string // resolved field names from the enum variant
	IsWildcard  bool
	IsBuiltin   bool // true for Ok, Err, Some, None (use tuple syntax)
}

// TryExpr represents a try expression (expr?).
type TryExpr struct {
	Expr Expr
	Type *checker.Type
}

func (e *TryExpr) ExprType() *checker.Type { return e.Type }
func (*TryExpr) exprNode()                 {}

// LambdaExpr represents a lambda/closure expression.
type LambdaExpr struct {
	Params []*Param
	Body   Expr
	Type   *checker.Type
}

func (e *LambdaExpr) ExprType() *checker.Type { return e.Type }
func (*LambdaExpr) exprNode()                 {}

// StringInterp represents a string with embedded expressions.
type StringInterp struct {
	Parts []StringInterpPart
	Type  *checker.Type
}

// StringInterpPart is a part of an interpolated string (static text or expression).
type StringInterpPart struct {
	IsExpr bool
	Static string // when IsExpr is false
	Expr   Expr   // when IsExpr is true
}

func (e *StringInterp) ExprType() *checker.Type { return e.Type }
func (*StringInterp) exprNode()                 {}

// StringConcat represents string concatenation (replaces BinaryExpr with + on strings).
type StringConcat struct {
	Left  Expr
	Right Expr
	Type  *checker.Type
}

func (e *StringConcat) ExprType() *checker.Type { return e.Type }
func (*StringConcat) exprNode()                 {}

// AwaitExpr represents an await expression that unwraps a Future<T> to T.
type AwaitExpr struct {
	Expr Expr
	Type *checker.Type // the unwrapped type (T from Future<T>)
	// IsJoinHandle is true when the inner expression yields a spawn result
	// (mapped to tokio::task::JoinHandle<T> in Rust). The Rust backend then
	// emits `.await.expect(...)` to unwrap the JoinError; direct async-fn
	// calls produce `impl Future<Output = T>` and use bare `.await` instead.
	IsJoinHandle bool
}

func (e *AwaitExpr) ExprType() *checker.Type { return e.Type }
func (*AwaitExpr) exprNode()                 {}

// SpawnExpr represents a spawn expression that creates a Future<T>.
type SpawnExpr struct {
	Expr Expr
	Type *checker.Type // Future<T>
}

func (e *SpawnExpr) ExprType() *checker.Type { return e.Type }
func (*SpawnExpr) exprNode()                 {}
