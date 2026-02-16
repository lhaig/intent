package testgen

import (
	"fmt"
	"strings"

	"github.com/lhaig/intent/internal/ast"
	"github.com/lhaig/intent/internal/codegen"
)

// Generate produces a #[cfg(test)] module with property-based contract tests
// for all functions and entities in the program.
func Generate(prog *ast.Program) string {
	var sb strings.Builder

	// Build lookup maps for codegen helpers
	entities := make(map[string]*ast.EntityDecl)
	enums := make(map[string]*ast.EnumDecl)
	functions := make(map[string]*ast.FunctionDecl)
	for _, e := range prog.Entities {
		entities[e.Name] = e
	}
	for _, e := range prog.Enums {
		enums[e.Name] = e
	}
	for _, f := range prog.Functions {
		functions[f.Name] = f
	}

	// Collect test functions to emit
	var testFns []string

	// Generate tests for standalone functions
	for _, f := range prog.Functions {
		if f.IsEntry {
			continue
		}
		if len(f.Requires) == 0 && len(f.Ensures) == 0 {
			continue
		}
		testFns = append(testFns, generateFunctionTest(f, entities, enums, functions))
	}

	// Generate tests for entities
	for _, e := range prog.Entities {
		testFns = append(testFns, generateEntityTests(e, entities, enums, functions)...)
	}

	if len(testFns) == 0 {
		return ""
	}

	// Emit module header
	sb.WriteString("\n#[cfg(test)]\nmod __contract_tests {\n    use super::*;\n\n")

	// Emit PRNG helpers
	sb.WriteString(prngHelpers)
	sb.WriteString("\n")

	// Emit all test functions
	for _, fn := range testFns {
		sb.WriteString(fn)
		sb.WriteString("\n")
	}

	sb.WriteString("}\n")
	return sb.String()
}

const prngHelpers = `    fn __xorshift64(state: &mut u64) -> u64 {
        let mut x = *state;
        x ^= x << 13;
        x ^= x >> 7;
        x ^= x << 17;
        *state = x;
        x
    }

    fn __rand_range(state: &mut u64, lo: i64, hi: i64) -> i64 {
        if lo >= hi { return lo; }
        let range_size = (hi - lo) as u64 + 1;
        lo + (__xorshift64(state) % range_size) as i64
    }
`

