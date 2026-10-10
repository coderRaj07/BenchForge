# Development

## Build and test

```bash
export PATH="$HOME/sdk/go/bin:$PATH"
export GOTOOLCHAIN=local
go test ./...
gofmt -w $(find . -name '*.go' -not -path './.git/*')
go vet ./...
go build -o bin/benchforge ./cmd/cli
```

`make test`, `make build`, `make vet`, and `make validate` wrap the same commands. `make integration` runs:

```bash
go test -count=1 -timeout 15m -tags=integration ./...
```

Integration tests call the local Docker daemon. They are not part of `go test ./...`. Expect several minutes: two query containers at scale 2 for 8 measured seconds plus warmup, and one pgbench container for 5 seconds, including image startup and seeding.

## Where to change behavior

| Change | Where |
| --- | --- |
| New manifest field | `internal/manifest/load.go` file structs and `Benchmark`, then [manifest](manifest.md) and the JSON Schema. Add a test that the shipped files still load and that the new mistake is rejected |
| Resource ceiling | `manifest.LocalLimits` only. Update [manifest](manifest.md) and [safety](safety.md) in the same change |
| New Postgres major | Add a digest or tag to `manifest.Images`, document it in [versions](versions.md), and do not accept free-form image strings |
| Regression rule | `internal/baseline/compare.go` and `compare_test.go`. Update [baselines](baselines.md) |
| Report field | `internal/report/report.go`, the summary if operators should see it, and [reports](reports.md) |
| Dataset shape | `internal/seed/orders.go` and `workloads/postgres/migrations`. Keep `Invariants` true |
| SQL denylist | `internal/sqlsafe` and [safety](safety.md) |

Do not add a placeholder package that returns success for a feature that is not implemented. The roadmap is the place to describe unfinished work.

## Conventions in this tree

- Contexts are passed into Docker, database, and runner calls. Cleanup uses a separate timeout so cancellation still deletes the container.
- Errors returned to the operator include the field or the phase (`provision postgres`, `apply 001_orders.sql`, `warmup`).
- Passwords stay out of slog fields and out of the report. New log lines should go through `redact` if they can contain a DSN.
- Percentiles and comparison are pure functions with table tests. The Docker test covers the path those functions cannot see.
- Exported names have a comment when the name alone is not the whole contract. Unexported helpers stay commented only where the control flow is easy to misread.

## Module path

`github.com/coderRaj07/benchforge`

The git author for commits in this project is `coderRaj07`. Do not rewrite git config to make that true. If `git config user.name` is a different account, stop and fix the identity outside the repository config before committing.
