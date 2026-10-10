// Package engine runs one local benchmark from a validated manifest.
package engine

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sync/atomic"
	"time"

	"github.com/coderRaj07/benchforge/internal/baseline"
	"github.com/coderRaj07/benchforge/internal/manifest"
	"github.com/coderRaj07/benchforge/internal/provisioner"
	"github.com/coderRaj07/benchforge/internal/redact"
	"github.com/coderRaj07/benchforge/internal/report"
	"github.com/coderRaj07/benchforge/internal/runner"
	"github.com/coderRaj07/benchforge/internal/seed"
	"github.com/coderRaj07/benchforge/internal/stats"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Options controls one local run.
type Options struct {
	ManifestPath     string
	Workspace        string
	OutputPath       string
	DSN              string
	AllowExternal    bool
	Keep             bool
	BaselinePath     string
	DurationOverride int
	ScaleOverride    float64
	RunID            string
	Logger           *slog.Logger
	Limits           manifest.Limits
}

// Run validates the manifest, prepares an isolated database, measures the
// workload, and compares it with a baseline when one was provided.
// A non-nil error means the run did not produce a trustworthy measurement.
// Assertion failures are reported on the returned report and are not errors.
func Run(ctx context.Context, opt Options) (*report.Report, error) {
	if opt.Logger == nil {
		opt.Logger = slog.New(slog.DiscardHandler)
	}
	if opt.Limits.MaxCPU == 0 {
		opt.Limits = manifest.LocalLimits()
	}
	if opt.RunID == "" {
		opt.RunID = uuid.NewString()
	}
	now := time.Now().UTC()
	rep := &report.Report{
		SchemaVersion: report.SchemaVersion,
		RunID:         opt.RunID,
		State:         "validating",
		Outcome:       report.OutcomeError,
		StartedAt:     now,
		Timeline:      []report.Mark{{State: "validating", At: now}},
	}
	var execErr error
	var env *provisioner.Environment
	defer func() {
		rep.FinishedAt = time.Now().UTC()
		if execErr != nil {
			rep.Outcome = report.OutcomeError
			rep.FailureReason = clean(execErr)
			if rep.State != "cleaned_up" {
				rep.State = "failed"
				rep.Timeline = append(rep.Timeline, report.Mark{State: "failed", At: time.Now().UTC()})
			}
		}
	}()
	defer func() {
		if env == nil {
			return
		}
		if opt.Keep {
			rep.Notes = append(rep.Notes, "ephemeral environment kept for inspection: "+env.ContainerName)
			return
		}
		if err := env.Remove(context.Background()); err != nil {
			rep.CleanupError = clean(err)
			opt.Logger.Error("cleanup failed", "run_id", rep.RunID, "err", rep.CleanupError)
			return
		}
		mark(rep, "cleaned_up")
		opt.Logger.Info("cleaned up", "run_id", rep.RunID)
	}()

	spec, err := manifest.Load(opt.ManifestPath, opt.Workspace, opt.Limits)
	if err != nil {
		execErr = err
		return rep, execErr
	}
	if opt.DurationOverride > 0 {
		if opt.DurationOverride < opt.Limits.MinDurationSeconds || opt.DurationOverride > opt.Limits.MaxDurationSeconds {
			execErr = fmt.Errorf("duration override %d is outside %d-%d seconds", opt.DurationOverride, opt.Limits.MinDurationSeconds, opt.Limits.MaxDurationSeconds)
			return rep, execErr
		}
		spec.DurationSeconds = opt.DurationOverride
		rep.Notes = append(rep.Notes, fmt.Sprintf("duration overridden to %d seconds", opt.DurationOverride))
	}
	if opt.ScaleOverride > 0 {
		if opt.ScaleOverride < 0.01 || opt.ScaleOverride > opt.Limits.MaxScale {
			execErr = fmt.Errorf("scale override %g is outside 0.01-%g", opt.ScaleOverride, opt.Limits.MaxScale)
			return rep, execErr
		}
		spec.Scale = opt.ScaleOverride
		rep.Notes = append(rep.Notes, fmt.Sprintf("scale overridden to %g", opt.ScaleOverride))
	}
	fillIdentity(rep, spec)
	opt.Logger.Info("manifest validated", "run_id", rep.RunID, "name", spec.Name, "workload", spec.WorkloadType)

	mark(rep, "provisioning")
	if opt.DSN != "" {
		if !opt.AllowExternal || spec.Source != "external" {
			execErr = fmt.Errorf("refusing external database: set target.source to external and pass --allow-external")
			return rep, execErr
		}
		env = provisioner.External(opt.RunID, opt.DSN)
		rep.Notes = append(rep.Notes, "external database requested; BenchForge will not create or destroy it, and will refuse to seed it unless it is empty")
	} else {
		if spec.Source != "ephemeral" {
			execErr = fmt.Errorf("target.source is external but no --dsn was provided")
			return rep, execErr
		}
		password, err := provisioner.NewPassword()
		if err != nil {
			execErr = err
			return rep, execErr
		}
		image, ok := manifest.Images[spec.PostgresVersion]
		if !ok {
			execErr = fmt.Errorf("no pinned image for postgres %s", spec.PostgresVersion)
			return rep, execErr
		}
		maxConn := spec.MaxConcurrency + 16
		env, err = (&provisioner.Docker{}).Provision(ctx, provisioner.Request{
			RunID:          opt.RunID,
			Image:          image,
			CPU:            spec.DatabaseCPU,
			MemoryBytes:    spec.DatabaseMemory,
			MaxConnections: maxConn,
			Password:       password,
		})
		if err != nil {
			execErr = err
			return rep, execErr
		}
	}
	opt.Logger.Info("database ready", "run_id", rep.RunID, "source", spec.Source)

	pool, err := connect(ctx, env.DSN, spec.MaxConcurrency)
	if err != nil {
		execErr = err
		return rep, execErr
	}
	defer pool.Close()

	mark(rep, "preparing")
	if err := applyMigrations(ctx, pool, spec.Migrations); err != nil {
		execErr = err
		return rep, execErr
	}
	data, err := seed.Build(spec.Seed, spec.Scale)
	if err != nil {
		execErr = err
		return rep, execErr
	}
	if err := seed.Apply(ctx, pool, data); err != nil {
		execErr = err
		return rep, execErr
	}
	counts := data.Counts()
	rep.Dataset.Customers = counts.Customers
	rep.Dataset.Products = counts.Products
	rep.Dataset.Orders = counts.Orders
	rep.Dataset.OrderItems = counts.OrderItems

	info, err := snapshot(ctx, pool)
	if err != nil {
		execErr = err
		return rep, execErr
	}
	major, err := provisioner.MajorVersion(info.Version)
	if err != nil {
		execErr = err
		return rep, execErr
	}
	if major != spec.PostgresVersion {
		execErr = fmt.Errorf("server major version %s does not match manifest postgresVersion %s", major, spec.PostgresVersion)
		return rep, execErr
	}
	rep.Environment.PostgresVersion = info.Version
	rep.Environment.PostgresMajor = major
	rep.Environment.Image = env.Image
	rep.Environment.ImageID = env.ImageID
	rep.Environment.MaxConnections = info.MaxConnections
	rep.Environment.SharedBuffers = info.SharedBuffers

	var maxActive atomic.Int32
	sampleCtx, stopSample := context.WithCancel(ctx)
	defer stopSample()
	go sampleActive(sampleCtx, pool, &maxActive)

	if spec.WorkloadType == "pgbench" {
		mark(rep, "warming_up")
		mark(rep, "running")
		before := info
		out, err := runner.RunPgbench(ctx, env, runner.PgbenchConfig{
			Queries:  spec.Queries,
			Script:   spec.Script,
			Clients:  spec.Clients,
			Threads:  spec.Threads,
			Duration: time.Duration(spec.DurationSeconds) * time.Second,
			Warmup:   time.Duration(spec.WarmupSeconds) * time.Second,
			Seed:     spec.Seed,
			Counts:   counts,
			Rate:     spec.ArrivalRate,
		})
		if err != nil {
			execErr = err
			return rep, execErr
		}
		after, err := snapshot(ctx, pool)
		if err != nil {
			execErr = err
			return rep, execErr
		}
		rep.Database = delta(before, after, int(maxActive.Load()))
		fillPgbench(rep, out, spec.DurationSeconds)
		rep.Load.Mode = "pgbench"
		rep.Load.Offered = out.Transactions + out.Failed + out.Skipped
		rep.Load.Completed = out.Transactions
		rep.Load.Errors = out.Failed
		rep.Load.Dropped = out.Skipped
	} else {
		before := info
		if spec.WarmupSeconds > 0 || spec.RampUpSeconds > 0 {
			mark(rep, "warming_up")
		}
		mark(rep, "running")
		qout, err := runner.RunQueries(ctx, pool, runner.QueryConfig{
			Queries:     spec.Queries,
			Duration:    time.Duration(spec.DurationSeconds) * time.Second,
			Warmup:      time.Duration(spec.WarmupSeconds) * time.Second,
			Ramp:        time.Duration(spec.RampUpSeconds) * time.Second,
			Concurrency: spec.MaxConcurrency,
			ArrivalRate: spec.ArrivalRate,
			Seed:        spec.Seed,
			Counts:      counts,
		})
		if err != nil {
			execErr = err
			return rep, execErr
		}
		after, err := snapshot(ctx, pool)
		if err != nil {
			execErr = err
			return rep, execErr
		}
		rep.Database = delta(before, after, int(maxActive.Load()))
		fillQueries(rep, qout, spec.DurationSeconds)
		if qout.Dropped > 0 && !qout.GeneratorSaturated {
			rep.Notes = append(rep.Notes, fmt.Sprintf("%d offered attempts were not started because all workers were busy; this is backpressure, not load-generator saturation", qout.Dropped))
		}
	}

	mark(rep, "analyzing")
	var base *report.Report
	baselinePath := opt.BaselinePath
	if baselinePath == "" {
		baselinePath = spec.BaselineFile
	}
	if baselinePath != "" {
		loaded, err := report.Read(baselinePath)
		if err != nil {
			execErr = fmt.Errorf("read baseline: %w", err)
			return rep, execErr
		}
		base = loaded
	}
	eval := baseline.Evaluate(*rep, base)
	rep.Outcome = eval.Outcome
	rep.Findings = eval.Findings
	rep.Comparison = eval.Comparison
	if rep.Database.Deadlocks > 0 {
		rep.Notes = append(rep.Notes, fmt.Sprintf("postgres reported %d new deadlocks during the run", rep.Database.Deadlocks))
	}
	rep.Notes = append(rep.Notes, "database counters include warmup and ramp when those phases ran")
	rep.Notes = append(rep.Notes, "query latency excludes pool acquire time; pool wait is reported separately")
	mark(rep, "completed")
	opt.Logger.Info("analysis complete", "run_id", rep.RunID, "outcome", rep.Outcome)
	return rep, nil
}

