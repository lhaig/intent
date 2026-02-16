package wasmbe

import (
	"testing"

	"github.com/lhaig/intent/internal/checker"
	"github.com/lhaig/intent/internal/ir"
	"github.com/lhaig/intent/internal/lexer"
)

func TestWasmMagicAndVersion(t *testing.T) {
	mod := &ir.Module{
		Name:    "test",
		IsEntry: true,
		Functions: []*ir.Function{
			{
				Name:       "__intent_main",
				IsEntry:    true,
				ReturnType: &checker.Type{Name: "Int"},
				Body: []ir.Stmt{
					&ir.ReturnStmt{
						Value: &ir.IntLit{Value: 42, Type: &checker.Type{Name: "Int"}},
					},
				},
			},
		},
	}

	result := Generate(mod)

	// Check WASM magic number: \0asm
	if len(result) < 8 {
		t.Fatalf("WASM output too short: %d bytes", len(result))
	}
	if result[0] != 0x00 || result[1] != 0x61 || result[2] != 0x73 || result[3] != 0x6D {
		t.Errorf("Expected WASM magic \\0asm, got %x %x %x %x", result[0], result[1], result[2], result[3])
	}

	// Check WASM version: 1
	if result[4] != 0x01 || result[5] != 0x00 || result[6] != 0x00 || result[7] != 0x00 {
		t.Errorf("Expected WASM version 1, got %x %x %x %x", result[4], result[5], result[6], result[7])
	}
}

func TestWasmSections(t *testing.T) {
	mod := &ir.Module{
		Name:    "test",
		IsEntry: true,
		Functions: []*ir.Function{
			{
				Name:       "__intent_main",
				IsEntry:    true,
				ReturnType: &checker.Type{Name: "Int"},
				Body: []ir.Stmt{
					&ir.ReturnStmt{
						Value: &ir.IntLit{Value: 0, Type: &checker.Type{Name: "Int"}},
					},
				},
			},
		},
	}

	result := Generate(mod)

	// Parse sections from the output
	sections := parseSections(result[8:])

	// Should have: type(1), function(3), memory(5), export(7), code(10)
	hasSectionType := false
	hasSectionFunc := false
	hasSectionMem := false
	hasSectionExport := false
	hasSectionCode := false

	for _, s := range sections {
		switch s.id {
		case 1:
			hasSectionType = true
		case 3:
			hasSectionFunc = true
		case 5:
			hasSectionMem = true
		case 7:
			hasSectionExport = true
		case 10:
			hasSectionCode = true
		}
	}

	if !hasSectionType {
		t.Error("Missing type section")
	}
	if !hasSectionFunc {
		t.Error("Missing function section")
	}
	if !hasSectionMem {
		t.Error("Missing memory section")
	}
	if !hasSectionExport {
		t.Error("Missing export section")
	}
	if !hasSectionCode {
		t.Error("Missing code section")
	}
}

