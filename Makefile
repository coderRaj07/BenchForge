GO ?= go
BIN := bin/benchforge

.PHONY: build test integration vet validate

build:
	$(GO) build -o $(BIN) ./cmd/cli

test:
	$(GO) test ./...

integration:
	$(GO) test -count=1 -timeout 15m -tags=integration ./...

vet:
	$(GO) vet ./...

validate: build
	./$(BIN) validate -f workloads/manifests/orders-api.yaml
	./$(BIN) validate -f workloads/manifests/orders-select.yaml
	./$(BIN) validate -f workloads/manifests/orders-select-regression.yaml
	./$(BIN) validate -f workloads/manifests/orders-pgbench.yaml
