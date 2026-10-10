// Package planner chooses benchmark manifests from a set of changed paths.
// It explains every choice. When it cannot tie a change to a workload, it
// asks for every manifest instead of skipping the run.
package planner

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/coderRaj07/benchforge/internal/manifest"
)

const (
	ConfidenceHigh = "high"
	ConfidenceLow  = "low"
)

var queryAnnotation = regexp.MustCompile(`(?m)benchforge:query(?:-id)?\s+([a-z0-9](?:[-a-z0-9]{0,61}[a-z0-9])?)`)

// Plan is the result of looking at one change set.
type Plan struct {
	Selections []Suite  `json:"selections"`
	Broader    bool     `json:"broader"`
	Confidence string   `json:"confidence"`
	Skipped    bool     `json:"skipped"`
	Reasons    []string `json:"reasons"`
}

// Suite is one manifest to run and why.
type Suite struct {
	Name         string   `json:"name"`
	ManifestPath string   `json:"manifestPath"`
	QueryIDs     []string `json:"queryIds,omitempty"`
	Reasons      []string `json:"reasons"`
}

// Select loads every manifest under workloads/manifests and maps changes onto them.
func Select(workspace string, changes []string) (*Plan, error) {
	specs, err := LoadManifests(workspace)
	if err != nil {
		return nil, err
	}
	return SelectSpecs(workspace, changes, specs), nil
	// What: loads every shipped manifest and maps changed paths onto them.
	// Used by: internal/cli/plan.go cmdPlan, and internal/planner/plan_test.go.
	// Logic: specs is the validated manifest list from workloads/manifests. changes is passed through unchanged to SelectSpecs, which returns the plan.
}

// LoadManifests reads workloads/manifests/*.yaml inside the workspace.
func LoadManifests(workspace string) ([]*manifest.Benchmark, error) {
	dir := filepath.Join(workspace, "workloads", "manifests")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read manifests: %w", err)
	}
	var out []*manifest.Benchmark
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		spec, err := manifest.Load(filepath.Join(dir, e.Name()), workspace, manifest.LocalLimits())
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", e.Name(), err)
		}
		out = append(out, spec)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// SelectSpecs is the pure selection step. Paths are relative to workspace.
func SelectSpecs(workspace string, changes []string, specs []*manifest.Benchmark) *Plan {
	plan := &Plan{Confidence: ConfidenceHigh}
	if len(changes) == 0 {
		plan.Skipped = true
		plan.Reasons = append(plan.Reasons, "no changed paths were provided")
		return plan
	}
	selected := map[string]*Suite{}
	var unmatched []string
	relevant := 0
	for _, raw := range changes {
		path := cleanRel(raw)
		if path == "" || isDocs(path) {
			continue
		}
		relevant++
		kind := classify(path)
		matched := false
		switch kind {
		case "manifest":
			for _, spec := range specs {
				if rel(workspace, spec.Path) == path {
					add(selected, spec, path, "", "manifest file changed")
					matched = true
				}
			}
		case "migration":
			for _, spec := range specs {
				if spec.Migrations != "" && strings.HasPrefix(path, rel(workspace, spec.Migrations)+"/") {
					add(selected, spec, path, "", "migration used by this manifest changed")
					matched = true
				}
			}
		case "sql":
			ids := annotationIDs(filepath.Join(workspace, path))
			for _, spec := range specs {
				for _, q := range spec.Queries {
					qrel := rel(workspace, q.File)
					if qrel == path || contains(ids, q.ID) {
						add(selected, spec, path, q.ID, "query file or benchforge:query-id annotation matches this manifest")
						matched = true
					}
				}
			}
		case "app":
			ids := annotationIDs(filepath.Join(workspace, path))
			if len(ids) == 0 {
				unmatched = append(unmatched, path)
				continue
			}
			for _, spec := range specs {
				for _, q := range spec.Queries {
					if contains(ids, q.ID) {
						add(selected, spec, path, q.ID, "application file declares benchforge:query "+q.ID)
						matched = true
					}
				}
			}
		case "infra":
			unmatched = append(unmatched, path+" (infrastructure)")
			continue
		default:
			unmatched = append(unmatched, path)
			continue
		}
		if !matched && (kind == "sql" || kind == "migration" || kind == "manifest") {
			unmatched = append(unmatched, path)
		}
	}
	if relevant == 0 {
		plan.Skipped = true
		plan.Reasons = append(plan.Reasons, "changed paths are documentation only; no benchmark was selected")
		return plan
	}
	if len(unmatched) > 0 {
		plan.Broader = true
		plan.Confidence = ConfidenceLow
		plan.Reasons = append(plan.Reasons, "broader testing is required because these paths could not be tied to a single workload: "+strings.Join(unmatched, ", "))
		for _, spec := range specs {
			add(selected, spec, "", "", "included because the change set needs the full suite")
		}
	}
	for _, spec := range specs {
		if s, ok := selected[spec.Path]; ok {
			sort.Strings(s.QueryIDs)
			plan.Selections = append(plan.Selections, *s)
		}
	}
	sort.Slice(plan.Selections, func(i, j int) bool { return plan.Selections[i].ManifestPath < plan.Selections[j].ManifestPath })
	if len(plan.Selections) == 0 {
		plan.Skipped = true
		plan.Reasons = append(plan.Reasons, "no manifest matched the changed paths")
	}
	return plan
	// What: maps changed paths to manifests and asks for the full suite when a path does not map.
	// Used by: Select, and internal/planner/plan_test.go.
	// Logic: plan starts at high confidence. path is each cleaned relative change. isDocs skips documentation. relevant counts non-doc paths. kind comes from classify. selected is a map from manifest path to Suite, filled by add. matched records whether that path hit a manifest. unmatched collects paths that did not. relevant == 0 sets Skipped. unmatched non-empty sets Broader, lowers confidence, and adds every spec. Selections is the map values sorted by manifest path. An empty selection also sets Skipped.
}

