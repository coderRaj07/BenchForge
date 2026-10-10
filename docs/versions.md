# Versions

Versions below are what version 0.1.0 was written and tested against. Do not float them in a manifest.

| Component | Version | Where it is pinned |
| --- | --- | --- |
| Go | 1.27.2 used to develop and test. Module declares `go 1.27` | `go.mod`, developer SDK at `$HOME/sdk/go` |
| PostgreSQL server and pgbench | 17.6 (Debian 17.6-2.pgdg12+1) | Image `postgres:17.6-bookworm@sha256:f3bd19c606e442c3d7bdfa8002e03fe260a1023351e0ea4598032022b68dd6e3` |
| PostgreSQL 16 image | 16.11 tag, not digest-pinned, not used by the integration test | `manifest.Images["16"]` |
| `github.com/jackc/pgx/v5` | v5.7.6 | `go.mod` |
| `gopkg.in/yaml.v3` | v3.0.1 | `go.mod` |
| `github.com/google/uuid` | v1.6.0 | `go.mod` |
| Compose file image | Same digest as PostgreSQL 17 | `deploy/docker-compose/docker-compose.yaml` |

Indirect modules recorded by `go mod tidy` are in `go.mod` and `go.sum`. Application code does not import them.

The host `psql` on the development machine was PostgreSQL 16 client packages. The benchmark does not use that client. pgbench inside the 17.6 image is `pgbench (PostgreSQL) 17.6`.

## Not in this release

k6, Prometheus, Grafana, OpenTelemetry, Terraform, AWS SDKs, Argo Rollouts, and a Kubernetes client are not dependencies yet. They are named in [roadmap](roadmap.md). Adding one of them means pinning an exact module or image version in this file in the same change.
