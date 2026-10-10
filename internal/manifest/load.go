package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/coderRaj07/benchforge/internal/sqlsafe"
	"gopkg.in/yaml.v3"
)

const (
	profileOrders    = "orders-mixed"
	defaultTimeoutMs = 1000
)

var (
	nameRE  = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$`)
	identRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

// Images maps a requested major version to a digest-pinned or tag-pinned image.
// PostgreSQL 17 is the version exercised by the local integration test.
var Images = map[string]string{
	"17": "postgres:17.6-bookworm@sha256:f3bd19c606e442c3d7bdfa8002e03fe260a1023351e0ea4598032022b68dd6e3",
	"16": "postgres:16.11-bookworm",
}

type fileBenchmark struct {
	APIVersion string         `yaml:"apiVersion"`
	Kind       string         `yaml:"kind"`
	Metadata   fileMetadata   `yaml:"metadata"`
	Target     fileTarget     `yaml:"target"`
	Dataset    fileDataset    `yaml:"dataset"`
	Workload   fileWorkload   `yaml:"workload"`
	Queries    []fileQuery    `yaml:"queries"`
	Assertions fileAssertions `yaml:"assertions"`
	Resources  fileResources  `yaml:"resources"`
	Cleanup    fileCleanup    `yaml:"cleanup"`
	Baseline   *fileBaseline  `yaml:"baseline"`
}

type fileMetadata struct {
	Name        string            `yaml:"name"`
	Owner       string            `yaml:"owner"`
	Annotations map[string]string `yaml:"annotations"`
}

type fileTarget struct {
	Type            string `yaml:"type"`
	Source          string `yaml:"source"`
	PostgresVersion string `yaml:"postgresVersion"`
}

type fileDataset struct {
	Seed          *int64   `yaml:"seed"`
	Scale         *float64 `yaml:"scale"`
	Profile       string   `yaml:"profile"`
	WarmupSeconds *int     `yaml:"warmupSeconds"`
	Migrations    string   `yaml:"migrations"`
}

type fileWorkload struct {
	Type                 string `yaml:"type"`
	DurationSeconds      *int   `yaml:"durationSeconds"`
	ArrivalRatePerSecond *int   `yaml:"arrivalRatePerSecond"`
	MaxConcurrency       *int   `yaml:"maxConcurrency"`
	RampUpSeconds        *int   `yaml:"rampUpSeconds"`
	Clients              *int   `yaml:"clients"`
	Threads              *int   `yaml:"threads"`
	Script               string `yaml:"script"`
	Soak                 *bool  `yaml:"soak"`
}

type fileQuery struct {
	ID            string      `yaml:"id"`
	File          string      `yaml:"file"`
	Weight        *int        `yaml:"weight"`
	TimeoutMs     *int        `yaml:"timeoutMs"`
	Transactional *bool       `yaml:"transactional"`
	Params        []fileParam `yaml:"params"`
}

type fileParam struct {
	Name         string `yaml:"name"`
	Type         string `yaml:"type"`
	Distribution string `yaml:"distribution"`
	Min          *int64 `yaml:"min"`
	Max          *int64 `yaml:"max"`
	MaxFrom      string `yaml:"maxFrom"`
	Value        string `yaml:"value"`
}

type fileAssertions struct {
	MaxP95Ms             *float64 `yaml:"maxP95Ms"`
	MaxP99Ms             *float64 `yaml:"maxP99Ms"`
	MaxErrorRate         *float64 `yaml:"maxErrorRate"`
	MaxRegressionPercent *float64 `yaml:"maxRegressionPercent"`
	MinIterations        *int     `yaml:"minIterations"`
}

type fileResources struct {
	Database      *fileResource `yaml:"database"`
	LoadGenerator *fileResource `yaml:"loadGenerator"`
}

type fileResource struct {
	CPU    string `yaml:"cpu"`
	Memory string `yaml:"memory"`
}

type fileCleanup struct {
	TTLSecondsAfterFinished *int `yaml:"ttlSecondsAfterFinished"`
}

type fileBaseline struct {
	ReportFile string `yaml:"reportFile"`
}

// Load reads a manifest, applies defaults, and validates it against limits.
// Paths are resolved against the manifest directory and must stay inside workspace.
func Load(path, workspace string, limits Limits) (*Benchmark, error) {
	absManifest, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("manifest path: %w", err)
	}
	if workspace == "" {
		workspace, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}
	workspace, err = filepath.Abs(workspace)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(absManifest)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var doc fileBenchmark
	dec := yaml.NewDecoder(strings.NewReader(string(raw)))
	dec.KnownFields(true)
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	sum := sha256.Sum256(raw)
	spec := &Benchmark{
		APIVersion:      doc.APIVersion,
		Kind:            doc.Kind,
		Name:            doc.Metadata.Name,
		Owner:           doc.Metadata.Owner,
		Annotations:     doc.Metadata.Annotations,
		TargetType:      doc.Target.Type,
		Source:          defaultString(doc.Target.Source, "ephemeral"),
		PostgresVersion: defaultString(doc.Target.PostgresVersion, "17"),
		Profile:         doc.Dataset.Profile,
		Fingerprint:     hex.EncodeToString(sum[:]),
		Dir:             filepath.Dir(absManifest),
		Path:            absManifest,
	}
	if doc.Dataset.Seed != nil {
		spec.Seed = *doc.Dataset.Seed
	} else {
		spec.Seed = 42
	}
	if doc.Dataset.Scale != nil {
		spec.Scale = *doc.Dataset.Scale
	} else {
		spec.Scale = 1
	}
	if doc.Dataset.WarmupSeconds != nil {
		spec.WarmupSeconds = *doc.Dataset.WarmupSeconds
	} else {
		spec.WarmupSeconds = 10
	}
	if doc.Workload.DurationSeconds != nil {
		spec.DurationSeconds = *doc.Workload.DurationSeconds
	} else {
		spec.DurationSeconds = 30
	}
	if doc.Workload.Soak != nil {
		spec.Soak = *doc.Workload.Soak
	}
	if doc.Workload.ArrivalRatePerSecond != nil {
		spec.ArrivalRate = *doc.Workload.ArrivalRatePerSecond
	}
	if doc.Workload.MaxConcurrency != nil {
		spec.MaxConcurrency = *doc.Workload.MaxConcurrency
	} else {
		spec.MaxConcurrency = 4
	}
	if doc.Workload.RampUpSeconds != nil {
		spec.RampUpSeconds = *doc.Workload.RampUpSeconds
	}
	spec.WorkloadType = doc.Workload.Type
	if doc.Workload.Clients != nil {
		spec.Clients = *doc.Workload.Clients
	}
	if doc.Workload.Threads != nil {
		spec.Threads = *doc.Workload.Threads
	}
	if doc.Assertions.MaxP95Ms != nil {
		spec.Assertions.MaxP95Ms = *doc.Assertions.MaxP95Ms
	}
	if doc.Assertions.MaxP99Ms != nil {
		spec.Assertions.MaxP99Ms = *doc.Assertions.MaxP99Ms
	}
	if doc.Assertions.MaxErrorRate != nil {
		spec.Assertions.MaxErrorRate = *doc.Assertions.MaxErrorRate
	} else {
		spec.Assertions.MaxErrorRate = 0.01
	}
	if doc.Assertions.MaxRegressionPercent != nil {
		spec.Assertions.MaxRegressionPercent = *doc.Assertions.MaxRegressionPercent
	} else {
		spec.Assertions.MaxRegressionPercent = 10
	}
	if doc.Assertions.MinIterations != nil {
		spec.Assertions.MinIterations = *doc.Assertions.MinIterations
	} else {
		spec.Assertions.MinIterations = 30
	}
	if doc.Cleanup.TTLSecondsAfterFinished != nil {
		spec.TTLSeconds = *doc.Cleanup.TTLSecondsAfterFinished
	} else {
		spec.TTLSeconds = 600
	}
	dbCPU, dbMem := "1", "1Gi"
	lgCPU, lgMem := "1", "512Mi"
	if doc.Resources.Database != nil {
		if doc.Resources.Database.CPU != "" {
			dbCPU = doc.Resources.Database.CPU
		}
		if doc.Resources.Database.Memory != "" {
			dbMem = doc.Resources.Database.Memory
		}
	}
	if doc.Resources.LoadGenerator != nil {
		if doc.Resources.LoadGenerator.CPU != "" {
			lgCPU = doc.Resources.LoadGenerator.CPU
		}
		if doc.Resources.LoadGenerator.Memory != "" {
			lgMem = doc.Resources.LoadGenerator.Memory
		}
	}
	spec.DatabaseCPUText = dbCPU
	spec.DatabaseMemoryText = dbMem
	spec.LoadCPUText = lgCPU
	spec.LoadMemoryText = lgMem

	errs := &MultiError{}
	if spec.APIVersion != APIVersion {
		errs.add("apiVersion", fmt.Sprintf("must be %s", APIVersion))
	}
	if spec.Kind != KindBenchmark {
		errs.add("kind", "must be Benchmark")
	}
	if !nameRE.MatchString(spec.Name) {
		errs.add("metadata.name", "must be a lowercase DNS label (letters, digits, hyphens)")
	}
	if len(spec.Owner) > 64 || strings.ContainsAny(spec.Owner, "\r\n") {
		errs.add("metadata.owner", "must be at most 64 characters and must not contain newlines")
	}
	if spec.TargetType != "postgres" {
		errs.add("target.type", "only postgres is supported in this release")
	}
	if spec.Source != "ephemeral" && spec.Source != "external" {
		errs.add("target.source", "must be ephemeral or external")
	}
	if _, ok := Images[spec.PostgresVersion]; !ok {
		errs.add("target.postgresVersion", "supported major versions are 16 and 17")
	}
	if spec.Profile != profileOrders {
		errs.add("dataset.profile", "supported profile is orders-mixed")
	}
	if spec.Seed < 0 {
		errs.add("dataset.seed", "must be >= 0")
	}
	if spec.Scale < 0.01 || spec.Scale > limits.MaxScale {
		errs.add("dataset.scale", fmt.Sprintf("must be between 0.01 and %g", limits.MaxScale))
	}
	if spec.WarmupSeconds < 0 || spec.WarmupSeconds > limits.MaxWarmupSeconds {
		errs.add("dataset.warmupSeconds", fmt.Sprintf("must be between 0 and %d", limits.MaxWarmupSeconds))
	}
	switch spec.WorkloadType {
	case "query", "mixed", "pgbench":
	default:
		errs.add("workload.type", "must be query, mixed, or pgbench")
	}
	if spec.DurationSeconds < limits.MinDurationSeconds || spec.DurationSeconds > limits.MaxDurationSeconds {
		errs.add("workload.durationSeconds", fmt.Sprintf("must be between %d and %d", limits.MinDurationSeconds, limits.MaxDurationSeconds))
	}
	if spec.Soak && spec.DurationSeconds < 60 {
		errs.add("workload.durationSeconds", "a soak run must last at least 60 seconds")
	}
	if spec.MaxConcurrency < 1 || spec.MaxConcurrency > limits.MaxConcurrency {
		errs.add("workload.maxConcurrency", fmt.Sprintf("must be between 1 and %d", limits.MaxConcurrency))
	}
	if spec.ArrivalRate < 0 || spec.ArrivalRate > limits.MaxArrivalRate {
		errs.add("workload.arrivalRatePerSecond", fmt.Sprintf("must be between 0 and %d", limits.MaxArrivalRate))
	}
	if spec.RampUpSeconds < 0 || spec.RampUpSeconds > limits.MaxRampSeconds {
		errs.add("workload.rampUpSeconds", fmt.Sprintf("must be between 0 and %d", limits.MaxRampSeconds))
	}
	if spec.WorkloadType == "pgbench" {
		if spec.Clients == 0 {
			spec.Clients = spec.MaxConcurrency
		}
		if spec.Threads == 0 {
			spec.Threads = spec.Clients
			if spec.Threads > 8 {
				spec.Threads = 8
			}
		}
		if spec.Clients < 1 || spec.Clients > spec.MaxConcurrency {
			errs.add("workload.clients", "must be between 1 and workload.maxConcurrency")
		}
		if spec.Threads < 1 || spec.Threads > spec.Clients {
			errs.add("workload.threads", "must be between 1 and workload.clients")
		}
	} else if spec.Clients != 0 || spec.Threads != 0 || doc.Workload.Script != "" {
		errs.add("workload.clients", "clients, threads, and script are only valid when workload.type is pgbench")
	}
	if spec.Assertions.MaxP95Ms < 0 || (spec.Assertions.MaxP95Ms > 0 && spec.Assertions.MaxP95Ms < 1) {
		errs.add("assertions.maxP95Ms", "omit the field or set it to at least 1")
	}
	if spec.Assertions.MaxP99Ms < 0 || (spec.Assertions.MaxP99Ms > 0 && spec.Assertions.MaxP99Ms < 1) {
		errs.add("assertions.maxP99Ms", "omit the field or set it to at least 1")
	}
	if spec.Assertions.MaxP95Ms > 0 && spec.Assertions.MaxP99Ms > 0 && spec.Assertions.MaxP99Ms < spec.Assertions.MaxP95Ms {
		errs.add("assertions.maxP99Ms", "must be greater than or equal to maxP95Ms")
	}
	if spec.Assertions.MaxErrorRate < 0 || spec.Assertions.MaxErrorRate > 1 {
		errs.add("assertions.maxErrorRate", "must be between 0 and 1")
	}
	if spec.Assertions.MaxRegressionPercent < 0 || spec.Assertions.MaxRegressionPercent > 500 {
		errs.add("assertions.maxRegressionPercent", "must be between 0 and 500")
	}
	if spec.Assertions.MinIterations < 1 || spec.Assertions.MinIterations > 1000000 {
		errs.add("assertions.minIterations", "must be between 1 and 1000000")
	}
	if spec.TTLSeconds < 0 || spec.TTLSeconds > 86400 {
		errs.add("cleanup.ttlSecondsAfterFinished", "must be between 0 and 86400")
	}

	cpu, err := ParseCPU(dbCPU)
	if err != nil {
		errs.add("resources.database.cpu", err.Error())
	} else if cpu > limits.MaxCPU {
		errs.add("resources.database.cpu", fmt.Sprintf("exceeds local policy maximum of %g cores", limits.MaxCPU))
	} else {
		spec.DatabaseCPU = cpu
	}
	mem, err := ParseMemory(dbMem)
	if err != nil {
		errs.add("resources.database.memory", err.Error())
	} else if mem > limits.MaxMemoryBytes {
		errs.add("resources.database.memory", "exceeds local policy maximum of 4Gi")
	} else if mem < 256<<20 {
		errs.add("resources.database.memory", "must be at least 256Mi")
	} else {
		spec.DatabaseMemory = mem
	}
	lcpu, err := ParseCPU(lgCPU)
	if err != nil {
		errs.add("resources.loadGenerator.cpu", err.Error())
	} else if lcpu > limits.MaxCPU {
		errs.add("resources.loadGenerator.cpu", fmt.Sprintf("exceeds local policy maximum of %g cores", limits.MaxCPU))
	} else {
		spec.LoadCPU = lcpu
	}
	lmem, err := ParseMemory(lgMem)
	if err != nil {
		errs.add("resources.loadGenerator.memory", err.Error())
	} else if lmem > limits.MaxMemoryBytes {
		errs.add("resources.loadGenerator.memory", "exceeds local policy maximum of 4Gi")
	} else {
		spec.LoadMemory = lmem
	}

	if doc.Dataset.Migrations == "" {
		errs.add("dataset.migrations", "set this to the directory of reviewed SQL migrations")
	} else if resolved, rerr := Resolve(workspace, spec.Dir, doc.Dataset.Migrations); rerr != nil {
		errs.add("dataset.migrations", rerr.Error())
	} else {
		info, statErr := os.Stat(resolved)
		if statErr != nil || !info.IsDir() {
			errs.add("dataset.migrations", "directory does not exist: "+doc.Dataset.Migrations)
		} else {
			spec.Migrations = resolved
		}
	}
	if doc.Workload.Script != "" {
		if spec.WorkloadType != "pgbench" {
			// already reported
		} else if resolved, rerr := Resolve(workspace, spec.Dir, doc.Workload.Script); rerr != nil {
			errs.add("workload.script", rerr.Error())
		} else if _, statErr := os.Stat(resolved); statErr != nil {
			errs.add("workload.script", "file does not exist: "+doc.Workload.Script)
		} else {
			spec.Script = resolved
		}
	}
	if doc.Baseline != nil && doc.Baseline.ReportFile != "" {
		if resolved, rerr := Resolve(workspace, spec.Dir, doc.Baseline.ReportFile); rerr != nil {
			errs.add("baseline.reportFile", rerr.Error())
		} else {
			spec.BaselineFile = resolved
		}
	}

	if spec.WorkloadType == "pgbench" && spec.Script != "" && len(doc.Queries) > 0 {
		errs.add("queries", "provide either workload.script or queries, not both")
	}
	if spec.Script == "" && len(doc.Queries) == 0 {
		errs.add("queries", "at least one query is required")
	}
	if len(doc.Queries) > limits.MaxQueries {
		errs.add("queries", fmt.Sprintf("at most %d queries are allowed", limits.MaxQueries))
	}
	seen := map[string]bool{}
	for i, q := range doc.Queries {
		field := fmt.Sprintf("queries[%d]", i)
		if !nameRE.MatchString(q.ID) {
			errs.add(field+".id", "must be a lowercase DNS label")
		}
		if seen[q.ID] {
			errs.add(field+".id", "duplicate query id "+q.ID)
		}
		seen[q.ID] = true
		weight := 1
		if q.Weight != nil {
			weight = *q.Weight
		}
		if weight < 1 || weight > 10000 {
			errs.add(field+".weight", "must be between 1 and 10000")
		}
		timeout := defaultTimeoutMs
		if q.TimeoutMs != nil {
			timeout = *q.TimeoutMs
		}
		if timeout < 1 || timeout > 120000 {
			errs.add(field+".timeoutMs", "must be between 1 and 120000")
		}
		resolved, rerr := Resolve(workspace, spec.Dir, q.File)
		if rerr != nil {
			errs.add(field+".file", rerr.Error())
			continue
		}
		body, rerr := os.ReadFile(resolved)
		if rerr != nil {
			errs.add(field+".file", "cannot read "+q.File)
			continue
		}
		if len(body) > limits.MaxSQLBytes {
			errs.add(field+".file", fmt.Sprintf("SQL file exceeds %d bytes", limits.MaxSQLBytes))
			continue
		}
		if err := sqlsafe.Check(string(body)); err != nil {
			errs.add(field+".file", err.Error())
			continue
		}
		statements, rerr := sqlsafe.Split(string(body))
		if rerr != nil {
			errs.add(field+".file", rerr.Error())
			continue
		}
		if len(statements) == 0 {
			errs.add(field+".file", "file does not contain a SQL statement")
			continue
		}
		nq := Query{
			ID:          q.ID,
			File:        resolved,
			Weight:      weight,
			Timeout:     time.Duration(timeout) * time.Millisecond,
			SQL:         string(body),
			Statements:  statements,
			Fingerprint: sqlsafe.Fingerprint(string(body)),
		}
		if q.Transactional != nil {
			nq.Transactional = *q.Transactional
		}
		for pi, p := range q.Params {
			pf := fmt.Sprintf("%s.params[%d]", field, pi)
			np, perr := normalizeParam(p)
			if perr != nil {
				errs.add(pf, perr.Error())
				continue
			}
			nq.Params = append(nq.Params, np)
		}
		spec.Queries = append(spec.Queries, nq)
	}

	if len(errs.Errs) > 0 {
		return nil, errs
	}
	return spec, nil
}

func normalizeParam(p fileParam) (Param, error) {
	if !identRE.MatchString(p.Name) {
		return Param{}, fmt.Errorf("name must be an identifier")
	}
	dist := p.Distribution
	if dist == "" {
		dist = "uniform"
	}
	out := Param{Name: p.Name, Type: p.Type, Distribution: dist, MaxFrom: p.MaxFrom, Value: p.Value}
	switch p.Type {
	case "int":
	case "text":
	default:
		return Param{}, fmt.Errorf("type must be int or text")
	}
	switch dist {
	case "uniform", "constant":
	default:
		return Param{}, fmt.Errorf("distribution %q is not supported (use uniform or constant)", dist)
	}
	if p.Max != nil && p.MaxFrom != "" {
		return Param{}, fmt.Errorf("set max or maxFrom, not both")
	}
	if p.MaxFrom != "" {
		switch p.MaxFrom {
		case "customers", "products", "orders":
		default:
			return Param{}, fmt.Errorf("maxFrom must be customers, products, or orders")
		}
		if p.Type != "int" || dist != "uniform" {
			return Param{}, fmt.Errorf("maxFrom is only valid for uniform int parameters")
		}
	}
	if dist == "constant" {
		if p.Value == "" {
			return Param{}, fmt.Errorf("constant distribution requires value")
		}
		if p.Type == "int" {
			n, err := parseInt(p.Value)
			if err != nil {
				return Param{}, fmt.Errorf("constant int value: %w", err)
			}
			out.Min, out.Max = n, n
		}
		return out, nil
	}
	if p.Type == "text" {
		return Param{}, fmt.Errorf("text parameters only support constant distribution")
	}
	min := int64(1)
	if p.Min != nil {
		min = *p.Min
	}
	out.Min = min
	if p.Max != nil {
		out.Max = *p.Max
		if out.Max < out.Min {
			return Param{}, fmt.Errorf("max must be >= min")
		}
	} else if p.MaxFrom == "" {
		return Param{}, fmt.Errorf("uniform int requires max or maxFrom")
	}
	return out, nil
}

func parseInt(s string) (int64, error) {
	var n int64
	_, err := fmt.Sscan(s, &n)
	return n, err
}

func defaultString(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// Resolve joins rel to base and requires the result to stay inside workspace.
func Resolve(workspace, base, rel string) (string, error) {
	if rel == "" {
		return "", fmt.Errorf("path is empty")
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("absolute paths are not allowed (%s)", rel)
	}
	if strings.Contains(rel, "\x00") {
		return "", fmt.Errorf("path contains a null byte")
	}
	joined := filepath.Clean(filepath.Join(base, rel))
	abs, err := filepath.Abs(joined)
	if err != nil {
		return "", err
	}
	if !inside(workspace, abs) {
		return "", fmt.Errorf("path %s escapes the workspace", rel)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		if !inside(workspace, resolved) {
			return "", fmt.Errorf("path %s escapes the workspace through a symlink", rel)
		}
		return resolved, nil
	}
	return abs, nil
}

func inside(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
