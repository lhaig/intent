package verify

import (
	"fmt"
	"strings"

	"github.com/lhaig/intent/internal/ir"
)

// RefStatus holds the verification status of a single verified_by reference.
type RefStatus struct {
	Ref     string // e.g., "BankAccount.invariant"
	Status  string // "verified", "unverified", "error", "timeout", "not_found"
	Message string
}

// IntentReport holds the verification report for a single intent block.
type IntentReport struct {
	Description string
	Refs        []*RefStatus
}

// AllVerified returns true if all references are verified.
func (r *IntentReport) AllVerified() bool {
	for _, ref := range r.Refs {
		if ref.Status != "verified" {
			return false
		}
	}
	return len(r.Refs) > 0
}

// BuildIntentReports matches intent block verified_by references to verify results.
func BuildIntentReports(mod *ir.Module, results []*VerifyResult) []*IntentReport {
	if len(mod.Intents) == 0 {
		return nil
	}

	// Build a lookup from qualified name -> list of results
	// A single qualified name like "BankAccount.withdraw.requires" may match multiple results
	// (one per contract clause). We also support partial matches like "BankAccount.invariant".
	resultsByQualified := make(map[string][]*VerifyResult)
	for _, r := range results {
		qn := r.QualifiedName()
		resultsByQualified[qn] = append(resultsByQualified[qn], r)
	}

	var reports []*IntentReport

	for _, intent := range mod.Intents {
		report := &IntentReport{
			Description: intent.Description,
		}

		for _, parts := range intent.VerifiedBy {
			refStr := strings.Join(parts, ".")
			matched := resultsByQualified[refStr]

			if len(matched) == 0 {
				report.Refs = append(report.Refs, &RefStatus{
					Ref:     refStr,
					Status:  "not_found",
					Message: "no matching contract found in verification results",
				})
				continue
			}

			// Aggregate results for this ref: use worst status
			worstStatus := "verified"
			var messages []string
			for _, m := range matched {
				if statusWorse(m.Status, worstStatus) {
					worstStatus = m.Status
				}
				if m.Status != "verified" {
					messages = append(messages, m.Message)
				}
			}

			report.Refs = append(report.Refs, &RefStatus{
				Ref:     refStr,
				Status:  worstStatus,
				Message: strings.Join(messages, "; "),
			})
		}

		reports = append(reports, report)
	}

	return reports
}

// statusWorse returns true if a is worse than b in the ordering:
// verified < timeout < unverified < error < not_found
func statusWorse(a, b string) bool {
	return statusRank(a) > statusRank(b)
}

func statusRank(s string) int {
	switch s {
	case "verified":
		return 0
	case "timeout":
		return 1
	case "unverified":
		return 2
	case "error":
		return 3
	case "not_found":
		return 4
	default:
		return 5
	}
}

// FormatReport produces human-readable output for intent verification reports.
func FormatReport(reports []*IntentReport) string {
	if len(reports) == 0 {
		return ""
	}

	var sb strings.Builder

	sb.WriteString("Intent Verification Report\n")
	sb.WriteString("==========================\n\n")

	for _, report := range reports {
		fmt.Fprintf(&sb, "Intent: %q\n", report.Description)

		verified := 0
		total := len(report.Refs)

		for _, ref := range report.Refs {
			status := strings.ToUpper(ref.Status)
			fmt.Fprintf(&sb, "  %-40s %s\n", ref.Ref, status)
			if ref.Status == "verified" {
				verified++
			}
		}

		if total > 0 {
			if verified == total {
				fmt.Fprintf(&sb, "  Status: all %d contracts verified\n", total)
			} else {
				fmt.Fprintf(&sb, "  Status: %d of %d contracts verified\n", verified, total)
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
