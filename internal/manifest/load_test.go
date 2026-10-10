package manifest

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLoadShippedManifests(t *testing.T) {
	root := repoRoot(t)
	files := []string{
		"workloads/manifests/orders-api.yaml",
		"workloads/manifests/orders-select.yaml",
		"workloads/manifests/orders-select-regression.yaml",
		"workloads/manifests/orders-pgbench.yaml",
	}
	for _, rel := range files {
		t.Run(rel, func(t *testing.T) {
			spec, err := Load(filepath.Join(root, rel), root, LocalLimits())
			if err != nil {
				t.Fatal(err)
			}
			if spec.Name == "" || spec.Fingerprint == "" {
				t.Fatalf("incomplete spec: %+v", spec)
			}
			if spec.DatabaseCPU > LocalLimits().MaxCPU {
				t.Fatal("cpu limit not enforced")
			}
		})
	}
}

func TestRejectsUnknownField(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, `
apiVersion: benchforge.dev/v1alpha1
kind: Benchmark
metadata:
  name: demo
target:
  type: postgres
dataset:
  profile: orders-mixed
  migrations: migrations
workload:
  type: query
queries:
  - id: get-customer-orders
    file: q.sql
surprise: true
`)
	os.WriteFile(filepath.Join(dir, "q.sql"), []byte("SELECT 1;\n"), 0o644)
	os.Mkdir(filepath.Join(dir, "migrations"), 0o755)
	os.WriteFile(filepath.Join(dir, "migrations", "001.sql"), []byte("SELECT 1;\n"), 0o644)
	_, err := Load(filepath.Join(dir, "bench.yaml"), dir, LocalLimits())
	if err == nil || !strings.Contains(err.Error(), "surprise") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}

func TestRejectsPathEscape(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, `
apiVersion: benchforge.dev/v1alpha1
kind: Benchmark
metadata:
  name: demo
target:
  type: postgres
dataset:
  profile: orders-mixed
  migrations: ../../../../etc
workload:
  type: query
queries:
  - id: get-customer-orders
    file: q.sql
`)
	os.WriteFile(filepath.Join(dir, "q.sql"), []byte("SELECT 1;\n"), 0o644)
	_, err := Load(filepath.Join(dir, "bench.yaml"), dir, LocalLimits())
	if err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("expected escape error, got %v", err)
	}
}

func TestRejectsUnlimitedCPU(t *testing.T) {
	dir := scaffold(t)
	writeManifest(t, dir, `
apiVersion: benchforge.dev/v1alpha1
kind: Benchmark
metadata:
  name: demo
target:
  type: postgres
dataset:
  profile: orders-mixed
  migrations: migrations
workload:
  type: query
queries:
  - id: get-customer-orders
    file: q.sql
resources:
  database:
    cpu: "64"
    memory: "1Gi"
`)
	_, err := Load(filepath.Join(dir, "bench.yaml"), dir, LocalLimits())
	if err == nil || !strings.Contains(err.Error(), "cpu") {
		t.Fatalf("expected cpu error, got %v", err)
	}
}

func TestSoakRequiresSixtySeconds(t *testing.T) {
	dir := scaffold(t)
	writeManifest(t, dir, `
apiVersion: benchforge.dev/v1alpha1
kind: Benchmark
metadata:
  name: demo
target:
  type: postgres
dataset:
  profile: orders-mixed
  migrations: migrations
workload:
  type: query
  durationSeconds: 30
  soak: true
queries:
  - id: get-customer-orders
    file: q.sql
`)
	_, err := Load(filepath.Join(dir, "bench.yaml"), dir, LocalLimits())
	if err == nil || !strings.Contains(err.Error(), "soak") {
		t.Fatalf("expected soak duration error, got %v", err)
	}
}

func TestRejectsFileReadSQL(t *testing.T) {
	dir := scaffold(t)
	os.WriteFile(filepath.Join(dir, "q.sql"), []byte("SELECT pg_read_file('/etc/passwd');\n"), 0o644)
	writeManifest(t, dir, `
apiVersion: benchforge.dev/v1alpha1
kind: Benchmark
metadata:
  name: demo
target:
  type: postgres
dataset:
  profile: orders-mixed
  migrations: migrations
workload:
  type: query
queries:
  - id: get-customer-orders
    file: q.sql
`)
	_, err := Load(filepath.Join(dir, "bench.yaml"), dir, LocalLimits())
	if err == nil || !strings.Contains(err.Error(), "pg_read_file") {
		t.Fatalf("expected denylist error, got %v", err)
	}
}

func TestParseCPUAndMemory(t *testing.T) {
	cpu, err := ParseCPU("500m")
	if err != nil || cpu != 0.5 {
		t.Fatalf("cpu %v %v", cpu, err)
	}
	mem, err := ParseMemory("4Gi")
	if err != nil || mem != 4<<30 {
		t.Fatalf("mem %v %v", mem, err)
	}
	if _, err := ParseMemory("4Ti"); err == nil {
		t.Fatal("expected unit error")
	}
}

func scaffold(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "migrations"), 0o755)
	os.WriteFile(filepath.Join(dir, "migrations", "001.sql"), []byte("SELECT 1;\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "q.sql"), []byte("SELECT 1;\n"), 0o644)
	return dir
}

func writeManifest(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "bench.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
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
