# Changelog

## 0.4.0

HTTP load and diagnosis.

- k6 constant-arrival-rate script generation and summary parsing. Dropped iterations mark the generator saturated.
- Demo HTTP app on localhost with a bounded slot count.
- Prometheus series and a Grafana dashboard JSON file.
- Diagnosis text stays at "likely" and lists a competing explanation.

## 0.3.0

Run admission.

- Explicit state machine, idempotent submit, global and per-repository limits, cancel, and stale-run recovery.
- HTTP API with bearer auth and GitHub webhook signature checks. Unknown repositories are rejected.
- Worker records the report, keeps infrastructure failures as `failed`, and still cleans up.
- `migrations/001_runs.sql` is the Postgres shape. The tested store is in-process memory.

## 0.2.0

CI planning.

- `benchforge plan` selects manifests from a git diff or an explicit path list and explains each choice.
- Unmapped application and infrastructure changes request the full suite instead of being skipped.
- `benchforge report` renders a pull-request comment and keeps the report exit code.
- GitHub Actions run unit tests, manifest validation, and the plan comment.

## 0.1.0

Local PostgreSQL benchmark.

- CLI commands `validate`, `run`, `compare`, and `version`.
- Manifest `benchforge.dev/v1alpha1` with defaults, known-field checks, workspace path checks, and a local CPU, memory, duration, and concurrency policy.
- Ephemeral PostgreSQL 17.6 container, digest-pinned, removed after the run.
- Deterministic `orders-mixed` dataset and reviewed orders schema.
- Go query runner and in-container pgbench runner.
- JSON report `benchforge.report/v1` and baseline comparison with pass, regression, inconclusive, and error outcomes.
- Documentation of the behavior above, and an explicit list of later phases that are not implemented.