func add(dst map[string]*Suite, spec *manifest.Benchmark, path, queryID, why string) {
	s, ok := dst[spec.Path]
	if !ok {
		s = &Suite{Name: spec.Name, ManifestPath: spec.Path}
		dst[spec.Path] = s
	}
	if path != "" {
		s.Reasons = append(s.Reasons, path+": "+why)
	} else if why != "" {
		s.Reasons = append(s.Reasons, why)
	}
	if queryID != "" && !contains(s.QueryIDs, queryID) {
		s.QueryIDs = append(s.QueryIDs, queryID)
	}
	// What: records one reason and optional query id on a suite.
	// Used by: SelectSpecs only.
	// Logic: dst is keyed by the manifest's absolute path. s is the existing Suite or a new one stored in dst. path non-empty prefixes the reason. queryID is appended once.
}

func classify(path string) string {
	switch {
	case strings.HasPrefix(path, "workloads/manifests/") && (strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")):
		return "manifest"
	case strings.Contains(path, "/migrations/") && strings.HasSuffix(path, ".sql"):
		return "migration"
	case strings.HasSuffix(path, ".sql"):
		return "sql"
	case strings.HasPrefix(path, "app/") || strings.HasPrefix(path, "services/") || strings.HasPrefix(path, "cmd/demoapp/"):
		return "app"
	case strings.HasPrefix(path, "deploy/"):
		return "infra"
	default:
		return "other"
	}
	// What: labels a relative path as manifest, migration, sql, app, infra, or other.
	// Used by: SelectSpecs.
	// Logic: the first matching prefix or suffix wins. app covers app/, services/, and cmd/demoapp/. Everything else, including internal Go files, is other and later forces the full suite.
}

func isDocs(path string) bool {
	if path == "README.md" || path == "CHANGELOG.md" {
		return true
	}
	return strings.HasPrefix(path, "docs/") && strings.HasSuffix(path, ".md")
	// What: reports whether a path is documentation and should not start a benchmark.
	// Used by: SelectSpecs.
	// Logic: README.md and CHANGELOG.md match exactly. Other markdown under docs/ also matches. A docs YAML file does not.
}

func annotationIDs(path string) []string {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	matches := queryAnnotation.FindAllStringSubmatch(string(body), -1)
	var ids []string
	for _, m := range matches {
		if !contains(ids, m[1]) {
			ids = append(ids, m[1])
		}
	}
	return ids
	// What: reads benchforge:query or benchforge:query-id markers from a file.
	// Used by: SelectSpecs for SQL and application files.
	// Logic: a missing file returns nil, which makes an application change unmapped. matches is every regex capture. ids keeps the first capture group, which is the query id, without duplicates.
}

func rel(root, path string) string {
	r, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(r)
	// What: makes an absolute path relative to the workspace, with forward slashes.
	// Used by: SelectSpecs when comparing a changed path with a manifest or query file.
	// Logic: r is the relative path. On error the original path is returned with slashes flipped. Success also flips slashes so Windows separators do not miss a match.
}

func cleanRel(path string) string {
	path = filepath.ToSlash(filepath.Clean(path))
	path = strings.TrimPrefix(path, "./")
	if path == "." || strings.HasPrefix(path, "../") {
		return ""
	}
	return path
	// What: normalizes one changed path and drops paths that leave the workspace.
	// Used by: SelectSpecs.
	// Logic: Clean removes . and extra separators. TrimPrefix drops a leading ./. A result of . or anything starting with ../ becomes an empty string, which SelectSpecs ignores.
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
	// What: reports whether v is already in list.
	// Used by: add and annotationIDs.
	// Logic: s walks list. The first equal element returns true. An exhausted list returns false.
}

// Format renders a plan for a pull request or a job summary.
func Format(plan *Plan) string {
	var b strings.Builder
	b.WriteString("## BenchForge plan\n\n")
	if plan.Skipped && len(plan.Selections) == 0 {
		b.WriteString("No benchmark was selected.\n\n")
	}
	fmt.Fprintf(&b, "Confidence: **%s**\n\n", plan.Confidence)
	for _, reason := range plan.Reasons {
		fmt.Fprintf(&b, "- %s\n", reason)
	}
	if len(plan.Reasons) > 0 {
		b.WriteString("\n")
	}
	if len(plan.Selections) == 0 {
		return b.String()
	}
	b.WriteString("| Manifest | Queries | Why |\n| --- | --- | --- |\n")
	for _, s := range plan.Selections {
		queries := strings.Join(s.QueryIDs, ", ")
		if queries == "" {
			queries = "all declared queries"
		}
		why := strings.Join(s.Reasons, "; ")
		fmt.Fprintf(&b, "| `%s` | %s | %s |\n", s.Name, queries, why)
	}
	b.WriteString("\n")
	return b.String()
	// What: renders a plan as markdown for the job summary or a pull-request comment.
	// Used by: internal/cli/plan.go cmdPlan.
	// Logic: b collects the heading and confidence. A skipped plan with no selections says no benchmark was selected. reason lines come from plan.Reasons. An empty Selections returns before the table. s is each suite. queries joins QueryIDs, or the words "all declared queries" when the list is empty. why joins Reasons with semicolons.
}
