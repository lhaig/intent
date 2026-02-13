package parser

import (
	"testing"

	"github.com/lhaig/intent/internal/ast"
	"github.com/lhaig/intent/internal/lexer"
)

func TestParseModuleDecl(t *testing.T) {
	input := `module banking version "0.1.0";`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}
	if prog.Module == nil {
		t.Fatal("expected module declaration")
	}
	if prog.Module.Name != "banking" {
		t.Errorf("expected module name 'banking', got %q", prog.Module.Name)
	}
	if prog.Module.Version != "0.1.0" {
		t.Errorf("expected version '0.1.0', got %q", prog.Module.Version)
	}
}

func TestParseSimpleFunction(t *testing.T) {
	input := `module test version "1.0.0";

function add(x: Int, y: Int) returns Int {
    return x + y;
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}
	if len(prog.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(prog.Functions))
	}

	fn := prog.Functions[0]
	if fn.Name != "add" {
		t.Errorf("expected function name 'add', got %q", fn.Name)
	}
	if fn.IsEntry {
		t.Error("expected non-entry function")
	}
	if len(fn.Params) != 2 {
		t.Fatalf("expected 2 params, got %d", len(fn.Params))
	}
	if fn.Params[0].Name != "x" || fn.Params[0].Type.Name != "Int" {
		t.Errorf("unexpected first param: %s: %s", fn.Params[0].Name, fn.Params[0].Type.Name)
	}
	if fn.ReturnType.Name != "Int" {
		t.Errorf("expected return type 'Int', got %q", fn.ReturnType.Name)
	}
}

func TestParseEntryFunction(t *testing.T) {
	input := `module test version "1.0.0";

entry function main() returns Int {
    return 0;
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}

	fn := prog.Functions[0]
	if !fn.IsEntry {
		t.Error("expected entry function")
	}
	if fn.Name != "main" {
		t.Errorf("expected 'main', got %q", fn.Name)
	}
}

func TestParseFunctionWithContracts(t *testing.T) {
	input := `module test version "1.0.0";

function divide(a: Int, b: Int) returns Int
    requires b != 0
    ensures result * b + a % b == a
{
    return a / b;
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}

	fn := prog.Functions[0]
	if len(fn.Requires) != 1 {
		t.Fatalf("expected 1 requires, got %d", len(fn.Requires))
	}
	if len(fn.Ensures) != 1 {
		t.Fatalf("expected 1 ensures, got %d", len(fn.Ensures))
	}
}

func TestParseEntity(t *testing.T) {
	input := `module test version "1.0.0";

entity BankAccount {
    field owner: String;
    field balance: Int;

    invariant self.balance >= 0;

    constructor(owner: String, initial_balance: Int)
        requires initial_balance >= 0
    {
        self.owner = owner;
        self.balance = initial_balance;
    }

    method deposit(amount: Int) returns Void
        requires amount > 0
        ensures self.balance == old(self.balance) + amount
    {
        self.balance = self.balance + amount;
    }
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}

	if len(prog.Entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(prog.Entities))
	}

	ent := prog.Entities[0]
	if ent.Name != "BankAccount" {
		t.Errorf("expected entity name 'BankAccount', got %q", ent.Name)
	}
	if len(ent.Fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(ent.Fields))
	}
	if len(ent.Invariants) != 1 {
		t.Errorf("expected 1 invariant, got %d", len(ent.Invariants))
	}
	if ent.Constructor == nil {
		t.Fatal("expected constructor")
	}
	if len(ent.Methods) != 1 {
		t.Errorf("expected 1 method, got %d", len(ent.Methods))
	}
	if ent.Methods[0].Name != "deposit" {
		t.Errorf("expected method name 'deposit', got %q", ent.Methods[0].Name)
	}
}

func TestParseIntentBlock(t *testing.T) {
	input := `module test version "1.0.0";

entity Account {
    field balance: Int;
    invariant self.balance >= 0;
    constructor(b: Int)
        requires b >= 0
    {
        self.balance = b;
    }
    method withdraw(amount: Int) returns Bool
        requires amount > 0
        ensures result == true implies self.balance == old(self.balance) - amount
    {
        if self.balance >= amount {
            self.balance = self.balance - amount;
            return true;
        }
        return false;
    }
}

intent "Safe withdrawal" {
    goal: "Ensure balance never goes negative";
    constraint: "Withdrawals cannot exceed balance";
    guarantee: "Failed withdrawals leave balance unchanged";
    verified_by: [Account.invariant, Account.withdraw.requires, Account.withdraw.ensures];
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}

	if len(prog.Intents) != 1 {
		t.Fatalf("expected 1 intent, got %d", len(prog.Intents))
	}

	intent := prog.Intents[0]
	if intent.Description != "Safe withdrawal" {
		t.Errorf("expected description 'Safe withdrawal', got %q", intent.Description)
	}
	if len(intent.Goals) != 1 {
		t.Errorf("expected 1 goal, got %d", len(intent.Goals))
	}
	if len(intent.Constraints) != 1 {
		t.Errorf("expected 1 constraint, got %d", len(intent.Constraints))
	}
	if len(intent.Guarantees) != 1 {
		t.Errorf("expected 1 guarantee, got %d", len(intent.Guarantees))
	}
	if len(intent.VerifiedBy) != 3 {
		t.Fatalf("expected 3 verified_by refs, got %d", len(intent.VerifiedBy))
	}
	if intent.VerifiedBy[0].Parts[0] != "Account" || intent.VerifiedBy[0].Parts[1] != "invariant" {
		t.Errorf("expected Account.invariant, got %v", intent.VerifiedBy[0].Parts)
	}
}

func TestParseLetStatements(t *testing.T) {
	input := `module test version "1.0.0";

entry function main() returns Int {
    let x: Int = 42;
    let mutable counter: Int = 0;
    return x;
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}

	body := prog.Functions[0].Body
	if len(body.Statements) != 3 {
		t.Fatalf("expected 3 statements, got %d", len(body.Statements))
	}

	let1 := body.Statements[0].(*ast.LetStmt)
	if let1.Name != "x" || let1.Mutable {
		t.Errorf("expected immutable 'x', got mutable=%v name=%q", let1.Mutable, let1.Name)
	}

	let2 := body.Statements[1].(*ast.LetStmt)
	if let2.Name != "counter" || !let2.Mutable {
		t.Errorf("expected mutable 'counter', got mutable=%v name=%q", let2.Mutable, let2.Name)
	}
}

