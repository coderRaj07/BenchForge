// Package cli is the BenchForge command line.
package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/coderRaj07/benchforge/internal/baseline"
	"github.com/coderRaj07/benchforge/internal/engine"
	"github.com/coderRaj07/benchforge/internal/manifest"
	"github.com/coderRaj07/benchforge/internal/report"
	"github.com/coderRaj07/benchforge/internal/version"
)

// Main runs the CLI and returns a process exit code.
func Main(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 1
	}
	switch args[0] {
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	case "version", "--version", "-v":
		fmt.Fprintf(stdout, "benchforge %s\n", version.Version)
		return 0
	case "validate":
		return cmdValidate(args[1:], stdout, stderr)
	case "run":
		return cmdRun(args[1:], stdout, stderr)
	case "compare":
		return cmdCompare(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", args[0], usage)
		return 1
	}
}

func cmdValidate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	file := fs.String("f", "", "path to the benchmark manifest")
	workspace := fs.String("w", "", "workspace root; paths in the manifest must stay inside it (default: current directory)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *file == "" {
		fmt.Fprintln(stderr, "validate: -f manifest is required")
		return 1
	}
	spec, err := manifest.Load(*file, *workspace, manifest.LocalLimits())
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	fmt.Fprintf(stdout, "valid: %s (%s, %s, postgres %s, %d queries)\n", spec.Name, spec.WorkloadType, spec.Source, spec.PostgresVersion, len(spec.Queries))
	return 0
}

func cmdRun(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	file := fs.String("f", "", "path to the benchmark manifest")
	out := fs.String("o", "", "write the JSON report to this path, or - for stdout")
	workspace := fs.String("w", "", "workspace root (default: current directory)")
	baselinePath := fs.String("baseline", "", "baseline report JSON to compare against")
	dsn := fs.String("dsn", "", "external Postgres URL; requires --allow-external and target.source external")
	allowExternal := fs.Bool("allow-external", false, "permit an operator-supplied database instead of an ephemeral container")
	keep := fs.Bool("keep", false, "leave the ephemeral container running after the run")
	duration := fs.Int("duration-seconds", 0, "override workload.durationSeconds")
	scale := fs.Float64("scale", 0, "override dataset.scale")
	logFormat := fs.String("log-format", "text", "log format: text or json")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *file == "" {
		fmt.Fprintln(stderr, "run: -f manifest is required")
		return 1
	}
	logger := newLogger(stderr, *logFormat)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	rep, err := engine.Run(ctx, engine.Options{
		ManifestPath:     *file,
		Workspace:        *workspace,
		DSN:              *dsn,
		AllowExternal:    *allowExternal,
		Keep:             *keep,
		BaselinePath:     *baselinePath,
		DurationOverride: *duration,
		ScaleOverride:    *scale,
		Logger:           logger,
	})
	if rep != nil && *out != "" && *out != "-" {
		if werr := report.Write(*out, rep); werr != nil {
			fmt.Fprintf(stderr, "write report: %v\n", werr)
			return 1
		}
		fmt.Fprintf(stderr, "report written to %s\n", *out)
	}
	if rep != nil && *out == "-" {
		if werr := report.Write("-", rep); werr != nil {
			fmt.Fprintf(stderr, "write report: %v\n", werr)
			return 1
		}
	}
	if rep != nil {
		dest := stdout
		if *out == "-" {
			dest = stderr
		}
		fmt.Fprint(dest, report.Summary(rep))
	}
	if err != nil {
		fmt.Fprintf(stderr, "error: %s\n", err.Error())
		return 1
	}
	if rep == nil {
		return 1
	}
	return report.ExitCode(rep)
}

func cmdCompare(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("compare", flag.ContinueOnError)
	fs.SetOutput(stderr)
	candidate := fs.String("candidate", "", "candidate report JSON")
	basePath := fs.String("baseline", "", "baseline report JSON")
	out := fs.String("o", "", "write the candidate report with comparison filled in")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *candidate == "" || *basePath == "" {
		fmt.Fprintln(stderr, "compare: --candidate and --baseline are required")
		return 1
	}
	cand, err := report.Read(*candidate)
	if err != nil {
		fmt.Fprintf(stderr, "read candidate: %v\n", err)
		return 1
	}
	base, err := report.Read(*basePath)
	if err != nil {
		fmt.Fprintf(stderr, "read baseline: %v\n", err)
		return 1
	}
	eval := baseline.Evaluate(*cand, base)
	cand.Outcome = baseline.FinalOutcome(cand.FailureReason, eval)
	cand.Findings = eval.Findings
	cand.Comparison = eval.Comparison
	if *out != "" {
		if err := report.Write(*out, cand); err != nil {
			fmt.Fprintf(stderr, "write report: %v\n", err)
			return 1
		}
	}
	fmt.Fprint(stdout, report.Summary(cand))
	return report.ExitCode(cand)
}

func newLogger(stderr io.Writer, format string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	var h slog.Handler
	if format == "json" {
		h = slog.NewJSONHandler(stderr, opts)
	} else {
		h = slog.NewTextHandler(stderr, opts)
	}
	return slog.New(h)
}

const usage = `BenchForge runs isolated PostgreSQL benchmarks and compares them with a baseline.

Usage:
  benchforge validate -f manifest.yaml [-w workspace]
  benchforge run -f manifest.yaml [-o report.json] [-w workspace]
                 [--baseline report.json] [--duration-seconds N] [--scale N]
                 [--dsn URL --allow-external] [--keep] [--log-format text|json]
  benchforge compare --candidate report.json --baseline report.json [-o out.json]
  benchforge version

Exit codes:
  0  PASS
  1  usage error, infrastructure error, or cleanup failure after a pass
  2  REGRESSION
  3  INCONCLUSIVE (incompatible baseline, too few samples, or saturated generator)

The default target is an ephemeral PostgreSQL container bound to 127.0.0.1.
A manifest cannot select the container image, raise the local resource policy,
or point the tool at a database by itself. External databases require both
target.source: external and the --allow-external flag.

Run benchforge from the repository root so manifest paths stay inside the workspace.
`
