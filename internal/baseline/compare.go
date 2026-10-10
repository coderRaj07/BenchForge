// Package baseline decides whether a candidate run regressed against a
// compatible baseline. Infrastructure failures stay ERROR.
package baseline

import (
	"fmt"
	"math"
	"sort"

	"github.com/coderRaj07/benchforge/internal/report"
)

// Result is the judgment for a candidate report.
type Result struct {
	Outcome    string
	Findings   []report.Finding
	Comparison *report.Comparison
}

// FinalOutcome keeps an execution failure from being reported as a regression.
func FinalOutcome(executionError string, eval Result) string {
	if executionError != "" {
		return report.OutcomeError
	}
	if eval.Outcome == "" {
		return report.OutcomeError
	}
	return eval.Outcome
	// What: keeps an infrastructure error from being reported as a regression.
	// Used by: internal/cli/cli.go cmdCompare. Tested in compare_test.go TestExecutionErrorWins.
	// Logic: executionError non-empty returns ERROR immediately. An empty eval.Outcome also returns ERROR. Otherwise the evaluation outcome is returned unchanged.
}

// Evaluate applies absolute gates and, when a baseline is present, relative gates.
func Evaluate(candidate report.Report, base *report.Report) Result {
	findings := []report.Finding{{
		Severity: report.SeverityInfo,
		Code:     "single_run_noise",
		Message:  "this comparison uses one run per side; ordinary noise is absorbed by assertions.maxRegressionPercent",
	}}
	if candidate.Load.GeneratorSaturated {
		findings = append(findings, report.Finding{
			Severity: report.SeverityFail,
			Code:     "load_generator_saturated",
			Message:  candidate.Load.SaturationReason,
		})
		return Result{Outcome: report.OutcomeInconclusive, Findings: findings}
	}

	samples := measuredIterations(candidate)
	if samples == 0 {
		findings = append(findings, report.Finding{
			Severity: report.SeverityFail,
			Code:     "no_samples",
			Message:  "the run recorded no successful iterations",
		})
		return Result{Outcome: report.OutcomeError, Findings: findings}
	}
	if samples < candidate.Assertions.MinIterations {
		findings = append(findings, report.Finding{
			Severity: report.SeverityFail,
			Code:     "insufficient_samples",
			Message:  fmt.Sprintf("recorded %d successful iterations, below minIterations %d", samples, candidate.Assertions.MinIterations),
		})
		return Result{Outcome: report.OutcomeInconclusive, Findings: findings}
	}

	var comparison *report.Comparison
	if base != nil {
		ok, reasons := Compatible(candidate, *base)
		comparison = &report.Comparison{
			BaselineRunID: base.RunID,
			Compatible:    ok,
			Reasons:       reasons,
		}
		if !ok {
			findings = append(findings, report.Finding{
				Severity: report.SeverityFail,
				Code:     "incompatible_baseline",
				Message:  "baseline is not comparable: " + join(reasons),
			})
			return Result{Outcome: report.OutcomeInconclusive, Findings: findings, Comparison: comparison}
		}
		findings = append(findings, fingerprintFindings(candidate, *base)...)
	}

	fail := false
	fail = checkAbsolute(candidate, &findings) || fail
	if base != nil && comparison != nil && comparison.Compatible {
		fail = checkRelative(candidate, *base, &findings) || fail
	}
	outcome := report.OutcomePass
	if fail {
		outcome = report.OutcomeRegression
	}
	return Result{Outcome: outcome, Findings: findings, Comparison: comparison}
	// What: judges a candidate report against absolute gates and an optional baseline.
	// Used by: internal/engine/run.go after measurement, and internal/cli/cli.go cmdCompare. Tested in compare_test.go.
	// Logic: findings starts with the single-run noise note. GeneratorSaturated returns INCONCLUSIVE immediately. measuredIterations of 0 returns ERROR. A count below MinIterations returns INCONCLUSIVE. base non-nil builds comparison through Compatible. An incompatible baseline returns INCONCLUSIVE. fingerprintFindings notes SQL text changes without failing. fail becomes true when checkAbsolute or checkRelative appends a failing finding. outcome is PASS unless fail is true, in which case it is REGRESSION.
}

