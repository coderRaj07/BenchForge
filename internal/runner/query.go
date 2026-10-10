// Package runner executes query mixes and pgbench scripts.
package runner

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/coderRaj07/benchforge/internal/manifest"
	"github.com/coderRaj07/benchforge/internal/seed"
	"github.com/coderRaj07/benchforge/internal/sqlsafe"
	"github.com/coderRaj07/benchforge/internal/stats"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const sampleLimit = 200_000

var placeholder = regexp.MustCompile(`\$([0-9]+)`)

// QueryConfig is the measured query workload.
type QueryConfig struct {
	Queries     []manifest.Query
	Duration    time.Duration
	Warmup      time.Duration
	Ramp        time.Duration
	Concurrency int
	ArrivalRate int
	Seed        int64
	Counts      seed.Counts
}

// QueryOutcome is the measured result of a query mix.
type QueryOutcome struct {
	Queries            []QueryStat
	AggregateSamples   []float64
	AggregateSeen      int
	AggregateApprox    bool
	PoolWait           stats.Latency
	Offered            int
	Completed          int
	Errors             int
	Dropped            int
	LateTicks          int
	Ticks              int
	Mode               string
	GeneratorSaturated bool
	SaturationReason   string
	Elapsed            time.Duration
	MaxActive          int
}

// QueryStat is one query's accumulator after the run.
type QueryStat struct {
	ID           string
	Fingerprint  string
	Iterations   int
	Errors       int
	Rows         int64
	Latency      stats.Latency
	Approximate  bool
	Histogram    stats.Histogram
	ErrorSamples []string
	PoolWait     stats.Latency
	Plan         string
}

type accumulator struct {
	mu      sync.Mutex
	res     *stats.Reservoir
	pool    *stats.Reservoir
	hist    stats.Histogram
	ok      int
	errors  int
	rows    int64
	samples []string
}

// RunQueries warms the database, then measures the query mix.
// A fatal SQL error (missing table, syntax) aborts the run.
func RunQueries(ctx context.Context, pool *pgxpool.Pool, cfg QueryConfig) (*QueryOutcome, error) {
	if cfg.Concurrency < 1 {
		return nil, fmt.Errorf("concurrency must be at least 1")
	}
	if len(cfg.Queries) == 0 {
		return nil, fmt.Errorf("query workload has no queries")
	}
	if cfg.Ramp > 0 {
		if err := runClosed(ctx, pool, cfg, cfg.Ramp, false, nil); err != nil {
			return nil, fmt.Errorf("ramp: %w", err)
		}
	}
	if cfg.Warmup > 0 {
		if err := runClosed(ctx, pool, cfg, cfg.Warmup, false, nil); err != nil {
			return nil, fmt.Errorf("warmup: %w", err)
		}
	}
	out := &QueryOutcome{
		Mode: "closed_loop",
	}
	if cfg.ArrivalRate > 0 {
		out.Mode = "open_loop"
	}
	acc := make([]*accumulator, len(cfg.Queries))
	for i, q := range cfg.Queries {
		acc[i] = &accumulator{
			res:  stats.NewReservoir(sampleLimit, uint64(cfg.Seed)+uint64(i+1)),
			pool: stats.NewReservoir(sampleLimit, uint64(cfg.Seed)+1000+uint64(i)),
			hist: stats.NewHistogram(),
		}
		_ = q
	}
	agg := stats.NewReservoir(sampleLimit, uint64(cfg.Seed)+9)
	poolRes := stats.NewReservoir(sampleLimit, uint64(cfg.Seed)+11)
	start := time.Now()
	var err error
	if cfg.ArrivalRate > 0 {
		err = runOpen(ctx, pool, cfg, cfg.Duration, acc, agg, poolRes, out)
	} else {
		err = runClosed(ctx, pool, cfg, cfg.Duration, true, &recordSink{acc: acc, agg: agg, pool: poolRes, out: out})
	}
	out.Elapsed = time.Since(start)
	if err != nil {
		return nil, err
	}
	out.PoolWait = stats.Summarize(mustValues(poolRes))
	for i, q := range cfg.Queries {
		vals, _ := acc[i].res.Values()
		pvals, _ := acc[i].pool.Values()
		out.Queries = append(out.Queries, QueryStat{
			ID:           q.ID,
			Fingerprint:  q.Fingerprint,
			Iterations:   acc[i].ok,
			Errors:       acc[i].errors,
			Rows:         acc[i].rows,
			Latency:      stats.Summarize(vals),
			Approximate:  acc[i].res.Approximate(),
			Histogram:    acc[i].hist,
			ErrorSamples: acc[i].samples,
			PoolWait:     stats.Summarize(pvals),
		})
		out.Completed += acc[i].ok
		out.Errors += acc[i].errors
	}
	out.AggregateSamples, out.AggregateSeen = agg.Values()
	out.AggregateApprox = agg.Approximate()
	if out.Mode == "closed_loop" {
		out.Offered = out.Completed + out.Errors
	}
	if out.Mode == "open_loop" {
		sat, reason := ClassifyLoad(out.Offered, out.Completed, out.LateTicks, out.Ticks)
		out.GeneratorSaturated = sat
		out.SaturationReason = reason
	}
	plans, err := capturePlans(ctx, pool, cfg)
	if err != nil {
		return out, nil
	}
	for i := range out.Queries {
		out.Queries[i].Plan = plans[out.Queries[i].ID]
	}
	return out, nil
}

