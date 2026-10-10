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
| [Roadmap](roadmap.md) | Later phases, marked as not implemented until a release says otherwise |

## Release status

Version **0.3.0** adds run admission on top of local benchmarks and diff planning.

It can validate a manifest, start a disposable PostgreSQL 17 container, seed a deterministic dataset, run a Go query mix or pgbench, write a JSON report, compare that report with a compatible baseline, choose manifests from a diff, and admit runs through an authenticated API.

It does not yet run k6, Prometheus, Terraform, the capacity planner, or the canary controller. Those are specified in [roadmap](roadmap.md).
