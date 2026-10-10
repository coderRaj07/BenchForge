// Package report is the JSON document produced by a benchmark run.
package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// SchemaVersion identifies the report document.
const SchemaVersion = "benchforge.report/v1"

const (
	OutcomePass         = "PASS"
	OutcomeRegression   = "REGRESSION"
	OutcomeInconclusive = "INCONCLUSIVE"
	OutcomeError        = "ERROR"

	SeverityInfo    = "info"
	SeverityWarning = "warning"
	SeverityFail    = "fail"
)

// Report is the reproducible record of one benchmark run.
type Report struct {
	SchemaVersion       string         `json:"schemaVersion"`
	RunID               string         `json:"runId"`
	State               string         `json:"state"`
	Outcome             string         `json:"outcome"`
	FailureReason       string         `json:"failureReason,omitempty"`
	CleanupError        string         `json:"cleanupError,omitempty"`
	ManifestName        string         `json:"manifestName"`
	ManifestPath        string         `json:"manifestPath,omitempty"`
	ManifestFingerprint string         `json:"manifestFingerprint,omitempty"`
	StartedAt           time.Time      `json:"startedAt"`
	FinishedAt          time.Time      `json:"finishedAt"`
	Timeline            []Mark         `json:"timeline,omitempty"`
	Environment         Environment    `json:"environment"`
	Dataset             Dataset        `json:"dataset"`
	Workload            Workload       `json:"workload"`
	Queries             []QueryResult  `json:"queries,omitempty"`
	Pgbench             *PgbenchResult `json:"pgbench,omitempty"`
	Aggregate           Aggregate      `json:"aggregate"`
	Database            DBStats        `json:"database"`
	Load                LoadStats      `json:"load"`
	Assertions          Assertions     `json:"assertions"`
	Findings            []Finding      `json:"findings,omitempty"`
	Comparison          *Comparison    `json:"comparison,omitempty"`
	Notes               []string       `json:"notes,omitempty"`
}

// Mark is one lifecycle timestamp.
type Mark struct {
	State string    `json:"state"`
	At    time.Time `json:"at"`
}

// Environment describes the database the run actually used.
type Environment struct {
	Source          string `json:"source"`
	PostgresVersion string `json:"postgresVersion,omitempty"`
	PostgresMajor   string `json:"postgresMajor,omitempty"`
	Image           string `json:"image,omitempty"`
	ImageID         string `json:"imageId,omitempty"`
	MaxConnections  int    `json:"maxConnections,omitempty"`
	SharedBuffers   string `json:"sharedBuffers,omitempty"`
	CPU             string `json:"cpu,omitempty"`
	Memory          string `json:"memory,omitempty"`
}

// Dataset is the seeded data identity used for baseline compatibility.
type Dataset struct {
	Profile    string  `json:"profile"`
	Seed       int64   `json:"seed"`
	Scale      float64 `json:"scale"`
	Customers  int64   `json:"customers,omitempty"`
	Products   int64   `json:"products,omitempty"`
	Orders     int64   `json:"orders,omitempty"`
	OrderItems int64   `json:"orderItems,omitempty"`
}

// Workload is the effective load configuration.
type Workload struct {
	Type                 string `json:"type"`
	DurationSeconds      int    `json:"durationSeconds"`
	WarmupSeconds        int    `json:"warmupSeconds"`
	RampUpSeconds        int    `json:"rampUpSeconds,omitempty"`
	MaxConcurrency       int    `json:"maxConcurrency"`
	ArrivalRatePerSecond int    `json:"arrivalRatePerSecond,omitempty"`
	Clients              int    `json:"clients,omitempty"`
	Threads              int    `json:"threads,omitempty"`
}

// QueryResult is one query's measured distribution.
type QueryResult struct {
	ID           string   `json:"id"`
	Fingerprint  string   `json:"fingerprint"`
	Iterations   int      `json:"iterations"`
	Errors       int      `json:"errors"`
	ErrorRate    float64  `json:"errorRate"`
	Rows         int64    `json:"rows"`
	Latency      Latency  `json:"latencyMs"`
	TPS          float64  `json:"tps"`
	Approximate  bool     `json:"approximate,omitempty"`
	Histogram    []Bucket `json:"histogram,omitempty"`
	ErrorSamples []string `json:"errorSamples,omitempty"`
	Plan         string   `json:"plan,omitempty"`
	PoolWait     Latency  `json:"poolWaitMs"`
}

