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
	Name       string
	IsEntry    bool
	Path       string // original file path
	Functions  []*Function
	Entities   []*Entity
	Enums      []*Enum
	Intents    []*Intent
}

// Function represents a function declaration in the IR.
type Function struct {
	Name       string
	IsEntry    bool
	IsPublic   bool
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
type Contract struct {
	Expr    Expr
	RawText string // original source text for error messages
}

// DecreasesClause represents a termination metric.
type DecreasesClause struct {
	Expr    Expr
	RawText string
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
	Object     Expr
	Method     string
	Args       []Expr
	IsModuleCall bool   // true if Object is a module name
	ModuleName   string // set when IsModuleCall is true
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
	Name string        // matches OldCapture.Name
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
	EnumName    string   // resolved enum name (e.g., "Color", "Result")
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

// StringConcat represents string concatenation (replaces BinaryExpr with + on strings).
type StringConcat struct {
	Left  Expr
	Right Expr
	Type  *checker.Type
}

func (e *StringConcat) ExprType() *checker.Type { return e.Type }
func (*StringConcat) exprNode()                 {}
