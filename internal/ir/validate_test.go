package ir

import (
	"testing"

	"github.com/lhaig/intent/internal/checker"
	"github.com/lhaig/intent/internal/lexer"
)

func TestValidateValidModule(t *testing.T) {
	src := `module test version "1.0";
entry function main() returns Int {
    let x: Int = 42;
    return x;
}
`
	mod := parseAndLower(t, src)
	errors := Validate(mod)
	if len(errors) > 0 {
		t.Errorf("expected no errors, got: %v", errors)
	}
}

func TestValidateMissingReturnType(t *testing.T) {
	// Create a module with a function that has nil ReturnType
	mod := &Module{
		Name:    "test",
		IsEntry: true,
		Functions: []*Function{
			{
				Name:       "main",
				IsEntry:    true,
				ReturnType: nil, // Invalid
				Body:       []Stmt{},
			},
		},
	}

	errors := Validate(mod)
	if len(errors) == 0 {
		t.Error("expected validation error for nil ReturnType")
	}
	found := false
	for _, err := range errors {
		if err == "function main has nil ReturnType" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error about nil ReturnType, got: %v", errors)
	}
}

func TestValidateNilContractExpr(t *testing.T) {
	// Create a module with a function that has a contract with nil Expr
	mod := &Module{
		Name:    "test",
		IsEntry: true,
		Functions: []*Function{
			{
				Name:       "main",
				IsEntry:    true,
				ReturnType: checker.TypeInt,
				Requires: []*Contract{
					{
						Expr:    nil, // Invalid
						RawText: "x > 0",
					},
				},
				Body: []Stmt{},
			},
		},
	}

	errors := Validate(mod)
	if len(errors) == 0 {
		t.Error("expected validation error for nil contract Expr")
	}
	found := false
	for _, err := range errors {
		if err == "function main requires: contract 0 has nil Expr" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error about nil contract Expr, got: %v", errors)
	}
}

func TestValidateCallVariantMissingEnum(t *testing.T) {
	// Create a module with a CallExpr that has CallVariant but empty EnumName
	mod := &Module{
		Name:    "test",
		IsEntry: true,
		Functions: []*Function{
			{
				Name:       "main",
				IsEntry:    true,
				ReturnType: checker.TypeInt,
				Body: []Stmt{
					&ExprStmt{
						Expr: &CallExpr{
							Function: "Red",
							Kind:     CallVariant,
							EnumName: "", // Invalid
							Type:     checker.TypeInt,
						},
					},
					&ReturnStmt{
						Value: &IntLit{Value: 0, Type: checker.TypeInt},
					},
				},
			},
		},
	}

	errors := Validate(mod)
	if len(errors) == 0 {
		t.Error("expected validation error for CallVariant with empty EnumName")
	}
	found := false
	for _, err := range errors {
		if err == "function main statement 0: CallExpr with CallVariant has empty EnumName" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error about CallVariant EnumName, got: %v", errors)
	}
}

