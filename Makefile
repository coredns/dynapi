.DEFAULT_GOAL := coredns

BINARY ?= coredns
GITCOMMIT ?= development
GO ?= go
BUILDOPTS ?= -tags=grpcnotrace

.PHONY: all coredns build format format-check test vet verify integration

all: coredns

coredns:
	CGO_ENABLED=0 $(GO) build $(BUILDOPTS) -ldflags="-s -w -X github.com/coredns/coredns/coremain.GitCommit=$(GITCOMMIT)" -o $(BINARY) .

build: coredns

format:
	gofmt -w .

format-check:
	@test -z "$$(gofmt -l .)" || { gofmt -l .; exit 1; }

test:
	$(GO) test -race $(BUILDOPTS) ./...

vet:
	$(GO) vet $(BUILDOPTS) ./...

verify: format-check
	$(GO) build $(BUILDOPTS) ./...
	$(GO) vet $(BUILDOPTS) ./...
	$(GO) test -race $(BUILDOPTS) ./...

integration: coredns
	COREDNS_DYNAPI_BINARY="$(abspath $(BINARY))" $(GO) test -race -tags=integration,grpcnotrace -run '^TestInstallation$$' -count=1 .