func TestParseIfElse(t *testing.T) {
	input := `module test version "1.0.0";

entry function main() returns Int {
    if x > 0 {
        return 1;
    } else {
        return 0;
    }
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}

	body := prog.Functions[0].Body
	ifStmt := body.Statements[0].(*ast.IfStmt)
	if ifStmt.Else == nil {
		t.Error("expected else clause")
	}
}

func TestParseExpressionPrecedence(t *testing.T) {
	input := `module test version "1.0.0";
entry function main() returns Int {
    let x: Int = 1 + 2 * 3;
    return 0;
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}

	body := prog.Functions[0].Body
	let := body.Statements[0].(*ast.LetStmt)

	// Should parse as 1 + (2 * 3)
	binExpr, ok := let.Value.(*ast.BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr, got %T", let.Value)
	}
	if binExpr.Op != lexer.PLUS {
		t.Errorf("expected PLUS, got %s", binExpr.Op)
	}
	rightBin, ok := binExpr.Right.(*ast.BinaryExpr)
	if !ok {
		t.Fatalf("expected right side to be BinaryExpr, got %T", binExpr.Right)
	}
	if rightBin.Op != lexer.STAR {
		t.Errorf("expected STAR, got %s", rightBin.Op)
	}
}

func TestParseMethodCall(t *testing.T) {
	input := `module test version "1.0.0";
entry function main() returns Int {
    account.deposit(50);
    return 0;
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}

	body := prog.Functions[0].Body
	exprStmt := body.Statements[0].(*ast.ExprStmt)
	call, ok := exprStmt.Expr.(*ast.MethodCallExpr)
	if !ok {
		t.Fatalf("expected MethodCallExpr, got %T", exprStmt.Expr)
	}
	if call.Method != "deposit" {
		t.Errorf("expected method 'deposit', got %q", call.Method)
	}
	if len(call.Args) != 1 {
		t.Errorf("expected 1 arg, got %d", len(call.Args))
	}
}

func TestParseFunctionCall(t *testing.T) {
	input := `module test version "1.0.0";
entry function main() returns Int {
    let x: Int = add(1, 2);
    return 0;
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}

	body := prog.Functions[0].Body
	let := body.Statements[0].(*ast.LetStmt)
	call, ok := let.Value.(*ast.CallExpr)
	if !ok {
		t.Fatalf("expected CallExpr, got %T", let.Value)
	}
	if call.Function != "add" {
		t.Errorf("expected function 'add', got %q", call.Function)
	}
	if len(call.Args) != 2 {
		t.Errorf("expected 2 args, got %d", len(call.Args))
	}
}

func TestParseSelfAssignment(t *testing.T) {
	input := `module test version "1.0.0";

entity Counter {
    field value: Int;
    constructor(v: Int) {
        self.value = v;
    }
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}

	ent := prog.Entities[0]
	body := ent.Constructor.Body
	assign, ok := body.Statements[0].(*ast.AssignStmt)
	if !ok {
		t.Fatalf("expected AssignStmt, got %T", body.Statements[0])
	}
	fieldAccess, ok := assign.Target.(*ast.FieldAccessExpr)
	if !ok {
		t.Fatalf("expected FieldAccessExpr, got %T", assign.Target)
	}
	if fieldAccess.Field != "value" {
		t.Errorf("expected field 'value', got %q", fieldAccess.Field)
	}
}

func TestParseImpliesExpression(t *testing.T) {
	input := `module test version "1.0.0";

entity A {
    field x: Int;
    method m() returns Bool
        ensures result == true implies self.x > 0
    {
        return true;
    }
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}

	method := prog.Entities[0].Methods[0]
	if len(method.Ensures) != 1 {
		t.Fatalf("expected 1 ensures, got %d", len(method.Ensures))
	}

	// The ensures expression should contain an 'implies' binary expression
	ensureExpr := method.Ensures[0].Expr
	binExpr, ok := ensureExpr.(*ast.BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr, got %T", ensureExpr)
	}
	if binExpr.Op != lexer.IMPLIES {
		t.Errorf("expected IMPLIES, got %s", binExpr.Op)
	}
}

func TestParseOldExpression(t *testing.T) {
	input := `module test version "1.0.0";
entity A {
    field x: Int;
    method inc() returns Void
        ensures self.x == old(self.x) + 1
    {
        self.x = self.x + 1;
    }
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}

	method := prog.Entities[0].Methods[0]
	ensureExpr := method.Ensures[0].Expr
	binExpr := ensureExpr.(*ast.BinaryExpr)

	// Right side should be old(self.x) + 1
	rightBin := binExpr.Right.(*ast.BinaryExpr)
	oldExpr, ok := rightBin.Left.(*ast.OldExpr)
	if !ok {
		t.Fatalf("expected OldExpr, got %T", rightBin.Left)
	}
	fieldAccess, ok := oldExpr.Expr.(*ast.FieldAccessExpr)
	if !ok {
		t.Fatalf("expected FieldAccessExpr inside old(), got %T", oldExpr.Expr)
	}
	if fieldAccess.Field != "x" {
		t.Errorf("expected field 'x', got %q", fieldAccess.Field)
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "missing semicolon after module",
			input: `module test version "1.0.0"`,
		},
		{
			name: "missing return type",
			input: `module test version "1.0.0";
function foo() {
    return 0;
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(tt.input)
			p.Parse()
			if !p.Diagnostics().HasErrors() {
				t.Error("expected parse errors")
			}
		})
	}
}