func TestValidateDuplicateOldCapture(t *testing.T) {
	// Create a module with duplicate OldCapture names
	mod := &Module{
		Name:    "test",
		IsEntry: false,
		Entities: []*Entity{
			{
				Name: "Counter",
				Fields: []*Field{
					{Name: "count", Type: checker.TypeInt},
				},
				Methods: []*Method{
					{
						Name:       "increment",
						ReturnType: checker.TypeVoid,
						OldCaptures: []*OldCapture{
							{
								Name: "__old_count",
								Expr: &IntLit{Value: 0, Type: checker.TypeInt},
							},
							{
								Name: "__old_count", // Duplicate
								Expr: &IntLit{Value: 1, Type: checker.TypeInt},
							},
						},
						Body: []Stmt{},
					},
				},
			},
		},
		Functions: []*Function{
			{
				Name:       "main",
				IsEntry:    true,
				ReturnType: checker.TypeInt,
				Body: []Stmt{
					&ReturnStmt{
						Value: &IntLit{Value: 0, Type: checker.TypeInt},
					},
				},
			},
		},
	}
	mod.IsEntry = true

	errors := Validate(mod)
	if len(errors) == 0 {
		t.Error("expected validation error for duplicate OldCapture")
	}
	found := false
	for _, err := range errors {
		if err == `entity Counter method increment: duplicate OldCapture name "__old_count"` {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error about duplicate OldCapture, got: %v", errors)
	}
}

func TestValidateLetStmtNilType(t *testing.T) {
	// Create a module with a LetStmt that has nil Type
	mod := &Module{
		Name:    "test",
		IsEntry: true,
		Functions: []*Function{
			{
				Name:       "main",
				IsEntry:    true,
				ReturnType: checker.TypeInt,
				Body: []Stmt{
					&LetStmt{
						Name:  "x",
						Type:  nil, // Invalid
						Value: &IntLit{Value: 42, Type: checker.TypeInt},
					},
					&ReturnStmt{
						Value: &IntLit{Value: 0, Type: checker.TypeInt},
					},
				},
			},
		},
	}

	errors := Validate(mod)
	if len(errors) == 0 {
		t.Error("expected validation error for LetStmt with nil Type")
	}
	found := false
	for _, err := range errors {
		if err == "function main statement 0: LetStmt has nil Type" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error about LetStmt nil Type, got: %v", errors)
	}
}

func TestValidateNilExprInStmt(t *testing.T) {
	// Create a module with nil expressions in various statements
	mod := &Module{
		Name:    "test",
		IsEntry: true,
		Functions: []*Function{
			{
				Name:       "main",
				IsEntry:    true,
				ReturnType: checker.TypeInt,
				Body: []Stmt{
					&ExprStmt{
						Expr: nil, // Invalid
					},
					&ReturnStmt{
						Value: &IntLit{Value: 0, Type: checker.TypeInt},
					},
				},
			},
		},
	}

	errors := Validate(mod)
	if len(errors) == 0 {
		t.Error("expected validation error for nil Expr in ExprStmt")
	}
	found := false
	for _, err := range errors {
		if err == "function main statement 0: ExprStmt has nil Expr" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error about ExprStmt nil Expr, got: %v", errors)
	}
}

func TestValidateVoidReturn(t *testing.T) {
	// Test that ReturnStmt with nil Value is allowed for Void returns
	mod := &Module{
		Name:    "test",
		IsEntry: false,
		Entities: []*Entity{
			{
				Name: "Test",
				Methods: []*Method{
					{
						Name:       "doNothing",
						ReturnType: checker.TypeVoid,
						Body: []Stmt{
							&ReturnStmt{
								Value: nil, // Valid for Void return
							},
						},
					},
				},
			},
		},
		Functions: []*Function{
			{
				Name:       "main",
				IsEntry:    true,
				ReturnType: checker.TypeInt,
				Body: []Stmt{
					&ReturnStmt{
						Value: &IntLit{Value: 0, Type: checker.TypeInt},
					},
				},
			},
		},
	}
	mod.IsEntry = true

	errors := Validate(mod)
	if len(errors) > 0 {
		t.Errorf("expected no errors for valid Void return, got: %v", errors)
	}
}

func TestValidateBinaryExprNilOperands(t *testing.T) {
	// Test BinaryExpr with nil operands
	mod := &Module{
		Name:    "test",
		IsEntry: true,
		Functions: []*Function{
			{
				Name:       "main",
				IsEntry:    true,
				ReturnType: checker.TypeInt,
				Body: []Stmt{
					&ReturnStmt{
						Value: &BinaryExpr{
							Left:  nil, // Invalid
							Op:    lexer.PLUS,
							Right: &IntLit{Value: 1, Type: checker.TypeInt},
							Type:  checker.TypeInt,
						},
					},
				},
			},
		},
	}

	errors := Validate(mod)
	if len(errors) == 0 {
		t.Error("expected validation error for BinaryExpr with nil Left")
	}
	found := false
	for _, err := range errors {
		if err == "function main statement 0: BinaryExpr has nil Left" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error about BinaryExpr nil Left, got: %v", errors)
	}
}

func TestValidateComplexEntity(t *testing.T) {
	src := `module test version "1.0";
entity Counter {
    field count: Int;

    invariant self.count >= 0;

    constructor(initial: Int)
        requires initial >= 0
        ensures self.count == initial
    {
        self.count = initial;
    }

    method increment() returns Void
        ensures self.count == old(self.count) + 1
    {
        self.count = self.count + 1;
    }
}
entry function main() returns Int {
    return 0;
}
`
	mod := parseAndLower(t, src)
	errors := Validate(mod)
	if len(errors) > 0 {
		t.Errorf("expected no errors for valid entity, got: %v", errors)
	}
}

func TestValidateEntryModuleWithoutMain(t *testing.T) {
	// Create an entry module without a main function
	mod := &Module{
		Name:      "test",
		IsEntry:   true,
		Functions: []*Function{},
	}

	errors := Validate(mod)
	if len(errors) == 0 {
		t.Error("expected validation error for entry module without main")
	}
	found := false
	for _, err := range errors {
		if err == "entry module must have a function named 'main'" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error about missing main function, got: %v", errors)
	}
}