func TestWasmFunctionWithParams(t *testing.T) {
	mod := &ir.Module{
		Name:    "test",
		IsEntry: false,
		Functions: []*ir.Function{
			{
				Name: "add",
				Params: []*ir.Param{
					{Name: "a", Type: &checker.Type{Name: "Int"}},
					{Name: "b", Type: &checker.Type{Name: "Int"}},
				},
				ReturnType: &checker.Type{Name: "Int"},
				Body: []ir.Stmt{
					&ir.ReturnStmt{
						Value: &ir.BinaryExpr{
							Left:  &ir.VarRef{Name: "a", Type: &checker.Type{Name: "Int"}},
							Op:    lexer.PLUS,
							Right: &ir.VarRef{Name: "b", Type: &checker.Type{Name: "Int"}},
							Type:  &checker.Type{Name: "Int"},
						},
					},
				},
			},
		},
	}

	result := Generate(mod)

	// Should produce valid WASM with proper magic
	if len(result) < 8 {
		t.Fatalf("WASM output too short: %d bytes", len(result))
	}
	if result[0] != 0x00 || result[1] != 0x61 || result[2] != 0x73 || result[3] != 0x6D {
		t.Error("Invalid WASM magic")
	}

	// Should have type section with (i64, i64) -> i64 signature
	sections := parseSections(result[8:])
	for _, s := range sections {
		if s.id == 1 { // type section
			// Should contain 0x60 (func type), 0x02 (2 params), 0x7E 0x7E (i64 i64), 0x01 (1 result), 0x7E (i64)
			found := false
			for i := 0; i < len(s.data)-5; i++ {
				if s.data[i] == 0x60 && s.data[i+1] == 0x02 && s.data[i+2] == 0x7E && s.data[i+3] == 0x7E && s.data[i+4] == 0x01 && s.data[i+5] == 0x7E {
					found = true
					break
				}
			}
			if !found {
				t.Error("Expected type section to contain (i64, i64) -> i64 signature")
			}
		}
	}
}

func TestWasmMultiModule(t *testing.T) {
	prog := &ir.Program{
		Modules: []*ir.Module{
			{
				Name:    "helper",
				IsEntry: false,
				Functions: []*ir.Function{
					{
						Name:       "square",
						ReturnType: &checker.Type{Name: "Int"},
						Params: []*ir.Param{
							{Name: "x", Type: &checker.Type{Name: "Int"}},
						},
						Body: []ir.Stmt{
							&ir.ReturnStmt{
								Value: &ir.BinaryExpr{
									Left:  &ir.VarRef{Name: "x", Type: &checker.Type{Name: "Int"}},
									Op:    lexer.STAR,
									Right: &ir.VarRef{Name: "x", Type: &checker.Type{Name: "Int"}},
									Type:  &checker.Type{Name: "Int"},
								},
							},
						},
					},
				},
			},
			{
				Name:    "main",
				IsEntry: true,
				Functions: []*ir.Function{
					{
						Name:       "__intent_main",
						IsEntry:    true,
						ReturnType: &checker.Type{Name: "Int"},
						Body: []ir.Stmt{
							&ir.ReturnStmt{
								Value: &ir.IntLit{Value: 0, Type: &checker.Type{Name: "Int"}},
							},
						},
					},
				},
			},
		},
	}

	result := GenerateAll(prog)

	if len(result) < 8 {
		t.Fatalf("WASM output too short: %d bytes", len(result))
	}

	// Check magic
	if result[0] != 0x00 || result[1] != 0x61 || result[2] != 0x73 || result[3] != 0x6D {
		t.Error("Invalid WASM magic")
	}

	// Should have 2 functions: helper_square and __intent_main
	sections := parseSections(result[8:])
	for _, s := range sections {
		if s.id == 3 { // function section
			// First byte is count
			if len(s.data) > 0 && s.data[0] != 2 {
				t.Errorf("Expected 2 functions in function section, got %d", s.data[0])
			}
		}
	}
}

func TestWasmExportNames(t *testing.T) {
	mod := &ir.Module{
		Name:    "test",
		IsEntry: false,
		Functions: []*ir.Function{
			{
				Name:       "hello",
				ReturnType: &checker.Type{Name: "Void"},
				Body:       []ir.Stmt{},
			},
		},
	}

	result := Generate(mod)
	sections := parseSections(result[8:])

	// Check export section contains "hello" and "memory"
	for _, s := range sections {
		if s.id == 7 { // export section
			data := string(s.data)
			if !containsBytes(s.data, []byte("hello")) {
				t.Errorf("Expected export 'hello' in export section, data: %q", data)
			}
			if !containsBytes(s.data, []byte("memory")) {
				t.Errorf("Expected export 'memory' in export section, data: %q", data)
			}
		}
	}
}

