# Reports

A report is a JSON document with `schemaVersion` `benchforge.report/v1`. `report.Write` pretty-prints it and replaces the destination by writing a sibling `.tmp` file and renaming it.

The text summary (`report.Summary`) is a subset. The JSON file is the record to keep.

## Top level

| Field | Meaning |
| --- | --- |
| `schemaVersion` | `benchforge.report/v1` |
| `runId` | UUID for this run. Also the Docker label `benchforge.run_id` |
| `state` | Latest lifecycle state. See [lifecycle](lifecycle.md) |
| `outcome` | `PASS`, `REGRESSION`, `INCONCLUSIVE`, or `ERROR` |
| `failureReason` | Present when the run did not finish a trustworthy measurement. Passwords are redacted |
| `cleanupError` | Present when `docker rm` failed |
| `manifestName` | `metadata.name` |
| `manifestPath` | Absolute path of the manifest that was loaded |
| `manifestFingerprint` | SHA-256 of the raw manifest bytes, hex |
| `startedAt`, `finishedAt` | UTC |
| `timeline` | `{state, at}` marks |
| `environment` | Database identity used for compatibility |
| `dataset` | Seed, scale, profile, and the row counts actually loaded |
| `workload` | Effective duration, concurrency, arrival rate, clients, threads. Includes CLI overrides |
| `queries` | One entry per manifest query, in manifest order |
| `pgbench` | Present for `workload.type: pgbench` |
| `aggregate` | All successful query samples, or the pgbench totals when that mode ran |
| `database` | Deltas from `pg_stat_database` plus peak active sessions |
| `load` | Offered, completed, dropped, generator saturation, pool wait |
| `assertions` | Gates that were applied, copied from the manifest after defaults |
| `findings` | Info, warning, and fail conclusions |
| `comparison` | Present when a baseline file was loaded |
| `notes` | Operator-facing caveats that are not pass/fail gates |

The report does not contain the DSN or the database password.

## Query result

| Field | Meaning |
| --- | --- |
| `id` | Manifest query id |
| `fingerprint` | SHA-256 of the whitespace-normalized SQL file |
| `iterations` | Successful executions |
| `errors` | Failed executions, including statement timeouts |
| `errorRate` | `errors / (errors + iterations)` |
| `rows` | Rows returned or rows affected, summed across successes |
| `latencyMs` | `p50`, `p95`, `p99`, `mean`, `max` in milliseconds, from successes only |
| `tps` | Successes divided by the measured wall clock, which includes in-flight queries that finish after the window |
| `approximate` | True when percentiles came from the 200,000-sample reservoir |
| `histogram` | Cumulative buckets. `leMs` is the upper bound in milliseconds. The last bucket uses a stand-in for `+Inf` |
| `errorSamples` | Up to five error strings |
| `plan` | `EXPLAIN (ANALYZE, BUFFERS)` text for a single read-only statement, truncated at 16 KiB. Absent for inserts. `explain failed: ...` when the explain itself failed; that does not fail the run |
| `poolWaitMs` | Latency of taking a connection from the pool before this query |

## Aggregate

`aggregate.iterations` is the sum of successful query executions. `aggregate.latencyMs` is the nearest-rank summary of those samples mixed together, not a weighted average of the per-query percentiles. `aggregate.tps` is total successes divided by the measured wall clock.

Absolute gates and regression gates are applied to the aggregate and to each query. One failing query fails the run. For pgbench, the same gates are also applied to a series named `pgbench`.

## pgbench object

| Field | Meaning |
| --- | --- |
| `transactions` | `number of transactions actually processed` |
| `failed` | `number of failed transactions` |
| `skipped` | `number of transactions skipped`, when pgbench printed it (`-R`) |
| `tps` | The `tps =` figure from pgbench, which excludes initial connection time |
| `latencyMs` | From the transaction log when it was collected. `mean` also echoes pgbench's `latency average` if the log did not replace it. If the log is missing, p50, p95, and p99 stay 0 and a note says so |
| `stddevMs` | pgbench `latency stddev` |
| `clients`, `threads`, `durationSeconds` | What was requested |
| `logSamples` | Lines read from `bfpg.*` |
| `rawSummary` | pgbench stdout, truncated to 4000 characters |

