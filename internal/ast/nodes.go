package ast

import "github.com/lhaig/intent/internal/lexer"

// Node is the base interface for all AST nodes
type Node interface {
	Pos() (line, col int)
}

// Statement nodes
type Statement interface {
	Node
	stmtNode()
}

// Expression nodes
type Expression interface {
	Node
	exprNode()
}

// Program represents the entire Intent program
type Program struct {
	Module    *ModuleDecl
	Imports   []*ImportDecl
	Functions []*FunctionDecl
	Entities  []*EntityDecl
	Enums     []*EnumDecl
	Intents   []*IntentDecl
}

func (p *Program) Pos() (int, int) {
	if p.Module != nil {
		return p.Module.Pos()
	}
	return 0, 0
}

// ModuleDecl represents a module declaration
type ModuleDecl struct {
	Name    string
	Version string
	Line    int
	Column  int
}

func (m *ModuleDecl) Pos() (int, int) { return m.Line, m.Column }

// ImportDecl represents an import declaration
type ImportDecl struct {
	Path   string // import path (e.g. "math.intent")
	Alias  string // empty for now (no aliasing in v1)
	Line   int
	Column int
}

func (i *ImportDecl) Pos() (int, int) { return i.Line, i.Column }

// FunctionDecl represents a function declaration
type FunctionDecl struct {
	Name       string
	IsEntry    bool
	IsPublic   bool
	Params     []*Param
	ReturnType *TypeRef
	Requires   []*ContractClause
	Ensures    []*ContractClause
	Body       *Block
	Line       int
	Column     int
}

func (f *FunctionDecl) Pos() (int, int) { return f.Line, f.Column }

// Param represents a function parameter
type Param struct {
	Name   string
	Type   *TypeRef
	Line   int
	Column int
}

func (p *Param) Pos() (int, int) { return p.Line, p.Column }

// TypeRef represents a type reference
type TypeRef struct {
	Name     string
	TypeArgs []*TypeRef // e.g., []*TypeRef{{Name:"Int"}} for Array<Int>
	Line     int
	Column   int
}

func (t *TypeRef) Pos() (int, int) { return t.Line, t.Column }

// ContractClause represents a requires/ensures clause
type ContractClause struct {
	Expr    Expression
	RawText string
	Line    int
	Column  int
}

func (c *ContractClause) Pos() (int, int) { return c.Line, c.Column }

// DecreaseClause represents a decreases clause for termination checking
type DecreaseClause struct {
	Expr    Expression
	RawText string
	Line    int
	Column  int
}

func (d *DecreaseClause) Pos() (int, int) { return d.Line, d.Column }

// EntityDecl represents an entity declaration
type EntityDecl struct {
	Name        string
	IsPublic    bool
	Fields      []*FieldDecl
	Invariants  []*InvariantDecl
	Constructor *ConstructorDecl
	Methods     []*MethodDecl
	Line        int
	Column      int
}

func (e *EntityDecl) Pos() (int, int) { return e.Line, e.Column }

// FieldDecl represents an entity field declaration
type FieldDecl struct {
	Name   string
	Type   *TypeRef
	Line   int
	Column int
}

func (f *FieldDecl) Pos() (int, int) { return f.Line, f.Column }

// InvariantDecl represents an entity invariant
type InvariantDecl struct {
	Expr    Expression
	RawText string
	Line    int
	Column  int
}

func (i *InvariantDecl) Pos() (int, int) { return i.Line, i.Column }

// ConstructorDecl represents an entity constructor
type ConstructorDecl struct {
	Params   []*Param
	Requires []*ContractClause
	Ensures  []*ContractClause
	Body     *Block
	Line     int
	Column   int
}

func (c *ConstructorDecl) Pos() (int, int) { return c.Line, c.Column }

// MethodDecl represents an entity method
type MethodDecl struct {
	Name       string
	Params     []*Param
	ReturnType *TypeRef
	Requires   []*ContractClause
	Ensures    []*ContractClause
	Body       *Block
	Line       int
	Column     int
}

func (m *MethodDecl) Pos() (int, int) { return m.Line, m.Column }

// IntentDecl represents an intent declaration
type IntentDecl struct {
	Description string
	Goals       []string
	Constraints []string
	Guarantees  []string
	VerifiedBy  []*VerifiedByRef
	Line        int
	Column      int
}

func (i *IntentDecl) Pos() (int, int) { return i.Line, i.Column }

// VerifiedByRef represents a reference in the verified_by clause
type VerifiedByRef struct {
	Parts  []string
	Line   int
	Column int
}

func (v *VerifiedByRef) Pos() (int, int) { return v.Line, v.Column }

// Block represents a block of statements
type Block struct {
	Statements []Statement
	Line       int
	Column     int
}