// generateFunctionTest generates a #[test] function for a standalone function with contracts.
func generateFunctionTest(f *ast.FunctionDecl, entities map[string]*ast.EntityDecl, enums map[string]*ast.EnumDecl, functions map[string]*ast.FunctionDecl) string {
	var sb strings.Builder

	constraints := AnalyzeConstraints(f.Params, f.Requires)

	sb.WriteString(fmt.Sprintf("    #[test]\n    fn test_%s_contracts() {\n", f.Name))

	// Generate value lists for each parameter
	paramValueVars := make(map[string]string) // param name -> variable name holding values
	for _, p := range f.Params {
		c := constraints[p.Name]
		varName := fmt.Sprintf("__%s_values", p.Name)
		paramValueVars[p.Name] = varName

		values := generateValuesForParam(c)
		sb.WriteString(fmt.Sprintf("        let %s: Vec<%s> = vec![%s];\n",
			varName, rustTypeForParam(p), strings.Join(values, ", ")))
	}

	if len(f.Params) == 0 {
		// No params -- just run the function once and check postconditions
		sb.WriteString("        {\n")
		if len(f.Ensures) > 0 && f.ReturnType.Name != "Void" {
			sb.WriteString(fmt.Sprintf("            let __result = %s();\n", f.Name))
			for _, ens := range f.Ensures {
				rustExpr := codegen.ExprToRust(ens.Expr, "", "__result", true, entities, enums, functions)
				sb.WriteString(fmt.Sprintf("            assert!(%s, \"Postcondition '%s' failed\");\n",
					rustExpr, codegen.EscapeRustString(ens.RawText)))
			}
		} else {
			sb.WriteString(fmt.Sprintf("            %s();\n", f.Name))
		}
		sb.WriteString("        }\n")
	} else {
		// Generate iteration over values (zipped, not cartesian)
		sb.WriteString(fmt.Sprintf("        let __max_len = %s;\n", maxLenExpr(f.Params, paramValueVars)))
		sb.WriteString("        for __idx in 0..__max_len {\n")

		// Extract parameter values (clone non-Copy types)
		for _, p := range f.Params {
			varName := paramValueVars[p.Name]
			if needsClone(p.Type) {
				sb.WriteString(fmt.Sprintf("            let %s = %s[__idx %% %s.len()].clone();\n",
					p.Name, varName, varName))
			} else {
				sb.WriteString(fmt.Sprintf("            let %s = %s[__idx %% %s.len()];\n",
					p.Name, varName, varName))
			}
		}

		// Filter by preconditions
		for _, req := range f.Requires {
			rustExpr := codegen.ExprToRust(req.Expr, "", "", false, entities, enums, functions)
			sb.WriteString(fmt.Sprintf("            if !(%s) { continue; }\n", rustExpr))
		}

		// Call the function
		hasReturn := f.ReturnType != nil && f.ReturnType.Name != "Void"
		args := buildFunctionArgs(f)
		if hasReturn && len(f.Ensures) > 0 {
			sb.WriteString(fmt.Sprintf("            let __result = %s(%s);\n", f.Name, args))
			// Check postconditions
			for _, ens := range f.Ensures {
				rustExpr := codegen.ExprToRust(ens.Expr, "", "__result", true, entities, enums, functions)
				sb.WriteString(fmt.Sprintf("            assert!(%s,\n                \"Postcondition '%s' failed for %s\",\n                %s);\n",
					rustExpr,
					codegen.EscapeRustString(ens.RawText),
					paramFormatStr(f.Params),
					paramFormatArgs(f.Params)))
			}
		} else {
			sb.WriteString(fmt.Sprintf("            %s(%s);\n", f.Name, args))
		}

		sb.WriteString("        }\n")
	}

	sb.WriteString("    }\n")
	return sb.String()
}

// generateEntityTests generates tests for entity constructors, methods, and workflow.
func generateEntityTests(e *ast.EntityDecl, entities map[string]*ast.EntityDecl, enums map[string]*ast.EnumDecl, functions map[string]*ast.FunctionDecl) []string {
	var tests []string

	hasInvariants := len(e.Invariants) > 0

	// Constructor test
	if e.Constructor != nil {
		t := generateConstructorTest(e, entities, enums, functions)
		if t != "" {
			tests = append(tests, t)
		}
	}

	// Method tests
	for _, m := range e.Methods {
		if len(m.Requires) == 0 && len(m.Ensures) == 0 && !hasInvariants {
			continue
		}
		t := generateMethodTest(e, m, entities, enums, functions)
		if t != "" {
			tests = append(tests, t)
		}
	}

	// Workflow test (construct + call each method once with valid args)
	if e.Constructor != nil && len(e.Methods) > 0 {
		t := generateWorkflowTest(e, entities, enums, functions)
		if t != "" {
			tests = append(tests, t)
		}
	}

	return tests
}

