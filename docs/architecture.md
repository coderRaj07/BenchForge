# Architecture

Version 0.1.0 is one process. `benchforge run` loads a manifest, prepares a database, measures a workload, writes a report, and removes the container. Later phases add a scheduler and remote execution in front of this same run path. They are not in the binary today. See [roadmap](roadmap.md).

## Trust boundary

Three inputs are treated differently.

| Input | Trust | What the process does with it |
| --- | --- | --- |
| Manifest YAML | Untrusted content, trusted operator | Parse with known fields only. Enforce `manifest.LocalLimits`. Resolve paths inside the workspace. |
| SQL and migration files | Reviewed files in the workspace | Read them, apply the denylist, then execute them on the run's database. |
| Container image | Hard-coded in `manifest.Images` | The manifest may select major version 16 or 17. It cannot supply an image name or a registry. |

The manifest's resource block is a request, not a grant. `resources.database.cpu: "64"` fails validation. There is no field for privileged mode, host networking, extra mounts, or an image override.

External databases are a second boundary. They require `target.source: external`, `--dsn`, and `--allow-external` together. The tool still refuses to seed a database that already contains customers, and it does not delete an external database.

## Components

```text
CLI (internal/cli)
  -> engine.Run
       -> manifest.Load
       -> provisioner.Docker or provisioner.External
       -> engine.applyMigrations
       -> seed.Build + seed.Apply
       -> runner.RunQueries or runner.RunPgbench
       -> baseline.Evaluate
       -> report.Write
```

| Package | Responsibility |
| --- | --- |
| `internal/cli` | Subcommands, process exit codes, signal context |
| `internal/manifest` | Defaults, validation, path jail, CPU and memory parsing |
| `internal/engine` | Ordering of one run, server-version check, report assembly |
| `internal/provisioner` | `docker run` / `docker rm`, pgbench executed inside the container |
| `internal/seed` | Deterministic row generation and `COPY` |
| `internal/runner` | Closed-loop and open-loop query execution, pgbench scripts and log parsing |
| `internal/baseline` | Compatibility and threshold judgment |
| `internal/report` | On-disk JSON and the text summary |
| `internal/sqlsafe` | Split, fingerprint, read-only check, denylist |
| `internal/stats` | Nearest-rank percentiles, reservoir, histogram |

The provisioner returns an `Environment`. Query execution uses its DSN. pgbench uses `Stage`, `RunPgbench`, and `CollectLogs`, so the runner does not put the database password in the pgbench argument list. `docker exec` receives it as `PGPASSWORD`, and command errors pass through `redact.Secrets`.

## Measurement path

For `workload.type` of `query` or `mixed`:

1. Ramp, if `rampUpSeconds` is set. Workers start across the ramp window. Samples are discarded. SQL that refers to a missing relation still aborts the run.
2. Warmup at full concurrency. Samples are discarded.
3. Measured window. Closed loop when `arrivalRatePerSecond` is 0: each worker starts the next query as soon as the previous one finishes. Open loop when the arrival rate is set: a scheduler offers one attempt per interval and counts a drop when every worker is busy.
4. One `EXPLAIN (ANALYZE, BUFFERS)` for each single read-only statement, after the window. That time is not included in the latency summary.

`mixed` in this release means a weighted mix of the declared queries executed by the Go runner. It does not also start pgbench. Use `workload.type: pgbench` for pgbench.

For pgbench, the runner writes a script per query (`\set` plus `:name` placeholders), copies it into the container, optionally runs an unmeasured warmup, then runs the timed script with `-l` so percentiles come from the transaction log. pgbench itself wraps each script execution in a transaction.

## Clocks and distributions

Latency is the time from `BEGIN` to `COMMIT`, including `statement_timeout` setup and the statements. Pool acquire time is recorded separately as `poolWaitMs` and is not added into the query latency.

Percentiles use the nearest-rank method: the rank for percentile `p` is `ceil(p/100*n)`, clamped to the last sorted sample. Up to 200,000 samples per query are kept. Above that, a reservoir keeps a uniform subset and the query result sets `approximate: true`.

Successful executions fill the latency distribution. Statement timeouts and other statement errors increment `errors` and are omitted from percentiles. The error rate is `errors / (errors + successes)`.

## Database counters

Before the measured call and after it, the engine reads `pg_stat_database` and subtracts. The "before" snapshot is after migrations, seed, and `ANALYZE`. The "after" snapshot is after warmup, the measured window, and the explain queries, so block and transaction deltas include warmup and explain. The report notes say this. Peak active sessions are sampled every 500ms during that same span and include BenchForge's own connections.

## Cleanup

Every ephemeral container is labeled `benchforge.managed=true` and `benchforge.run_id=<uuid>`. `Remove` calls `docker rm -f` with a fresh 30 second context, including when the run context is already cancelled. `--keep` skips removal and records the container name in the report notes. Cleanup failure is stored in `cleanupError`. A pass with a cleanup failure exits 1. A regression with a cleanup failure still exits 2 so the performance result stays visible. The report contains both.

External environments have no remove function.

## Configuration surface

There is no environment-variable configuration in 0.1.0. Behavior comes from the manifest, CLI flags, and the constants in code (`manifest.Images`, `manifest.LocalLimits`). That is deliberate: a shell environment should not be able to retarget the image or the resource ceiling.

The optional Compose file at `deploy/docker-compose/docker-compose.yaml` publishes a long-lived database on `127.0.0.1:54329` for manual inspection. The CLI does not read that file.