func TestWasmIfElse(t *testing.T) {
	mod := &ir.Module{
		Name:    "test",
		IsEntry: false,
		Functions: []*ir.Function{
			{
				Name: "abs",
				Params: []*ir.Param{
					{Name: "x", Type: &checker.Type{Name: "Int"}},
				},
				ReturnType: &checker.Type{Name: "Int"},
				Body: []ir.Stmt{
					&ir.IfStmt{
						Condition: &ir.BinaryExpr{
							Left:  &ir.VarRef{Name: "x", Type: &checker.Type{Name: "Int"}},
							Op:    lexer.LT,
							Right: &ir.IntLit{Value: 0, Type: &checker.Type{Name: "Int"}},
							Type:  &checker.Type{Name: "Bool"},
						},
						Then: []ir.Stmt{
							&ir.ReturnStmt{
								Value: &ir.BinaryExpr{
									Left:  &ir.IntLit{Value: 0, Type: &checker.Type{Name: "Int"}},
									Op:    lexer.MINUS,
									Right: &ir.VarRef{Name: "x", Type: &checker.Type{Name: "Int"}},
									Type:  &checker.Type{Name: "Int"},
								},
							},
						},
						Else: []ir.Stmt{
							&ir.ReturnStmt{
								Value: &ir.VarRef{Name: "x", Type: &checker.Type{Name: "Int"}},
							},
						},
					},
				},
			},
		},
	}

	result := Generate(mod)

	// Should produce valid WASM
	if len(result) < 8 {
		t.Fatalf("WASM output too short: %d bytes", len(result))
	}
	if result[0] != 0x00 || result[1] != 0x61 || result[2] != 0x73 || result[3] != 0x6D {
		t.Error("Invalid WASM magic")
	}

	// Check code section contains if/else opcodes
	sections := parseSections(result[8:])
	for _, s := range sections {
		if s.id == 10 { // code section
			if !containsByte(s.data, opIf) {
				t.Error("Expected if opcode in code section")
			}
			if !containsByte(s.data, opElse) {
				t.Error("Expected else opcode in code section")
			}
		}
	}
}

func TestWasmWhileLoop(t *testing.T) {
	mod := &ir.Module{
		Name:    "test",
		IsEntry: false,
		Functions: []*ir.Function{
			{
				Name: "countdown",
				Params: []*ir.Param{
					{Name: "n", Type: &checker.Type{Name: "Int"}},
				},
				ReturnType: &checker.Type{Name: "Int"},
				Body: []ir.Stmt{
					&ir.LetStmt{
						Name:    "i",
						Mutable: true,
						Type:    &checker.Type{Name: "Int"},
						Value:   &ir.VarRef{Name: "n", Type: &checker.Type{Name: "Int"}},
					},
					&ir.WhileStmt{
						Condition: &ir.BinaryExpr{
							Left:  &ir.VarRef{Name: "i", Type: &checker.Type{Name: "Int"}},
							Op:    lexer.GT,
							Right: &ir.IntLit{Value: 0, Type: &checker.Type{Name: "Int"}},
							Type:  &checker.Type{Name: "Bool"},
						},
						Body: []ir.Stmt{
							&ir.AssignStmt{
								Target: &ir.VarRef{Name: "i", Type: &checker.Type{Name: "Int"}},
								Value: &ir.BinaryExpr{
									Left:  &ir.VarRef{Name: "i", Type: &checker.Type{Name: "Int"}},
									Op:    lexer.MINUS,
									Right: &ir.IntLit{Value: 1, Type: &checker.Type{Name: "Int"}},
									Type:  &checker.Type{Name: "Int"},
								},
							},
						},
					},
					&ir.ReturnStmt{
						Value: &ir.VarRef{Name: "i", Type: &checker.Type{Name: "Int"}},
					},
				},
			},
		},
	}

	result := Generate(mod)

	if len(result) < 8 {
		t.Fatalf("WASM output too short: %d bytes", len(result))
	}

	// Check code section contains block/loop opcodes
	sections := parseSections(result[8:])
	for _, s := range sections {
		if s.id == 10 { // code section
			if !containsByte(s.data, opBlock) {
				t.Error("Expected block opcode in code section")
			}
			if !containsByte(s.data, opLoop) {
				t.Error("Expected loop opcode in code section")
			}
		}
	}
}

