package verify

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/lhaig/intent/internal/ir"
)

// VerifyResult holds the result of verifying a single contract
type VerifyResult struct {
	FunctionName string
	EntityName   string // non-empty for entity contracts (methods, constructors, invariants)
	ContractKind string // "requires", "ensures", "invariant"
	ContractText string
	IsEnsures    bool
	Status       string // "verified", "unverified", "error", "timeout"
	Message      string
	SMTOutput    string // raw SMT-LIB for debugging
}

// QualifiedName returns the fully qualified contract name (e.g., "BankAccount.withdraw.requires")
func (r *VerifyResult) QualifiedName() string {
	if r.EntityName != "" {
		if r.FunctionName != "" {
			return r.EntityName + "." + r.FunctionName + "." + r.ContractKind
		}
		return r.EntityName + "." + r.ContractKind
	}
	if r.FunctionName != "" {
		return r.FunctionName + "." + r.ContractKind
	}
	return r.ContractKind
}

// Verify verifies all contracts in a module
func Verify(mod *ir.Module) []*VerifyResult {
	var results []*VerifyResult

	// Check if z3 is available
	z3Path, err := exec.LookPath("z3")
	if err != nil {
		return []*VerifyResult{{
			FunctionName: "",
			ContractText: "",
			Status:       "error",
			Message:      "z3 not found on PATH",
		}}
	}

	// Verify contracts for each function
	for _, fn := range mod.Functions {
		fnResults := verifyFunctionWithZ3(fn, z3Path)
		results = append(results, fnResults...)
	}

	// Verify contracts for each entity
	for _, ent := range mod.Entities {
		entResults := verifyEntityWithZ3(ent, z3Path)
		results = append(results, entResults...)
	}

	return results
}

// VerifyFunction verifies contracts for a single function
func VerifyFunction(fn *ir.Function) []*VerifyResult {
	// Check if z3 is available
	z3Path, err := exec.LookPath("z3")
	if err != nil {
		return []*VerifyResult{{
			FunctionName: fn.Name,
			ContractText: "",
			Status:       "error",
			Message:      "z3 not found on PATH",
		}}
	}

	return verifyFunctionWithZ3(fn, z3Path)
}

// verifyFunctionWithZ3 verifies all contracts for a function using z3
func verifyFunctionWithZ3(fn *ir.Function, z3Path string) []*VerifyResult {
	var results []*VerifyResult

	// Verify requires clauses (satisfiability check)
	for _, req := range fn.Requires {
		smtLib := TranslateContract(fn, req, false)
		result := runZ3(z3Path, smtLib, false)
		result.FunctionName = fn.Name
		result.ContractKind = "requires"
		result.ContractText = req.RawText
		result.IsEnsures = false
		result.SMTOutput = smtLib
		results = append(results, result)
	}

	// Verify ensures clauses (validity check)
	for _, ens := range fn.Ensures {
		smtLib := TranslateContract(fn, ens, true)
		result := runZ3(z3Path, smtLib, true)
		result.FunctionName = fn.Name
		result.ContractKind = "ensures"
		result.ContractText = ens.RawText
		result.IsEnsures = true
		result.SMTOutput = smtLib
		results = append(results, result)
	}

	// Verify loop invariants in function body
	loops := findWhileStmts(fn.Body)
	for i, loop := range loops {
		for _, inv := range loop.Invariants {
			smtLib := TranslateLoopInvariant(fn, loop, inv)
			result := runZ3(z3Path, smtLib, true)
			result.FunctionName = fmt.Sprintf("%s.loop_%d", fn.Name, i+1)
			result.ContractKind = "loop_invariant"
			result.ContractText = inv.RawText
			result.IsEnsures = true
			result.SMTOutput = smtLib
			results = append(results, result)
		}
	}

	return results
}

// runZ3 executes z3 with the given SMT-LIB input.
// isEnsures controls result interpretation:
//   - true (ensures/invariant): unsat = verified (negated contract has no counterexample)
//   - false (requires): sat = verified (precondition is satisfiable/consistent)
func runZ3(z3Path, smtLib string, isEnsures bool) *VerifyResult {
	result := &VerifyResult{}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create command
	cmd := exec.CommandContext(ctx, z3Path, "-in", "-T:5")
	cmd.Stdin = strings.NewReader(smtLib)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run z3
	err := cmd.Run()

	// Check for timeout
	if ctx.Err() == context.DeadlineExceeded {
		result.Status = "timeout"
		result.Message = "z3 timed out after 5 seconds"
		return result
	}

	// Check for other errors
	if err != nil {
		result.Status = "error"
		result.Message = fmt.Sprintf("z3 error: %v", err)
		if stderr.Len() > 0 {
			result.Message += "\n" + stderr.String()
		}
		return result
	}

	// Parse z3 output
	output := strings.TrimSpace(stdout.String())

	// For ensures/invariants (negated): unsat = verified, sat = counterexample found
	// For requires (satisfiability): sat = verified (consistent), unsat = contradictory

	switch output {
	case "unsat":
		if isEnsures {
			result.Status = "verified"
			result.Message = "contract verified (no counterexample exists)"
		} else {
			result.Status = "unverified"
			result.Message = "precondition is contradictory (unsatisfiable)"
		}
	case "sat":
		if isEnsures {
			result.Status = "unverified"
			result.Message = "counterexample found (contract may not hold)"
		} else {
			result.Status = "verified"
			result.Message = "precondition is satisfiable (consistent)"
		}
	case "timeout":
		result.Status = "timeout"
		result.Message = "z3 timed out"
	case "unknown":
		result.Status = "timeout"
		result.Message = "z3 returned unknown (likely timeout or too complex)"
	default:
		result.Status = "error"
		result.Message = fmt.Sprintf("unexpected z3 output: %s", output)
	}

	return result
}

