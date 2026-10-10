# Safety

Version 0.1.0 defaults to an isolated database and a bounded amount of local CPU and memory. These controls are the product, not an extra mode.

## Isolation

`target.source` defaults to `ephemeral`. The container is published on `127.0.0.1` and an ephemeral port. It is not published on `0.0.0.0`. The Docker invocation does not use `--privileged`, `--network host`, or a mount of the Docker socket. The only file copied into the container is a pgbench script staged by the runner.

The image comes from `manifest.Images`:

| Major | Reference |
| --- | --- |
| 17 | `postgres:17.6-bookworm@sha256:f3bd19c606e442c3d7bdfa8002e03fe260a1023351e0ea4598032022b68dd6e3` |
| 16 | `postgres:16.11-bookworm` |

Version 17 is the digest exercised by the integration test. Version 16 is an allowed tag. A manifest that names any other version fails validation. There is no environment variable that replaces the image.

The database role and database name inside the container are both `benchforge`. The password is 18 random bytes, hex-encoded, written to a mode `0600` env file for `docker run`, and removed after the container starts. Reports and logs are passed through `redact.DSN` and `redact.Secrets`.

`--keep` is an operator choice. The container remains labeled `benchforge.managed=true` so it can be found with:

```bash
docker ps -a --filter label=benchforge.managed=true
```

## External databases

All three are required:

1. `target.source: external`
2. `--dsn`
3. `--allow-external`

Missing any one of them is an error. The tool does not infer a production host from a git remote, a `.env` file, or an unset flag.

Seeding refuses a non-empty `customers` table. There is no `--reset` flag in this version, because a truncate of an operator-supplied database is easy to point at the wrong place. Migrations still run. Point an external run at an empty database created for the test, not at a shared one.

pgbench on an external DSN is not wired to a host `pgbench` binary in this version. Ephemeral runs use the pgbench shipped in the database image. An external query-mode run uses the Go runner over the DSN.

## SQL

Before a file is executed, `sqlsafe.Check` rejects:

| Construct | Why |
| --- | --- |
| `pg_read_file`, `pg_read_binary_file` | Reads the server filesystem |
| `pg_ls_dir`, `pg_stat_file` | Lists or stats server files |
| `lo_import`, `lo_export` | Moves large objects to or from files |
| `COPY ... PROGRAM` | Runs a program as the server user |
| A line whose first non-space character is `\` | psql and pgbench meta-commands, including `\!` |

The check is a denylist applied to the raw file, including comments. It is not a sandbox. A novel filesystem function would get through until it is added. The container boundary is the control that still holds. Do not mount host directories into the database container.

`EXPLAIN (ANALYZE, BUFFERS)` runs only for a single statement that `sqlsafe.IsReadOnly` accepts. A statement that mentions `insert`, `update`, `delete`, `merge`, `copy`, `create`, `alter`, `drop`, `truncate`, `grant`, `revoke`, `vacuum`, `analyze`, `call`, or `do` is not explained. Writes are still executed as the workload. They run on the ephemeral database.

Migration files are the `*.sql` files in `dataset.migrations`, sorted by name. They pass the same denylist. They are not taken from a pull-request-supplied archive or from an arbitrary URL.

## Resource ceiling

`manifest.LocalLimits` caps CPU, memory, duration, concurrency, arrival rate, scale, query count, and SQL size. The Docker `--cpus` and `--memory` flags are set from the validated request. `shared_buffers` is one eighth of the container memory, never more than a quarter, and clamped between 16 MiB and 1 GiB. `max_connections` is at least 40 and at least `maxConcurrency + 16`. The pool opens `maxConcurrency + 2` connections.

`cleanup.ttlSecondsAfterFinished` does not keep the container alive. Immediate removal is the local behavior. The field is stored so a later reconciler has a declared TTL. `--keep` is the only way to retain a container, and it is a CLI flag, not a manifest field.

## Workspace jail

`manifest.Resolve` rejects absolute paths, null bytes, and any relative path that leaves the workspace. Symlinks that exist are evaluated and rejected if the target leaves the workspace. A missing file is rejected later as "cannot read" or "does not exist" after the lexical check.

## What this version does not authorize

- Benchmarks against a shared production database by default.
- Manifest-supplied shell commands.
- Manifest-supplied Terraform.
- Manifest-supplied container images.
- Unlimited concurrency or memory.
- Treating a cleanup failure or a connection error as a performance regression.

Production measurement, if it is ever added, has to be a separate explicit authorization path. It is not a flag on this CLI.