func fillIdentity(rep *report.Report, spec *manifest.Benchmark) {
	rep.ManifestName = spec.Name
	rep.ManifestPath = spec.Path
	rep.ManifestFingerprint = spec.Fingerprint
	rep.Environment = report.Environment{
		Source: spec.Source,
		CPU:    spec.DatabaseCPUText,
		Memory: spec.DatabaseMemoryText,
	}
	rep.Dataset = report.Dataset{Profile: spec.Profile, Seed: spec.Seed, Scale: spec.Scale}
	rep.Workload = report.Workload{
		Type:                 spec.WorkloadType,
		DurationSeconds:      spec.DurationSeconds,
		WarmupSeconds:        spec.WarmupSeconds,
		RampUpSeconds:        spec.RampUpSeconds,
		MaxConcurrency:       spec.MaxConcurrency,
		ArrivalRatePerSecond: spec.ArrivalRate,
		Clients:              spec.Clients,
		Threads:              spec.Threads,
	}
	rep.Assertions = report.Assertions{
		MaxP95Ms:             spec.Assertions.MaxP95Ms,
		MaxP99Ms:             spec.Assertions.MaxP99Ms,
		MaxErrorRate:         spec.Assertions.MaxErrorRate,
		MaxRegressionPercent: spec.Assertions.MaxRegressionPercent,
		MinIterations:        spec.Assertions.MinIterations,
	}
}

