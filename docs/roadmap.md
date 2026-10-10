# Roadmap

Items below are not implemented. Nothing in the 0.1.0 binary, the Docker test, or the report schema performs them. They are written here so the intended shape is reviewable without being mistaken for a feature.

## Phase 2 — CI regression detection

Implemented in 0.2.0. See [planner](planner.md) and [ci](ci.md).

- GitHub Actions for build, vet, unit tests, and manifest validation.
- A planner that reads a git diff, associates changed SQL with `-- benchforge:query-id` comments and manifest query ids, and selects a manifest.
- When the planner cannot tie an application change to a workload, it asks for the broader suite instead of skipping the run.
- `benchforge report` renders a pull-request comment. Its exit code preserves pass, regression, inconclusive, and error. The plan workflow posts the suite selection; it does not itself run the database benchmark.

## Phase 3 — API, queue, and state machine

Implemented in 0.3.0. See [api](api.md).

- `cmd/api` and `cmd/worker`.
- `migrations/001_runs.sql` for runs, events, and idempotency keys. The running API uses the in-process store covered by the tests.
- States `queued → validating → provisioning → preparing → warming_up → running → analyzing → completed|failed|inconclusive → cleaned_up`.
- Global and per-repository concurrency, cancellation, and recovery of runs whose worker disappeared.
- Bearer authentication and GitHub webhook signature verification. A webhook does not enqueue a repository that is not on the allow list.

## Phase 4 — HTTP load and diagnosis

Implemented in 0.4.0. See [observe](observe.md).

- k6 scripts generated for a constant arrival rate, and a summary parser that flags dropped iterations.
- A small demo HTTP service that does not open a database.
- Prometheus metrics with low-cardinality labels.
- One Grafana dashboard JSON file.
- Diagnosis that stays at "likely" and records a competing explanation.

OpenTelemetry tracing is not wired in this release.

## Phase 5 — AWS and Terraform

Implemented in 0.5.0. See [cloud](cloud.md).

- Reviewed Terraform for VPC, ECR, S3, SQS, IAM, and Secrets Manager. Pull requests do not supply Terraform to apply. This module does not create an EKS control plane.
- Local artifact files with a path check. The tests do not call S3.
- The SQS queue is declared in Terraform. The in-process queue from 0.3.0 remains the tested scheduler.
- Cleanup selection for managed, labeled, expired resources, plus a Kubernetes Job manifest with its own TTL.
- A cost budget check.

## Phase 6 — Capacity and canary

- Sustainable throughput from a concurrency or arrival sweep.
- Little's Law for average concurrency, and CPU seconds per request for a core estimate, each labeled as an estimate with the utilization target and the headroom that was assumed.
- Memory estimates only from measured working-set samples, not from request rate alone.
- Argo Rollouts (or an equivalent) manifests and a pure function that promotes, pauses, or rolls back from latency, error rate, sample count, and window length.
- No destructive benchmark pointed at the canary's shared database.

## Phase 7 — Extensions

- Shard-key, scatter-gather, and per-shard fields on the manifest, with a routing check. Not a distributed database.
- Tenant skew distributions.
- Optional pprof capture when the manifest asks for it, off otherwise.
- Soak duration within the existing timeout machinery.
- A cost-per-run figure from measured resource seconds and a price table stored in the repo.

Each phase should leave `go test ./...` passing and should update [versions](versions.md), this file, and the limitation list.