func checkAbsolute(c report.Report, findings *[]report.Finding) bool {
	fail := false
	series := seriesOf(c)
	for _, s := range series {
		if c.Assertions.MaxP95Ms > 0 && s.p95 > c.Assertions.MaxP95Ms {
			fail = true
			*findings = append(*findings, report.Finding{
				Severity: report.SeverityFail,
				Code:     "latency_p95_absolute",
				QueryID:  s.id,
				Message:  fmt.Sprintf("%s p95 %.3fms exceeds maxP95Ms %.3fms", s.id, s.p95, c.Assertions.MaxP95Ms),
			})
		}
		if c.Assertions.MaxP99Ms > 0 && s.p99 > c.Assertions.MaxP99Ms {
			fail = true
			*findings = append(*findings, report.Finding{
				Severity: report.SeverityFail,
				Code:     "latency_p99_absolute",
				QueryID:  s.id,
				Message:  fmt.Sprintf("%s p99 %.3fms exceeds maxP99Ms %.3fms", s.id, s.p99, c.Assertions.MaxP99Ms),
			})
		}
		if s.attempts > 0 && s.errorRate > c.Assertions.MaxErrorRate {
			fail = true
			*findings = append(*findings, report.Finding{
				Severity: report.SeverityFail,
				Code:     "error_rate",
				QueryID:  s.id,
				Message:  fmt.Sprintf("%s error rate %.4f exceeds maxErrorRate %.4f", s.id, s.errorRate, c.Assertions.MaxErrorRate),
			})
		}
	}
	return fail
}

func checkRelative(c, b report.Report, findings *[]report.Finding) bool {
	pct := c.Assertions.MaxRegressionPercent
	factor := 1 + pct/100
	drop := 1 - pct/100
	if drop < 0 {
		drop = 0
	}
	bf := indexSeries(b)
	fail := false
	for _, s := range seriesOf(c) {
		base, ok := bf[s.id]
		if !ok {
			continue
		}
		if base.p95 > 0 && s.p95 > base.p95*factor {
			fail = true
			*findings = append(*findings, report.Finding{
				Severity: report.SeverityFail,
				Code:     "latency_p95_regression",
				QueryID:  s.id,
				Message:  fmt.Sprintf("%s p95 increased from %.3fms to %.3fms (threshold %.1f%%)", s.id, base.p95, s.p95, pct),
			})
		}
		if base.p99 > 0 && s.p99 > base.p99*factor {
			fail = true
			*findings = append(*findings, report.Finding{
				Severity: report.SeverityFail,
				Code:     "latency_p99_regression",
				QueryID:  s.id,
				Message:  fmt.Sprintf("%s p99 increased from %.3fms to %.3fms (threshold %.1f%%)", s.id, base.p99, s.p99, pct),
			})
		}
		if base.tps > 0 && s.tps < base.tps*drop {
			fail = true
			*findings = append(*findings, report.Finding{
				Severity: report.SeverityFail,
				Code:     "throughput_regression",
				QueryID:  s.id,
				Message:  fmt.Sprintf("%s throughput fell from %.2f to %.2f tps (threshold %.1f%%)", s.id, base.tps, s.tps, pct),
			})
		}
	}
	return fail
}

func fingerprintFindings(c, b report.Report) []report.Finding {
	bf := map[string]string{}
	for _, q := range b.Queries {
		bf[q.ID] = q.Fingerprint
	}
	var out []report.Finding
	for _, q := range c.Queries {
		if prev, ok := bf[q.ID]; ok && prev != "" && prev != q.Fingerprint {
			out = append(out, report.Finding{
				Severity: report.SeverityInfo,
				Code:     "sql_fingerprint_changed",
				QueryID:  q.ID,
				Message:  "query text changed relative to the baseline; comparison still uses the query id",
			})
		}
	}
	return out
}

type series struct {
	id        string
	p95       float64
	p99       float64
	tps       float64
	errorRate float64
	attempts  int
}

func seriesOf(rep report.Report) []series {
	var out []series
	if len(rep.Queries) > 0 || rep.Aggregate.Iterations+rep.Aggregate.Errors > 0 {
		out = append(out, series{
			id:        "aggregate",
			p95:       rep.Aggregate.Latency.P95,
			p99:       rep.Aggregate.Latency.P99,
			tps:       rep.Aggregate.TPS,
			errorRate: rep.Aggregate.ErrorRate,
			attempts:  rep.Aggregate.Iterations + rep.Aggregate.Errors,
		})
	}
	for _, q := range rep.Queries {
		out = append(out, series{
			id:        q.ID,
			p95:       q.Latency.P95,
			p99:       q.Latency.P99,
			tps:       q.TPS,
			errorRate: q.ErrorRate,
			attempts:  q.Iterations + q.Errors,
		})
	}
	if rep.Pgbench != nil {
		attempts := rep.Pgbench.Transactions + rep.Pgbench.Failed
		rate := 0.0
		if attempts > 0 {
			rate = float64(rep.Pgbench.Failed) / float64(attempts)
		}
		out = append(out, series{
			id:        "pgbench",
			p95:       rep.Pgbench.Latency.P95,
			p99:       rep.Pgbench.Latency.P99,
			tps:       rep.Pgbench.TPS,
			errorRate: rate,
			attempts:  attempts,
		})
	}
	return out
}

