package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/coderRaj07/benchforge/internal/report"
)

func TestValidateAndVersion(t *testing.T) {
	root := repoRoot(t)
	var stdout, stderr bytes.Buffer
	if code := Main([]string{"version"}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "benchforge") {
		t.Fatalf("version code %d out %s err %s", code, stdout.String(), stderr.String())
	}
	for _, rel := range []string{
		"workloads/manifests/orders-api.yaml",
		"workloads/manifests/orders-select.yaml",
		"workloads/manifests/orders-select-regression.yaml",
		"workloads/manifests/orders-pgbench.yaml",
	} {
		stdout.Reset()
		stderr.Reset()
		code := Main([]string{"validate", "-f", filepath.Join(root, rel), "-w", root}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("%s: %s", rel, stderr.String())
		}
	}
}

func TestCompareRegressionExitCode(t *testing.T) {
	dir := t.TempDir()
	base := sample("base", 4, 200)
	cand := sample("cand", 12, 100)
	basePath := filepath.Join(dir, "base.json")
	candPath := filepath.Join(dir, "cand.json")
	if err := report.Write(basePath, base); err != nil {
		t.Fatal(err)
	}
	if err := report.Write(candPath, cand); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := Main([]string{"compare", "--candidate", candPath, "--baseline", basePath}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d\n%s\n%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "REGRESSION") {
		t.Fatalf("summary missing regression: %s", stdout.String())
	}
}

func sample(id string, p95, tps float64) *report.Report {
	return &report.Report{
		SchemaVersion: report.SchemaVersion,
		RunID:         id,
		Outcome:       report.OutcomePass,
		ManifestName:  "orders-select",
		Environment:   report.Environment{PostgresMajor: "17", CPU: "1", Memory: "1Gi"},
		Dataset:       report.Dataset{Profile: "orders-mixed", Seed: 42, Scale: 1},
		Workload:      report.Workload{Type: "query", DurationSeconds: 15, WarmupSeconds: 2, MaxConcurrency: 4},
		Aggregate: report.Aggregate{
			Iterations: 100,
			Latency:    report.Latency{P50: 1, P95: p95, P99: p95 + 1},
			TPS:        tps,
		},
		Queries: []report.QueryResult{{
			ID:          "get-customer-orders",
			Fingerprint: id,
			Iterations:  100,
			Latency:     report.Latency{P50: 1, P95: p95, P99: p95 + 1},
			TPS:         tps,
		}},
		Assertions: report.Assertions{MaxP95Ms: 500, MaxP99Ms: 800, MaxErrorRate: 0.01, MaxRegressionPercent: 20, MinIterations: 30},
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