func (b *Block) Pos() (int, int)  { return b.Line, b.Column }
func (b *Block) stmtNode()        {}

// LetStmt represents a let statement
type LetStmt struct {
	Name    string
	Mutable bool
	Type    *TypeRef
	Value   Expression
	Line    int
	Column  int
}

func (l *LetStmt) Pos() (int, int) { return l.Line, l.Column }
func (l *LetStmt) stmtNode()       {}

// AssignStmt represents an assignment statement
type AssignStmt struct {
	Target Expression
	Value  Expression
	Line   int
	Column int
}

func (a *AssignStmt) Pos() (int, int) { return a.Line, a.Column }
func (a *AssignStmt) stmtNode()       {}

// ReturnStmt represents a return statement
type ReturnStmt struct {
	Value  Expression
	Line   int
	Column int
}

func (r *ReturnStmt) Pos() (int, int) { return r.Line, r.Column }
func (r *ReturnStmt) stmtNode()       {}

// IfStmt represents an if statement
type IfStmt struct {
	Condition Expression
	Then      *Block
	Else      Statement
	Line      int
	Column    int
}

func (i *IfStmt) Pos() (int, int) { return i.Line, i.Column }
func (i *IfStmt) stmtNode()       {}

// WhileStmt represents a while statement
type WhileStmt struct {
	Condition  Expression
	Invariants []*ContractClause // zero or more invariant clauses
	Decreases  *DecreaseClause   // optional decreases clause
	Body       *Block
	Line       int
	Column     int
}

func (w *WhileStmt) Pos() (int, int) { return w.Line, w.Column }
func (w *WhileStmt) stmtNode()       {}

// BreakStmt represents a break statement
type BreakStmt struct {
	Line   int
	Column int
}

func (b *BreakStmt) Pos() (int, int) { return b.Line, b.Column }
func (b *BreakStmt) stmtNode()       {}

// ContinueStmt represents a continue statement
type ContinueStmt struct {
	Line   int
	Column int
}

func (c *ContinueStmt) Pos() (int, int) { return c.Line, c.Column }
func (c *ContinueStmt) stmtNode()       {}

// ExprStmt represents an expression statement
type ExprStmt struct {
	Expr   Expression
	Line   int
	Column int
}

func (e *ExprStmt) Pos() (int, int) { return e.Line, e.Column }
func (e *ExprStmt) stmtNode()       {}

// BinaryExpr represents a binary expression
type BinaryExpr struct {
	Left   Expression
	Op     lexer.TokenType
	Right  Expression
	Line   int
	Column int
}

func (b *BinaryExpr) Pos() (int, int) { return b.Line, b.Column }
func (b *BinaryExpr) exprNode()       {}

// UnaryExpr represents a unary expression
type UnaryExpr struct {
	Op      lexer.TokenType
	Operand Expression
	Line    int
	Column  int
}

func (u *UnaryExpr) Pos() (int, int) { return u.Line, u.Column }
func (u *UnaryExpr) exprNode()       {}

// CallExpr represents a function call
type CallExpr struct {
	Function string
	Args     []Expression
	Line     int
	Column   int
}

func (c *CallExpr) Pos() (int, int) { return c.Line, c.Column }
func (c *CallExpr) exprNode()       {}

// MethodCallExpr represents a method call
type MethodCallExpr struct {
	Object Expression
	Method string
	Args   []Expression
	Line   int
	Column int
}

func (m *MethodCallExpr) Pos() (int, int) { return m.Line, m.Column }
func (m *MethodCallExpr) exprNode()       {}

// FieldAccessExpr represents a field access
type FieldAccessExpr struct {
	Object Expression
	Field  string
	Line   int
	Column int
}

func (f *FieldAccessExpr) Pos() (int, int) { return f.Line, f.Column }
func (f *FieldAccessExpr) exprNode()       {}

// OldExpr represents an old() expression in contracts
type OldExpr struct {
	Expr   Expression
	Line   int
	Column int
}

func (o *OldExpr) Pos() (int, int) { return o.Line, o.Column }
func (o *OldExpr) exprNode()       {}

// Identifier represents an identifier
type Identifier struct {
	Name   string
	Line   int
	Column int
}

func (i *Identifier) Pos() (int, int) { return i.Line, i.Column }
func (i *Identifier) exprNode()       {}

// SelfExpr represents the self keyword
type SelfExpr struct {
	Line   int
	Column int
}

func (s *SelfExpr) Pos() (int, int) { return s.Line, s.Column }
func (s *SelfExpr) exprNode()       {}

// ResultExpr represents the result keyword
type ResultExpr struct {
	Line   int
	Column int
}

func (r *ResultExpr) Pos() (int, int) { return r.Line, r.Column }
func (r *ResultExpr) exprNode()       {}