// generateConstructorTest generates a test for an entity constructor.
func generateConstructorTest(e *ast.EntityDecl, entities map[string]*ast.EntityDecl, enums map[string]*ast.EnumDecl, functions map[string]*ast.FunctionDecl) string {
	ctor := e.Constructor
	constraints := AnalyzeConstraints(ctor.Params, ctor.Requires)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("    #[test]\n    fn test_%s_constructor_contracts() {\n",
		strings.ToLower(e.Name)))

	// Generate value lists for constructor params
	paramValueVars := make(map[string]string)
	for _, p := range ctor.Params {
		c := constraints[p.Name]
		varName := fmt.Sprintf("__%s_values", p.Name)
		paramValueVars[p.Name] = varName

		values := generateValuesForParam(c)
		sb.WriteString(fmt.Sprintf("        let %s: Vec<%s> = vec![%s];\n",
			varName, rustTypeForParam(p), strings.Join(values, ", ")))
	}

	if len(ctor.Params) > 0 {
		sb.WriteString(fmt.Sprintf("        let __max_len = %s;\n", maxLenExpr(ctor.Params, paramValueVars)))
		sb.WriteString("        for __idx in 0..__max_len {\n")

		for _, p := range ctor.Params {
			varName := paramValueVars[p.Name]
			if needsClone(p.Type) {
				sb.WriteString(fmt.Sprintf("            let %s = %s[__idx %% %s.len()].clone();\n",
					p.Name, varName, varName))
			} else {
				sb.WriteString(fmt.Sprintf("            let %s = %s[__idx %% %s.len()];\n",
					p.Name, varName, varName))
			}
		}

		// Filter by preconditions
		for _, req := range ctor.Requires {
			rustExpr := codegen.ExprToRust(req.Expr, "", "", false, entities, enums, functions)
			sb.WriteString(fmt.Sprintf("            if !(%s) { continue; }\n", rustExpr))
		}

		// Construct entity
		args := buildConstructorArgs(ctor)
		sb.WriteString(fmt.Sprintf("            let __entity = %s::new(%s);\n", e.Name, args))

		// Check ensures (self -> __entity)
		for _, ens := range ctor.Ensures {
			rustExpr := codegen.ExprToRust(ens.Expr, "__entity", "", true, entities, enums, functions)
			sb.WriteString(fmt.Sprintf("            assert!(%s, \"Constructor postcondition '%s' failed\");\n",
				rustExpr, codegen.EscapeRustString(ens.RawText)))
		}

		// Check invariants
		for _, inv := range e.Invariants {
			rustExpr := codegen.ExprToRust(inv.Expr, "__entity", "", false, entities, enums, functions)
			sb.WriteString(fmt.Sprintf("            assert!(%s, \"Invariant '%s' failed after construction\");\n",
				rustExpr, codegen.EscapeRustString(inv.RawText)))
		}

		sb.WriteString("        }\n")
	} else {
		// No params, just test once
		sb.WriteString(fmt.Sprintf("        let __entity = %s::new();\n", e.Name))
		for _, ens := range ctor.Ensures {
			rustExpr := codegen.ExprToRust(ens.Expr, "__entity", "", true, entities, enums, functions)
			sb.WriteString(fmt.Sprintf("        assert!(%s, \"Constructor postcondition '%s' failed\");\n",
				rustExpr, codegen.EscapeRustString(ens.RawText)))
		}
		for _, inv := range e.Invariants {
			rustExpr := codegen.ExprToRust(inv.Expr, "__entity", "", false, entities, enums, functions)
			sb.WriteString(fmt.Sprintf("        assert!(%s, \"Invariant '%s' failed after construction\");\n",
				rustExpr, codegen.EscapeRustString(inv.RawText)))
		}
	}

	sb.WriteString("    }\n")
	return sb.String()
}

