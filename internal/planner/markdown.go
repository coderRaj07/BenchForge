package planner

import (
	"fmt"
	"strings"

	"github.com/coderRaj07/benchforge/internal/report"
)

// PullRequestMarkdown renders a finished report for a pull-request comment.
func PullRequestMarkdown(rep *report.Report) string {
	if rep == nil {
		return "BenchForge did not produce a report.\n"
	}
	var b strings.Builder
	b.WriteString("## BenchForge result\n\n")
	fmt.Fprintf(&b, "**Outcome:** `%s`\n\n", rep.Outcome)
	fmt.Fprintf(&b, "- Run: `%s`\n", rep.RunID)
	fmt.Fprintf(&b, "- Manifest: `%s`\n", rep.ManifestName)
	fmt.Fprintf(&b, "- State: `%s`\n", rep.State)
	if rep.FailureReason != "" {
		fmt.Fprintf(&b, "- Failure: %s\n", rep.FailureReason)
	}
	b.WriteString("\n")
	if len(rep.Queries) > 0 {
		b.WriteString("| Query | p95 ms | p99 ms | TPS | Errors |\n| --- | ---: | ---: | ---: | ---: |\n")
		for _, q := range rep.Queries {
			fmt.Fprintf(&b, "| `%s` | %.3f | %.3f | %.2f | %d |\n", q.ID, q.Latency.P95, q.Latency.P99, q.TPS, q.Errors)
		}
		b.WriteString("\n")
	}
	if rep.Pgbench != nil {
		fmt.Fprintf(&b, "pgbench TPS %.2f, transactions %d, failed %d, log samples %d.\n\n", rep.Pgbench.TPS, rep.Pgbench.Transactions, rep.Pgbench.Failed, rep.Pgbench.LogSamples)
	}
	var fails []report.Finding
	for _, f := range rep.Findings {
		if f.Severity == report.SeverityFail {
			fails = append(fails, f)
		}
	}
	if len(fails) > 0 {
		b.WriteString("### Findings\n\n")
		for _, f := range fails {
			id := ""
			if f.QueryID != "" {
				id = " (" + f.QueryID + ")"
			}
			fmt.Fprintf(&b, "- `%s`%s: %s\n", f.Code, id, f.Message)
		}
		b.WriteString("\n")
	}
	if rep.Comparison != nil && !rep.Comparison.Compatible {
		b.WriteString("Baseline was not comparable:\n\n")
		for _, reason := range rep.Comparison.Reasons {
			fmt.Fprintf(&b, "- %s\n", reason)
		}
		b.WriteString("\n")
	}
	b.WriteString("Infrastructure failures stay `ERROR`. An inconclusive result is not a performance regression.\n")
	return b.String()
	// What: renders a finished report as pull-request markdown.
	// Used by: internal/cli/plan.go cmdReport. Tested in internal/planner/plan_test.go TestPullRequestMarkdownRegression.
	// Logic: a nil report returns one sentence. b collects the outcome, run id, manifest, and state. q rows are one table line each. fails keeps only SeverityFail findings. id is the query id in parentheses when set. Incompatible comparison reasons are listed after the findings.
}
