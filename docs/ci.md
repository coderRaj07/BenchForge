# CI

Two workflows are in `.github/workflows`.

## ci.yml

Runs on push and pull request.

1. `actions/checkout@v4`
2. `actions/setup-go@v5` with Go 1.27.2
3. `go test ./...`
4. `go vet ./...`
5. Build `bin/benchforge`
6. Validate the four shipped manifests

This job does not start Docker and does not run the integration tag. Those checks stay local or on a labeled run.

## benchmark-plan.yml

Runs on pull request, with `contents: read` and `pull-requests: write`.

1. Checkout with full history.
2. `benchforge plan --base origin/<base ref> -o plan.md`
3. Append `plan.md` to the job summary.
4. `gh pr comment` using `github.token`.
5. Upload `plan.md` as the `benchmark-plan` artifact.

The comment is the suite selection, not a performance result. A performance result is produced by `benchforge run` and rendered with `benchforge report`.

Action references use the major tags `v4` and `v5` published by GitHub for those actions. The Go version is pinned to 1.27.2.