func TestParseFullBankingExample(t *testing.T) {
	input := `module banking version "0.1.0";

entity BankAccount {
    field owner: String;
    field balance: Int;

    invariant self.balance >= 0;

    constructor(owner: String, initial_balance: Int)
        requires initial_balance >= 0
    {
        self.owner = owner;
        self.balance = initial_balance;
    }

    method deposit(amount: Int) returns Void
        requires amount > 0
        ensures self.balance == old(self.balance) + amount
    {
        self.balance = self.balance + amount;
    }

    method withdraw(amount: Int) returns Bool
        requires amount > 0
        ensures result == true implies self.balance == old(self.balance) - amount
        ensures result == false implies self.balance == old(self.balance)
    {
        if self.balance >= amount {
            self.balance = self.balance - amount;
            return true;
        }
        return false;
    }
}

intent "Safe withdrawal preserves non-negative balance" {
    goal: "BankAccount.withdraw never results in balance < 0";
    guarantee: "if withdraw returns false then balance is unchanged";
    verified_by: [BankAccount.invariant, BankAccount.withdraw.requires, BankAccount.withdraw.ensures];
}

entry function main() returns Int {
    let mutable account: BankAccount = BankAccount("Alice", 100);
    account.deposit(50);
    let success: Bool = account.withdraw(30);
    return 0;
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}

	if prog.Module.Name != "banking" {
		t.Errorf("expected module 'banking', got %q", prog.Module.Name)
	}
	if len(prog.Entities) != 1 {
		t.Errorf("expected 1 entity, got %d", len(prog.Entities))
	}
	if len(prog.Functions) != 1 {
		t.Errorf("expected 1 function, got %d", len(prog.Functions))
	}
	if len(prog.Intents) != 1 {
		t.Errorf("expected 1 intent, got %d", len(prog.Intents))
	}

	ent := prog.Entities[0]
	if len(ent.Fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(ent.Fields))
	}
	if len(ent.Invariants) != 1 {
		t.Errorf("expected 1 invariant, got %d", len(ent.Invariants))
	}
	if len(ent.Methods) != 2 {
		t.Errorf("expected 2 methods, got %d", len(ent.Methods))
	}
}

func TestParseParameterizedType(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantErr     bool
		checkType   func(t *testing.T, typeRef *ast.TypeRef)
	}{
		{
			name: "Array<Int> type",
			input: `module test version "1.0.0";
entry function main() returns Int {
    let x: Array<Int> = arr;
    return 0;
}`,
			wantErr: false,
			checkType: func(t *testing.T, typeRef *ast.TypeRef) {
				if typeRef.Name != "Array" {
					t.Errorf("expected type name 'Array', got %q", typeRef.Name)
				}
				if len(typeRef.TypeArgs) != 1 {
					t.Fatalf("expected 1 type arg, got %d", len(typeRef.TypeArgs))
				}
				if typeRef.TypeArgs[0].Name != "Int" {
					t.Errorf("expected type arg 'Int', got %q", typeRef.TypeArgs[0].Name)
				}
			},
		},
		{
			name: "Array<String> type",
			input: `module test version "1.0.0";
entry function main() returns Int {
    let x: Array<String> = arr;
    return 0;
}`,
			wantErr: false,
			checkType: func(t *testing.T, typeRef *ast.TypeRef) {
				if typeRef.Name != "Array" {
					t.Errorf("expected type name 'Array', got %q", typeRef.Name)
				}
				if len(typeRef.TypeArgs) != 1 {
					t.Fatalf("expected 1 type arg, got %d", len(typeRef.TypeArgs))
				}
				if typeRef.TypeArgs[0].Name != "String" {
					t.Errorf("expected type arg 'String', got %q", typeRef.TypeArgs[0].Name)
				}
			},
		},
		{
			name: "Nested Array<Array<Int>> type",
			input: `module test version "1.0.0";
entry function main() returns Int {
    let x: Array<Array<Int>> = arr;
    return 0;
}`,
			wantErr: false,
			checkType: func(t *testing.T, typeRef *ast.TypeRef) {
				if typeRef.Name != "Array" {
					t.Errorf("expected type name 'Array', got %q", typeRef.Name)
				}
				if len(typeRef.TypeArgs) != 1 {
					t.Fatalf("expected 1 type arg, got %d", len(typeRef.TypeArgs))
				}
				inner := typeRef.TypeArgs[0]
				if inner.Name != "Array" {
					t.Errorf("expected inner type 'Array', got %q", inner.Name)
				}
				if len(inner.TypeArgs) != 1 {
					t.Fatalf("expected inner type to have 1 type arg, got %d", len(inner.TypeArgs))
				}
				if inner.TypeArgs[0].Name != "Int" {
					t.Errorf("expected innermost type arg 'Int', got %q", inner.TypeArgs[0].Name)
				}
			},
		},
		{
			name: "Simple type no args",
			input: `module test version "1.0.0";
entry function main() returns Int {
    let x: Int = 5;
    return 0;
}`,
			wantErr: false,
			checkType: func(t *testing.T, typeRef *ast.TypeRef) {
				if typeRef.Name != "Int" {
					t.Errorf("expected type name 'Int', got %q", typeRef.Name)
				}
				if len(typeRef.TypeArgs) != 0 {
					t.Errorf("expected no type args, got %d", len(typeRef.TypeArgs))
				}
			},
		},
		{
			name: "Entity type no args",
			input: `module test version "1.0.0";
entity MyEntity {
    field x: Int;
    constructor(x: Int) {
        self.x = x;
    }
}
entry function main() returns Int {
    let x: MyEntity = e;
    return 0;
}`,
			wantErr: false,
			checkType: func(t *testing.T, typeRef *ast.TypeRef) {
				if typeRef.Name != "MyEntity" {
					t.Errorf("expected type name 'MyEntity', got %q", typeRef.Name)
				}
				if len(typeRef.TypeArgs) != 0 {
					t.Errorf("expected no type args, got %d", len(typeRef.TypeArgs))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(tt.input)
			prog := p.Parse()

			if tt.wantErr {
				if !p.Diagnostics().HasErrors() {
					t.Error("expected parse errors")
				}
				return
			}

			if p.Diagnostics().HasErrors() {
				t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
			}

			// Find the let statement with the type
			fn := prog.Functions[0]
			if fn.Body == nil || len(fn.Body.Statements) == 0 {
				t.Fatal("expected function body with statements")
			}

			letStmt, ok := fn.Body.Statements[0].(*ast.LetStmt)
			if !ok {
				t.Fatalf("expected LetStmt, got %T", fn.Body.Statements[0])
			}

			if tt.checkType != nil {
				tt.checkType(t, letStmt.Type)
			}
		})
	}
}

func TestParseWhileStmt(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name: "simple while true",
			input: `module test version "1.0.0";
entry function main() returns Int {
    while true {
        break;
    }
    return 0;
}`,
			wantErr: false,
		},
		{
			name: "while with condition",
			input: `module test version "1.0.0";
entry function main() returns Int {
    let mutable x: Int = 0;
    while x < 10 {
        x = x + 1;
    }
    return x;
}`,
			wantErr: false,
		},
		{
			name: "while with break and continue",
			input: `module test version "1.0.0";
entry function main() returns Int {
    let mutable x: Int = 0;
    while x < 10 {
        if x == 5 {
            break;
        }
        if x == 3 {
            x = x + 1;
            continue;
        }
        x = x + 1;
    }
    return x;
}`,
			wantErr: false,
		},
		{
			name: "nested while loops",
			input: `module test version "1.0.0";
entry function main() returns Int {
    let mutable i: Int = 0;
    while i < 3 {
        let mutable j: Int = 0;
        while j < 3 {
            if j == 1 {
                break;
            }
            j = j + 1;
        }
        i = i + 1;
    }
    return i;
}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(tt.input)
			prog := p.Parse()

			if tt.wantErr {
				if !p.Diagnostics().HasErrors() {
					t.Error("expected parse errors")
				}
				return
			}

			if p.Diagnostics().HasErrors() {
				t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
			}

			// Verify the while statement was parsed
			fn := prog.Functions[0]
			if fn.Body == nil || len(fn.Body.Statements) == 0 {
				t.Fatal("expected function body with statements")
			}

			// Find the while statement
			var foundWhile bool
			for _, stmt := range fn.Body.Statements {
				if whileStmt, ok := stmt.(*ast.WhileStmt); ok {
					foundWhile = true
					if whileStmt.Condition == nil {
						t.Error("while statement has nil condition")
					}
					if whileStmt.Body == nil {
						t.Error("while statement has nil body")
					}
					break
				}
			}

			if !foundWhile {
				t.Error("expected to find a WhileStmt in the function body")
			}
		})
	}
}

