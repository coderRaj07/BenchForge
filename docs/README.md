# BenchForge documentation

This directory is the operator and contributor reference. The Go loader, runner, and comparison code are the behavior that actually runs. When a document and the code disagree, the code and its tests win, and the document should be corrected.

## Read this first

| Document | What it answers |
| --- | --- |
| [Architecture](architecture.md) | Components, trust boundaries, and which parts exist in this release |
| [Lifecycle](lifecycle.md) | Run states, timestamps, and what each state is allowed to do |
| [Manifest](manifest.md) | Every manifest field, default, and limit |
| [Dataset](dataset.md) | How `orders-mixed` data is generated and checked |
| [CLI](cli.md) | Commands, flags, and exit codes |
| [Reports](reports.md) | JSON report fields and how to read a result |
| [Baselines](baselines.md) | Compatibility rules and regression math |
| [Safety](safety.md) | What a manifest is not allowed to do |
| [Operations](operations.md) | How to run locally, clean up, and stay inside resource limits |
| [Versions](versions.md) | Pinned language, database, and module versions |
| [Development](development.md) | How to build, test, and extend the code |
| [Limitations](limitations.md) | What this release does not claim |
| [Planner](planner.md) | Which manifests a diff selects, and why |
| [CI](ci.md) | GitHub Actions workflows |
| [API](api.md) | Run admission, states, and webhook checks |
| [Observe](observe.md) | k6 summaries, metrics, and diagnosis |
| [Cloud](cloud.md) | Artifacts, budgets, Terraform, and the benchmark Job |
| [Capacity](capacity.md) | Sustainable rate, CPU and memory estimates, canary gates |
| [Advanced](advanced.md) | Routing checks, skew, soak, profiles, and cost per run |
| [What runs](what-runs.md) | Canary, CI, and the local Docker lifecycle |
| [Roadmap](roadmap.md) | What each phase contains |

## Release status

Version **0.7.0** is the current release.

It can validate a manifest, benchmark an isolated PostgreSQL database, compare a baseline, plan a diff, admit runs through the API, parse a k6 summary, list likely bottlenecks, reject an over-budget estimate, size a measured sweep, gate a canary step, check shard contact counts, build a skewed tenant mix, capture a CPU profile on request, require a 60 second soak, and price a run from measured CPU and memory time.

Canary rollout, CI Docker benchmarking, Terraform apply, and a sharded database do not run. [What runs](what-runs.md) lists each path.
