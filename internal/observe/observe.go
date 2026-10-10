// Package observe builds a k6 script, reads a k6 summary, registers metrics,
// and lists likely bottlenecks. Each exported function contains its whole rule set.
package observe

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/coderRaj07/benchforge/internal/report"
	"github.com/prometheus/client_golang/prometheus"
)

// Script is a constant-arrival-rate k6 program for one base URL.
func Script(url string, rate, durationSeconds, maxVUs int) string {
	if rate < 1 {
		rate = 1
	}
	if durationSeconds < 1 {
		durationSeconds = 1
	}
	if maxVUs < 1 {
		maxVUs = 1
	}
	return fmt.Sprintf(`import http from 'k6/http';
import { check } from 'k6';

export const options = {
  scenarios: {
    benchforge: {
      executor: 'constant-arrival-rate',
      rate: %d,
      timeUnit: '1s',
      duration: '%ds',
      preAllocatedVUs: %d,
      maxVUs: %d,
    },
  },
};

export default function () {
  const res = http.get('%s');
  check(res, { 'status is 200': (r) => r.status === 200 });
}
`, rate, durationSeconds, maxVUs, maxVUs, strings.ReplaceAll(url, "'", ""))
	// What: returns a k6 constant-arrival-rate script as text.
	// Used by: internal/observe/observe_test.go TestScriptAndParseK6. k6 is not executed here.
	// Logic: rate, durationSeconds, and maxVUs are raised to at least 1 before they are printed. The format string places rate, duration, maxVUs twice (preAllocatedVUs and maxVUs), then the URL with single quotes removed so the generated JavaScript string cannot be broken by a quote in the URL.
}

// K6 is the parsed summary of a k6 JSON export.
type K6 struct {
	P50Ms              float64
	P95Ms              float64
	P99Ms              float64
	RequestsPerSecond  float64
	ErrorRate          float64
	DroppedIterations  float64
	MaxVUs             float64
	GeneratorSaturated bool
	Reason             string
}