func TestParseArrayLiteral(t *testing.T) {
	input := `module test version "1.0.0";
entry function main() returns Int {
    let x: Array<Int> = [1, 2, 3];
    return 0;
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}

	fn := prog.Functions[0]
	letStmt := fn.Body.Statements[0].(*ast.LetStmt)
	arrayLit, ok := letStmt.Value.(*ast.ArrayLit)
	if !ok {
		t.Fatalf("expected ArrayLit, got %T", letStmt.Value)
	}
	if len(arrayLit.Elements) != 3 {
		t.Errorf("expected 3 elements, got %d", len(arrayLit.Elements))
	}
}

func TestParseEmptyArrayLiteral(t *testing.T) {
	input := `module test version "1.0.0";
entry function main() returns Int {
    let x: Array<Int> = [];
    return 0;
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}

	fn := prog.Functions[0]
	letStmt := fn.Body.Statements[0].(*ast.LetStmt)
	arrayLit, ok := letStmt.Value.(*ast.ArrayLit)
	if !ok {
		t.Fatalf("expected ArrayLit, got %T", letStmt.Value)
	}
	if len(arrayLit.Elements) != 0 {
		t.Errorf("expected 0 elements, got %d", len(arrayLit.Elements))
	}
}

func TestParseIndexExpr(t *testing.T) {
	input := `module test version "1.0.0";
entry function main() returns Int {
    let y: Int = arr[0];
    return 0;
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}

	fn := prog.Functions[0]
	letStmt := fn.Body.Statements[0].(*ast.LetStmt)
	indexExpr, ok := letStmt.Value.(*ast.IndexExpr)
	if !ok {
		t.Fatalf("expected IndexExpr, got %T", letStmt.Value)
	}
	ident, ok := indexExpr.Object.(*ast.Identifier)
	if !ok || ident.Name != "arr" {
		t.Errorf("expected identifier 'arr', got %v", indexExpr.Object)
	}
	intLit, ok := indexExpr.Index.(*ast.IntLit)
	if !ok || intLit.Value != "0" {
		t.Errorf("expected int literal '0', got %v", indexExpr.Index)
	}
}

func TestParseIndexExprVariable(t *testing.T) {
	input := `module test version "1.0.0";
entry function main() returns Int {
    let y: Int = arr[i];
    return 0;
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}

	fn := prog.Functions[0]
	letStmt := fn.Body.Statements[0].(*ast.LetStmt)
	indexExpr, ok := letStmt.Value.(*ast.IndexExpr)
	if !ok {
		t.Fatalf("expected IndexExpr, got %T", letStmt.Value)
	}
	ident, ok := indexExpr.Index.(*ast.Identifier)
	if !ok || ident.Name != "i" {
		t.Errorf("expected identifier 'i', got %v", indexExpr.Index)
	}
}

func TestParseNestedIndex(t *testing.T) {
	input := `module test version "1.0.0";
entry function main() returns Int {
    let y: Int = arr[arr2[0]];
    return 0;
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}

	fn := prog.Functions[0]
	letStmt := fn.Body.Statements[0].(*ast.LetStmt)
	indexExpr, ok := letStmt.Value.(*ast.IndexExpr)
	if !ok {
		t.Fatalf("expected IndexExpr, got %T", letStmt.Value)
	}
	// The index should be another IndexExpr
	innerIndex, ok := indexExpr.Index.(*ast.IndexExpr)
	if !ok {
		t.Fatalf("expected nested IndexExpr, got %T", indexExpr.Index)
	}
	ident, ok := innerIndex.Object.(*ast.Identifier)
	if !ok || ident.Name != "arr2" {
		t.Errorf("expected identifier 'arr2', got %v", innerIndex.Object)
	}
}

func TestParseForInArray(t *testing.T) {
	input := `module test version "1.0.0";
entry function main() returns Int {
    for x in arr {
        print(x);
    }
    return 0;
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}

	fn := prog.Functions[0]
	forStmt, ok := fn.Body.Statements[0].(*ast.ForInStmt)
	if !ok {
		t.Fatalf("expected ForInStmt, got %T", fn.Body.Statements[0])
	}
	if forStmt.Variable != "x" {
		t.Errorf("expected variable 'x', got %q", forStmt.Variable)
	}
	ident, ok := forStmt.Iterable.(*ast.Identifier)
	if !ok {
		t.Fatalf("expected Identifier, got %T", forStmt.Iterable)
	}
	if ident.Name != "arr" {
		t.Errorf("expected iterable 'arr', got %q", ident.Name)
	}
}

func TestParseForInRange(t *testing.T) {
	input := `module test version "1.0.0";
entry function main() returns Int {
    for i in 0..10 {
        print(i);
    }
    return 0;
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}

	fn := prog.Functions[0]
	forStmt, ok := fn.Body.Statements[0].(*ast.ForInStmt)
	if !ok {
		t.Fatalf("expected ForInStmt, got %T", fn.Body.Statements[0])
	}
	if forStmt.Variable != "i" {
		t.Errorf("expected variable 'i', got %q", forStmt.Variable)
	}
	rangeExpr, ok := forStmt.Iterable.(*ast.RangeExpr)
	if !ok {
		t.Fatalf("expected RangeExpr, got %T", forStmt.Iterable)
	}
	startLit, ok := rangeExpr.Start.(*ast.IntLit)
	if !ok || startLit.Value != "0" {
		t.Errorf("expected start '0', got %v", rangeExpr.Start)
	}
	endLit, ok := rangeExpr.End.(*ast.IntLit)
	if !ok || endLit.Value != "10" {
		t.Errorf("expected end '10', got %v", rangeExpr.End)
	}
}

func TestParseForInRangeVariable(t *testing.T) {
	input := `module test version "1.0.0";
entry function main() returns Int {
    for i in 0..n {
        print(i);
    }
    return 0;
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}

	fn := prog.Functions[0]
	forStmt, ok := fn.Body.Statements[0].(*ast.ForInStmt)
	if !ok {
		t.Fatalf("expected ForInStmt, got %T", fn.Body.Statements[0])
	}
	rangeExpr, ok := forStmt.Iterable.(*ast.RangeExpr)
	if !ok {
		t.Fatalf("expected RangeExpr, got %T", forStmt.Iterable)
	}
	endIdent, ok := rangeExpr.End.(*ast.Identifier)
	if !ok || endIdent.Name != "n" {
		t.Errorf("expected end identifier 'n', got %v", rangeExpr.End)
	}
}

func TestParseForInWithBreak(t *testing.T) {
	input := `module test version "1.0.0";
entry function main() returns Int {
    for x in arr {
        break;
    }
    return 0;
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}

	fn := prog.Functions[0]
	forStmt, ok := fn.Body.Statements[0].(*ast.ForInStmt)
	if !ok {
		t.Fatalf("expected ForInStmt, got %T", fn.Body.Statements[0])
	}
	breakStmt, ok := forStmt.Body.Statements[0].(*ast.BreakStmt)
	if !ok {
		t.Fatalf("expected BreakStmt, got %T", forStmt.Body.Statements[0])
	}
	_ = breakStmt // avoid unused variable warning
}

func TestParseWhileWithInvariant(t *testing.T) {
	input := `module test version "1.0.0";
entry function main() returns Int {
    let mutable i: Int = 0;
    while i < 10 invariant i >= 0 {
        i = i + 1;
    }
    return i;
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}

	fn := prog.Functions[0]
	whileStmt, ok := fn.Body.Statements[1].(*ast.WhileStmt)
	if !ok {
		t.Fatalf("expected WhileStmt, got %T", fn.Body.Statements[1])
	}
	if len(whileStmt.Invariants) != 1 {
		t.Fatalf("expected 1 invariant, got %d", len(whileStmt.Invariants))
	}
	if whileStmt.Invariants[0].Expr == nil {
		t.Error("invariant expression should not be nil")
	}
}

