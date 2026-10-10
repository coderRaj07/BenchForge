# BenchForge

BenchForge is a Go tool for repeatable PostgreSQL benchmarks. Version 0.7.0 runs an isolated database benchmark, compares it with a baseline, plans a diff, admits runs through an API, and estimates capacity from measured samples.

A developer points the CLI at a YAML manifest. BenchForge checks the manifest against a fixed resource policy, starts PostgreSQL in a container bound to `127.0.0.1`, applies the reviewed migration in the repo, loads a deterministic dataset, measures the declared queries, and deletes the container. The report records throughput, latency, errors, pool-acquire time, and database counters. A second report can be judged against the first.

This repository is an engineering project inspired by publicly described benchmarking practice. It is not an implementation of anyone else's internal system.

## What the local benchmark does

- Validates `benchforge.dev/v1alpha1` manifests and rejects unknown fields, path escapes, unsafe SQL, and resource requests above the local policy.
- Starts `postgres:17.6-bookworm` at a pinned digest, or PostgreSQL 16 from its pinned tag when the manifest asks for 16.
- Seeds the `orders-mixed` dataset from `dataset.seed` and `dataset.scale`. The same inputs produce the same rows.
- Runs a weighted query mix in-process, or runs pgbench from inside the database container.
- Writes a JSON report and a text summary.
- Compares a candidate with a baseline only when the workload and environment match.
- Returns separate exit codes for pass, regression, inconclusive, and failure.

## What a run does not do by itself

CI validates manifests and posts which suites a diff selected. It does not start Docker, save a benchmark, or delete a container. Canary support is a promote, pause, or rollback decision plus a rollout file. It does not deploy. Details are in [docs/what-runs.md](docs/what-runs.md). [docs/limitations.md](docs/limitations.md) lists measurement limits that still apply to the local runner.

## Quick start

Install Go 1.27.2 and Docker. From the repository root:

```bash
export PATH="$HOME/sdk/go/bin:$PATH"   # if Go is installed in the user SDK
go test ./...
go build -o bin/benchforge ./cmd/cli

./bin/benchforge validate -f workloads/manifests/orders-select.yaml
./bin/benchforge run -f workloads/manifests/orders-select.yaml -o reports/orders-select.json --duration-seconds 8
```

`orders-select.yaml` is the short indexed query. `orders-api.yaml` is a longer illustrative mix (180 seconds, 100 clients) and is meant to be validated, then run deliberately. `orders-select-regression.yaml` is the same workload with a query that cannot use the customer index:

```bash
./bin/benchforge run \
  -f workloads/manifests/orders-select-regression.yaml \
  -o reports/orders-select-slow.json \
  --baseline reports/orders-select.json \
  --duration-seconds 8 \
  --scale 2
```

Use the same `--duration-seconds` and `--scale` for both runs. Those overrides are part of the compatibility check.

pgbench:

```bash
./bin/benchforge run -f workloads/manifests/orders-pgbench.yaml -o reports/pgbench.json --duration-seconds 5
```

Exit codes are `0` pass, `1` usage or infrastructure failure, `2` regression, `3` inconclusive. A passing run whose container could not be removed also exits `1`. Details are in [docs/cli.md](docs/cli.md).

## Layout

```text
cmd/cli/                 command entrypoint
internal/cli/            flag parsing and exit codes
internal/manifest/       YAML load, defaults, limits
internal/engine/         one local run
internal/provisioner/    disposable PostgreSQL container
internal/seed/           orders-mixed generator
internal/runner/         query mix and pgbench
internal/baseline/       compatibility and regression judgment
internal/report/         JSON report and text summary
internal/sqlsafe/        statement splitting and the SQL denylist
internal/stats/          percentiles and reservoir sampling
workloads/               reviewed manifests, migrations, and SQL
schemas/                 JSON Schema for the manifest, subordinate to the Go loader
deploy/docker-compose/   optional long-lived database for inspection
docs/                    operator and contributor reference
```

## Documentation

Start at [docs/README.md](docs/README.md). The manifest reference, report reference, safety rules, and baseline rules each have their own page so the field lists stay complete.

## Safety default

The CLI creates an ephemeral database unless the manifest says `target.source: external` and the operator also passes `--dsn` and `--allow-external`. A manifest cannot choose the container image, mount the Docker socket, or raise the CPU and memory ceilings in `manifest.LocalLimits`. SQL files are read from inside the workspace and rejected if they contain server-side file or shell access. See [docs/safety.md](docs/safety.md).

## Tests

```bash
go test ./...
go test -count=1 -timeout 15m -tags=integration ./...
```

The integration tag starts Docker, runs the indexed query and the sequential-scan variant, checks that the slow run is a regression, runs a short pgbench script, and checks that labeled containers are gone afterward.
