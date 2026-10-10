package planner

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// ChangedFiles returns paths changed between base and HEAD, using git diff.
// base is a revision such as origin/master. The caller supplies it; this
// function does not invent a default branch.
func ChangedFiles(ctx context.Context, workspace, base string) ([]string, error) {
	if strings.TrimSpace(base) == "" {
		return nil, fmt.Errorf("git base revision is required")
	}
	cmd := exec.CommandContext(ctx, "git", "-C", workspace, "diff", "--name-only", base+"...HEAD")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff %s: %w", base, err)
	}
	var files []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
	// What: lists files changed between a git revision and HEAD.
	// Used by: internal/cli/plan.go cmdPlan when --base is set. .github/workflows/benchmark-plan.yml passes origin/<base ref>.
	// Logic: base empty returns an error. cmd is git diff --name-only base...HEAD inside workspace. out is the command stdout. files collects non-empty trimmed lines from that stdout.
}
