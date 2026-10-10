package capacity

import "testing"

func TestAnalyzeUsesMeasuredSamples(t *testing.T) {
	got := Analyze([]Sample{
		{Offered: 50, Achieved: 50, MeanLatencyMs: 20, P95Ms: 40, CPUSeconds: 0.002, MemoryBytes: 1000},
		{Offered: 100, Achieved: 100, MeanLatencyMs: 25, P95Ms: 80, CPUSeconds: 0.002, MemoryBytes: 1500},
		{Offered: 200, Achieved: 120, MeanLatencyMs: 400, P95Ms: 900, ErrorRate: 0.2, CPUSeconds: 0.01, MemoryBytes: 2000},
		{Offered: 400, Achieved: 10, GeneratorSaturated: true, MemoryBytes: 2000},
	}, SLO{MaxP95Ms: 100, MaxErrorRate: 0.01, TargetCPUUtil: 0.5, Headroom: 0.2})
	if !got.Valid || got.SustainableRPS != 100 {
		t.Fatalf("%+v", got)
	}
	if got.LittlesConcurrency != 100*(0.025) {
		t.Fatalf("little %v", got.LittlesConcurrency)
	}
	wantCPU := 100 * 0.002 / 0.5 * 1.2
	if got.CPUCores != wantCPU {
		t.Fatalf("cpu %v want %v", got.CPUCores, wantCPU)
	}
	if got.MemoryBytes != 2000*1.2 {
		t.Fatalf("memory came from the rate instead of the working set: %v", got.MemoryBytes)
	}
	if got.FirstSaturation == "" || got.Confidence != "low" {
		t.Fatalf("two valid samples stay low confidence: %+v", got)
	}
}

func TestSaturatedGeneratorIsNotCapacity(t *testing.T) {
	got := Analyze([]Sample{{Offered: 10, Achieved: 10, GeneratorSaturated: true}}, SLO{MaxP95Ms: 100})
	if got.Valid {
		t.Fatal("saturated sample was treated as capacity")
	}
}