func TestParseWhileWithMultipleInvariants(t *testing.T) {
	input := `module test version "1.0.0";
entry function main() returns Int {
    let mutable i: Int = 0;
    let mutable sum: Int = 0;
    while i < 10 invariant i >= 0 invariant sum >= 0 {
        sum = sum + i;
        i = i + 1;
    }
    return sum;
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}

	fn := prog.Functions[0]
	whileStmt, ok := fn.Body.Statements[2].(*ast.WhileStmt)
	if !ok {
		t.Fatalf("expected WhileStmt, got %T", fn.Body.Statements[2])
	}
	if len(whileStmt.Invariants) != 2 {
		t.Fatalf("expected 2 invariants, got %d", len(whileStmt.Invariants))
	}
}

func TestParseWhileWithDecreases(t *testing.T) {
	input := `module test version "1.0.0";
entry function main() returns Int {
    let mutable n: Int = 10;
    while n > 0 decreases n {
        n = n - 1;
    }
    return 0;
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}

	fn := prog.Functions[0]
	whileStmt, ok := fn.Body.Statements[1].(*ast.WhileStmt)
	if !ok {
		t.Fatalf("expected WhileStmt, got %T", fn.Body.Statements[1])
	}
	if whileStmt.Decreases == nil {
		t.Fatal("expected decreases clause")
	}
	ident, ok := whileStmt.Decreases.Expr.(*ast.Identifier)
	if !ok {
		t.Fatalf("expected identifier, got %T", whileStmt.Decreases.Expr)
	}
	if ident.Name != "n" {
		t.Errorf("expected identifier 'n', got %q", ident.Name)
	}
}

func TestParseWhileWithInvariantAndDecreases(t *testing.T) {
	input := `module test version "1.0.0";
entry function main() returns Int {
    let mutable i: Int = 0;
    let mutable n: Int = 10;
    while i < n invariant i >= 0 decreases n - i {
        i = i + 1;
    }
    return i;
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}

	fn := prog.Functions[0]
	whileStmt, ok := fn.Body.Statements[2].(*ast.WhileStmt)
	if !ok {
		t.Fatalf("expected WhileStmt, got %T", fn.Body.Statements[2])
	}
	if len(whileStmt.Invariants) != 1 {
		t.Fatalf("expected 1 invariant, got %d", len(whileStmt.Invariants))
	}
	if whileStmt.Decreases == nil {
		t.Fatal("expected decreases clause")
	}
	binExpr, ok := whileStmt.Decreases.Expr.(*ast.BinaryExpr)
	if !ok {
		t.Fatalf("expected binary expression, got %T", whileStmt.Decreases.Expr)
	}
	if binExpr.Op != lexer.MINUS {
		t.Errorf("expected MINUS operator, got %s", binExpr.Op)
	}
}

func TestParseWhileWithoutContracts(t *testing.T) {
	input := `module test version "1.0.0";
entry function main() returns Int {
    let mutable i: Int = 0;
    while i < 10 {
        i = i + 1;
    }
    return i;
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}

	fn := prog.Functions[0]
	whileStmt, ok := fn.Body.Statements[1].(*ast.WhileStmt)
	if !ok {
		t.Fatalf("expected WhileStmt, got %T", fn.Body.Statements[1])
	}
	if len(whileStmt.Invariants) != 0 {
		t.Errorf("expected no invariants, got %d", len(whileStmt.Invariants))
	}
	if whileStmt.Decreases != nil {
		t.Error("expected no decreases clause")
	}
}

func TestParseForallExpr(t *testing.T) {
	input := `module test version "1.0.0";

function check_positive(arr: Array<Int>, n: Int) returns Bool
    requires forall i in 0..n: arr[i] >= 0
{
    return true;
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}
	if len(prog.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(prog.Functions))
	}

	fn := prog.Functions[0]
	if len(fn.Requires) != 1 {
		t.Fatalf("expected 1 requires clause, got %d", len(fn.Requires))
	}

	forallExpr, ok := fn.Requires[0].Expr.(*ast.ForallExpr)
	if !ok {
		t.Fatalf("expected ForallExpr, got %T", fn.Requires[0].Expr)
	}
	if forallExpr.Variable != "i" {
		t.Errorf("expected variable 'i', got %q", forallExpr.Variable)
	}
	if forallExpr.Domain == nil {
		t.Fatal("expected domain (RangeExpr)")
	}
	if forallExpr.Body == nil {
		t.Fatal("expected body expression")
	}

	// Check that body is a BinaryExpr (arr[i] >= 0)
	_, ok = forallExpr.Body.(*ast.BinaryExpr)
	if !ok {
		t.Errorf("expected BinaryExpr body, got %T", forallExpr.Body)
	}
}

func TestParseExistsExpr(t *testing.T) {
	input := `module test version "1.0.0";

function find_target(arr: Array<Int>, n: Int, target: Int) returns Bool
    requires exists i in 0..n: arr[i] == target
{
    return false;
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}
	if len(prog.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(prog.Functions))
	}

	fn := prog.Functions[0]
	if len(fn.Requires) != 1 {
		t.Fatalf("expected 1 requires clause, got %d", len(fn.Requires))
	}

	existsExpr, ok := fn.Requires[0].Expr.(*ast.ExistsExpr)
	if !ok {
		t.Fatalf("expected ExistsExpr, got %T", fn.Requires[0].Expr)
	}
	if existsExpr.Variable != "i" {
		t.Errorf("expected variable 'i', got %q", existsExpr.Variable)
	}
	if existsExpr.Domain == nil {
		t.Fatal("expected domain (RangeExpr)")
	}
	if existsExpr.Body == nil {
		t.Fatal("expected body expression")
	}
}

