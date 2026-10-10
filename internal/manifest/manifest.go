// Package manifest loads and validates BenchForge benchmark manifests.
package manifest

import "time"

const (
	// APIVersion is the only manifest version accepted by this release.
	APIVersion = "benchforge.dev/v1alpha1"
	// KindBenchmark is the only supported document kind.
	KindBenchmark = "Benchmark"
)

// Limits bound what a manifest is allowed to ask for.
// A manifest cannot raise these limits.
type Limits struct {
	MaxCPU             float64
	MaxMemoryBytes     int64
	MaxDurationSeconds int
	MaxWarmupSeconds   int
	MaxRampSeconds     int
	MaxConcurrency     int
	MaxArrivalRate     int
	MaxScale           float64
	MaxQueries         int
	MaxSQLBytes        int
	MinDurationSeconds int
}

// LocalLimits is the policy for a developer workstation.
func LocalLimits() Limits {
	return Limits{
		MaxCPU:             4,
		MaxMemoryBytes:     4 << 30,
		MaxDurationSeconds: 1800,
		MinDurationSeconds: 1,
		MaxWarmupSeconds:   600,
		MaxRampSeconds:     600,
		MaxConcurrency:     128,
		MaxArrivalRate:     20000,
		MaxScale:           20,
		MaxQueries:         32,
		MaxSQLBytes:        64 << 10,
	}
}

// Benchmark is a normalized, validated manifest.
type Benchmark struct {
	APIVersion         string
	Kind               string
	Name               string
	Owner              string
	Annotations        map[string]string
	TargetType         string
	Source             string
	PostgresVersion    string
	Seed               int64
	Scale              float64
	Profile            string
	WarmupSeconds      int
	Migrations         string
	WorkloadType       string
	DurationSeconds    int
	ArrivalRate        int
	MaxConcurrency     int
	RampUpSeconds      int
	Clients            int
	Threads            int
	Script             string
	Queries            []Query
	Assertions         Assertions
	DatabaseCPU        float64
	DatabaseMemory     int64
	DatabaseCPUText    string
	DatabaseMemoryText string
	LoadCPU            float64
	LoadMemory         int64
	LoadCPUText        string
	LoadMemoryText     string
	TTLSeconds         int
	BaselineFile       string
	Fingerprint        string
	Dir                string
	Path               string
}

// Query is one statement group in a workload mix.
type Query struct {
	ID            string
	File          string
	Weight        int
	Timeout       time.Duration
	Transactional bool
	Params        []Param
	SQL           string
	Statements    []string
	Fingerprint   string
}

// Param describes one bound parameter.
type Param struct {
	Name         string
	Type         string
	Distribution string
	Min          int64
	Max          int64
	MaxFrom      string
	Value        string
}

// Assertions are the gates applied after a run.
type Assertions struct {
	MaxP95Ms             float64
	MaxP99Ms             float64
	MaxErrorRate         float64
	MaxRegressionPercent float64
	MinIterations        int
}

// ValidationError points at the field a developer needs to change.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

// MultiError is a list of validation failures.
type MultiError struct {
	Errs []*ValidationError
}

func (m *MultiError) Error() string {
	if m == nil || len(m.Errs) == 0 {
		return "manifest validation failed"
	}
	b := "manifest validation failed:"
	for _, e := range m.Errs {
		b += "\n  - " + e.Error()
	}
	return b
}

func (m *MultiError) add(field, message string) {
	m.Errs = append(m.Errs, &ValidationError{Field: field, Message: message})
}
