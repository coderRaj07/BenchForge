// Package stats summarizes latency samples for benchmark reports.
package stats

import (
	"math"
	"math/rand/v2"
	"sort"
)

// Latency is a millisecond summary. Zero values mean no samples.
type Latency struct {
	P50  float64
	P95  float64
	P99  float64
	Mean float64
	Max  float64
}

// Summarize computes nearest-rank percentiles from a copy of samples.
// The rank for percentile p is ceil(p/100 * n), clamped to the last sample.
func Summarize(samples []float64) Latency {
	if len(samples) == 0 {
		return Latency{}
	}
	cp := append([]float64(nil), samples...)
	sort.Float64s(cp)
	var sum float64
	for _, v := range cp {
		sum += v
	}
	return Latency{
		P50:  percentile(cp, 50),
		P95:  percentile(cp, 95),
		P99:  percentile(cp, 99),
		Mean: sum / float64(len(cp)),
		Max:  cp[len(cp)-1],
	}
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 100 {
		return sorted[len(sorted)-1]
	}
	rank := int(math.Ceil(p/100*float64(len(sorted)))) - 1
	if rank < 0 {
		rank = 0
	}
	if rank >= len(sorted) {
		rank = len(sorted) - 1
	}
	return sorted[rank]
}

// Reservoir keeps a bounded uniform sample so long runs stay in memory.
type Reservoir struct {
	limit int
	data  []float64
	seen  int
	rng   *rand.Rand
}

// NewReservoir returns a sampler that stores at most limit values.
func NewReservoir(limit int, seed uint64) *Reservoir {
	if limit < 1 {
		limit = 1
	}
	return &Reservoir{
		limit: limit,
		rng:   rand.New(rand.NewPCG(seed, 0x9e3779b97f4a7c15)),
	}
}

// Add records one observation.
func (r *Reservoir) Add(v float64) {
	r.seen++
	if len(r.data) < r.limit {
		r.data = append(r.data, v)
		return
	}
	j := r.rng.IntN(r.seen)
	if j < r.limit {
		r.data[j] = v
	}
}

// Values returns the retained sample. Seen reports how many observations were offered.
func (r *Reservoir) Values() ([]float64, int) {
	return append([]float64(nil), r.data...), r.seen
}

// Approximate reports whether percentiles are estimated from a subset.
func (r *Reservoir) Approximate() bool {
	return r.seen > len(r.data)
}

// Histogram counts observations into cumulative millisecond buckets.
type Histogram struct {
	Bounds []float64
	Counts []int
	Total  int
}

// NewHistogram returns cumulative buckets from 1ms through 5s, plus an overflow bucket.
func NewHistogram() Histogram {
	bounds := []float64{1, 2, 5, 10, 20, 50, 100, 200, 500, 1000, 2000, 5000, math.Inf(1)}
	return Histogram{Bounds: bounds, Counts: make([]int, len(bounds))}
}

// Add increments every bucket whose upper bound is at least v.
func (h *Histogram) Add(v float64) {
	h.Total++
	for i, b := range h.Bounds {
		if v <= b {
			for j := i; j < len(h.Counts); j++ {
				h.Counts[j]++
			}
			return
		}
	}
}

// Round3 rounds a millisecond value for stable JSON.
func Round3(v float64) float64 {
	return math.Round(v*1000) / 1000
}
