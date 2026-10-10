package advanced

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/coderRaj07/benchforge/internal/report"
)

func TestRouteSkewCostAndCompare(t *testing.T) {
	if err := CheckRoute("targeted", 1, 4); err != nil {
		t.Fatal(err)
	}
	if err := CheckRoute("targeted", 3, 4); err == nil {
		t.Fatal("targeted query was allowed to touch 3 shards")
	}
	if err := CheckRoute("scatter", 4, 4); err != nil {
		t.Fatal(err)
	}
	if err := CheckRoute("aggregate", 2, 4); err == nil {
		t.Fatal("partial aggregate was accepted")
	}
	weights, err := Skew(4, 0.7)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(weights[0]-0.7) > 1e-9 {
		t.Fatalf("hot share %v", weights[0])
	}
	sum := 0.0
	for _, w := range weights {
		sum += w
	}
	if math.Abs(sum-1) > 1e-9 {
		t.Fatalf("weights sum %v", sum)
	}
	got := CostPerRun(2, 4, 3600, 0, 0)
	if math.Abs(got-(2*0.04+4*0.005)) > 1e-9 {
		t.Fatalf("cost %v", got)
	}
	ratio, note := CompareEnvironments(
		report.Report{Aggregate: report.Aggregate{Latency: report.Latency{P95: 20}}, Environment: report.Environment{CPU: "2", Memory: "4Gi"}},
		report.Report{Aggregate: report.Aggregate{Latency: report.Latency{P95: 10}}, Environment: report.Environment{CPU: "1", Memory: "1Gi"}},
	)
	if ratio != 2 || !strings.Contains(note, "not a regression") {
		t.Fatalf("ratio %v note %s", ratio, note)
	}
}

func TestCaptureCPU(t *testing.T) {
	buf, err := CaptureCPU(50 * time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if len(buf) == 0 {
		t.Fatal("empty profile")
	}
	skipped, err := CaptureCPU(0)
	if err != nil || skipped != nil {
		t.Fatalf("disabled capture returned %d bytes, err %v", len(skipped), err)
	}
}