// IntLit represents an integer literal
type IntLit struct {
	Value  string
	Line   int
	Column int
}

func (i *IntLit) Pos() (int, int) { return i.Line, i.Column }
func (i *IntLit) exprNode()       {}

// FloatLit represents a float literal
type FloatLit struct {
	Value  string
	Line   int
	Column int
}

func (f *FloatLit) Pos() (int, int) { return f.Line, f.Column }
func (f *FloatLit) exprNode()       {}

// StringLit represents a string literal
type StringLit struct {
	Value  string
	Line   int
	Column int
}

func (s *StringLit) Pos() (int, int) { return s.Line, s.Column }
func (s *StringLit) exprNode()       {}

// BoolLit represents a boolean literal
type BoolLit struct {
	Value  bool
	Line   int
	Column int
}

func (b *BoolLit) Pos() (int, int) { return b.Line, b.Column }
func (b *BoolLit) exprNode()       {}

// ArrayLit represents an array literal [expr, expr, ...]
type ArrayLit struct {
	Elements []Expression
	Line     int
	Column   int
}

func (a *ArrayLit) Pos() (int, int) { return a.Line, a.Column }
func (a *ArrayLit) exprNode()       {}

// IndexExpr represents an index access arr[i]
type IndexExpr struct {
	Object Expression // the array being indexed
	Index  Expression // the index expression
	Line   int
	Column int
}

func (i *IndexExpr) Pos() (int, int) { return i.Line, i.Column }
func (i *IndexExpr) exprNode()       {}

// ForInStmt represents a for-in loop: for <variable> in <iterable> { ... }
type ForInStmt struct {
	Variable string      // loop variable name
	Iterable Expression  // array expression or RangeExpr
	Body     *Block
	Line     int
	Column   int
}

func (f *ForInStmt) Pos() (int, int) { return f.Line, f.Column }
func (f *ForInStmt) stmtNode()       {}

// RangeExpr represents an integer range: start..end (exclusive)
type RangeExpr struct {
	Start  Expression
	End    Expression
	Line   int
	Column int
}

func (r *RangeExpr) Pos() (int, int) { return r.Line, r.Column }
func (r *RangeExpr) exprNode()       {}

// ForallExpr represents: forall <variable> in <range>: <body>
type ForallExpr struct {
	Variable string       // bound variable name (e.g., "i")
	Domain   *RangeExpr   // bounded range (e.g., 0..n)
	Body     Expression   // predicate (must be Bool)
	Line     int
	Column   int
}

func (f *ForallExpr) Pos() (int, int) { return f.Line, f.Column }
func (f *ForallExpr) exprNode()       {}

// ExistsExpr represents: exists <variable> in <range>: <body>
type ExistsExpr struct {
	Variable string       // bound variable name (e.g., "i")
	Domain   *RangeExpr   // bounded range (e.g., 0..n)
	Body     Expression   // predicate (must be Bool)
	Line     int
	Column   int
}

func (e *ExistsExpr) Pos() (int, int) { return e.Line, e.Column }
func (e *ExistsExpr) exprNode()       {}

// EnumDecl represents an enum declaration
type EnumDecl struct {
	Name     string
	IsPublic bool
	Variants []*EnumVariant
	Line     int
	Column   int
}

func (e *EnumDecl) Pos() (int, int) { return e.Line, e.Column }

// EnumVariant represents a variant in an enum
type EnumVariant struct {
	Name   string
	Fields []*FieldDecl // nil/empty for unit variants
	Line   int
	Column int
}

func (e *EnumVariant) Pos() (int, int) { return e.Line, e.Column }

// MatchExpr represents a match expression
type MatchExpr struct {
	Scrutinee Expression
	Arms      []*MatchArm
	Line      int
	Column    int
}

func (m *MatchExpr) Pos() (int, int) { return m.Line, m.Column }
func (m *MatchExpr) exprNode()       {}

// MatchArm represents an arm in a match expression
type MatchArm struct {
	Pattern *MatchPattern
	Body    Expression
	Line    int
	Column  int
}

func (m *MatchArm) Pos() (int, int) { return m.Line, m.Column }

// MatchPattern represents a pattern in a match arm
type MatchPattern struct {
	VariantName string
	Bindings    []string // positional variable names bound from variant fields
	IsWildcard  bool     // true if pattern is "_"
	Line        int
	Column      int
}

func (m *MatchPattern) Pos() (int, int) { return m.Line, m.Column }

// TryExpr represents a try expression (expr?)
type TryExpr struct {
	Expr   Expression
	Line   int
	Column int
}

func (t *TryExpr) Pos() (int, int) { return t.Line, t.Column }
func (t *TryExpr) exprNode()       {}
