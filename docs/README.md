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
| [Roadmap](roadmap.md) | Later phases, marked as not implemented until a release says otherwise |

## Release status

Version **0.5.0** adds local artifacts, a cost gate, and reviewed Terraform on top of the load and diagnosis tools.

It can validate a manifest, start a disposable PostgreSQL 17 container, seed a deterministic dataset, run a Go query mix or pgbench, write a JSON report, compare that report with a compatible baseline, choose manifests from a diff, admit runs through an authenticated API, parse a k6 summary, list likely bottlenecks, and reject a run whose estimate exceeds its budget.

It does not yet size capacity or gate a canary. Those are specified in [roadmap](roadmap.md). The Terraform file is not applied by the test suite.
