// Package advanced holds the shard check, tenant skew, profile capture,
// environment comparison, and cost-per-run calculation.
package advanced

import (
	"bytes"
	"fmt"
	"runtime/pprof"
	"time"

	"github.com/coderRaj07/benchforge/internal/report"
)

// CheckRoute checks a single-node description of shard contact. It does not run a distributed database.
func CheckRoute(kind string, contacted, expected int) error {
	switch kind {
	case "targeted":
		if contacted != 1 {
			return fmt.Errorf("targeted query contacted %d shards, want 1", contacted)
		}
	case "scatter", "aggregate":
		if expected < 1 {
			return fmt.Errorf("%s query needs an expected shard count", kind)
		}
		if contacted != expected {
			return fmt.Errorf("%s query contacted %d shards, want %d", kind, contacted, expected)
		}
	default:
		return fmt.Errorf("route kind %q must be targeted, scatter, or aggregate", kind)
	}
	return nil
	// What: checks that a query touched the right number of shards.
	// Used by: internal/advanced/advanced_test.go TestRouteSkewCostAndCompare. Nothing in the runner calls it yet.
	// Logic: kind selects the rule. contacted is how many shards the caller says were hit. expected is how many should have been hit. targeted ignores expected and requires contacted == 1. scatter and aggregate require expected >= 1 and contacted == expected. Any other kind returns an error. nil means the counts match.
}

// Skew gives the first tenant hotShare of the traffic and splits the rest evenly.
func Skew(tenants int, hotShare float64) ([]float64, error) {
	if tenants < 2 {
		return nil, fmt.Errorf("skew needs at least 2 tenants")
	}
	if hotShare <= 0 || hotShare >= 1 {
		return nil, fmt.Errorf("hot share must be between 0 and 1")
	}
	out := make([]float64, tenants)
	out[0] = hotShare
	each := (1 - hotShare) / float64(tenants-1)
	for i := 1; i < tenants; i++ {
		out[i] = each
	}
	return out, nil
	// What: builds per-tenant traffic weights with one hot tenant.
	// Used by: internal/advanced/advanced_test.go TestRouteSkewCostAndCompare.
	// Logic: tenants is the slice length. hotShare is the fraction stored in out[0]. each is the leftover share split across the other tenants, written into out[i] by the loop. The returned slice sums to 1. Values outside (0,1) or tenants < 2 return an error and no slice.
}

// CostPerRun converts measured CPU and memory over the run into dollars.
// A zero price uses the documented assumption of $0.04 per CPU-hour and $0.005 per GiB-hour.
func CostPerRun(cpuCores, memoryGiB, seconds, cpuHourly, memoryHourly float64) float64 {
	if cpuHourly == 0 {
		cpuHourly = 0.04
	}
	if memoryHourly == 0 {
		memoryHourly = 0.005
	}
	hours := seconds / 3600
	return cpuCores*hours*cpuHourly + memoryGiB*hours*memoryHourly
	// What: turns measured CPU and memory over a run into a dollar figure.
	// Used by: internal/advanced/advanced_test.go TestRouteSkewCostAndCompare.
	// Logic: cpuHourly and memoryHourly are overwritten with 0.04 and 0.005 when the caller passes 0. hours is seconds/3600. The return multiplies cores and GiB by hours and those prices and adds the two products.
}

// CompareEnvironments reports the p95 ratio and the resource difference.
// Different CPU or memory is a comparison, not a regression.
func CompareEnvironments(a, b report.Report) (float64, string) {
	ratio := 0.0
	if b.Aggregate.Latency.P95 > 0 {
		ratio = a.Aggregate.Latency.P95 / b.Aggregate.Latency.P95
	}
	note := fmt.Sprintf("p95 ratio %.3f", ratio)
	if a.Environment.CPU != b.Environment.CPU || a.Environment.Memory != b.Environment.Memory {
		note += fmt.Sprintf("; resources %s/%s versus %s/%s, so this is not a regression verdict", a.Environment.CPU, a.Environment.Memory, b.Environment.CPU, b.Environment.Memory)
	}
	return ratio, note
	// What: compares two reports when the machines may differ.
	// Used by: internal/advanced/advanced_test.go TestRouteSkewCostAndCompare. baseline.Evaluate is a separate gate and does not call this.
	// Logic: ratio starts at 0 and becomes a.p95/b.p95 when b.p95 is positive. note starts as that ratio. If CPU or memory text differs, note is appended with both resource pairs and the words "not a regression".
}

// CaptureCPU records a CPU profile for the given duration. Pass zero to skip.
func CaptureCPU(d time.Duration) ([]byte, error) {
	if d <= 0 {
		return nil, nil
	}
	var buf bytes.Buffer
	if err := pprof.StartCPUProfile(&buf); err != nil {
		return nil, err
	}
	time.Sleep(d)
	pprof.StopCPUProfile()
	return buf.Bytes(), nil
	// What: writes a Go CPU profile for duration d, or returns nil when d is not positive.
	// Used by: internal/advanced/advanced_test.go TestCaptureCPU. Manifests do not call it automatically.
	// Logic: d <= 0 returns immediately. buf receives profile bytes from pprof.StartCPUProfile. time.Sleep holds the process so the profiler has samples. StopCPUProfile flushes buf. The returned slice is buf.Bytes().
}
