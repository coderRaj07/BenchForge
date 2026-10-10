package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/coderRaj07/benchforge/internal/planner"
	"github.com/coderRaj07/benchforge/internal/report"
)

func cmdPlan(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("plan", flag.ContinueOnError)
	fs.SetOutput(stderr)
	workspace := fs.String("w", "", "workspace root (default: current directory)")
	base := fs.String("base", "", "git revision to diff against, using the three-dot diff base...HEAD")
	changed := fs.String("changed", "", "comma-separated paths, instead of a git diff")
	out := fs.String("o", "", "write the markdown plan to this path")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	root := *workspace
	if root == "" {
		wd, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		root = wd
	}
	var files []string
	switch {
	case *changed != "":
		for _, part := range strings.Split(*changed, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				files = append(files, part)
			}
		}
	case *base != "":
		listed, err := planner.ChangedFiles(context.Background(), root, *base)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		files = listed
	default:
		fmt.Fprintln(stderr, "plan: provide --changed or --base")
		return 1
	}
	plan, err := planner.Select(root, files)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	text := planner.Format(plan)
	if *out != "" {
		if err := os.WriteFile(*out, []byte(text), 0o644); err != nil {
			fmt.Fprintf(stderr, "write plan: %v\n", err)
			return 1
		}
	}
	fmt.Fprint(stdout, text)
	return 0
}

func cmdReport(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(stderr)
	file := fs.String("f", "", "benchmark report JSON")
	out := fs.String("o", "", "write markdown to this path")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *file == "" {
		fmt.Fprintln(stderr, "report: -f report.json is required")
		return 1
	}
	rep, err := report.Read(*file)
	if err != nil {
		fmt.Fprintf(stderr, "read report: %v\n", err)
		return 1
	}
	text := planner.PullRequestMarkdown(rep)
	if *out != "" {
		if err := os.WriteFile(*out, []byte(text), 0o644); err != nil {
			fmt.Fprintf(stderr, "write markdown: %v\n", err)
			return 1
		}
	}
	fmt.Fprint(stdout, text)
	return report.ExitCode(rep)
}
