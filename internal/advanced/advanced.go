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
}
