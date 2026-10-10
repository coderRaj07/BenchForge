# What actually runs

This page records which paths start real infrastructure and which paths only compute a decision or write a file. Version 0.7.0.

## Canary deployment

BenchForge does not deploy a canary.

`canary.Decide` in `internal/canary/canary.go` returns one word: `pause`, `rollback`, or `promote`. The inputs are candidate and stable p95, the candidate error rate, the sample count, and the list of traffic weights. The function does not call Kubernetes, Argo Rollouts, or a load balancer.

`deploy/argo-rollouts/orders-rollout.yaml` is the rollout shape for weights 10, 25, 50, and 100. Nothing in this repository installs that controller, applies the file, shifts traffic, or promotes a running service. A benchmark must not be aimed at a shared production database.

## Docker on a code change, then store and destroy, in CI

GitHub Actions does not do that.

| Workflow | When it runs | What it does | What it does not do |
| --- | --- | --- | --- |
| `.github/workflows/ci.yml` | push and pull request | `go test ./...`, `go vet`, build the CLI, validate the four manifests | Start Docker, run a benchmark, write a performance report, delete a container |
| `.github/workflows/benchmark-plan.yml` | pull request | `benchforge plan` from the diff, append `plan.md` to the job summary, comment on the pull request, upload `plan.md` | Start Docker, execute the selected suite, store TPS or latency, delete a container |

The pull-request comment is the list of manifests to run. It is not a benchmark result. `go test ./...` does not include the `integration` build tag, so the Docker test stays out of CI.

## Where Docker start, benchmark, save, and destroy does happen

That sequence is the local CLI:

```bash
./bin/benchforge run -f workloads/manifests/orders-select.yaml -o reports/orders-select.json
```

`engine.Run` starts `postgres:17.6-bookworm` bound to `127.0.0.1`, applies the reviewed migration, seeds `orders-mixed`, measures the workload, writes the JSON report to the `-o` path, and runs `docker rm -f` on the labeled container. `--keep` skips removal. SIGINT still attempts removal.

The same sequence is covered by:

```bash
go test -count=1 -timeout 15m -tags=integration ./internal/engine/
```

That test is local. It is not a job in either workflow. The report file stays on the machine that ran the command. CI does not upload it.

## Other paths that look like deployment and are not

| Path | What the code does | What it does not do |
| --- | --- | --- |
| `cmd/api` and `cmd/worker` | Admit a run, walk the state machine, and in the worker call `engine.Run` when a job is in the same process memory | Share a queue between two processes. The shipped API store is in-memory. `migrations/001_runs.sql` is the Postgres shape and is not applied by CI. |
| `deploy/terraform/main.tf` | Declares a VPC, subnet, S3 bucket, SQS queue, ECR repository, IAM role, and secret | Apply itself. Tests do not call AWS. The module does not create an EKS cluster. |
| `deploy/kubernetes/benchmark-job.yaml` | Describes a Job with CPU and memory limits, `activeDeadlineSeconds`, and `ttlSecondsAfterFinished` | Submit the Job. No cluster is contacted. |
| `observe.Script` and `observe.ParseK6` | Write a k6 script and read a saved summary. Dropped iterations mark the generator saturated. | Install or run k6. CI does not execute the script. |
| `observe.NewMetrics` and `dashboards/benchforge.json` | Register Prometheus series and provide a dashboard file | Scrape metrics or start Grafana. |
| `capacity.Analyze` | Estimate sustainable rate, Little's Law concurrency, CPU cores, and memory from samples you pass in | Read a benchmark report by itself or resize a cluster. |
| `cloud.Put` and `cloud.Allow` | Write a local artifact file and reject an estimate above a budget | Upload to S3 or stop a provisioner in CI. |
| `advanced.CheckRoute` | Check that a targeted query contacted one shard and that scatter or aggregate contacted the expected count | Start a sharded database. |
| `advanced.CaptureCPU` | Record a Go CPU profile when the caller passes a positive duration | Turn profiling on for a normal benchmark. |
| OpenTelemetry | Not wired | Emit traces. |
