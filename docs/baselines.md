# Baselines and regression detection

A baseline is a previous report file. It is not a branch name or a floating "production" pointer. You choose the file with `--baseline` or `baseline.reportFile`.

Compare a candidate to a baseline only when they measured the same thing. `baseline.Compatible` requires all of the following:

| Property | Rule |
| --- | --- |
| `manifestName` | Equal |
| `dataset.profile` | Equal |
| `dataset.seed` | Equal |
| `dataset.scale` | Equal within `1e-9` |
| `environment.postgresMajor` | Equal |
| `workload.type` | Equal |
| `workload.durationSeconds` | Within 10% of the larger value |
| `workload.maxConcurrency` | Equal |
| `workload.arrivalRatePerSecond` | Equal |
| `workload.warmupSeconds` | Equal |
| `environment.cpu` and `environment.memory` | Both equal, compared as the manifest text |
| Query ids | Same set, order does not matter |

SQL fingerprints are allowed to differ. That difference is the change under test. The report records `sql_fingerprint_changed` as information.

Image digest is not part of compatibility. `environment.imageId` is stored so a person can see it. Two runs on the same major version and the same resource text are treated as comparable. If you need a stricter rule, do not mix reports whose `imageId` differs; the tool will not reject that pair by itself.

CLI overrides are part of the stored workload and dataset. A baseline taken with `--scale 2` does not match a candidate taken with `--scale 1`. The outcome is `INCONCLUSIVE` and `comparison.reasons` lists the mismatch. That is not a performance regression.

## Gates

Absolute gates read the candidate only.

- If `maxP95Ms` is set and a series p95 is greater, fail `latency_p95_absolute`.
- If `maxP99Ms` is set and a series p99 is greater, fail `latency_p99_absolute`.
- If `errorRate` is greater than `maxErrorRate`, fail `error_rate`.

Relative gates read both sides, and only when the baseline was compatible.

- p95 regression when `candidate > baseline * (1 + maxRegressionPercent/100)` and the baseline p95 is above 0.
- p99 the same way.
- Throughput regression when `candidateTPS < baselineTPS * (1 - maxRegressionPercent/100)` and the baseline TPS is above 0. A threshold above 100% floors the allowed TPS at 0.

Series are `aggregate`, each query id, and `pgbench` when that section exists. A failure on any series fails the run.

Percentiles are nearest-rank over one run. The info finding `single_run_noise` is always attached so a report does not look like a multi-run statistical test. The regression percent is the allowance for that noise. It is not a confidence interval. If two fast runs disagree by more than the threshold, the result is a regression even when the SQL did not change. Raise the threshold, or keep the measured window long enough that the distribution settles, rather than treating a single noisy pair as a product defect.

## Sample floor

Fewer than `minIterations` successes is `INCONCLUSIVE`, not a pass and not a regression. Zero successes is `ERROR`. The default floor is 30. The integration test's windows are sized to clear it.

## What is not a regression

- Docker failing to start.
- A migration error.
- Refusing to seed a non-empty database.
- A syntax error or a missing table.
- A saturated open-loop generator.
- An incompatible baseline.
- A cleanup failure. That is `cleanupError` and, on an otherwise passing run, exit code 1.

`baseline.FinalOutcome` returns `ERROR` whenever the execution error string is non-empty, even if a caller also computed a regression.

## Worked cases covered by tests

| Case | Outcome |
| --- | --- |
| Candidate p95 within 10% of the baseline and inside the absolute cap | `PASS` |
| Candidate p95 more than the threshold above the baseline | `REGRESSION` |
| Candidate p95 above `maxP95Ms` with no baseline | `REGRESSION` |
| Throughput drop beyond the threshold | `REGRESSION` |
| Error rate above `maxErrorRate` | `REGRESSION` |
| Different dataset seed | `INCONCLUSIVE` |
| Fewer successes than `minIterations` | `INCONCLUSIVE` |
| Generator saturated, even if p95 is large | `INCONCLUSIVE` |
| No successes | `ERROR` |
| Execution error string set, evaluation said regression | `ERROR` |

The Docker integration test runs `orders-select` and `orders-select-regression` at scale 2 for 8 seconds. It expects an index plan on the first, a sequential scan on the second, a pass when the first report is compared with itself, and a regression when the second is compared with the first. It also checks that both containers are gone.
