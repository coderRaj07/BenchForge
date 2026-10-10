# Change detection and suite selection

`benchforge plan` decides which manifests to run for a set of changed paths. It does not start a benchmark. A selected manifest is a recommendation with a reason. An unmapped change does not disappear.

## Command

```bash
./bin/benchforge plan --changed workloads/postgres/sql/get_customer_orders.sql
./bin/benchforge plan --base origin/master -o plan.md
```

`--base` runs `git diff --name-only <base>...HEAD` in the workspace. `--changed` is a comma-separated list and does not call git. One of the two is required.

## How a path is classified

| Path | Selection |
| --- | --- |
| `docs/*.md`, `README.md`, `CHANGELOG.md` | Ignored. A docs-only diff selects nothing. |
| `workloads/manifests/*.yaml` | That manifest. |
| `workloads/postgres/migrations/*.sql` | Every manifest whose `dataset.migrations` directory contains the file. |
| Other `*.sql` | Manifests whose query file is that path, or whose query id appears in a `benchforge:query-id` comment in the changed file. |
| `app/`, `services/`, `cmd/demoapp/` | If the file contains `benchforge:query <id>`, the manifests that declare that id. Otherwise the full suite. |
| `deploy/` | Full suite. Infrastructure is not guessed down to one query. |
| Anything else that is not documentation | Full suite. |

The SQL comment form is:

```sql
-- benchforge:query-id get-customer-orders
```

Application files use the same marker, for example `// benchforge:query get-customer-orders`. The comment is evidence supplied by the author. The planner does not infer SQL from arbitrary Go.

## Confidence

`high` means every relevant path mapped onto one or more manifests.

`low` means at least one path did not map. `broader` is true and every manifest is included. The plan text lists the unmapped paths. That is the signal to run a wider test, not a skip.

## Pull-request text

`benchforge report -f report.json` renders the outcome, the query table, and the fail findings. Exit code matches the report: 0 pass, 2 regression, 3 inconclusive, 1 error.

The workflow `.github/workflows/benchmark-plan.yml` builds the plan from the pull-request base, appends it to the GitHub job summary, uploads `plan.md`, and posts it as a pull-request comment. It does not run the database benchmark. `.github/workflows/ci.yml` runs `go test`, `go vet`, the CLI build, and manifest validation on every push and pull request.

A regression, an inconclusive measurement, and an infrastructure error stay distinct because `benchforge report` uses the report's outcome as its exit code.
