# Run lifecycle

A local run records the states it actually entered. The names match the longer orchestration state machine so later phases can persist the same words. Version 0.1.0 does not have a queue, and it does not sit in `queued`.

## States written by version 0.1.0

| State | When it is recorded |
| --- | --- |
| `validating` | Immediately, before the manifest is parsed |
| `provisioning` | After the manifest is valid, before Docker or the external DSN is used |
| `preparing` | Database accepts connections; migrations and seed run here |
| `warming_up` | Before the query runner when ramp or warmup is non-zero. Before pgbench when that workload starts. The query runner performs ramp and warmup inside the call, so this mark is the start of that call, not a separate process. |
| `running` | Start of measurement. For the query runner this is immediately after `warming_up` when both are used. |
| `analyzing` | Samples are in memory and baseline comparison starts |
| `completed` | Comparison finished and the run function is about to return success |
| `cleaned_up` | `docker rm -f` succeeded |
| `failed` | The run returned an error and the container was not cleaned up. If cleanup did succeed, the state stays `cleaned_up` and `failureReason` still explains the error. |

`timeline` is the ordered list of `{state, at}` marks in UTC. `state` is the latest mark. `startedAt` is the validating mark. `finishedAt` is set when `Run` returns, after cleanup.

## Outcomes are not states

`state` says how far the machinery got. `outcome` says what the measurement means.

| Outcome | Meaning |
| --- | --- |
| `PASS` | Enough successful samples, generator not saturated, absolute gates met, and any compatible baseline is inside the regression threshold |
| `REGRESSION` | Enough samples and at least one latency, throughput, or error-rate gate failed |
| `INCONCLUSIVE` | The generator lagged, the baseline describes a different workload, or there were fewer successes than `minIterations` |
| `ERROR` | The run produced no successful samples, or infrastructure, migration, seed, or SQL setup failed |

An infrastructure error is always `ERROR`, including when a partial latency number happens to look bad. `baseline.FinalOutcome` enforces that. A statement timeout during an otherwise valid measured window is an error sample, not an infrastructure failure. If the error rate crosses `maxErrorRate`, the outcome is `REGRESSION`.

Missing relations and syntax errors (`42P01`, `42601`, `42703`, `42P02`) abort the run as `ERROR`. Continuing would only count the same failure.

## Cancellation

The CLI cancels the run context on SIGINT and SIGTERM. The query context is that same context, so in-flight work stops. Cleanup uses `context.Background` with its own timeout so a cancelled run still removes the container. A query cancelled only because the window ended is dropped and is not counted as an error. A query cancelled by its own `timeoutMs` while the window is still open is an error.

## Idempotency

Each run has a UUID. The container name is `bf-` plus the first 12 hex characters of that id. The CLI does not deduplicate repeated invocations. Deduplication belongs to the scheduler phase.
