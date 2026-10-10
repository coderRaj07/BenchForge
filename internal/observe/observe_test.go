package observe

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/coderRaj07/benchforge/internal/report"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestScriptAndParseK6(t *testing.T) {
	script := Script("http://127.0.0.1:8081/orders", 50, 30, 20)
	if !strings.Contains(script, "constant-arrival-rate") || !strings.Contains(script, "rate: 50") {
		t.Fatalf("script missing arrival rate:\n%s", script)
	}
	body := []byte(`{"metrics":{"http_req_duration":{"values":{"med":5,"p(95)":12,"p(99)":20}},"http_reqs":{"values":{"rate":48}},"http_req_failed":{"values":{"rate":0.01}},"dropped_iterations":{"values":{"count":7}}}}`)
	got, err := ParseK6(body)
	if err != nil {
		t.Fatal(err)
	}
	if got.P95Ms != 12 || !got.GeneratorSaturated || got.DroppedIterations != 7 {
		t.Fatalf("%+v", got)
	}
}

func TestDiagnoseDoesNotClaimCertainty(t *testing.T) {
	rep := report.Report{
		Aggregate:  report.Aggregate{Latency: report.Latency{P95: 80}, ErrorRate: 0.2},
		Assertions: report.Assertions{MaxErrorRate: 0.01},
		Load:       report.LoadStats{GeneratorSaturated: true, SaturationReason: "lagged", PoolWait: report.Latency{P95: 40}, Dropped: 3},
		Database:   report.DBStats{CacheHitRatio: 0.5, BlksRead: 1000, Deadlocks: 2},
		Queries:    []report.QueryResult{{ID: "get-customer-orders", Plan: "Seq Scan on orders", Latency: report.Latency{P95: 30}}},
	}
	found := Diagnose(rep)
	if len(found) < 4 {
		t.Fatalf("expected several likely causes, got %d", len(found))
	}
	for _, d := range found {
		if !strings.Contains(d.Symptom, "likely") {
			t.Fatalf("symptom must stay uncertain: %s", d.Symptom)
		}
		if len(d.Alternatives) == 0 || d.Confirm == "" {
			t.Fatalf("incomplete diagnosis: %+v", d)
		}
	}
}

func TestMetricNames(t *testing.T) {
	m := NewMetrics()
	m.Runs.WithLabelValues("PASS").Inc()
	m.Failures.WithLabelValues("ERROR").Inc()
	m.QueueDepth.Set(2)
	if err := testutil.CollectAndCompare(m.QueueDepth, strings.NewReader(`
# HELP benchforge_queue_depth Jobs waiting to start.
# TYPE benchforge_queue_depth gauge
benchforge_queue_depth 2
`)); err != nil {
		t.Fatal(err)
	}
	gathered, err := m.Registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, fam := range gathered {
		names[fam.GetName()] = true
	}
	for _, name := range []string{
		"benchforge_runs_total",
		"benchforge_run_failures_total",
		"benchforge_run_duration_seconds",
		"benchforge_queue_depth",
		"benchforge_active_runs",
		"benchforge_cleanup_failures_total",
		"benchforge_resource_budget_exceeded_total",
	} {
		if !names[name] {
			t.Fatalf("missing %s in %v", name, names)
		}
	}
	_, file, _, _ := runtime.Caller(0)
	dash, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "dashboards", "benchforge.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(dash), "benchforge_runs_total") || !strings.Contains(string(dash), "benchforge_queue_depth") {
		t.Fatal("dashboard is missing run metrics")
	}
}
