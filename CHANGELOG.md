# Changelog

## 0.1.0

Local PostgreSQL benchmark.

- CLI commands `validate`, `run`, `compare`, and `version`.
- Manifest `benchforge.dev/v1alpha1` with defaults, known-field checks, workspace path checks, and a local CPU, memory, duration, and concurrency policy.
- Ephemeral PostgreSQL 17.6 container, digest-pinned, removed after the run.
- Deterministic `orders-mixed` dataset and reviewed orders schema.
- Go query runner and in-container pgbench runner.
- JSON report `benchforge.report/v1` and baseline comparison with pass, regression, inconclusive, and error outcomes.
- Documentation of the behavior above, and an explicit list of later phases that are not implemented.
