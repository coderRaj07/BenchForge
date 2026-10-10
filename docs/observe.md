# Load tests, metrics, and diagnosis

Version 0.4.0 adds an HTTP target, a k6 script, Prometheus series, one Grafana dashboard, and a diagnosis function. k6 itself is not required to parse a saved summary.

`observe.Script` writes a constant-arrival-rate script. `observe.ParseK6` reads the summary JSON. `dropped_iterations` greater than zero marks the generator saturated. That run must not be treated as a valid capacity result.

`cmd/demoapp` listens on `127.0.0.1:8081` unless `BENCHFORGE_DEMO_ADDR` is set. `GET /healthz` and `GET /orders` are the routes. A fixed slot count records how long a request waited for a slot. The process does not connect to a database.

`observe.NewMetrics` registers:

- `benchforge_runs_total`
- `benchforge_run_failures_total`
- `benchforge_run_duration_seconds`
- `benchforge_queue_depth`
- `benchforge_active_runs`
- `benchforge_cleanup_failures_total`
- `benchforge_resource_budget_exceeded_total`

Labels are `outcome` only. SQL text, run ids, and commit hashes are not labels.

`dashboards/benchforge.json` graphs those series. Import it into Grafana. This repo does not start Grafana.

`observe.Diagnose` returns zero or more likely explanations. Every item includes the symptom, the metrics used, a competing explanation, a confidence word, what to inspect, a possible change, and what would confirm it. The symptom text says "likely". It does not name a root cause.