func fillQueries(rep *report.Report, out *runner.QueryOutcome, configuredSeconds int) {
	elapsed := out.Elapsed.Seconds()
	if elapsed <= 0 {
		elapsed = float64(configuredSeconds)
	}
	rep.Load = report.LoadStats{
		Mode:               out.Mode,
		Offered:            out.Offered,
		Completed:          out.Completed,
		Errors:             out.Errors,
		Dropped:            out.Dropped,
		LateTicks:          out.LateTicks,
		Ticks:              out.Ticks,
		GeneratorSaturated: out.GeneratorSaturated,
		SaturationReason:   out.SaturationReason,
		PoolWait:           toLatency(out.PoolWait),
	}
	var aggErr int
	for _, q := range out.Queries {
		attempts := q.Iterations + q.Errors
		rate := 0.0
		if attempts > 0 {
			rate = float64(q.Errors) / float64(attempts)
		}
		tps := 0.0
		if elapsed > 0 {
			tps = float64(q.Iterations) / elapsed
		}
		rep.Queries = append(rep.Queries, report.QueryResult{
			ID:           q.ID,
			Fingerprint:  q.Fingerprint,
			Iterations:   q.Iterations,
			Errors:       q.Errors,
			ErrorRate:    round4(rate),
			Rows:         q.Rows,
			Latency:      toLatency(q.Latency),
			TPS:          round3(tps),
			Approximate:  q.Approximate,
			Histogram:    toBuckets(q.Histogram),
			ErrorSamples: q.ErrorSamples,
			Plan:         q.Plan,
			PoolWait:     toLatency(q.PoolWait),
		})
		aggErr += q.Errors
	}
	aggLat := stats.Summarize(out.AggregateSamples)
	attempts := out.Completed + out.Errors
	rate := 0.0
	if attempts > 0 {
		rate = float64(out.Errors) / float64(attempts)
	}
	tps := 0.0
	if elapsed > 0 {
		tps = float64(out.Completed) / elapsed
	}
	rep.Aggregate = report.Aggregate{
		Iterations: out.Completed,
		Errors:     aggErr,
		ErrorRate:  round4(rate),
		Latency:    toLatency(aggLat),
		TPS:        round3(tps),
	}
	_ = configuredSeconds
}