func TestParseForallInEnsures(t *testing.T) {
	input := `module test version "1.0.0";

function make_positive(arr: Array<Int>) returns Array<Int>
    ensures forall i in 0..len(result): result[i] > 0
{
    return arr;
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}
	if len(prog.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(prog.Functions))
	}

	fn := prog.Functions[0]
	if len(fn.Ensures) != 1 {
		t.Fatalf("expected 1 ensures clause, got %d", len(fn.Ensures))
	}

	forallExpr, ok := fn.Ensures[0].Expr.(*ast.ForallExpr)
	if !ok {
		t.Fatalf("expected ForallExpr, got %T", fn.Ensures[0].Expr)
	}
	if forallExpr.Variable != "i" {
		t.Errorf("expected variable 'i', got %q", forallExpr.Variable)
	}
}

func TestParseForallWithComplex(t *testing.T) {
	input := `module test version "1.0.0";

function sort(arr: Array<Int>) returns Array<Int>
    ensures forall i in 0..len(result)-1: result[i] <= result[i+1]
{
    return arr;
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}
	if len(prog.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(prog.Functions))
	}

	fn := prog.Functions[0]
	if len(fn.Ensures) != 1 {
		t.Fatalf("expected 1 ensures clause, got %d", len(fn.Ensures))
	}

	forallExpr, ok := fn.Ensures[0].Expr.(*ast.ForallExpr)
	if !ok {
		t.Fatalf("expected ForallExpr, got %T", fn.Ensures[0].Expr)
	}
	if forallExpr.Variable != "i" {
		t.Errorf("expected variable 'i', got %q", forallExpr.Variable)
	}

	// Check that domain end is a complex expression (len(result)-1)
	if forallExpr.Domain == nil {
		t.Fatal("expected domain (RangeExpr)")
	}

	// Check that body handles complex index expressions (result[i] <= result[i+1])
	binaryExpr, ok := forallExpr.Body.(*ast.BinaryExpr)
	if !ok {
		t.Errorf("expected BinaryExpr body, got %T", forallExpr.Body)
	}
	if ok && binaryExpr.Op != lexer.LEQ {
		t.Errorf("expected <= operator, got %v", binaryExpr.Op)
	}
}

func TestParseEnumSimpleVariants(t *testing.T) {
	input := `module test version "1.0.0";

enum Status {
	Pending,
	Running,
	Complete,
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}
	if len(prog.Enums) != 1 {
		t.Fatalf("expected 1 enum, got %d", len(prog.Enums))
	}

	enum := prog.Enums[0]
	if enum.Name != "Status" {
		t.Errorf("expected enum name 'Status', got %q", enum.Name)
	}
	if len(enum.Variants) != 3 {
		t.Fatalf("expected 3 variants, got %d", len(enum.Variants))
	}

	// Check that all variants are unit variants (no fields)
	for i, v := range enum.Variants {
		if len(v.Fields) != 0 {
			t.Errorf("variant %d should be unit variant, got %d fields", i, len(v.Fields))
		}
	}

	// Check variant names
	expectedNames := []string{"Pending", "Running", "Complete"}
	for i, expected := range expectedNames {
		if enum.Variants[i].Name != expected {
			t.Errorf("variant %d: expected name %q, got %q", i, expected, enum.Variants[i].Name)
		}
	}
}

func TestParseEnumDataCarryingVariants(t *testing.T) {
	input := `module test version "1.0.0";

enum Shape {
	Circle(radius: Float),
	Rectangle(width: Float, height: Float),
	Point,
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}
	if len(prog.Enums) != 1 {
		t.Fatalf("expected 1 enum, got %d", len(prog.Enums))
	}

	enum := prog.Enums[0]
	if enum.Name != "Shape" {
		t.Errorf("expected enum name 'Shape', got %q", enum.Name)
	}
	if len(enum.Variants) != 3 {
		t.Fatalf("expected 3 variants, got %d", len(enum.Variants))
	}

	// Check Circle variant
	circle := enum.Variants[0]
	if circle.Name != "Circle" {
		t.Errorf("expected variant name 'Circle', got %q", circle.Name)
	}
	if len(circle.Fields) != 1 {
		t.Fatalf("Circle: expected 1 field, got %d", len(circle.Fields))
	}
	if circle.Fields[0].Name != "radius" {
		t.Errorf("Circle field: expected name 'radius', got %q", circle.Fields[0].Name)
	}
	if circle.Fields[0].Type.Name != "Float" {
		t.Errorf("Circle field: expected type 'Float', got %q", circle.Fields[0].Type.Name)
	}

	// Check Rectangle variant
	rect := enum.Variants[1]
	if rect.Name != "Rectangle" {
		t.Errorf("expected variant name 'Rectangle', got %q", rect.Name)
	}
	if len(rect.Fields) != 2 {
		t.Fatalf("Rectangle: expected 2 fields, got %d", len(rect.Fields))
	}
	if rect.Fields[0].Name != "width" || rect.Fields[1].Name != "height" {
		t.Errorf("Rectangle fields: expected 'width' and 'height', got %q and %q",
			rect.Fields[0].Name, rect.Fields[1].Name)
	}

	// Check Point variant (unit variant)
	point := enum.Variants[2]
	if point.Name != "Point" {
		t.Errorf("expected variant name 'Point', got %q", point.Name)
	}
	if len(point.Fields) != 0 {
		t.Errorf("Point: expected 0 fields (unit variant), got %d", len(point.Fields))
	}
}

func TestParseEnumTrailingComma(t *testing.T) {
	input := `module test version "1.0.0";

enum Color {
	Red,
	Green,
	Blue,
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}
	if len(prog.Enums) != 1 {
		t.Fatalf("expected 1 enum, got %d", len(prog.Enums))
	}

	enum := prog.Enums[0]
	if len(enum.Variants) != 3 {
		t.Fatalf("expected 3 variants, got %d", len(enum.Variants))
	}
}

func TestParseEnumMixedVariants(t *testing.T) {
	input := `module test version "1.0.0";

enum Result {
	Ok(value: Int),
	Error(message: String, code: Int),
	Empty,
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}
	if len(prog.Enums) != 1 {
		t.Fatalf("expected 1 enum, got %d", len(prog.Enums))
	}

	enum := prog.Enums[0]
	if enum.Name != "Result" {
		t.Errorf("expected enum name 'Result', got %q", enum.Name)
	}
	if len(enum.Variants) != 3 {
		t.Fatalf("expected 3 variants, got %d", len(enum.Variants))
	}

	// Check Ok variant (1 field)
	if len(enum.Variants[0].Fields) != 1 {
		t.Errorf("Ok: expected 1 field, got %d", len(enum.Variants[0].Fields))
	}

	// Check Error variant (2 fields)
	if len(enum.Variants[1].Fields) != 2 {
		t.Errorf("Error: expected 2 fields, got %d", len(enum.Variants[1].Fields))
	}

	// Check Empty variant (0 fields)
	if len(enum.Variants[2].Fields) != 0 {
		t.Errorf("Empty: expected 0 fields, got %d", len(enum.Variants[2].Fields))
	}
}

