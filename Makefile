.PHONY: build install test lint fmt

VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT    ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_TIME ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

LDFLAGS := -X main.version=$(VERSION) \
           -X main.commit=$(COMMIT) \
           -X main.buildTime=$(BUILD_TIME)

build:
	go build -ldflags "$(LDFLAGS)" -o cctidy ./cmd/cctidy/

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/cctidy/

test:
	go test -tags integration ./...

lint:
	golangci-lint run ./...

fmt:
	golangci-lint fmt ./...
