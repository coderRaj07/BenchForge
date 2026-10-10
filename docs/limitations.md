# Limitations

These limits still apply. [What runs](what-runs.md) separates local Docker benchmarks from CI and from the canary decision function.

## Measurement

- One run per side. The regression percent is a fixed allowance, not a confidence interval from repeated samples.
- Latency percentiles exclude failed attempts. A query that always times out shows up as an error rate, not as a large p99.
- Query latency excludes pool acquire time. Pool wait has its own summary. Connection wait is not added into p95.
- Closed-loop tests measure achieved throughput. They do not prove the load generator had spare CPU. Open-loop saturation is detected only from scheduler lag plus a shortfall in completed attempts, not from host CPU metrics.
- `resources.loadGenerator` is validated and then not used to place a separate process. The CLI is the generator.
- Database counter deltas include warmup, ramp, and the explain queries that run after the window.
- Peak active sessions include BenchForge's own connections.
- `shared_buffers` and `max_connections` are chosen by the provisioner from the memory and concurrency request. Other PostgreSQL settings stay at the image defaults, including `fsync`. Results are not comparable with a server that has a different configuration even when the major version matches.
- pgbench percentiles require the transaction log. If the log cannot be copied out, the report keeps pgbench's average and says the percentiles are missing. A missing p95 compared with a baseline p95 above zero can look like a huge improvement or can fail a later absolute gate only if some other series trips it. Read `logSamples` before trusting pgbench p95.
- `transactional: true` does not change execution. Both runners already use one transaction per attempt.
- Inserts in a mix change the dataset while the run is in progress. Select-only manifests are the ones that stay comparable row for row.
- Scale 20 is the policy maximum (200,000 orders). Generation holds the dataset in memory and loads it with `COPY`. That is fine at the cap and is not a streaming loader.

## Safety limits that are intentional

- The SQL denylist is not a parser. Novel filesystem functions are not automatically rejected.
- External mode will still apply migrations. Empty of customers does not mean empty of other objects.
- `--keep` and a crashed Docker daemon can leave a container. Cleanup is inline, not a reconciling controller.
- Image digest mismatch does not make a baseline incompatible.

## Not collected by a normal benchmark run

A report from `benchforge run` stores query latency, pool wait, pgbench output, and `pg_stat_database` deltas. It does not store host CPU, host memory, disk utilization, `pg_stat_statements`, lock waits, or a flame graph. k6 is not executed. `capacity.Analyze` and `advanced.CostPerRun` run only when a caller passes samples and prices. They do not read the report by themselves. CI does not run any of those steps. See [what runs](what-runs.md).

## Compatibility of this document

If a later version removes one of these limits, delete the bullet in that same change. Leaving it would describe a guarantee the code no longer needs, or hide a guarantee the code still has.
