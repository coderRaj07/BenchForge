package runner

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/coderRaj07/benchforge/internal/manifest"
	"github.com/coderRaj07/benchforge/internal/provisioner"
	"github.com/coderRaj07/benchforge/internal/seed"
	"github.com/coderRaj07/benchforge/internal/stats"
)

// PgbenchConfig is one pgbench invocation.
type PgbenchConfig struct {
	Queries  []manifest.Query
	Script   string
	Clients  int
	Threads  int
	Duration time.Duration
	Warmup   time.Duration
	Seed     int64
	Counts   seed.Counts
	Rate     int
}

// PgbenchOutcome is parsed pgbench output plus log percentiles when available.
type PgbenchOutcome struct {
	Transactions int
	Failed       int
	Skipped      int
	TPS          float64
	StddevMs     float64
	Latency      stats.Latency
	LogSamples   int
	Approximate  bool
	Clients      int
	Threads      int
	Raw          string
}

// RunPgbench writes scripts, optionally warms up, then measures.
func RunPgbench(ctx context.Context, env *provisioner.Environment, cfg PgbenchConfig) (*PgbenchOutcome, error) {
	dir, err := os.MkdirTemp("", "benchforge-pgbench-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	var files []staged
	if cfg.Script != "" {
		files = append(files, staged{local: cfg.Script, weight: 1})
	} else {
		for i, q := range cfg.Queries {
			path := filepath.Join(dir, fmt.Sprintf("q%d.sql", i))
			body, err := pgbenchScript(q, cfg.Counts)
			if err != nil {
				return nil, fmt.Errorf("query %s: %w", q.ID, err)
			}
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				return nil, err
			}
			files = append(files, staged{local: path, weight: q.Weight})
		}
	}
	stagedFiles := make([]staged, len(files))
	for i, f := range files {
		remote, err := env.Stage(ctx, f.local)
		if err != nil {
			return nil, err
		}
		stagedFiles[i] = staged{local: remote, weight: f.weight}
	}
	if cfg.Warmup > 0 {
		args := pgbenchArgs(cfg, stagedFiles, int(cfg.Warmup.Seconds()), false, env.LogPrefix())
		if _, err := env.RunPgbench(ctx, args); err != nil {
			return nil, fmt.Errorf("pgbench warmup: %w", err)
		}
	}
	logDir, err := os.MkdirTemp("", "benchforge-pgbench-log-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(logDir)
	args := pgbenchArgs(cfg, stagedFiles, int(cfg.Duration.Seconds()), true, env.LogPrefix())
	stdout, err := env.RunPgbench(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("pgbench: %w\n%s", err, trim(stdout, 2000))
	}
	out := parsePgbench(stdout)
	out.Clients = cfg.Clients
	out.Threads = cfg.Threads
	logs, err := env.CollectLogs(ctx, logDir)
	if err == nil && len(logs) > 0 {
		res := stats.NewReservoir(sampleLimit, uint64(cfg.Seed)+42)
		for _, path := range logs {
			if err := readLog(path, res); err != nil {
				continue
			}
		}
		vals, seen := res.Values()
		out.Latency = stats.Summarize(vals)
		out.LogSamples = seen
		out.Approximate = res.Approximate()
	}
	return out, nil
}

type staged struct {
	local  string
	weight int
}

func pgbenchArgs(cfg PgbenchConfig, files []staged, seconds int, log bool, logPrefix string) []string {
	if seconds < 1 {
		seconds = 1
	}
	args := []string{
		"-n",
		"-c", strconv.Itoa(cfg.Clients),
		"-j", strconv.Itoa(cfg.Threads),
		"-T", strconv.Itoa(seconds),
		"--random-seed", strconv.FormatInt(cfg.Seed, 10),
	}
	if cfg.Rate > 0 {
		args = append(args, "-R", strconv.Itoa(cfg.Rate))
	}
	for _, f := range files {
		weight := f.weight
		if weight < 1 {
			weight = 1
		}
		args = append(args, "-f", fmt.Sprintf("%s@%d", f.local, weight))
	}
	if log {
		if logPrefix == "" {
			logPrefix = "/tmp/bfpg"
		}
		args = append(args, "-l", "--log-prefix="+logPrefix)
	}
	return args
}

func pgbenchScript(q manifest.Query, counts seed.Counts) (string, error) {
	var b strings.Builder
	for _, p := range q.Params {
		line, err := pgbenchSet(p, counts)
		if err != nil {
			return "", err
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	for _, stmt := range q.Statements {
		sql, err := substitutePlaceholders(stmt, q.Params)
		if err != nil {
			return "", err
		}
		b.WriteString(sql)
		if !strings.HasSuffix(strings.TrimSpace(sql), ";") {
			b.WriteString(";")
		}
		b.WriteByte('\n')
	}
	return b.String(), nil
}

func pgbenchSet(p manifest.Param, counts seed.Counts) (string, error) {
	switch p.Distribution {
	case "constant":
		if p.Type == "text" {
			return fmt.Sprintf(`\set %s '%s'`, p.Name, strings.ReplaceAll(p.Value, "'", "''")), nil
		}
		return fmt.Sprintf(`\set %s %d`, p.Name, p.Min), nil
	case "uniform":
		max := p.Max
		if p.MaxFrom != "" {
			m, err := counts.Max(p.MaxFrom)
			if err != nil {
				return "", err
			}
			max = m
		}
		return fmt.Sprintf(`\set %s random(%d, %d)`, p.Name, p.Min, max), nil
	default:
		return "", fmt.Errorf("distribution %s is not supported by the pgbench script generator", p.Distribution)
	}
}

func substitutePlaceholders(sql string, params []manifest.Param) (string, error) {
	if len(params) == 0 {
		return sql, nil
	}
	return placeholder.ReplaceAllStringFunc(sql, func(m string) string {
		n, _ := strconv.Atoi(m[1:])
		if n < 1 || n > len(params) {
			return m
		}
		return ":" + params[n-1].Name
	}), nil
}

var (
	reTx     = regexp.MustCompile(`number of transactions actually processed:\s+(\d+)`)
	reFail   = regexp.MustCompile(`number of failed transactions:\s+(\d+)`)
	reSkip   = regexp.MustCompile(`number of transactions skipped:\s+(\d+)`)
	reTPS    = regexp.MustCompile(`tps = ([0-9.]+)`)
	reLat    = regexp.MustCompile(`latency average = ([0-9.]+) ms`)
	reStddev = regexp.MustCompile(`latency stddev = ([0-9.]+) ms`)
)

func parsePgbench(stdout string) *PgbenchOutcome {
	out := &PgbenchOutcome{Raw: trim(stdout, 4000)}
	out.Transactions = firstInt(reTx, stdout)
	out.Failed = firstInt(reFail, stdout)
	out.Skipped = firstInt(reSkip, stdout)
	out.TPS = firstFloat(reTPS, stdout)
	avg := firstFloat(reLat, stdout)
	out.StddevMs = firstFloat(reStddev, stdout)
	out.Latency = stats.Latency{Mean: avg}
	return out
}

func readLog(path string, res *stats.Reservoir) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 3 {
			continue
		}
		us, err := strconv.ParseFloat(fields[2], 64)
		if err != nil {
			continue
		}
		res.Add(us / 1000)
	}
	return sc.Err()
}

func firstInt(re *regexp.Regexp, s string) int {
	m := re.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

func firstFloat(re *regexp.Regexp, s string) float64 {
	m := re.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	n, _ := strconv.ParseFloat(m[1], 64)
	return n
}

func trim(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n-- truncated"
}