// generateMethodTest generates a test for an entity method.
func generateMethodTest(e *ast.EntityDecl, m *ast.MethodDecl, entities map[string]*ast.EntityDecl, enums map[string]*ast.EnumDecl, functions map[string]*ast.FunctionDecl) string {
	if e.Constructor == nil {
		return "" // Can't test without a constructor
	}

	constraints := AnalyzeConstraints(m.Params, m.Requires)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("    #[test]\n    fn test_%s_%s_contracts() {\n",
		strings.ToLower(e.Name), m.Name))

	// Construct a valid base entity using first valid constructor args
	ctorArgs := buildDefaultConstructorArgs(e.Constructor)
	sb.WriteString(fmt.Sprintf("        let mut __entity = %s::new(%s);\n", e.Name, ctorArgs))

	// Generate value lists for method params
	paramValueVars := make(map[string]string)
	for _, p := range m.Params {
		c := constraints[p.Name]
		varName := fmt.Sprintf("__%s_values", p.Name)
		paramValueVars[p.Name] = varName

		values := generateValuesForParam(c)
		sb.WriteString(fmt.Sprintf("        let %s: Vec<%s> = vec![%s];\n",
			varName, rustTypeForParam(p), strings.Join(values, ", ")))
	}

	if len(m.Params) > 0 {
		sb.WriteString(fmt.Sprintf("        let __max_len = %s;\n", maxLenExpr(m.Params, paramValueVars)))
		sb.WriteString("        for __idx in 0..__max_len {\n")

		for _, p := range m.Params {
			varName := paramValueVars[p.Name]
			if needsClone(p.Type) {
				sb.WriteString(fmt.Sprintf("            let %s = %s[__idx %% %s.len()].clone();\n",
					p.Name, varName, varName))
			} else {
				sb.WriteString(fmt.Sprintf("            let %s = %s[__idx %% %s.len()];\n",
					p.Name, varName, varName))
			}
		}

		// Filter by preconditions
		for _, req := range m.Requires {
			rustExpr := codegen.ExprToRust(req.Expr, "__entity", "", false, entities, enums, functions)
			sb.WriteString(fmt.Sprintf("            if !(%s) { continue; }\n", rustExpr))
		}

		// Capture old values for ensures clauses
		oldCaptures := collectOldExprs(m.Ensures, entities, enums, functions)
		for mangledName, rustExpr := range oldCaptures {
			sb.WriteString(fmt.Sprintf("            let %s = %s;\n", mangledName, rustExpr))
		}

		// Call method
		hasReturn := m.ReturnType != nil && m.ReturnType.Name != "Void"
		methodArgs := buildMethodArgs(m)
		if hasReturn {
			sb.WriteString(fmt.Sprintf("            let __result = __entity.%s(%s);\n", m.Name, methodArgs))
		} else {
			sb.WriteString(fmt.Sprintf("            __entity.%s(%s);\n", m.Name, methodArgs))
		}

		// Check ensures
		for _, ens := range m.Ensures {
			rustExpr := codegen.ExprToRust(ens.Expr, "__entity", "__result", true, entities, enums, functions)
			// Replace old() captures with our pre-captured values
			for mangledName := range oldCaptures {
				// The ExprToRust function should handle old() expressions via ensuresCtx
				_ = mangledName
			}
			sb.WriteString(fmt.Sprintf("            assert!(%s, \"Postcondition '%s' failed\");\n",
				rustExpr, codegen.EscapeRustString(ens.RawText)))
		}

		// Check invariants
		for _, inv := range e.Invariants {
			rustExpr := codegen.ExprToRust(inv.Expr, "__entity", "", false, entities, enums, functions)
			sb.WriteString(fmt.Sprintf("            assert!(%s, \"Invariant '%s' failed after %s\");\n",
				rustExpr, codegen.EscapeRustString(inv.RawText), m.Name))
		}

		// Reset entity for next iteration (re-construct)
		sb.WriteString(fmt.Sprintf("            __entity = %s::new(%s);\n", e.Name, ctorArgs))

		sb.WriteString("        }\n")
	} else {
		// No method params - call once
		oldCaptures := collectOldExprs(m.Ensures, entities, enums, functions)
		for mangledName, rustExpr := range oldCaptures {
			sb.WriteString(fmt.Sprintf("        let %s = %s;\n", mangledName, rustExpr))
		}

		hasReturn := m.ReturnType != nil && m.ReturnType.Name != "Void"
		if hasReturn {
			sb.WriteString(fmt.Sprintf("        let __result = __entity.%s();\n", m.Name))
		} else {
			sb.WriteString(fmt.Sprintf("        __entity.%s();\n", m.Name))
		}
		for _, ens := range m.Ensures {
			rustExpr := codegen.ExprToRust(ens.Expr, "__entity", "__result", true, entities, enums, functions)
			sb.WriteString(fmt.Sprintf("        assert!(%s, \"Postcondition '%s' failed\");\n",
				rustExpr, codegen.EscapeRustString(ens.RawText)))
		}
		for _, inv := range e.Invariants {
			rustExpr := codegen.ExprToRust(inv.Expr, "__entity", "", false, entities, enums, functions)
			sb.WriteString(fmt.Sprintf("        assert!(%s, \"Invariant '%s' failed after %s\");\n",
				rustExpr, codegen.EscapeRustString(inv.RawText), m.Name))
		}
	}

	sb.WriteString("    }\n")
	return sb.String()
}

