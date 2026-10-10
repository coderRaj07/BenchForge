package report

import (
	"fmt"
	"strings"
)

// Summary is the human-readable result printed by the CLI.
func Summary(rep *Report) string {
	if rep == nil {
		return "no report"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Run:       %s\n", rep.RunID)
	fmt.Fprintf(&b, "State:     %s\n", rep.State)
	fmt.Fprintf(&b, "Outcome:   %s\n", rep.Outcome)
	fmt.Fprintf(&b, "Manifest:  %s\n", rep.ManifestName)
	if rep.Environment.PostgresVersion != "" {
		fmt.Fprintf(&b, "Postgres:  %s\n", rep.Environment.PostgresVersion)
	}
	fmt.Fprintf(&b, "Workload:  %s  duration=%ds  concurrency=%d", rep.Workload.Type, rep.Workload.DurationSeconds, rep.Workload.MaxConcurrency)
	if rep.Workload.ArrivalRatePerSecond > 0 {
		fmt.Fprintf(&b, "  arrival=%d/s", rep.Workload.ArrivalRatePerSecond)
	}
	b.WriteByte('\n')
	fmt.Fprintf(&b, "Dataset:   %s  seed=%d  scale=%g  orders=%d\n", rep.Dataset.Profile, rep.Dataset.Seed, rep.Dataset.Scale, rep.Dataset.Orders)
	if rep.Aggregate.Iterations+rep.Aggregate.Errors > 0 {
		fmt.Fprintf(&b, "Aggregate: tps=%.2f  p50=%.3fms  p95=%.3fms  p99=%.3fms  errors=%d/%d\n",
			rep.Aggregate.TPS, rep.Aggregate.Latency.P50, rep.Aggregate.Latency.P95, rep.Aggregate.Latency.P99,
			rep.Aggregate.Errors, rep.Aggregate.Iterations+rep.Aggregate.Errors)
	}
	for _, q := range rep.Queries {
		fmt.Fprintf(&b, "  %-24s  tps=%8.2f  p50=%8.3fms  p95=%8.3fms  p99=%8.3fms  errors=%d  rows=%d\n",
			q.ID, q.TPS, q.Latency.P50, q.Latency.P95, q.Latency.P99, q.Errors, q.Rows)
	}
	if rep.Pgbench != nil {
		fmt.Fprintf(&b, "pgbench:   tps=%.2f  mean=%.3fms  p95=%.3fms  tx=%d  failed=%d  skipped=%d  log_samples=%d\n",
			rep.Pgbench.TPS, rep.Pgbench.Latency.Mean, rep.Pgbench.Latency.P95, rep.Pgbench.Transactions, rep.Pgbench.Failed, rep.Pgbench.Skipped, rep.Pgbench.LogSamples)
	}
	if rep.Load.Mode != "" {
		fmt.Fprintf(&b, "Load:      mode=%s  offered=%d  completed=%d  dropped=%d  generator_saturated=%t\n",
			rep.Load.Mode, rep.Load.Offered, rep.Load.Completed, rep.Load.Dropped, rep.Load.GeneratorSaturated)
		if rep.Load.PoolWait.P95 > 0 {
			fmt.Fprintf(&b, "Pool wait: p95=%.3fms  max=%.3fms\n", rep.Load.PoolWait.P95, rep.Load.PoolWait.Max)
		}
	}
	if rep.Database.BlksHit+rep.Database.BlksRead > 0 || rep.Database.MaxActiveSessions > 0 {
		fmt.Fprintf(&b, "Database:  cache_hit=%.3f  blks_read=%d  active_peak=%d  deadlocks=%d\n",
			rep.Database.CacheHitRatio, rep.Database.BlksRead, rep.Database.MaxActiveSessions, rep.Database.Deadlocks)
	}
	if rep.Comparison != nil {
		fmt.Fprintf(&b, "Baseline:  %s  compatible=%t\n", rep.Comparison.BaselineRunID, rep.Comparison.Compatible)
		for _, reason := range rep.Comparison.Reasons {
			fmt.Fprintf(&b, "           %s\n", reason)
		}
	}
	for _, f := range rep.Findings {
		if f.Severity == SeverityInfo {
			continue
		}
		id := f.QueryID
		if id != "" {
			id = " [" + id + "]"
		}
		fmt.Fprintf(&b, "Finding:   %s %s%s: %s\n", f.Severity, f.Code, id, f.Message)
	}
	if rep.FailureReason != "" {
		fmt.Fprintf(&b, "Failure:   %s\n", rep.FailureReason)
	}
	if rep.CleanupError != "" {
		fmt.Fprintf(&b, "Cleanup:   %s\n", rep.CleanupError)
	}
	return b.String()
	// What: formats a report as the text the CLI prints.
	// Used by: internal/cli/cli.go cmdRun and cmdCompare. Tested in internal/report/summary_test.go.
	// Logic: b accumulates lines. A nil report returns "no report". The first lines are run id, state, outcome, and manifest name. Query rows and the pgbench line are appended only when those sections exist. Info findings are skipped. id is the query id wrapped in brackets when it is set. FailureReason and CleanupError are printed last.
}

// ExitCode maps an outcome to a process status.
// Cleanup failures on an otherwise passing run use code 1.
func ExitCode(rep *Report) int {
	if rep == nil {
		return 1
	}
	code := 1
	switch rep.Outcome {
	case OutcomePass:
		code = 0
	case OutcomeRegression:
		code = 2
	case OutcomeInconclusive:
		code = 3
	case OutcomeError:
		code = 1
	}
	if rep.CleanupError != "" && code == 0 {
		return 1
	}
	return code
	// What: maps a report outcome to a process exit code.
	// Used by: internal/cli/cli.go cmdRun and cmdReport, and internal/report/summary_test.go.
	// Logic: a nil report is 1. code starts at 1, then PASS sets 0, REGRESSION sets 2, INCONCLUSIVE sets 3, and ERROR stays 1. CleanupError changes a 0 into 1 and leaves 2 or 3 alone so a regression is still visible.
}