// verifyEntityWithZ3 verifies all contracts for an entity (invariants, constructor, methods)
func verifyEntityWithZ3(ent *ir.Entity, z3Path string) []*VerifyResult {
	var results []*VerifyResult

	// Verify invariants
	for _, inv := range ent.Invariants {
		smtLib := TranslateInvariant(ent.Name, ent.Fields, inv)
		result := runZ3(z3Path, smtLib, true) // invariants proved by contradiction like ensures
		result.EntityName = ent.Name
		result.FunctionName = ""
		result.ContractKind = "invariant"
		result.ContractText = inv.RawText
		result.IsEnsures = true
		result.SMTOutput = smtLib
		results = append(results, result)
	}

	// Verify constructor contracts
	if ent.Constructor != nil {
		ctor := ent.Constructor
		for _, req := range ctor.Requires {
			smtLib := TranslateMethodContract(ent.Name, "constructor", ent.Fields, ctor.Params, nil, nil, ent.Invariants, req, false, nil)
			result := runZ3(z3Path, smtLib, false)
			result.EntityName = ent.Name
			result.FunctionName = "constructor"
			result.ContractKind = "requires"
			result.ContractText = req.RawText
			result.IsEnsures = false
			result.SMTOutput = smtLib
			results = append(results, result)
		}
		for _, ens := range ctor.Ensures {
			smtLib := TranslateMethodContract(ent.Name, "constructor", ent.Fields, ctor.Params, nil, ctor.Requires, ent.Invariants, ens, true, ctor.OldCaptures)
			result := runZ3(z3Path, smtLib, true)
			result.EntityName = ent.Name
			result.FunctionName = "constructor"
			result.ContractKind = "ensures"
			result.ContractText = ens.RawText
			result.IsEnsures = true
			result.SMTOutput = smtLib
			results = append(results, result)
		}
	}

	// Verify loop invariants in constructor body
	if ent.Constructor != nil {
		loops := findWhileStmts(ent.Constructor.Body)
		for i, loop := range loops {
			for _, inv := range loop.Invariants {
				smtLib := TranslateLoopInvariantForMethod(ent.Name, "constructor", ent.Fields, ent.Constructor.Params, loop, inv)
				result := runZ3(z3Path, smtLib, true)
				result.EntityName = ent.Name
				result.FunctionName = fmt.Sprintf("constructor.loop_%d", i+1)
				result.ContractKind = "loop_invariant"
				result.ContractText = inv.RawText
				result.IsEnsures = true
				result.SMTOutput = smtLib
				results = append(results, result)
			}
		}
	}

	// Verify method contracts
	for _, m := range ent.Methods {
		for _, req := range m.Requires {
			smtLib := TranslateMethodContract(ent.Name, m.Name, ent.Fields, m.Params, m.ReturnType, nil, ent.Invariants, req, false, nil)
			result := runZ3(z3Path, smtLib, false)
			result.EntityName = ent.Name
			result.FunctionName = m.Name
			result.ContractKind = "requires"
			result.ContractText = req.RawText
			result.IsEnsures = false
			result.SMTOutput = smtLib
			results = append(results, result)
		}
		for _, ens := range m.Ensures {
			smtLib := TranslateMethodContract(ent.Name, m.Name, ent.Fields, m.Params, m.ReturnType, m.Requires, ent.Invariants, ens, true, m.OldCaptures)
			result := runZ3(z3Path, smtLib, true)
			result.EntityName = ent.Name
			result.FunctionName = m.Name
			result.ContractKind = "ensures"
			result.ContractText = ens.RawText
			result.IsEnsures = true
			result.SMTOutput = smtLib
			results = append(results, result)
		}

		// Verify loop invariants in method body
		loops := findWhileStmts(m.Body)
		for i, loop := range loops {
			for _, inv := range loop.Invariants {
				smtLib := TranslateLoopInvariantForMethod(ent.Name, m.Name, ent.Fields, m.Params, loop, inv)
				result := runZ3(z3Path, smtLib, true)
				result.EntityName = ent.Name
				result.FunctionName = fmt.Sprintf("%s.loop_%d", m.Name, i+1)
				result.ContractKind = "loop_invariant"
				result.ContractText = inv.RawText
				result.IsEnsures = true
				result.SMTOutput = smtLib
				results = append(results, result)
			}
		}
	}

	return results
}

// findWhileStmts recursively finds all WhileStmt nodes in a statement list
func findWhileStmts(stmts []ir.Stmt) []*ir.WhileStmt {
	var loops []*ir.WhileStmt
	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case *ir.WhileStmt:
			if len(s.Invariants) > 0 {
				loops = append(loops, s)
			}
			// Also search nested loops
			loops = append(loops, findWhileStmts(s.Body)...)
		case *ir.IfStmt:
			loops = append(loops, findWhileStmts(s.Then)...)
			loops = append(loops, findWhileStmts(s.Else)...)
		case *ir.ForInStmt:
			loops = append(loops, findWhileStmts(s.Body)...)
		}
	}
	return loops
}