// generateWorkflowTest constructs an entity, calls each method once with valid args, and checks invariants.
func generateWorkflowTest(e *ast.EntityDecl, entities map[string]*ast.EntityDecl, enums map[string]*ast.EnumDecl, functions map[string]*ast.FunctionDecl) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("    #[test]\n    fn test_%s_workflow() {\n",
		strings.ToLower(e.Name)))

	// Construct entity
	ctorArgs := buildDefaultConstructorArgs(e.Constructor)
	sb.WriteString(fmt.Sprintf("        let mut __entity = %s::new(%s);\n", e.Name, ctorArgs))

	// Check invariants after construction
	for _, inv := range e.Invariants {
		rustExpr := codegen.ExprToRust(inv.Expr, "__entity", "", false, entities, enums, functions)
		sb.WriteString(fmt.Sprintf("        assert!(%s, \"Invariant '%s' failed after construction\");\n",
			rustExpr, codegen.EscapeRustString(inv.RawText)))
	}

	// Call each method once with default valid args
	for _, m := range e.Methods {
		methodConstraints := AnalyzeConstraints(m.Params, m.Requires)

		// Declare parameter variables with default values so precondition guards can reference them
		for _, p := range m.Params {
			c := methodConstraints[p.Name]
			sb.WriteString(fmt.Sprintf("        let %s = %s;\n", p.Name, defaultValueForType(c)))
		}

		methodArgs := buildWorkflowMethodArgs(m)

		// Add a precondition guard
		if len(m.Requires) > 0 {
			var conditions []string
			for _, req := range m.Requires {
				rustExpr := codegen.ExprToRust(req.Expr, "__entity", "", false, entities, enums, functions)
				conditions = append(conditions, rustExpr)
			}
			sb.WriteString(fmt.Sprintf("        if %s {\n", strings.Join(conditions, " && ")))

			hasReturn := m.ReturnType != nil && m.ReturnType.Name != "Void"
			if hasReturn {
				sb.WriteString(fmt.Sprintf("            let _ = __entity.%s(%s);\n", m.Name, methodArgs))
			} else {
				sb.WriteString(fmt.Sprintf("            __entity.%s(%s);\n", m.Name, methodArgs))
			}

			// Check invariants after each method call
			for _, inv := range e.Invariants {
				rustExpr := codegen.ExprToRust(inv.Expr, "__entity", "", false, entities, enums, functions)
				sb.WriteString(fmt.Sprintf("            assert!(%s, \"Invariant '%s' failed after %s in workflow\");\n",
					rustExpr, codegen.EscapeRustString(inv.RawText), m.Name))
			}
			sb.WriteString("        }\n")
		} else {
			hasReturn := m.ReturnType != nil && m.ReturnType.Name != "Void"
			if hasReturn {
				sb.WriteString(fmt.Sprintf("        let _ = __entity.%s(%s);\n", m.Name, methodArgs))
			} else {
				sb.WriteString(fmt.Sprintf("        __entity.%s(%s);\n", m.Name, methodArgs))
			}
			for _, inv := range e.Invariants {
				rustExpr := codegen.ExprToRust(inv.Expr, "__entity", "", false, entities, enums, functions)
				sb.WriteString(fmt.Sprintf("        assert!(%s, \"Invariant '%s' failed after %s in workflow\");\n",
					rustExpr, codegen.EscapeRustString(inv.RawText), m.Name))
			}
		}
	}

	sb.WriteString("    }\n")
	return sb.String()
}

// --- Helper functions ---

// generateValuesForParam selects value generation strategy based on type.
func generateValuesForParam(c *ParamConstraint) []string {
	switch c.TypeName {
	case "Int":
		return GenerateIntValues(c)
	case "Float":
		return GenerateFloatValues(c)
	case "Bool":
		return GenerateBoolValues()
	case "String":
		return GenerateStringValues()
	case "Array":
		return GenerateArrayIntValues(c)
	default:
		// Unknown type -- return empty set
		return []string{}
	}
}