func TestParseMatchSimple(t *testing.T) {
	input := `module test version "1.0.0";

function f(status: Status) returns Int {
    return match status { Running => 1, Pending => 0, Complete => 2 };
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}
	if len(prog.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(prog.Functions))
	}

	fn := prog.Functions[0]
	retStmt, ok := fn.Body.Statements[0].(*ast.ReturnStmt)
	if !ok {
		t.Fatal("expected return statement")
	}

	matchExpr, ok := retStmt.Value.(*ast.MatchExpr)
	if !ok {
		t.Fatal("expected match expression")
	}

	// Check scrutinee is an identifier
	scrutinee, ok := matchExpr.Scrutinee.(*ast.Identifier)
	if !ok || scrutinee.Name != "status" {
		t.Errorf("expected scrutinee to be 'status' identifier")
	}

	// Check 3 arms
	if len(matchExpr.Arms) != 3 {
		t.Fatalf("expected 3 arms, got %d", len(matchExpr.Arms))
	}

	// Check first arm: Running => 1
	if matchExpr.Arms[0].Pattern.VariantName != "Running" {
		t.Errorf("arm 0: expected variant 'Running', got %q", matchExpr.Arms[0].Pattern.VariantName)
	}
	if len(matchExpr.Arms[0].Pattern.Bindings) != 0 {
		t.Errorf("arm 0: expected 0 bindings, got %d", len(matchExpr.Arms[0].Pattern.Bindings))
	}

	// Check second arm: Pending => 0
	if matchExpr.Arms[1].Pattern.VariantName != "Pending" {
		t.Errorf("arm 1: expected variant 'Pending', got %q", matchExpr.Arms[1].Pattern.VariantName)
	}

	// Check third arm: Complete => 2
	if matchExpr.Arms[2].Pattern.VariantName != "Complete" {
		t.Errorf("arm 2: expected variant 'Complete', got %q", matchExpr.Arms[2].Pattern.VariantName)
	}
}

func TestParseMatchWithDestructuring(t *testing.T) {
	input := `module test version "1.0.0";

function f(shape: Shape) returns Float {
    return match shape { Circle(r) => r, Rectangle(w, h) => w, Point => 0.0 };
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}

	fn := prog.Functions[0]
	retStmt := fn.Body.Statements[0].(*ast.ReturnStmt)
	matchExpr := retStmt.Value.(*ast.MatchExpr)

	// Check 3 arms
	if len(matchExpr.Arms) != 3 {
		t.Fatalf("expected 3 arms, got %d", len(matchExpr.Arms))
	}

	// Check Circle(r) => r
	if matchExpr.Arms[0].Pattern.VariantName != "Circle" {
		t.Errorf("arm 0: expected variant 'Circle', got %q", matchExpr.Arms[0].Pattern.VariantName)
	}
	if len(matchExpr.Arms[0].Pattern.Bindings) != 1 {
		t.Fatalf("arm 0: expected 1 binding, got %d", len(matchExpr.Arms[0].Pattern.Bindings))
	}
	if matchExpr.Arms[0].Pattern.Bindings[0] != "r" {
		t.Errorf("arm 0: expected binding 'r', got %q", matchExpr.Arms[0].Pattern.Bindings[0])
	}

	// Check Rectangle(w, h) => w
	if matchExpr.Arms[1].Pattern.VariantName != "Rectangle" {
		t.Errorf("arm 1: expected variant 'Rectangle', got %q", matchExpr.Arms[1].Pattern.VariantName)
	}
	if len(matchExpr.Arms[1].Pattern.Bindings) != 2 {
		t.Fatalf("arm 1: expected 2 bindings, got %d", len(matchExpr.Arms[1].Pattern.Bindings))
	}
	if matchExpr.Arms[1].Pattern.Bindings[0] != "w" {
		t.Errorf("arm 1: expected first binding 'w', got %q", matchExpr.Arms[1].Pattern.Bindings[0])
	}
	if matchExpr.Arms[1].Pattern.Bindings[1] != "h" {
		t.Errorf("arm 1: expected second binding 'h', got %q", matchExpr.Arms[1].Pattern.Bindings[1])
	}

	// Check Point => 0.0 (unit variant, no bindings)
	if matchExpr.Arms[2].Pattern.VariantName != "Point" {
		t.Errorf("arm 2: expected variant 'Point', got %q", matchExpr.Arms[2].Pattern.VariantName)
	}
	if len(matchExpr.Arms[2].Pattern.Bindings) != 0 {
		t.Errorf("arm 2: expected 0 bindings, got %d", len(matchExpr.Arms[2].Pattern.Bindings))
	}
}

func TestParseMatchWithWildcard(t *testing.T) {
	input := `module test version "1.0.0";

function f(status: Status) returns Int {
    return match status { Running => 1, _ => 0 };
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}

	fn := prog.Functions[0]
	retStmt := fn.Body.Statements[0].(*ast.ReturnStmt)
	matchExpr := retStmt.Value.(*ast.MatchExpr)

	// Check 2 arms
	if len(matchExpr.Arms) != 2 {
		t.Fatalf("expected 2 arms, got %d", len(matchExpr.Arms))
	}

	// Check first arm: Running => 1
	if matchExpr.Arms[0].Pattern.IsWildcard {
		t.Error("arm 0: expected non-wildcard pattern")
	}
	if matchExpr.Arms[0].Pattern.VariantName != "Running" {
		t.Errorf("arm 0: expected variant 'Running', got %q", matchExpr.Arms[0].Pattern.VariantName)
	}

	// Check second arm: _ => 0
	if !matchExpr.Arms[1].Pattern.IsWildcard {
		t.Error("arm 1: expected wildcard pattern")
	}
}

func TestParseMatchAsExpression(t *testing.T) {
	input := `module test version "1.0.0";

function f(status: Status) returns Int {
    let x: Int = match status { Running => 1, Pending => 0, Complete => 2 };
    return x;
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}

	fn := prog.Functions[0]
	letStmt, ok := fn.Body.Statements[0].(*ast.LetStmt)
	if !ok {
		t.Fatal("expected let statement")
	}

	matchExpr, ok := letStmt.Value.(*ast.MatchExpr)
	if !ok {
		t.Fatal("expected match expression in let statement")
	}

	// Check it's a valid match expression
	if len(matchExpr.Arms) != 3 {
		t.Fatalf("expected 3 arms, got %d", len(matchExpr.Arms))
	}
}

func TestParseMatchNestedExprBody(t *testing.T) {
	input := `module test version "1.0.0";

function f(shape: Shape) returns Float {
    return match shape { Circle(r) => 3.14 * r * r, Rectangle(w, h) => w * h, Point => 0.0 };
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}

	fn := prog.Functions[0]
	retStmt := fn.Body.Statements[0].(*ast.ReturnStmt)
	matchExpr := retStmt.Value.(*ast.MatchExpr)

	// Check first arm has binary expression body: 3.14 * r * r
	binExpr, ok := matchExpr.Arms[0].Body.(*ast.BinaryExpr)
	if !ok {
		t.Fatal("arm 0: expected binary expression body")
	}
	if binExpr.Op != lexer.STAR {
		t.Errorf("arm 0: expected * operator in body")
	}
}

func TestParseTryExpr(t *testing.T) {
	input := `module test version "1.0.0";

function f() returns Result<Int, String> {
    return step1(a)?;
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}

	fn := prog.Functions[0]
	retStmt, ok := fn.Body.Statements[0].(*ast.ReturnStmt)
	if !ok {
		t.Fatal("expected return statement")
	}

	tryExpr, ok := retStmt.Value.(*ast.TryExpr)
	if !ok {
		t.Fatal("expected TryExpr")
	}

	callExpr, ok := tryExpr.Expr.(*ast.CallExpr)
	if !ok {
		t.Fatal("expected CallExpr inside TryExpr")
	}
	if callExpr.Function != "step1" {
		t.Errorf("expected function name 'step1', got %q", callExpr.Function)
	}
}

