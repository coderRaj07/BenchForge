# CLI

Binary: `bin/benchforge`, built from `./cmd/cli`.

```text
benchforge validate -f manifest.yaml [-w workspace]
benchforge run -f manifest.yaml [-o report.json] [-w workspace]
               [--baseline report.json] [--duration-seconds N] [--scale N]
               [--dsn URL --allow-external] [--keep] [--log-format text|json]
benchforge compare --candidate report.json --baseline report.json [-o out.json]
benchforge plan [--changed path,path | --base git-rev] [-w workspace] [-o plan.md]
benchforge report -f report.json [-o comment.md]
benchforge version
benchforge help
```

Flags are defined on the subcommand. `benchforge -f x` is not valid; the command comes first.

## validate

Loads the manifest with `manifest.LocalLimits` and prints one line on success. On failure it prints every field error to stderr and exits 1. It does not start Docker.

`-w` is the workspace root. The default is the process working directory. Run from the repository root when using the shipped manifests.

## run

Runs `engine.Run`. Logs go to stderr. The text summary goes to stdout. With `-o -`, the JSON report goes to stdout and the summary goes to stderr.

| Flag | Meaning |
| --- | --- |
| `-f` | Manifest path. Required |
| `-o` | Report path. Parent directories are created. `-` writes JSON to stdout |
| `-w` | Workspace root |
| `--baseline` | Report JSON to compare against. Overrides `baseline.reportFile` in the manifest |
| `--duration-seconds` | Replaces `workload.durationSeconds` after validation. Must stay inside the local policy. Recorded in the report workload and in `notes` |
| `--scale` | Replaces `dataset.scale` the same way |
| `--dsn` | Postgres URL for an external database |
| `--allow-external` | Required together with `--dsn` and `target.source: external` |
| `--keep` | Do not `docker rm` the container. The name is printed in the summary notes |
| `--log-format` | `text` (default) or `json` |

SIGINT and SIGTERM cancel the run context. Cleanup still runs.

The report file is written whenever a report object exists, including infrastructure failures after validation succeeded. A manifest that fails validation does not produce a report file. The error is only on stderr.

## compare

Reads two reports, runs `baseline.Evaluate` again using the assertions stored in the candidate, prints the summary, and optionally writes the candidate back out with `comparison` and `findings` filled in. This does not re-run the benchmark. Editing the assertions in the candidate JSON changes the gates. Treat report files as results, not as a second manifest language.

## version

Prints `benchforge` and `internal/version.Version`. The 0.1.0 constant is the local benchmark release.

## Exit codes

| Code | When |
| --- | --- |
| 0 | `outcome` is `PASS` and `cleanupError` is empty |
| 1 | Usage error, failed validation, infrastructure or SQL setup error, unreadable baseline, or `PASS` with a cleanup error |
| 2 | `outcome` is `REGRESSION`. Kept even if cleanup also failed |
| 3 | `outcome` is `INCONCLUSIVE` |

`report.ExitCode` is the function the CLI uses. `report.Summary` is the text it prints.

## Logs

Info logs are state transitions: manifest validated, database ready, cleaned up, analysis complete. They include `run_id` and the manifest name. They do not include the DSN or the generated password. Docker failures are passed through `redact.Secrets` and `redact.DSN` before they are stored on the report.

## Examples

```bash
./bin/benchforge validate -f workloads/manifests/orders-api.yaml

./bin/benchforge run \
  -f workloads/manifests/orders-select.yaml \
  -o reports/base.json \
  --duration-seconds 8 \
  --scale 2

./bin/benchforge run \
  -f workloads/manifests/orders-select-regression.yaml \
  -o reports/slow.json \
  --baseline reports/base.json \
  --duration-seconds 8 \
  --scale 2

./bin/benchforge compare --candidate reports/slow.json --baseline reports/base.json
```

The second command exits 2 when the sequential scan is slower than the threshold stored in the candidate report. It exits 3 when the two reports are not the same workload, for example when the scale overrides differ.