// rustTypeForParam returns the Rust type for generating value vectors.
func rustTypeForParam(p *ast.Param) string {
	switch p.Type.Name {
	case "Int":
		return "i64"
	case "Float":
		return "f64"
	case "Bool":
		return "bool"
	case "String":
		return "String"
	case "Array":
		if len(p.Type.TypeArgs) > 0 {
			return "Vec<" + codegen.MapType(p.Type.TypeArgs[0]) + ">"
		}
		return "Vec<i64>"
	default:
		return codegen.MapType(p.Type)
	}
}

// maxLenExpr builds a Rust expression that computes the max length across value vectors.
func maxLenExpr(params []*ast.Param, paramValueVars map[string]string) string {
	if len(params) == 1 {
		return fmt.Sprintf("%s.len()", paramValueVars[params[0].Name])
	}
	parts := make([]string, len(params))
	for i, p := range params {
		parts[i] = fmt.Sprintf("%s.len()", paramValueVars[p.Name])
	}
	// Build nested max calls: parts[0].max(parts[1]).max(parts[2])...
	result := parts[0]
	for _, p := range parts[1:] {
		result = fmt.Sprintf("%s.max(%s)", result, p)
	}
	return result
}

// buildFunctionArgs generates the argument list for a function call.
// Array params are passed by reference.
func buildFunctionArgs(f *ast.FunctionDecl) string {
	args := make([]string, len(f.Params))
	for i, p := range f.Params {
		if p.Type.Name == "Array" {
			args[i] = "&" + p.Name
		} else if p.Type.Name == "String" {
			args[i] = p.Name + ".clone()"
		} else {
			args[i] = p.Name
		}
	}
	return strings.Join(args, ", ")
}

// buildConstructorArgs generates the argument list for a constructor call.
func buildConstructorArgs(ctor *ast.ConstructorDecl) string {
	args := make([]string, len(ctor.Params))
	for i, p := range ctor.Params {
		if p.Type.Name == "String" {
			args[i] = p.Name + ".clone()"
		} else {
			args[i] = p.Name
		}
	}
	return strings.Join(args, ", ")
}

// buildMethodArgs generates the argument list for a method call.
func buildMethodArgs(m *ast.MethodDecl) string {
	args := make([]string, len(m.Params))
	for i, p := range m.Params {
		if p.Type.Name == "String" {
			args[i] = p.Name + ".clone()"
		} else {
			args[i] = p.Name
		}
	}
	return strings.Join(args, ", ")
}

// buildDefaultConstructorArgs generates first-valid args for constructing a base entity.
func buildDefaultConstructorArgs(ctor *ast.ConstructorDecl) string {
	constraints := AnalyzeConstraints(ctor.Params, ctor.Requires)
	args := make([]string, len(ctor.Params))
	for i, p := range ctor.Params {
		c := constraints[p.Name]
		args[i] = defaultValueForType(c)
	}
	return strings.Join(args, ", ")
}

// buildWorkflowMethodArgs generates the argument list using declared variable names.
// Used in workflow tests where params are declared as let bindings.
func buildWorkflowMethodArgs(m *ast.MethodDecl) string {
	args := make([]string, len(m.Params))
	for i, p := range m.Params {
		if p.Type.Name == "String" {
			args[i] = p.Name + ".clone()"
		} else {
			args[i] = p.Name
		}
	}
	return strings.Join(args, ", ")
}

