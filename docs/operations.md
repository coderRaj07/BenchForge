# Operations

This page is how to run version 0.1.0 on a workstation and how to remove what it creates.

## Prerequisites

- Go 1.27.2. The module line is `go 1.27`. A user-level install at `$HOME/sdk/go` matches the toolchain used to develop this tree. Put `$HOME/sdk/go/bin` on `PATH`.
- Docker, with permission to run containers. The integration test and `benchforge run` both call `docker`.
- A checkout of this repository. Run commands from the repository root so workload paths stay inside the workspace.

Host `pgbench` is not required. The PostgreSQL image contains pgbench, and the runner executes it with `docker exec`.

No cloud account, Kubernetes cluster, or Postgres superuser on the host is required.

## Validate, then run

```bash
go build -o bin/benchforge ./cmd/cli
./bin/benchforge validate -f workloads/manifests/orders-select.yaml
./bin/benchforge run \
  -f workloads/manifests/orders-select.yaml \
  -o reports/orders-select.json \
  --duration-seconds 8
```

The first ephemeral run uses the already pulled image. If the image is missing, `docker run` pulls the digest-pinned reference. That pull is the slow part.

`reports/` is gitignored.

## Resource use

A quickstart run asks for 1 CPU and 1 GiB for the database. `shared_buffers` for that size is 128 MiB. The Go process is the load generator. Four goroutines is the default concurrency on `orders-select.yaml`.

`orders-api.yaml` asks for 2 CPU, 4 GiB, 100 concurrent workers, and 180 seconds. That fits in the local policy and is a poor default for a first run. Validate it, then run it when the machine can spare it.

The policy ceiling is 4 CPU and 4 GiB per database request, 128 clients, and 30 minutes. Those numbers live in `manifest.LocalLimits`.

## Cleanup

Successful, failed, and cancelled runs call `docker rm -f` unless `--keep` was passed. Confirm with the run id from the summary:

```bash
docker ps -a --filter label=benchforge.run_id=RUN_ID
```

An empty result means the container is gone. List anything this tool left behind:

```bash
docker ps -a --filter label=benchforge.managed=true
```

Remove a kept container yourself:

```bash
docker rm -f CONTAINER
```

The integration test fails if a run id still has a container after `Run` returns.

There is no cloud account to bill in this version. Cost control is the local CPU, memory, duration, and concurrency ceilings, plus immediate container removal.

## Inspecting a database

`--keep` leaves the container running. The summary notes include the container name (`bf-` plus 12 hex characters). The published port is on `127.0.0.1` only. The password was generated for that run and is not printed. If you need a shell inside the container:

```bash
docker exec -it CONTAINER psql -U benchforge -d benchforge
```

Peer-style local connections inside the official image accept the `benchforge` user without putting the password on the command line.

For a database that outlives a run, the Compose file publishes `127.0.0.1:54329` with the local-only password `benchforge`:

```bash
docker compose -f deploy/docker-compose/docker-compose.yaml up -d
```

The CLI will not use that port unless you set `target.source: external` and pass `--dsn` and `--allow-external`. Do not point that flag at a database you care about. Seeding refuses non-empty data, but migrations would still execute.

Stop the Compose database with `docker compose -f deploy/docker-compose/docker-compose.yaml down`. Add `-v` only when you intend to drop its volume.

## Failure signs

| Symptom | What it usually means |
| --- | --- |
| `docker was not found on PATH` | Install Docker or use an external empty database with the two explicit flags |
| `postgres did not become ready` | The container never finished its init restart. The error includes the tail of `docker logs`. With `--keep`, inspect `bf-...` yourself |
| `server major version ... does not match` | The DSN is not the version the manifest named |
| `database is not empty` | External target already has customers. Use an empty database |
| `manifest validation failed` | The field list on stderr is the fix list |
| Exit code 3 | Read `comparison.reasons` or the saturation finding. The run is not a regression and not a pass |
| Exit code 2 | A gate failed. The summary lists the finding code and both numbers |
| `cleanupError` set | The result file is still valid. Remove the labeled container |

## Logs and reports

Text logs are the default. `--log-format json` is one JSON object per log line on stderr. Reports are pretty JSON. Keep the report if you want to compare later. The manifest fingerprint and the SQL fingerprints inside it say whether you are comparing the same files.

## Signals

Ctrl-C cancels measurement and still attempts `docker rm -f`. If the machine dies hard, the container can remain. The label filter above is the way to find it. There is no background reaper in this version.
