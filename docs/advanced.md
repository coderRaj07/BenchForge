# Routing, skew, profiles, soak, and cost

Version 0.7.0 adds checks around a single PostgreSQL database. It does not run a sharded cluster.

`advanced.CheckRoute` accepts three kinds:

- `targeted` must contact exactly one shard.
- `scatter` and `aggregate` must contact every expected shard.

`advanced.Skew` puts `hotShare` of the traffic on the first tenant and splits the remainder evenly. The weights sum to 1.

`advanced.CaptureCPU` records a Go CPU profile for the duration you pass. A zero duration returns no bytes, so profiling stays off unless a caller asks for it.

`workload.soak: true` requires `durationSeconds` of at least 60. Shorter soak manifests fail validation.

`advanced.CompareEnvironments` returns the p95 ratio and, when CPU or memory differ, says the pair is not a regression verdict.

`advanced.CostPerRun` is `cpuCores × hours × cpu price + memoryGiB × hours × memory price`. A zero price uses $0.04 per CPU-hour and $0.005 per GiB-hour. Those are assumptions, not a cloud bill.