func fillPgbench(rep *report.Report, out *runner.PgbenchOutcome, seconds int) {
	attempts := out.Transactions + out.Failed
	rate := 0.0
	if attempts > 0 {
		rate = float64(out.Failed) / float64(attempts)
	}
	rep.Pgbench = &report.PgbenchResult{
		Transactions:    out.Transactions,
		Failed:          out.Failed,
		Skipped:         out.Skipped,
		TPS:             round3(out.TPS),
		Latency:         toLatency(out.Latency),
		StddevMs:        round3(out.StddevMs),
		Clients:         out.Clients,
		Threads:         out.Threads,
		DurationSeconds: seconds,
		LogSamples:      out.LogSamples,
		RawSummary:      out.Raw,
	}
	rep.Aggregate = report.Aggregate{
		Iterations: out.Transactions,
		Errors:     out.Failed,
		ErrorRate:  round4(rate),
		Latency:    toLatency(out.Latency),
		TPS:        round3(out.TPS),
	}
	if out.LogSamples == 0 {
		rep.Notes = append(rep.Notes, "pgbench transaction log was not collected; p95 and p99 are empty and only the average latency is known")
	}
}

func connect(ctx context.Context, dsn string, concurrency int) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	max := concurrency + 2
	if max < 2 {
		max = 2
	}
	cfg.MaxConns = int32(max)
	cfg.MinConns = 1
	cfg.MaxConnIdleTime = time.Minute
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	var pingErr error
	deadline := time.Now().Add(20 * time.Second)
	for {
		pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		pingErr = pool.Ping(pingCtx)
		cancel()
		if pingErr == nil {
			return pool, nil
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			pool.Close()
			return nil, fmt.Errorf("ping database: %w", pingErr)
		}
		time.Sleep(300 * time.Millisecond)
	}
}

func sampleActive(ctx context.Context, pool *pgxpool.Pool, dst *atomic.Int32) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			var n int32
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND state = 'active'`).Scan(&n); err != nil {
				continue
			}
			for {
				cur := dst.Load()
				if n <= cur || dst.CompareAndSwap(cur, n) {
					break
				}
			}
		}
	}
}

func mark(r *report.Report, state string) {
	r.State = state
	r.Timeline = append(r.Timeline, report.Mark{State: state, At: time.Now().UTC()})
}

func toLatency(s stats.Latency) report.Latency {
	return report.Latency{
		P50:  round3(s.P50),
		P95:  round3(s.P95),
		P99:  round3(s.P99),
		Mean: round3(s.Mean),
		Max:  round3(s.Max),
	}
}

func toBuckets(h stats.Histogram) []report.Bucket {
	out := make([]report.Bucket, len(h.Bounds))
	for i, b := range h.Bounds {
		le := b
		if math.IsInf(le, 0) {
			le = 1e99
		}
		out[i] = report.Bucket{LeMs: le, Count: h.Counts[i]}
	}
	return out
}

func round3(v float64) float64 { return math.Round(v*1000) / 1000 }
func round4(v float64) float64 { return math.Round(v*10000) / 10000 }

func clean(err error) string {
	if err == nil {
		return ""
	}
	return redact.Secrets(redact.DSN(err.Error()))
}