// defaultValueForType picks a single valid default value for a constrained parameter.
func defaultValueForType(c *ParamConstraint) string {
	switch c.TypeName {
	case "Int":
		lo := int64(0)
		if c.Lower != nil {
			lo = *c.Lower
		}
		// Pick first value that's not excluded
		for _, ne := range c.NotEqual {
			if lo == ne {
				lo++
			}
		}
		return fmt.Sprintf("%di64", lo)
	case "Float":
		if c.Lower != nil {
			return formatFloat(float64(*c.Lower))
		}
		return "0.0f64"
	case "Bool":
		return "true"
	case "String":
		return `"test".to_string()`
	case "Array":
		minLen := int64(1)
		if c.MinLen != nil {
			minLen = *c.MinLen
		}
		elemLo := int64(1)
		if c.ElemLower != nil {
			elemLo = *c.ElemLower
		}
		return makeArrayLiteral(minLen, elemLo, elemLo+10, 0xdeadbeef)
	default:
		return "Default::default()"
	}
}

// needsClone returns true if a type is not Copy in Rust (String, Array, etc.).
func needsClone(t *ast.TypeRef) bool {
	return t.Name == "String" || t.Name == "Array"
}

// paramFormatStr builds a format string for error messages showing param values.
func paramFormatStr(params []*ast.Param) string {
	parts := make([]string, len(params))
	for i, p := range params {
		parts[i] = fmt.Sprintf("%s={:?}", p.Name)
	}
	return strings.Join(parts, ", ")
}

// paramFormatArgs builds format arguments for error messages.
func paramFormatArgs(params []*ast.Param) string {
	args := make([]string, len(params))
	for i, p := range params {
		args[i] = p.Name
	}
	return strings.Join(args, ", ")
}

// collectOldExprs extracts old() expression captures needed for ensures clauses.
// Returns map of mangled name -> Rust expression to capture.
func collectOldExprs(ensures []*ast.ContractClause, entities map[string]*ast.EntityDecl, enums map[string]*ast.EnumDecl, functions map[string]*ast.FunctionDecl) map[string]string {
	result := make(map[string]string)
	for _, ens := range ensures {
		walkForOld(ens.Expr, result, entities, enums, functions)
	}
	return result
}

// walkForOld recursively finds OldExpr nodes and records their captures.
func walkForOld(expr ast.Expression, captures map[string]string, entities map[string]*ast.EntityDecl, enums map[string]*ast.EnumDecl, functions map[string]*ast.FunctionDecl) {
	switch e := expr.(type) {
	case *ast.BinaryExpr:
		walkForOld(e.Left, captures, entities, enums, functions)
		walkForOld(e.Right, captures, entities, enums, functions)
	case *ast.UnaryExpr:
		walkForOld(e.Operand, captures, entities, enums, functions)
	case *ast.OldExpr:
		mangledName := mangleOldExpr(e.Expr)
		rustExpr := codegen.ExprToRust(e.Expr, "__entity", "", false, entities, enums, functions)
		captures[mangledName] = rustExpr
		walkForOld(e.Expr, captures, entities, enums, functions)
	case *ast.CallExpr:
		for _, arg := range e.Args {
			walkForOld(arg, captures, entities, enums, functions)
		}
	case *ast.MethodCallExpr:
		walkForOld(e.Object, captures, entities, enums, functions)
		for _, arg := range e.Args {
			walkForOld(arg, captures, entities, enums, functions)
		}
	case *ast.FieldAccessExpr:
		walkForOld(e.Object, captures, entities, enums, functions)
	case *ast.IndexExpr:
		walkForOld(e.Object, captures, entities, enums, functions)
		walkForOld(e.Index, captures, entities, enums, functions)
	}
}

// mangleOldExpr creates a mangled variable name for an old() expression.
// Must match the pattern used by codegen so ExprToRust old() substitution works.
func mangleOldExpr(e ast.Expression) string {
	text := exprToText(e)
	text = strings.ReplaceAll(text, ".", "_")
	text = strings.ReplaceAll(text, "(", "")
	text = strings.ReplaceAll(text, ")", "")
	text = strings.ReplaceAll(text, " ", "_")
	return "__old_" + text
}

// exprToText converts an expression to a simple text representation.
func exprToText(e ast.Expression) string {
	switch expr := e.(type) {
	case *ast.FieldAccessExpr:
		return exprToText(expr.Object) + "." + expr.Field
	case *ast.SelfExpr:
		return "self"
	case *ast.Identifier:
		return expr.Name
	default:
		return "expr"
	}
}
