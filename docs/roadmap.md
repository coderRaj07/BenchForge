# Roadmap

Items below are not implemented. Nothing in the 0.1.0 binary, the Docker test, or the report schema performs them. They are written here so the intended shape is reviewable without being mistaken for a feature.

## Phase 2 — CI regression detection

- GitHub Actions for build, vet, unit tests, and manifest validation.
- A planner that reads a git diff, associates changed SQL with `-- benchforge:query-id` comments and manifest query ids, and selects a manifest.
- When the planner cannot tie an application change to a workload, it asks for the broader suite instead of skipping the run.
- A pull-request comment rendered from the report, with exit codes preserved so a regression, an inconclusive run, and an infrastructure error are different checks.

## Phase 3 — API, queue, and state machine

- `cmd/api` and `cmd/worker`.
- PostgreSQL tables for runs, events, and idempotency keys.
- A queue interface with an in-process adapter first. SQS comes with the AWS phase.
- Persisted states `queued → validating → provisioning → preparing → warming_up → running → analyzing → completed|failed|inconclusive → cleaned_up`.
- Global and per-repository concurrency, retries, cancellation, and recovery of runs whose worker disappeared.
- Authentication on the API and webhook signature verification. Not a shared anonymous port.

## Phase 4 — HTTP load and diagnosis

- k6 scripts generated from a manifest, constant-arrival-rate, and a check that the generator itself was not saturated.
- A small demo HTTP service in front of the orders schema so the k6 run has a target that is not production.
- Prometheus metrics with low-cardinality labels: `benchforge_runs_total`, `benchforge_run_duration_seconds`, `benchforge_run_failures_total`, `benchforge_queue_depth`, `benchforge_active_runs`, `benchforge_cleanup_failures_total`, `benchforge_resource_budget_exceeded_total`.
- Grafana dashboards backed by those metrics.
- OpenTelemetry traces around a run.
- A rule-based diagnosis report: symptom, metrics, competing explanations, confidence, and what evidence is still missing. Language stays at "likely" until the evidence says otherwise.

## Phase 5 — AWS and Terraform

- Reviewed Terraform modules for VPC, EKS, ECR, S3, SQS, IAM, and Secrets Manager. Pull requests do not supply Terraform to apply.
- S3 for artifacts, with a local filesystem implementation of the same interface.
- SQS implementation of the queue interface.
- A cleanup reconciler for Jobs, volumes, and Terraform-owned ephemeral stacks. TTL on a Job is not enough.
- A cost budget that fails a run before it provisions past the cap.

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