pgbench latency in the log is microseconds in the third column and is converted to milliseconds.

## Database block

Deltas can include warmup, ramp, and the post-run explain. The notes say so.

| Field | Meaning |
| --- | --- |
| `maxActiveSessions` | Peak of `pg_stat_activity` rows in state `active` for this database, sampled about twice a second. Includes BenchForge sessions |
| `xactCommit`, `xactRollback` | Delta |
| `blksHit`, `blksRead` | Delta |
| `cacheHitRatio` | `blksHit / (blksHit + blksRead)`, or 1 when both deltas are 0 |
| `deadlocks` | Delta. A non-zero value adds a note. It does not by itself change the outcome |
| `tupReturned` | Delta |

`environment.postgresVersion` is `server_version` from the server, not the manifest string. `environment.postgresMajor` is parsed from it and must equal `target.postgresVersion` or the run is `ERROR`. `environment.image` and `environment.imageId` are set for ephemeral containers. `environment.cpu` and `environment.memory` are the manifest text (`1`, `1Gi`) and are part of baseline compatibility.

## Load block

| Field | Meaning |
| --- | --- |
| `mode` | `closed_loop`, `open_loop`, or `pgbench` |
| `offered` | Attempts the scheduler tried to start. In a closed loop this equals completed plus errors |
| `completed` | Successes |
| `errors` | Statement failures |
| `dropped` | Open-loop attempts that found every worker busy, or pgbench skipped transactions |
| `lateTicks`, `ticks` | Open-loop schedule. A tick is late when the loop woke more than one interval after the planned time |
| `generatorSaturated` | True only when more than 20% of ticks were late and fewer than 90% of offered attempts completed |
| `saturationReason` | The sentence stored when that is true |
| `poolWaitMs` | Summary of acquire time across attempts |

Drops with `generatorSaturated: false` mean the workers were busy. That is backpressure from the database or from the concurrency cap, not proof that the load generator ran out of CPU. The report notes that distinction. A saturated generator forces `INCONCLUSIVE` even if latency also looks bad, because the measurement did not deliver the schedule it declared.

Closed-loop mode has no target rate. High worker utilization is the point of the test. It is not marked saturated.

## Findings

| Severity | Effect on outcome |
| --- | --- |
| `info` | None. Printed only in the JSON report, not in the summary, so the summary stays about failures |
| `warning` | Reserved. 0.1.0 does not emit warnings |
| `fail` | Decides `REGRESSION`, `INCONCLUSIVE`, or `ERROR` depending on the code |

Codes:

| Code | Outcome effect |
| --- | --- |
| `single_run_noise` | Info. One run per side; the regression percent is the noise allowance |
| `sql_fingerprint_changed` | Info. Query text changed. Comparison still uses the query id |
| `latency_p95_absolute`, `latency_p99_absolute` | Regression when the candidate exceeds the absolute gate |
| `latency_p95_regression`, `latency_p99_regression` | Regression when the candidate exceeds the baseline by more than `maxRegressionPercent` |
| `throughput_regression` | Regression when TPS falls by more than that percent |
| `error_rate` | Regression when the error rate exceeds `maxErrorRate` |
| `incompatible_baseline` | Inconclusive |
| `insufficient_samples` | Inconclusive |
| `load_generator_saturated` | Inconclusive |
| `no_samples` | Error |

Judgment order is: saturated generator, then zero samples, then too few samples, then an incompatible baseline, then absolute and relative gates. An earlier stop skips later gates. A compatible baseline that also breaks an absolute gate is a regression, not an inconclusive result.

## Comparison object

| Field | Meaning |
| --- | --- |
| `baselineRunId` | `runId` of the baseline file |
| `compatible` | Whether the compatibility rules passed |
| `reasons` | Human-readable mismatches. Empty when compatible |

The rules are documented in [baselines](baselines.md).