func TestWasmBooleans(t *testing.T) {
	mod := &ir.Module{
		Name:    "test",
		IsEntry: false,
		Functions: []*ir.Function{
			{
				Name:       "isTrue",
				ReturnType: &checker.Type{Name: "Bool"},
				Body: []ir.Stmt{
					&ir.ReturnStmt{
						Value: &ir.BoolLit{Value: true, Type: &checker.Type{Name: "Bool"}},
					},
				},
			},
		},
	}

	result := Generate(mod)

	if len(result) < 8 {
		t.Fatalf("WASM output too short: %d bytes", len(result))
	}

	// Check code section contains i32.const 1
	sections := parseSections(result[8:])
	for _, s := range sections {
		if s.id == 10 { // code section
			if !containsByte(s.data, opI32Const) {
				t.Error("Expected i32.const opcode for boolean")
			}
		}
	}
}

func TestLEB128Encoding(t *testing.T) {
	// Test unsigned LEB128
	tests := []struct {
		value    uint64
		expected []byte
	}{
		{0, []byte{0}},
		{1, []byte{1}},
		{127, []byte{0x7F}},
		{128, []byte{0x80, 0x01}},
		{624485, []byte{0xE5, 0x8E, 0x26}},
	}

	for _, tt := range tests {
		result := encodeLEB128U(tt.value)
		if len(result) != len(tt.expected) {
			t.Errorf("encodeLEB128U(%d): expected %v, got %v", tt.value, tt.expected, result)
			continue
		}
		for i := range result {
			if result[i] != tt.expected[i] {
				t.Errorf("encodeLEB128U(%d): expected %v, got %v", tt.value, tt.expected, result)
				break
			}
		}
	}

	// Test signed LEB128
	stests := []struct {
		value    int64
		expected []byte
	}{
		{0, []byte{0}},
		{1, []byte{1}},
		{-1, []byte{0x7F}},
		{63, []byte{0x3F}},
		{-64, []byte{0x40}},
		{-128, []byte{0x80, 0x7F}},
	}

	for _, tt := range stests {
		result := encodeLEB128S(tt.value)
		if len(result) != len(tt.expected) {
			t.Errorf("encodeLEB128S(%d): expected %v, got %v", tt.value, tt.expected, result)
			continue
		}
		for i := range result {
			if result[i] != tt.expected[i] {
				t.Errorf("encodeLEB128S(%d): expected %v, got %v", tt.value, tt.expected, result)
				break
			}
		}
	}
}

// --- Helpers ---

type section struct {
	id   byte
	data []byte
}

func parseSections(data []byte) []section {
	var sections []section
	i := 0
	for i < len(data) {
		if i >= len(data) {
			break
		}
		id := data[i]
		i++
		size, n := decodeLEB128U(data[i:])
		i += n
		if i+int(size) > len(data) {
			break
		}
		sections = append(sections, section{id: id, data: data[i : i+int(size)]})
		i += int(size)
	}
	return sections
}

func decodeLEB128U(data []byte) (uint64, int) {
	var result uint64
	var shift uint
	for i := 0; i < len(data); i++ {
		b := data[i]
		result |= uint64(b&0x7F) << shift
		shift += 7
		if b&0x80 == 0 {
			return result, i + 1
		}
	}
	return result, len(data)
}

func containsByte(data []byte, b byte) bool {
	for _, d := range data {
		if d == b {
			return true
		}
	}
	return false
}

func containsBytes(data, sub []byte) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i <= len(data)-len(sub); i++ {
		match := true
		for j := range sub {
			if data[i+j] != sub[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