func indexSeries(rep report.Report) map[string]series {
	m := map[string]series{}
	for _, s := range seriesOf(rep) {
		m[s.id] = s
	}
	return m
}

func measuredIterations(rep report.Report) int {
	n := rep.Aggregate.Iterations
	if rep.Pgbench != nil && rep.Pgbench.Transactions > n {
		n = rep.Pgbench.Transactions
	}
	return n
}

// Compatible reports whether two runs measured the same workload and environment.
func Compatible(a, b report.Report) (bool, []string) {
	var reasons []string
	if a.ManifestName != b.ManifestName {
		reasons = append(reasons, fmt.Sprintf("manifest name %q != %q", a.ManifestName, b.ManifestName))
	}
	if a.Dataset.Profile != b.Dataset.Profile {
		reasons = append(reasons, fmt.Sprintf("dataset profile %q != %q", a.Dataset.Profile, b.Dataset.Profile))
	}
	if a.Dataset.Seed != b.Dataset.Seed {
		reasons = append(reasons, fmt.Sprintf("dataset seed %d != %d", a.Dataset.Seed, b.Dataset.Seed))
	}
	if math.Abs(a.Dataset.Scale-b.Dataset.Scale) > 1e-9 {
		reasons = append(reasons, fmt.Sprintf("dataset scale %g != %g", a.Dataset.Scale, b.Dataset.Scale))
	}
	if a.Environment.PostgresMajor != b.Environment.PostgresMajor {
		reasons = append(reasons, fmt.Sprintf("postgres major %q != %q", a.Environment.PostgresMajor, b.Environment.PostgresMajor))
	}
	if a.Workload.Type != b.Workload.Type {
		reasons = append(reasons, fmt.Sprintf("workload type %q != %q", a.Workload.Type, b.Workload.Type))
	}
	if !closePct(float64(a.Workload.DurationSeconds), float64(b.Workload.DurationSeconds), 0.10) {
		reasons = append(reasons, fmt.Sprintf("duration %ds is not within 10%% of %ds", a.Workload.DurationSeconds, b.Workload.DurationSeconds))
	}
	if a.Workload.MaxConcurrency != b.Workload.MaxConcurrency {
		reasons = append(reasons, fmt.Sprintf("concurrency %d != %d", a.Workload.MaxConcurrency, b.Workload.MaxConcurrency))
	}
	if a.Workload.ArrivalRatePerSecond != b.Workload.ArrivalRatePerSecond {
		reasons = append(reasons, fmt.Sprintf("arrival rate %d != %d", a.Workload.ArrivalRatePerSecond, b.Workload.ArrivalRatePerSecond))
	}
	if a.Workload.WarmupSeconds != b.Workload.WarmupSeconds {
		reasons = append(reasons, fmt.Sprintf("warmup %ds != %ds", a.Workload.WarmupSeconds, b.Workload.WarmupSeconds))
	}
	if a.Environment.CPU != b.Environment.CPU || a.Environment.Memory != b.Environment.Memory {
		reasons = append(reasons, fmt.Sprintf("database resources %s/%s != %s/%s", a.Environment.CPU, a.Environment.Memory, b.Environment.CPU, b.Environment.Memory))
	}
	if !sameIDs(queryIDs(a), queryIDs(b)) {
		reasons = append(reasons, "query id sets differ")
	}
	return len(reasons) == 0, reasons
}

func closePct(a, b, pct float64) bool {
	if a == 0 && b == 0 {
		return true
	}
	base := math.Max(math.Abs(a), math.Abs(b))
	if base == 0 {
		return true
	}
	return math.Abs(a-b)/base <= pct
}

func queryIDs(rep report.Report) []string {
	ids := make([]string, 0, len(rep.Queries))
	for _, q := range rep.Queries {
		ids = append(ids, q.ID)
	}
	sort.Strings(ids)
	return ids
}

func sameIDs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func join(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += "; "
		}
		out += p
	}
	return out
}
