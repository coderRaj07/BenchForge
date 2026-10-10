// Package capacity estimates a sustainable rate from measured samples.
package capacity

import "fmt"

// Sample is one step of a sweep. CPUSeconds is CPU time per request, not a host total.
type Sample struct {
	Offered            float64
	Achieved           float64
	MeanLatencyMs      float64
	P95Ms              float64
	ErrorRate          float64
	CPUSeconds         float64
	MemoryBytes        float64
	GeneratorSaturated bool
}

// SLO is the bound for a sample to count as sustainable.
type SLO struct {
	MaxP95Ms      float64
	MaxErrorRate  float64
	TargetCPUUtil float64
	Headroom      float64
}

// Result separates what was measured from what is estimated.
type Result struct {
	SustainableRPS     float64
	LittlesConcurrency float64
	CPUCores           float64
	MemoryBytes        float64
	Confidence         string
	FirstSaturation    string
	Assumptions        []string
	Valid              bool
}

// Analyze walks the samples in order. The body of this function is the whole model.
func Analyze(samples []Sample, slo SLO) Result {
	out := Result{
		Confidence: "low",
		Assumptions: []string{
			"Little's Law uses achieved requests per second times mean latency.",
			"CPU cores use requests per second times measured CPU seconds per request, divided by the target utilization.",
			"Memory uses the largest measured working set. It is not inferred from the request rate.",
			"A sample with a saturated generator is not a sustainable rate.",
		},
	}
	if slo.TargetCPUUtil <= 0 || slo.TargetCPUUtil > 1 {
		slo.TargetCPUUtil = 0.7
	}
	if slo.Headroom < 0 {
		slo.Headroom = 0
	}
	valid := 0
	var best Sample
	var mem float64
	for _, sample := range samples {
		if sample.MemoryBytes > mem {
			mem = sample.MemoryBytes
		}
		if sample.GeneratorSaturated {
			if out.FirstSaturation == "" {
				out.FirstSaturation = fmt.Sprintf("generator saturated at offered %.2f rps", sample.Offered)
			}
			continue
		}
		breaks := (slo.MaxP95Ms > 0 && sample.P95Ms > slo.MaxP95Ms) || (slo.MaxErrorRate > 0 && sample.ErrorRate > slo.MaxErrorRate)
		if breaks {
			if out.FirstSaturation == "" {
				out.FirstSaturation = fmt.Sprintf("service limit at offered %.2f rps (p95 %.2fms, error rate %.4f)", sample.Offered, sample.P95Ms, sample.ErrorRate)
			}
			continue
		}
		valid++
		if sample.Achieved >= best.Achieved {
			best = sample
		}
	}
	if valid == 0 {
		out.Assumptions = append(out.Assumptions, "no sample satisfied the SLO without generator saturation")
		return out
	}
	out.Valid = true
	out.SustainableRPS = best.Achieved
	out.LittlesConcurrency = best.Achieved * (best.MeanLatencyMs / 1000)
	out.CPUCores = best.Achieved * best.CPUSeconds / slo.TargetCPUUtil * (1 + slo.Headroom)
	out.MemoryBytes = mem * (1 + slo.Headroom)
	if valid >= 3 {
		out.Confidence = "medium"
	}
	if valid >= 5 && out.FirstSaturation != "" {
		out.Confidence = "high"
	}
	return out
}