type recordSink struct {
	acc  []*accumulator
	agg  *stats.Reservoir
	pool *stats.Reservoir
	out  *QueryOutcome
}

func runClosed(ctx context.Context, pool *pgxpool.Pool, cfg QueryConfig, window time.Duration, record bool, sink *recordSink) error {
	wctx, cancel := context.WithTimeout(ctx, window)
	defer cancel()
	var fatal error
	var mu sync.Mutex
	var wg sync.WaitGroup
	weights := weightsOf(cfg.Queries)
	for i := 0; i < cfg.Concurrency; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			rng := rand.New(rand.NewPCG(uint64(cfg.Seed), uint64(worker+1)))
			for wctx.Err() == nil {
				idx := pick(weights, rng)
				if err := one(wctx, pool, cfg, idx, rng, record, sink); err != nil {
					if wctx.Err() != nil && isCancel(err) {
						return
					}
					if isFatalSQL(err) {
						mu.Lock()
						if fatal == nil {
							fatal = err
						}
						mu.Unlock()
						cancel()
						return
					}
				}
			}
		}(i)
	}
	wg.Wait()
	return fatal
}

func runOpen(ctx context.Context, pool *pgxpool.Pool, cfg QueryConfig, window time.Duration, acc []*accumulator, agg, poolRes *stats.Reservoir, out *QueryOutcome) error {
	wctx, cancel := context.WithTimeout(ctx, window)
	defer cancel()
	jobs := make(chan struct{})
	var fatal error
	var mu sync.Mutex
	var wg sync.WaitGroup
	sink := &recordSink{acc: acc, agg: agg, pool: poolRes, out: out}
	weights := weightsOf(cfg.Queries)
	for i := 0; i < cfg.Concurrency; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			rng := rand.New(rand.NewPCG(uint64(cfg.Seed), uint64(worker+1)))
			for range jobs {
				idx := pick(weights, rng)
				if err := one(wctx, pool, cfg, idx, rng, true, sink); err != nil {
					if wctx.Err() != nil && isCancel(err) {
						continue
					}
					if isFatalSQL(err) {
						mu.Lock()
						if fatal == nil {
							fatal = err
						}
						mu.Unlock()
						cancel()
						return
					}
				}
			}
		}(i)
	}
	interval := time.Second / time.Duration(cfg.ArrivalRate)
	if interval <= 0 {
		interval = time.Microsecond
	}
	next := time.Now()
	for wctx.Err() == nil {
		now := time.Now()
		out.Ticks++
		if now.After(next.Add(interval)) {
			out.LateTicks++
		}
		select {
		case <-wctx.Done():
		case jobs <- struct{}{}:
			out.Offered++
		default:
			out.Offered++
			out.Dropped++
		}
		next = next.Add(interval)
		sleep := time.Until(next)
		if sleep <= 0 {
			next = time.Now()
			continue
		}
		timer := time.NewTimer(sleep)
		select {
		case <-wctx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
	close(jobs)
	wg.Wait()
	return fatal
}

func one(ctx context.Context, pool *pgxpool.Pool, cfg QueryConfig, idx int, rng *rand.Rand, record bool, sink *recordSink) error {
	q := cfg.Queries[idx]
	args, err := bindParams(q.Params, cfg.Counts, rng)
	if err != nil {
		return err
	}
	lat, wait, rows, err := execQuery(ctx, pool, q, args)
	if !record || sink == nil {
		return err
	}
	if ctx.Err() != nil && isCancel(err) {
		return err
	}
	sink.pool.Add(wait)
	a := sink.acc[idx]
	a.mu.Lock()
	defer a.mu.Unlock()
	a.pool.Add(wait)
	if err != nil {
		a.errors++
		if len(a.samples) < 5 {
			a.samples = append(a.samples, err.Error())
		}
		return err
	}
	a.res.Add(lat)
	a.hist.Add(lat)
	a.rows += rows
	a.ok++
	sink.agg.Add(lat)
	return nil
}

func execQuery(ctx context.Context, pool *pgxpool.Pool, q manifest.Query, args []any) (latMs, poolWaitMs float64, rows int64, err error) {
	t0 := time.Now()
	conn, err := pool.Acquire(ctx)
	poolWaitMs = float64(time.Since(t0).Microseconds()) / 1000
	if err != nil {
		return 0, poolWaitMs, 0, err
	}
	defer conn.Release()
	qctx, cancel := context.WithTimeout(ctx, q.Timeout)
	defer cancel()
	start := time.Now()
	tx, err := conn.Begin(qctx)
	if err != nil {
		return 0, poolWaitMs, 0, err
	}
	defer tx.Rollback(qctx)
	ms := q.Timeout.Milliseconds()
	if ms < 1 {
		ms = 1
	}
	if _, err := tx.Exec(qctx, "SELECT set_config('statement_timeout', $1, true)", fmt.Sprintf("%d", ms)); err != nil {
		return 0, poolWaitMs, 0, err
	}
	var total int64
	for _, stmt := range q.Statements {
		var stmtArgs []any
		if placeholder.MatchString(stmt) {
			stmtArgs = args
		}
		rws, err := tx.Query(qctx, stmt, stmtArgs...)
		if err != nil {
			return 0, poolWaitMs, total, err
		}
		n := int64(0)
		for rws.Next() {
			n++
		}
		tag := rws.CommandTag()
		rws.Close()
		if err := rws.Err(); err != nil {
			return 0, poolWaitMs, total, err
		}
		if n == 0 && tag.RowsAffected() > 0 {
			n = tag.RowsAffected()
		}
		total += n
	}
	if err := tx.Commit(qctx); err != nil {
		return 0, poolWaitMs, total, err
	}
	latMs = float64(time.Since(start).Microseconds()) / 1000
	return latMs, poolWaitMs, total, nil
}

func bindParams(params []manifest.Param, counts seed.Counts, rng *rand.Rand) ([]any, error) {
	args := make([]any, len(params))
	for i, p := range params {
		switch p.Type {
		case "text":
			args[i] = p.Value
		case "int":
			if p.Distribution == "constant" {
				args[i] = p.Min
				continue
			}
			max := p.Max
			if p.MaxFrom != "" {
				m, err := counts.Max(p.MaxFrom)
				if err != nil {
					return nil, err
				}
				max = m
			}
			if max < p.Min {
				return nil, fmt.Errorf("param %s max %d is below min %d", p.Name, max, p.Min)
			}
			if max == p.Min {
				args[i] = p.Min
				continue
			}
			args[i] = p.Min + rng.Int64N(max-p.Min+1)
		default:
			return nil, fmt.Errorf("param %s has unsupported type %s", p.Name, p.Type)
		}
	}
	return args, nil
}

func capturePlans(ctx context.Context, pool *pgxpool.Pool, cfg QueryConfig) (map[string]string, error) {
	rng := rand.New(rand.NewPCG(uint64(cfg.Seed), 99))
	out := map[string]string{}
	for _, q := range cfg.Queries {
		if len(q.Statements) != 1 || !sqlsafe.IsReadOnly(q.Statements[0]) {
			continue
		}
		args, err := bindParams(q.Params, cfg.Counts, rng)
		if err != nil {
			continue
		}
		pctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		plan, err := explain(pctx, pool, q.Statements[0], args)
		cancel()
		if err != nil {
			out[q.ID] = "explain failed: " + err.Error()
			continue
		}
		if len(plan) > 16*1024 {
			plan = plan[:16*1024] + "\n-- truncated"
		}
		out[q.ID] = plan
	}
	return out, nil
}

func explain(ctx context.Context, pool *pgxpool.Pool, sql string, args []any) (string, error) {
	rows, err := pool.Query(ctx, "EXPLAIN (ANALYZE, BUFFERS) "+sql, args...)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var b strings.Builder
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return "", err
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String(), rows.Err()
}

func weightsOf(qs []manifest.Query) []int {
	w := make([]int, len(qs))
	for i, q := range qs {
		w[i] = q.Weight
		if w[i] < 1 {
			w[i] = 1
		}
	}
	return w
}

func pick(weights []int, rng *rand.Rand) int {
	total := 0
	for _, w := range weights {
		total += w
	}
	n := rng.IntN(total)
	for i, w := range weights {
		n -= w
		if n < 0 {
			return i
		}
	}
	return len(weights) - 1
}

func isCancel(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func isFatalSQL(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	switch pgErr.Code {
	case "42601", "42P01", "42703", "42P02":
		return true
	default:
		return false
	}
}

func mustValues(r *stats.Reservoir) []float64 {
	if r == nil {
		return nil
	}
	v, _ := r.Values()
	return v
}
