# API, worker, and state machine

Version 0.3.0 admits benchmark runs over HTTP and walks them through the lifecycle. The tested store is in-process memory in `internal/scheduler/scheduler.go`. `migrations/001_runs.sql` is the Postgres schema for a later shared database. The API process started by `cmd/api` does not share that memory with a second `cmd/worker` process.

## States

`queued → validating → provisioning → preparing → warming_up → running → analyzing → completed | failed | inconclusive → cleaned_up`

A run cannot skip from `queued` to `running`. `cleaned_up` has no successor. `completed` still occupies a concurrency slot until cleanup. Cancellation of an in-progress run sets `failed` with reason `cancelled`. A run that stops updating is marked `failed` with reason `abandoned` by `Service.Recover`.

## HTTP

`BENCHFORGE_API_TOKEN` is required. Send `Authorization: Bearer <token>`. The token is compared in constant time and is not echoed.

| Request | Behavior |
| --- | --- |
| `GET /healthz` | No authentication. Body `ok`. |
| `POST /v1/runs` | JSON `repository`, `manifestPath`, `workspace`, `idempotencyKey`. Same key returns the original run and does not enqueue twice. `429` when the global or per-repository limit is full. |
| `GET /v1/runs/{id}` | Current run and its events. |
| `POST /v1/runs/{id}/cancel` | Moves an in-progress run to `failed`. |
| `POST /v1/webhooks/github` | Checks `X-Hub-Signature-256`. A valid signature for a repository not listed in `BENCHFORGE_ALLOWED_REPOS` is `403` and does not enqueue work. |

`BENCHFORGE_API_ADDR` defaults to `:8080`.

## Worker

`Worker.Process` in `internal/worker/worker.go` is the whole execution step. It walks the states, calls the executor, stores the report, and then calls the cleaner. A regression is stored as `completed` with reason `regression` because the run finished; the report JSON still says `REGRESSION`. An executor error is `failed`, and cleanup still runs. An infrastructure failure is not recorded as a performance regression.

Limits on the shipped API are 4 global runs and 2 per repository, with a 30 minute deadline.
