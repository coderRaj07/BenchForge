package cloud

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestArtifactRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := Put(dir, "report.json", []byte(`{"ok":true}`)); err != nil {
		t.Fatal(err)
	}
	got, err := Get(dir, "report.json")
	if err != nil || !bytes.Equal(got, []byte(`{"ok":true}`)) {
		t.Fatalf("%s %v", got, err)
	}
	if err := Put(dir, "../escape", []byte("no")); err == nil {
		t.Fatal("path escape was accepted")
	}
}

func TestBudgetAndExpiry(t *testing.T) {
	cost := EstimateUSD(2, 8, 0.04, 0.005)
	if cost != 2*0.04+8*0.005 {
		t.Fatalf("cost %v", cost)
	}
	if err := Allow(cost, 0.05); err == nil {
		t.Fatal("over budget was allowed")
	}
	if err := Allow(cost, 10); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	got := Expired(now, time.Hour, []Resource{
		{ID: "old", RunID: "r1", Created: now.Add(-2 * time.Hour), Managed: true},
		{ID: "fresh", RunID: "r2", Created: now.Add(-10 * time.Minute), Managed: true},
		{ID: "foreign", RunID: "r3", Created: now.Add(-3 * time.Hour), Managed: false},
		{ID: "nolabel", Created: now.Add(-3 * time.Hour), Managed: true},
	})
	if len(got) != 1 || got[0].ID != "old" {
		t.Fatalf("%+v", got)
	}
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..")
	tf, err := os.ReadFile(filepath.Join(root, "deploy", "terraform", "main.tf"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(tf)
	for _, needle := range []string{"aws_s3_bucket", "aws_sqs_queue", "aws_ecr_repository", "aws_iam_role", "aws_secretsmanager_secret"} {
		if !strings.Contains(text, needle) {
			t.Fatalf("terraform missing %s", needle)
		}
	}
	if strings.Contains(text, "local-exec") {
		t.Fatal("module must not run local-exec")
	}
	job, err := os.ReadFile(filepath.Join(root, "deploy", "kubernetes", "benchmark-job.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(job), "ttlSecondsAfterFinished") || !strings.Contains(string(job), "memory: 1Gi") {
		t.Fatal("job is missing ttl or memory limit")
	}
}
