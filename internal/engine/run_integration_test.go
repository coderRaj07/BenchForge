//go:build integration

package engine

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/coderRaj07/benchforge/internal/baseline"
	"github.com/coderRaj07/benchforge/internal/report"
)

func TestEphemeralQueryRegressionAndCleanup(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip(err)
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skipf("docker daemon unavailable: %v", err)
	}
	root := repoRoot(t)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	base, err := Run(ctx, Options{
		ManifestPath:     filepath.Join(root, "workloads/manifests/orders-select.yaml"),
		Workspace:        root,
		DurationOverride: 8,
		ScaleOverride:    2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if base.Outcome == report.OutcomeError {
		t.Fatalf("baseline outcome error: %s", base.FailureReason)
	}
	if base.Aggregate.Iterations < 30 {
		t.Fatalf("too few samples: %+v", base.Aggregate)
	}
	if len(base.Queries) != 1 || !strings.Contains(base.Queries[0].Plan, "Index") {
		t.Fatalf("expected an index plan, got %q", planOf(base))
	}
	assertGone(t, base.RunID)

	cand, err := Run(ctx, Options{
		ManifestPath:     filepath.Join(root, "workloads/manifests/orders-select-regression.yaml"),
		Workspace:        root,
		DurationOverride: 8,
		ScaleOverride:    2,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertGone(t, cand.RunID)
	if !strings.Contains(planOf(cand), "Seq Scan") {
		t.Fatalf("expected a sequential scan, got %q", planOf(cand))
	}
	same := baseline.Evaluate(*base, base)
	if same.Outcome != report.OutcomePass {
		t.Fatalf("report compared with itself: %s %+v", same.Outcome, same.Findings)
	}
	got := baseline.Evaluate(*cand, base)
	if got.Outcome != report.OutcomeRegression {
		t.Fatalf("outcome %s findings %+v\nbase p95 %.3f cand p95 %.3f", got.Outcome, got.Findings, base.Aggregate.Latency.P95, cand.Aggregate.Latency.P95)
	}
}

func TestEphemeralPgbench(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip(err)
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skipf("docker daemon unavailable: %v", err)
	}
	root := repoRoot(t)
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	rep, err := Run(ctx, Options{
		ManifestPath:     filepath.Join(root, "workloads/manifests/orders-pgbench.yaml"),
		Workspace:        root,
		DurationOverride: 5,
		ScaleOverride:    1,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertGone(t, rep.RunID)
	if rep.Pgbench == nil || rep.Pgbench.Transactions < 30 || rep.Pgbench.TPS <= 0 {
		t.Fatalf("pgbench result %+v", rep.Pgbench)
	}
	if rep.Pgbench.LogSamples < 30 {
		t.Fatalf("expected pgbench latency log, got %d samples\n%s", rep.Pgbench.LogSamples, rep.Pgbench.RawSummary)
	}
}

func planOf(rep *report.Report) string {
	if rep == nil || len(rep.Queries) == 0 {
		return ""
	}
	return rep.Queries[0].Plan
}

func assertGone(t *testing.T, runID string) {
	t.Helper()
	out, err := exec.Command("docker", "ps", "-aq", "--filter", "label=benchforge.run_id="+runID).Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(out)) != "" {
		t.Fatalf("container for %s was not removed: %s", runID, out)
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
