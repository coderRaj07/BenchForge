# Benchmark manifest

Manifests are YAML documents with `apiVersion: benchforge.dev/v1alpha1` and `kind: Benchmark`. The loader uses `yaml.v3` with known fields enabled, so a typo is a validation error rather than a silently ignored setting.

`schemas/benchmark.v1alpha1.schema.json` documents the shape. The Go loader is the authority. It also enforces limits and path rules that a schema file cannot fully express.

Shipped examples:

| File | Use |
| --- | --- |
| `workloads/manifests/orders-select.yaml` | Short indexed query. This is the quickstart. |
| `workloads/manifests/orders-select-regression.yaml` | Same identity, sequential scan. Use it to see a regression. |
| `workloads/manifests/orders-pgbench.yaml` | pgbench custom scripts generated from the queries. |
| `workloads/manifests/orders-api.yaml` | Longer illustrative mix. Validate it before spending three minutes of load. |

Run the CLI from the repository root, or pass `-w` as that root. Every relative path is joined to the manifest directory and must remain inside the workspace after `Clean` and, when the path exists, after `EvalSymlinks`. Absolute paths are rejected.

## Document

| Field | Required | Default | Bounds and notes |
| --- | --- | --- | --- |
| `apiVersion` | yes | | Exactly `benchforge.dev/v1alpha1` |
| `kind` | yes | | Exactly `Benchmark` |
| `metadata.name` | yes | | Lowercase DNS label, max 63 characters |
| `metadata.owner` | no | empty | At most 64 characters, no newlines |
| `metadata.annotations` | no | empty | String map. Not interpreted in 0.1.0 |
| `target.type` | yes | | `postgres` only |
| `target.source` | no | `ephemeral` | `ephemeral` or `external` |
| `target.postgresVersion` | no | `"17"` | `"16"` or `"17"`. Selects `manifest.Images`, not a free-form image |
| `dataset.seed` | no | `42` | Integer `>= 0`. `0` is a valid seed |
| `dataset.scale` | no | `1` | `0.01` to `20` under the local policy |
| `dataset.profile` | yes | | `orders-mixed` only |
| `dataset.warmupSeconds` | no | `10` | `0` to `600`. Unmeasured full-concurrency work before the window |
| `dataset.migrations` | yes | | Relative directory of `*.sql` files, applied in name order |
| `workload.type` | yes | | `query`, `mixed`, or `pgbench` |
| `workload.durationSeconds` | no | `30` | `1` to `1800` |
| `workload.arrivalRatePerSecond` | no | `0` | `0` means closed loop. Otherwise open loop, max `20000` |
| `workload.maxConcurrency` | no | `4` | `1` to `128`. Also the ceiling for pgbench clients |
| `workload.rampUpSeconds` | no | `0` | `0` to `600`. Unmeasured. Query and mixed workloads only |
| `workload.clients` | no | `maxConcurrency` for pgbench | pgbench only. `1` to `maxConcurrency` |
| `workload.threads` | no | clients, capped at 8 | pgbench only. `1` to `clients` |
| `workload.script` | no | | pgbench only. A reviewed script path. Cannot be combined with `queries` |
| `assertions.maxP95Ms` | no | unset | Omit it to disable the absolute p95 gate. If set, must be `>= 1` |
| `assertions.maxP99Ms` | no | unset | Same, and must be `>= maxP95Ms` when both are set |
| `assertions.maxErrorRate` | no | `0.01` | `0` to `1`. `0` means any error fails the gate |
| `assertions.maxRegressionPercent` | no | `10` | `0` to `500`. Used only when a baseline is present |
| `assertions.minIterations` | no | `30` | Successful iterations required before a pass or regression is declared |
| `resources.database.cpu` | no | `"1"` | Cores or millicores (`500m`). Local max is 4 |
| `resources.database.memory` | no | `"1Gi"` | `Ki`, `Mi`, `Gi`, `K`, `M`, `G`. At least `256Mi`, at most `4Gi` |
| `resources.loadGenerator.cpu` | no | `"1"` | Validated and stored. The CLI process is the generator in 0.1.0; this does not start a second container |
| `resources.loadGenerator.memory` | no | `"512Mi"` | Same. Above 4Gi is rejected |
| `cleanup.ttlSecondsAfterFinished` | no | `600` | `0` to `86400`. Recorded for later reconciliation. The CLI deletes the container immediately unless `--keep` is set |
| `baseline.reportFile` | no | | Relative path to a previous report. The CLI `--baseline` flag wins when both are set |

`query` and `mixed` both execute the weighted query list in the Go runner. The split exists so a manifest can say "this is a mix" without pretending pgbench ran. `pgbench` does not also run the Go loop.

## Queries

At least one query is required unless `workload.script` is set. At most 32 queries. Ids are lowercase DNS labels and must be unique. Weights are relative, not percentages: weights 70 and 30 select the first query 70% of the time. A missing weight defaults to 1. The allowed range is 1 to 10000.

`timeoutMs` defaults to 1000 and must be between 1 and 120000. It becomes both the Go context deadline and PostgreSQL `statement_timeout` for that execution.

`transactional: true` is accepted and stored. In 0.1.0 every Go execution already runs inside a transaction so the timeout is local to that transaction. pgbench also runs one script as one transaction. The flag does not change that behavior; it records the author's intent in the validated spec.

`file` is a relative path to SQL. The file must be at most 64KiB. Multiple statements are split on semicolons that are outside quotes, comments, and dollar quotes. Parameters use `$1`, `$2`, in the order of `params`.

## Parameters

| Field | Notes |
| --- | --- |
| `name` | SQL identifier. Used as the pgbench variable name |
| `type` | `int` or `text` |
| `distribution` | `uniform` (default) or `constant` |
| `min` | Uniform ints. Defaults to 1 when `maxFrom` is set |
| `max` | Inclusive upper bound |
| `maxFrom` | `customers`, `products`, or `orders`. Resolved after seeding. Do not also set `max` |
| `value` | Required for `constant` |

`text` parameters only support `constant`. Uniform text and distributions other than uniform and constant are rejected, not approximated.

## SQL annotation comment

Shipped SQL files start with a comment of the form:

```sql
-- benchforge:query-id get-customer-orders
```

Version 0.1.0 does not parse that comment. The manifest `queries[].id` is what the runner and the comparison use. The comment is there so a later change detector can associate a file with an id without guessing.

## Local policy

`manifest.LocalLimits` is compiled into the binary:

| Limit | Value |
| --- | --- |
| Database and load-generator CPU | 4 cores |
| Memory | 4 GiB |
| Duration | 1 to 1800 seconds |
| Warmup and ramp | 0 to 600 seconds |
| Concurrency | 1 to 128 |
| Arrival rate | 0 to 20000 per second |
| Scale | 0.01 to 20 |
| Queries | 32 |
| SQL file size | 64 KiB |

A manifest cannot raise these numbers. Changing them means changing the code and reviewing that change.

## Validation errors

`manifest.Load` returns every problem it can see in one pass, except for YAML syntax and unknown-field errors, which stop at decode time. Each message includes the field path, for example `resources.database.cpu: exceeds local policy maximum of 4 cores`.