// ParseK6 reads a k6 handleSummary or --summary-export JSON document.
func ParseK6(body []byte) (K6, error) {
	var raw struct {
		Metrics map[string]struct {
			Values map[string]float64 `json:"values"`
		} `json:"metrics"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return K6{}, fmt.Errorf("parse k6 summary: %w", err)
	}
	val := func(metric, key string) float64 {
		return raw.Metrics[metric].Values[key]
	}
	out := K6{
		P50Ms:             val("http_req_duration", "med"),
		P95Ms:             val("http_req_duration", "p(95)"),
		P99Ms:             val("http_req_duration", "p(99)"),
		RequestsPerSecond: val("http_reqs", "rate"),
		ErrorRate:         val("http_req_failed", "rate"),
		DroppedIterations: val("dropped_iterations", "count"),
		MaxVUs:            val("vus_max", "max"),
	}
	if out.MaxVUs == 0 {
		out.MaxVUs = val("vus", "max")
	}
	if out.DroppedIterations > 0 {
		out.GeneratorSaturated = true
		out.Reason = fmt.Sprintf("k6 dropped %.0f iterations, so the load generator did not sustain the arrival rate", out.DroppedIterations)
	}
	return out, nil
	// What: reads a k6 summary JSON into latency, rate, errors, and a saturation flag.
	// Used by: internal/observe/observe_test.go TestScriptAndParseK6.
	// Logic: raw holds metrics[name].values[key]. val looks up one number and returns 0 when the key is missing. out is filled from http_req_duration, http_reqs, http_req_failed, and dropped_iterations. MaxVUs prefers vus_max and falls back to vus. DroppedIterations > 0 sets GeneratorSaturated and Reason.
}

// Diagnosis is one possible explanation. Wording stays at "likely".
type Diagnosis struct {
	Symptom       string   `json:"symptom"`
	Metrics       []string `json:"metrics"`
	Alternatives  []string `json:"alternatives"`
	Confidence    string   `json:"confidence"`
	Investigation string   `json:"investigation"`
	Remediation   string   `json:"remediation"`
	Confirm       string   `json:"confirm"`
}

// Diagnose applies every rule to one report. It does not declare a root cause.
func Diagnose(rep report.Report) []Diagnosis {
	var out []Diagnosis
	if rep.Load.GeneratorSaturated {
		out = append(out, Diagnosis{
			Symptom:       "likely load-generator saturation",
			Metrics:       []string{rep.Load.SaturationReason, fmt.Sprintf("dropped=%d", rep.Load.Dropped)},
			Alternatives:  []string{"the system under test is slow enough that the generator falls behind"},
			Confidence:    "medium",
			Investigation: "Check generator CPU and k6 dropped_iterations before changing the database.",
			Remediation:   "Give the generator more CPU or lower the arrival rate, then rerun.",
			Confirm:       "A repeat with generator CPU below the target and zero dropped iterations.",
		})
	}
	if rep.Load.PoolWait.P95 >= 20 && rep.Aggregate.Latency.P95 > rep.Load.PoolWait.P95 {
		out = append(out, Diagnosis{
			Symptom:       "likely connection-pool waiting",
			Metrics:       []string{fmt.Sprintf("pool wait p95 %.3fms", rep.Load.PoolWait.P95), fmt.Sprintf("query p95 %.3fms", rep.Aggregate.Latency.P95)},
			Alternatives:  []string{"query execution itself is slow", "the pool size is smaller than concurrency"},
			Confidence:    "medium",
			Investigation: "Compare pool wait with database execution time for the same window.",
			Remediation:   "Raise the pool only after confirming the database is not already out of connections.",
			Confirm:       "Pool wait drops while database active sessions stay under max_connections.",
		})
	}
	if rep.Database.BlksRead > 0 && rep.Database.CacheHitRatio < 0.9 {
		out = append(out, Diagnosis{
			Symptom:       "likely disk I/O pressure",
			Metrics:       []string{fmt.Sprintf("cache hit ratio %.3f", rep.Database.CacheHitRatio), fmt.Sprintf("blks_read %d", rep.Database.BlksRead)},
			Alternatives:  []string{"shared_buffers is smaller than the working set", "a sequential scan is reading cold pages"},
			Confidence:    "low",
			Investigation: "Look at the query plan and buffer counters for the slow statement.",
			Remediation:   "Do not raise memory until the plan shows the extra reads are expected.",
			Confirm:       "blks_read falls on a repeat with the same data in cache, or the plan changes to an index.",
		})
	}
	if rep.Database.Deadlocks > 0 {
		out = append(out, Diagnosis{
			Symptom:       "likely lock contention",
			Metrics:       []string{fmt.Sprintf("deadlocks %d", rep.Database.Deadlocks)},
			Alternatives:  []string{"the deadlocks are from the seed transaction rather than the measured mix"},
			Confidence:    "medium",
			Investigation: "Read pg_stat_database.deadlocks against the measured window only.",
			Remediation:   "Shorten transactions that update the same rows.",
			Confirm:       "A repeat with deadlocks staying at zero.",
		})
	}
	for _, q := range rep.Queries {
		if strings.Contains(q.Plan, "Seq Scan") {
			out = append(out, Diagnosis{
				Symptom:       "likely sequential scan on " + q.ID,
				Metrics:       []string{"plan contains Seq Scan", fmt.Sprintf("p95 %.3fms", q.Latency.P95)},
				Alternatives:  []string{"the table is small enough that a sequential scan is cheaper", "statistics are stale"},
				Confidence:    "medium",
				Investigation: "Compare this plan with the baseline plan for the same query id.",
				Remediation:   "Restore a predicate the index can use, then ANALYZE.",
				Confirm:       "EXPLAIN shows an index scan and p95 moves back toward the baseline.",
			})
		}
	}
	if rep.Aggregate.ErrorRate > rep.Assertions.MaxErrorRate && rep.Assertions.MaxErrorRate > 0 {
		out = append(out, Diagnosis{
			Symptom:       "likely error-rate pressure",
			Metrics:       []string{fmt.Sprintf("error rate %.4f", rep.Aggregate.ErrorRate)},
			Alternatives:  []string{"statement timeouts", "connection failures", "wrong SQL"},
			Confidence:    "low",
			Investigation: "Separate timeout errors from syntax errors in the error samples.",
			Remediation:   "Fix syntax before treating timeouts as a capacity problem.",
			Confirm:       "Error samples are timeouts and they disappear when the load is reduced.",
		})
	}
	return out
	// What: lists likely bottleneck explanations for one report. It does not name a root cause.
	// Used by: internal/observe/observe_test.go TestDiagnoseDoesNotClaimCertainty.
	// Logic: out grows by one Diagnosis per matching rule. GeneratorSaturated adds a generator item. Pool wait p95 at least 20ms, and larger than query p95, adds a pool item. Cache hit under 0.9 with blks_read > 0 adds an I/O item. Deadlocks > 0 adds a lock item. q is each query; a plan containing "Seq Scan" adds one item per such query. Error rate above the assertion adds an error-rate item. Each item's Symptom starts with "likely" and includes an alternative explanation.
}

// Metrics holds the low-cardinality series for a BenchForge process.
type Metrics struct {
	Registry        *prometheus.Registry
	Runs            *prometheus.CounterVec
	Failures        *prometheus.CounterVec
	Duration        prometheus.Histogram
	QueueDepth      prometheus.Gauge
	ActiveRuns      prometheus.Gauge
	CleanupFailures prometheus.Counter
	BudgetExceeded  prometheus.Counter
}

// NewMetrics registers the required series. Labels are outcome and repository only.
func NewMetrics() *Metrics {
	reg := prometheus.NewRegistry()
	m := &Metrics{
		Registry: reg,
		Runs: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "benchforge_runs_total",
			Help: "Benchmark runs by outcome.",
		}, []string{"outcome"}),
		Failures: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "benchforge_run_failures_total",
			Help: "Runs that failed or regressed, by outcome.",
		}, []string{"outcome"}),
		Duration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "benchforge_run_duration_seconds",
			Help:    "Wall-clock duration of a finished run.",
			Buckets: prometheus.DefBuckets,
		}),
		QueueDepth: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "benchforge_queue_depth",
			Help: "Jobs waiting to start.",
		}),
		ActiveRuns: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "benchforge_active_runs",
			Help: "Runs that have not been cleaned up.",
		}),
		CleanupFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "benchforge_cleanup_failures_total",
			Help: "Cleanup attempts that returned an error.",
		}),
		BudgetExceeded: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "benchforge_resource_budget_exceeded_total",
			Help: "Runs refused because the cost or resource budget was exceeded.",
		}),
	}
	reg.MustRegister(m.Runs, m.Failures, m.Duration, m.QueueDepth, m.ActiveRuns, m.CleanupFailures, m.BudgetExceeded)
	return m
	// What: creates a private Prometheus registry with the BenchForge series.
	// Used by: internal/observe/observe_test.go TestMetricNames. The API process does not serve these metrics yet.
	// Logic: reg is a new registry, not the global one. m points at the counters, histogram, and gauges. MustRegister attaches all seven series to reg. The only label is outcome, on the two counters. Gauges start at zero until a caller sets them.
}
