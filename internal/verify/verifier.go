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
	ContractText string
	IsEnsures    bool
	Status       string // "verified", "unverified", "error", "timeout"
	Message      string
	SMTOutput    string // raw SMT-LIB for debugging
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
		result := runZ3(z3Path, smtLib)
		result.FunctionName = fn.Name
		result.ContractText = req.RawText
		result.IsEnsures = false
		result.SMTOutput = smtLib
		results = append(results, result)
	}

	// Verify ensures clauses (validity check)
	for _, ens := range fn.Ensures {
		smtLib := TranslateContract(fn, ens, true)
		result := runZ3(z3Path, smtLib)
		result.FunctionName = fn.Name
		result.ContractText = ens.RawText
		result.IsEnsures = true
		result.SMTOutput = smtLib
		results = append(results, result)
	}

	return results
}

// runZ3 executes z3 with the given SMT-LIB input
func runZ3(z3Path, smtLib string) *VerifyResult {
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

	// Note: We're checking satisfiability for both requires and ensures (negated)
	// For ensures (negated): unsat means contract is verified (no counterexample exists)
	// For requires: sat means the preconditions are satisfiable (they're not contradictory)

	switch output {
	case "unsat":
		result.Status = "verified"
		result.Message = "contract verified (no counterexample exists)"
	case "sat":
		result.Status = "unverified"
		result.Message = "counterexample found (contract may not hold)"
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
