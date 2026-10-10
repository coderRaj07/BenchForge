package stats

import "testing"

func TestPercentileNearestRank(t *testing.T) {
	samples := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	got := Summarize(samples)
	if got.P50 != 5 {
		t.Fatalf("p50 = %v", got.P50)
	}
	if got.Max != 10 {
		t.Fatalf("max = %v", got.Max)
	}
	if got.P95 != 10 {
		t.Fatalf("p95 = %v, want 10", got.P95)
	}
	if got.Mean != 5.5 {
		t.Fatalf("mean = %v", got.Mean)
	}
}

func TestSummarizeEmpty(t *testing.T) {
	got := Summarize(nil)
	if got.P95 != 0 || got.Mean != 0 {
		t.Fatalf("expected zeros, got %+v", got)
	}
}

func TestReservoirKeepsAllWhenSmall(t *testing.T) {
	r := NewReservoir(10, 1)
	for i := 0; i < 5; i++ {
		r.Add(float64(i))
	}
	vals, seen := r.Values()
	if seen != 5 || len(vals) != 5 {
		t.Fatalf("seen=%d len=%d", seen, len(vals))
	}
	if r.Approximate() {
		t.Fatal("small sample should be exact")
	}
}

func TestReservoirBoundsMemory(t *testing.T) {
	r := NewReservoir(100, 7)
	for i := 0; i < 10000; i++ {
		r.Add(float64(i))
	}
	vals, seen := r.Values()
	if seen != 10000 || len(vals) != 100 {
		t.Fatalf("seen=%d len=%d", seen, len(vals))
	}
	if !r.Approximate() {
		t.Fatal("expected approximate sample")
	}
}

func TestHistogramCumulative(t *testing.T) {
	h := NewHistogram()
	h.Add(0.5)
	h.Add(3)
	h.Add(40)
	if h.Counts[0] != 1 {
		t.Fatalf("le 1ms = %d", h.Counts[0])
	}
	if h.Counts[2] != 2 { // le 5 includes 0.5 and 3
		t.Fatalf("le 5ms = %d", h.Counts[2])
	}
	if h.Total != 3 {
		t.Fatalf("total = %d", h.Total)
	}
}