func TestParseTryExprInLet(t *testing.T) {
	input := `module test version "1.0.0";

function f() returns Result<Int, String> {
    let x: Int = divide(10, 2)?;
    return Ok(x);
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}

	fn := prog.Functions[0]
	letStmt, ok := fn.Body.Statements[0].(*ast.LetStmt)
	if !ok {
		t.Fatal("expected let statement")
	}

	tryExpr, ok := letStmt.Value.(*ast.TryExpr)
	if !ok {
		t.Fatal("expected TryExpr in let value")
	}

	callExpr, ok := tryExpr.Expr.(*ast.CallExpr)
	if !ok {
		t.Fatal("expected CallExpr inside TryExpr")
	}
	if callExpr.Function != "divide" {
		t.Errorf("expected function name 'divide', got %q", callExpr.Function)
	}
}

func TestParseTryExprChained(t *testing.T) {
	input := `module test version "1.0.0";

function f() returns Result<Int, String> {
    let a: Int = f1()?;
    let b: Int = f2(a)?;
    return Ok(b);
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}

	fn := prog.Functions[0]
	
	// Check first let statement
	letStmt1, ok := fn.Body.Statements[0].(*ast.LetStmt)
	if !ok {
		t.Fatal("expected first let statement")
	}
	tryExpr1, ok := letStmt1.Value.(*ast.TryExpr)
	if !ok {
		t.Fatal("expected TryExpr in first let")
	}
	callExpr1, ok := tryExpr1.Expr.(*ast.CallExpr)
	if !ok || callExpr1.Function != "f1" {
		t.Fatal("expected f1() call in first TryExpr")
	}

	// Check second let statement
	letStmt2, ok := fn.Body.Statements[1].(*ast.LetStmt)
	if !ok {
		t.Fatal("expected second let statement")
	}
	tryExpr2, ok := letStmt2.Value.(*ast.TryExpr)
	if !ok {
		t.Fatal("expected TryExpr in second let")
	}
	callExpr2, ok := tryExpr2.Expr.(*ast.CallExpr)
	if !ok || callExpr2.Function != "f2" {
		t.Fatal("expected f2() call in second TryExpr")
	}
}

func TestParseImportDeclaration(t *testing.T) {
	input := `module test version "1.0.0";

import "math.intent";

function main() returns Int {
    return 0;
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}

	if len(prog.Imports) != 1 {
		t.Fatalf("expected 1 import, got %d", len(prog.Imports))
	}

	imp := prog.Imports[0]
	if imp.Path != "math.intent" {
		t.Errorf("expected import path 'math.intent', got %q", imp.Path)
	}
}

func TestParseMultipleImports(t *testing.T) {
	input := `module test version "1.0.0";

import "math.intent";
import "utils.intent";
import "helpers/string.intent";

function main() returns Int {
    return 0;
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}

	if len(prog.Imports) != 3 {
		t.Fatalf("expected 3 imports, got %d", len(prog.Imports))
	}

	expectedPaths := []string{"math.intent", "utils.intent", "helpers/string.intent"}
	for i, exp := range expectedPaths {
		if prog.Imports[i].Path != exp {
			t.Errorf("import[%d] - expected path %q, got %q", i, exp, prog.Imports[i].Path)
		}
	}
}

func TestParsePublicFunction(t *testing.T) {
	input := `module test version "1.0.0";

public function add(x: Int, y: Int) returns Int {
    return x + y;
}

function helper() returns Int {
    return 42;
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}

	if len(prog.Functions) != 2 {
		t.Fatalf("expected 2 functions, got %d", len(prog.Functions))
	}

	// First function should be public
	if !prog.Functions[0].IsPublic {
		t.Error("expected first function to be public")
	}
	if prog.Functions[0].Name != "add" {
		t.Errorf("expected first function name 'add', got %q", prog.Functions[0].Name)
	}

	// Second function should not be public
	if prog.Functions[1].IsPublic {
		t.Error("expected second function to not be public")
	}
	if prog.Functions[1].Name != "helper" {
		t.Errorf("expected second function name 'helper', got %q", prog.Functions[1].Name)
	}
}

func TestParsePublicEntity(t *testing.T) {
	input := `module test version "1.0.0";

public entity Point {
    field x: Int;
    field y: Int;
}

entity PrivateHelper {
    field value: Int;
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}

	if len(prog.Entities) != 2 {
		t.Fatalf("expected 2 entities, got %d", len(prog.Entities))
	}

	// First entity should be public
	if !prog.Entities[0].IsPublic {
		t.Error("expected first entity to be public")
	}
	if prog.Entities[0].Name != "Point" {
		t.Errorf("expected first entity name 'Point', got %q", prog.Entities[0].Name)
	}

	// Second entity should not be public
	if prog.Entities[1].IsPublic {
		t.Error("expected second entity to not be public")
	}
}

func TestParsePublicEnum(t *testing.T) {
	input := `module test version "1.0.0";

public enum Status {
    Active,
    Inactive,
}

enum PrivateState {
    On,
    Off,
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}

	if len(prog.Enums) != 2 {
		t.Fatalf("expected 2 enums, got %d", len(prog.Enums))
	}

	// First enum should be public
	if !prog.Enums[0].IsPublic {
		t.Error("expected first enum to be public")
	}
	if prog.Enums[0].Name != "Status" {
		t.Errorf("expected first enum name 'Status', got %q", prog.Enums[0].Name)
	}

	// Second enum should not be public
	if prog.Enums[1].IsPublic {
		t.Error("expected second enum to not be public")
	}
}

func TestParsePublicWithInvalidDeclaration(t *testing.T) {
	input := `module test version "1.0.0";

public let x: Int = 5;`
	p := New(input)
	_ = p.Parse()

	if !p.Diagnostics().HasErrors() {
		t.Fatal("expected error for public before let statement")
	}
}

func TestParsePublicEntryFunction(t *testing.T) {
	input := `module test version "1.0.0";

public entry function main() returns Int {
    return 0;
}`
	p := New(input)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected errors: %s", p.Diagnostics().Format("test"))
	}

	if len(prog.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(prog.Functions))
	}

	fn := prog.Functions[0]
	if !fn.IsPublic {
		t.Error("expected function to be public")
	}
	if !fn.IsEntry {
		t.Error("expected function to be entry")
	}
	if fn.Name != "main" {
		t.Errorf("expected function name 'main', got %q", fn.Name)
	}
}
