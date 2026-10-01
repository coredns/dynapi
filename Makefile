.DEFAULT_GOAL := coredns

BINARY ?= coredns
GITCOMMIT ?= development
GO ?= go
BUILDOPTS ?= -tags=grpcnotrace
GOLANGCI_LINT_VERSION := v2.14.0
GOLANGCI_LINT ?= $(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

.PHONY: all coredns build format format-check test vet lint verify integration openapi openapi-check benchmark

all: coredns

coredns:
	CGO_ENABLED=0 $(GO) build $(BUILDOPTS) -ldflags="-s -w -X github.com/coredns/coredns/coremain.GitCommit=$(GITCOMMIT)" -o $(BINARY) .

build: coredns

format:
	$(GOLANGCI_LINT) fmt

format-check:
	$(GOLANGCI_LINT) fmt --diff

test:
	$(GO) test -race $(BUILDOPTS) ./...

vet:
	$(GO) vet $(BUILDOPTS) ./...

lint:
	$(GOLANGCI_LINT) config verify
	$(GOLANGCI_LINT) run

verify: format-check lint openapi-check
	$(GO) build $(BUILDOPTS) ./...
	$(GO) vet $(BUILDOPTS) ./...
	$(GO) test -race $(BUILDOPTS) ./...

integration:
	$(GO) build -race $(BUILDOPTS) -o $(BINARY) .
	COREDNS_DYNAPI_BINARY="$(abspath $(BINARY))" $(GO) test -race -tags=integration,grpcnotrace -run '^Test(Installation|HTTPAPI)$$' -count=1 .

openapi:
	@$(GO) run $(BUILDOPTS) ./cmd/openapi > openapi.yaml.tmp && mv openapi.yaml.tmp openapi.yaml

openapi-check:
	@set -eu; document=$$(mktemp); trap 'rm -f "$$document"' EXIT; \
	$(GO) run $(BUILDOPTS) ./cmd/openapi > "$$document"; diff -u openapi.yaml "$$document"

benchmark:
	$(GO) test $(BUILDOPTS) -run '^$$' -bench . -benchmem ./plugins/dynapi
