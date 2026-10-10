package baseline

import (
	"testing"

	"github.com/coderRaj07/benchforge/internal/report"
)

func baseReport() report.Report {
	return report.Report{
		RunID:        "base",
		ManifestName: "orders-select",
		Environment:  report.Environment{PostgresMajor: "17", CPU: "1", Memory: "1Gi"},
		Dataset:      report.Dataset{Profile: "orders-mixed", Seed: 42, Scale: 1},
		Workload:     report.Workload{Type: "query", DurationSeconds: 10, WarmupSeconds: 2, MaxConcurrency: 4},
		Aggregate: report.Aggregate{
			Iterations: 100,
			Errors:     0,
			ErrorRate:  0,
			Latency:    report.Latency{P50: 1, P95: 4, P99: 8, Mean: 1.5},
			TPS:        200,
		},
		Queries: []report.QueryResult{{
			ID:          "get-customer-orders",
			Fingerprint: "aaa",
			Iterations:  100,
			Latency:     report.Latency{P50: 1, P95: 4, P99: 8},
			TPS:         200,
		}},
		Assertions: report.Assertions{MaxP95Ms: 50, MaxP99Ms: 100, MaxErrorRate: 0.01, MaxRegressionPercent: 10, MinIterations: 30},
	}
}

func TestPassWithinThreshold(t *testing.T) {
	base := baseReport()
	cand := base
	cand.RunID = "cand"
	cand.Aggregate.Latency.P95 = 4.2
	cand.Queries[0].Latency.P95 = 4.2
	got := Evaluate(cand, &base)
	if got.Outcome != report.OutcomePass {
		t.Fatalf("outcome %s findings %+v", got.Outcome, got.Findings)
	}
}

func TestRegressionOnP95(t *testing.T) {
	base := baseReport()
	cand := base
	cand.RunID = "cand"
	cand.Queries = append([]report.QueryResult(nil), base.Queries...)
	cand.Aggregate.Latency.P95 = 9
	cand.Queries[0].Latency.P95 = 9
	cand.Queries[0].Fingerprint = "bbb"
	got := Evaluate(cand, &base)
	if got.Outcome != report.OutcomeRegression {
		t.Fatalf("outcome %s", got.Outcome)
	}
	if !hasCode(got.Findings, "latency_p95_regression") {
		t.Fatalf("missing regression finding: %+v", got.Findings)
	}
	if !hasCode(got.Findings, "sql_fingerprint_changed") {
		t.Fatal("expected fingerprint note")
	}
}

func TestAbsoluteSLO(t *testing.T) {
	cand := baseReport()
	cand.Aggregate.Latency.P95 = 80
	cand.Queries[0].Latency.P95 = 80
	got := Evaluate(cand, nil)
	if got.Outcome != report.OutcomeRegression || !hasCode(got.Findings, "latency_p95_absolute") {
		t.Fatalf("got %s %+v", got.Outcome, got.Findings)
	}
}

func TestIncompatibleBaseline(t *testing.T) {
	base := baseReport()
	cand := base
	cand.Dataset.Seed = 99
	got := Evaluate(cand, &base)
	if got.Outcome != report.OutcomeInconclusive {
		t.Fatalf("outcome %s", got.Outcome)
	}
}

func TestInsufficientSamples(t *testing.T) {
	cand := baseReport()
	cand.Aggregate.Iterations = 5
	cand.Queries[0].Iterations = 5
	got := Evaluate(cand, nil)
	if got.Outcome != report.OutcomeInconclusive {
		t.Fatalf("outcome %s", got.Outcome)
	}
}

func TestSaturatedIsInconclusive(t *testing.T) {
	cand := baseReport()
	cand.Aggregate.Latency.P95 = 500
	cand.Load.GeneratorSaturated = true
	cand.Load.SaturationReason = "load generator lagged"
	got := Evaluate(cand, nil)
	if got.Outcome != report.OutcomeInconclusive {
		t.Fatalf("outcome %s", got.Outcome)
	}
}

func TestNoSamplesIsError(t *testing.T) {
	cand := baseReport()
	cand.Aggregate.Iterations = 0
	cand.Queries[0].Iterations = 0
	got := Evaluate(cand, nil)
	if got.Outcome != report.OutcomeError {
		t.Fatalf("outcome %s", got.Outcome)
	}
}

func TestThroughputRegression(t *testing.T) {
	base := baseReport()
	cand := base
	cand.Aggregate.TPS = 100
	cand.Queries[0].TPS = 100
	got := Evaluate(cand, &base)
	if got.Outcome != report.OutcomeRegression || !hasCode(got.Findings, "throughput_regression") {
		t.Fatalf("got %s %+v", got.Outcome, got.Findings)
	}
}

func TestExecutionErrorWins(t *testing.T) {
	if FinalOutcome("container exited", Result{Outcome: report.OutcomeRegression}) != report.OutcomeError {
		t.Fatal("infrastructure failure must not be reported as a regression")
	}
}

func TestErrorRate(t *testing.T) {
	cand := baseReport()
	cand.Aggregate.Errors = 10
	cand.Aggregate.Iterations = 90
	cand.Aggregate.ErrorRate = 0.1
	got := Evaluate(cand, nil)
	if got.Outcome != report.OutcomeRegression || !hasCode(got.Findings, "error_rate") {
		t.Fatalf("got %s %+v", got.Outcome, got.Findings)
	}
}

func hasCode(findings []report.Finding, code string) bool {
	for _, f := range findings {
		if f.Code == code {
			return true
		}
	}
	return false
}