// Aggregate rolls every successful query sample into one distribution.
type Aggregate struct {
	Iterations int     `json:"iterations"`
	Errors     int     `json:"errors"`
	ErrorRate  float64 `json:"errorRate"`
	Latency    Latency `json:"latencyMs"`
	TPS        float64 `json:"tps"`
}

// Latency is a millisecond summary.
type Latency struct {
	P50  float64 `json:"p50"`
	P95  float64 `json:"p95"`
	P99  float64 `json:"p99"`
	Mean float64 `json:"mean"`
	Max  float64 `json:"max"`
}

// Bucket is a cumulative latency histogram bucket.
type Bucket struct {
	LeMs  float64 `json:"leMs"`
	Count int     `json:"count"`
}

// PgbenchResult is parsed from pgbench stdout and its transaction log.
type PgbenchResult struct {
	Transactions    int     `json:"transactions"`
	Failed          int     `json:"failed"`
	Skipped         int     `json:"skipped,omitempty"`
	TPS             float64 `json:"tps"`
	Latency         Latency `json:"latencyMs"`
	StddevMs        float64 `json:"stddevMs,omitempty"`
	Clients         int     `json:"clients"`
	Threads         int     `json:"threads"`
	DurationSeconds int     `json:"durationSeconds"`
	LogSamples      int     `json:"logSamples"`
	RawSummary      string  `json:"rawSummary,omitempty"`
}

// DBStats holds deltas sampled from the isolated database.
type DBStats struct {
	MaxActiveSessions int     `json:"maxActiveSessions"`
	XactCommit        int64   `json:"xactCommit"`
	XactRollback      int64   `json:"xactRollback"`
	BlksHit           int64   `json:"blksHit"`
	BlksRead          int64   `json:"blksRead"`
	CacheHitRatio     float64 `json:"cacheHitRatio"`
	Deadlocks         int64   `json:"deadlocks"`
	TupReturned       int64   `json:"tupReturned"`
}

// LoadStats describes whether the runner kept up with the requested schedule.
type LoadStats struct {
	Mode               string  `json:"mode"`
	Offered            int     `json:"offered"`
	Completed          int     `json:"completed"`
	Errors             int     `json:"errors"`
	Dropped            int     `json:"dropped"`
	LateTicks          int     `json:"lateTicks"`
	Ticks              int     `json:"ticks"`
	GeneratorSaturated bool    `json:"generatorSaturated"`
	SaturationReason   string  `json:"saturationReason,omitempty"`
	PoolWait           Latency `json:"poolWaitMs"`
}

// Assertions are the gates that were applied.
type Assertions struct {
	MaxP95Ms             float64 `json:"maxP95Ms,omitempty"`
	MaxP99Ms             float64 `json:"maxP99Ms,omitempty"`
	MaxErrorRate         float64 `json:"maxErrorRate"`
	MaxRegressionPercent float64 `json:"maxRegressionPercent"`
	MinIterations        int     `json:"minIterations"`
}

// Finding is one conclusion with a severity.
type Finding struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	QueryID  string `json:"queryId,omitempty"`
	Message  string `json:"message"`
}

// Comparison records whether a baseline was eligible.
type Comparison struct {
	BaselineRunID string   `json:"baselineRunId"`
	Compatible    bool     `json:"compatible"`
	Reasons       []string `json:"reasons,omitempty"`
}

// Read loads a report from disk.
func Read(path string) (*Report, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rep Report
	if err := json.Unmarshal(b, &rep); err != nil {
		return nil, err
	}
	return &rep, nil
}

// Write atomically writes a pretty JSON report.
func Write(path string, rep *Report) error {
	if path == "" || path == "-" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(rep)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(rep); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}
