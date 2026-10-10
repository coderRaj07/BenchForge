package planner

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/coderRaj07/benchforge/internal/report"
)

func TestSelectSQLChange(t *testing.T) {
	root := repoRoot(t)
	plan, err := Select(root, []string{"workloads/postgres/sql/get_customer_orders.sql"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Broader || plan.Confidence != ConfidenceHigh {
		t.Fatalf("expected a confident specific plan: %+v", plan)
	}
	if !hasManifest(plan, "orders-select") || !hasManifest(plan, "orders-api") {
		t.Fatalf("indexed query should select the manifests that reference it: %+v", plan.Selections)
	}
	if hasManifest(plan, "orders-select-regression") && !querySelected(plan, "orders-select") {
		t.Fatal("missing query id")
	}
}

func TestUnmappedAppChangeRequiresBroaderSuite(t *testing.T) {
	root := repoRoot(t)
	plan, err := Select(root, []string{"app/orders/handler.go"})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Broader || plan.Confidence != ConfidenceLow {
		t.Fatalf("unmapped application change must not be skipped: %+v", plan)
	}
	if len(plan.Selections) < 4 {
		t.Fatalf("expected every manifest, got %d", len(plan.Selections))
	}
}

func TestDocsAreSkipped(t *testing.T) {
	root := repoRoot(t)
	plan, err := Select(root, []string{"docs/cli.md", "README.md"})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Skipped || len(plan.Selections) != 0 {
		t.Fatalf("docs should not start a benchmark: %+v", plan)
	}
}

func TestShippedSQLAnnotation(t *testing.T) {
	root := repoRoot(t)
	ids := annotationIDs(filepath.Join(root, "workloads/postgres/sql/get_customer_orders.sql"))
	if !contains(ids, "get-customer-orders") {
		t.Fatalf("ids = %v", ids)
	}
}

func TestMigrationChangeSelectsManifests(t *testing.T) {
	root := repoRoot(t)
	plan, err := Select(root, []string{"workloads/postgres/migrations/001_orders.sql"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Broader || len(plan.Selections) < 4 {
		t.Fatalf("migration should select every orders manifest: %+v", plan)
	}
}

func TestPullRequestMarkdownRegression(t *testing.T) {
	text := PullRequestMarkdown(&report.Report{
		Outcome:      report.OutcomeRegression,
		RunID:        "run-1",
		ManifestName: "orders-select",
		State:        "cleaned_up",
		Queries: []report.QueryResult{{
			ID:      "get-customer-orders",
			Latency: report.Latency{P95: 12, P99: 20},
			TPS:     100,
			Errors:  0,
		}},
		Findings: []report.Finding{{
			Severity: report.SeverityFail,
			Code:     "latency_p95_regression",
			QueryID:  "get-customer-orders",
			Message:  "p95 increased",
		}},
	})
	if !strings.Contains(text, "REGRESSION") || !strings.Contains(text, "latency_p95_regression") {
		t.Fatalf("markdown missing judgment:\n%s", text)
	}
	if strings.Contains(text, "password") {
		t.Fatal("unexpected secret-shaped text")
	}
}

func hasManifest(plan *Plan, name string) bool {
	for _, s := range plan.Selections {
		if s.Name == name {
			return true
		}
	}
	return false
}

func querySelected(plan *Plan, name string) bool {
	for _, s := range plan.Selections {
		if s.Name == name {
			for _, id := range s.QueryIDs {
				if id == "get-customer-orders" {
					return true
				}
			}
		}
	}
	return false
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
