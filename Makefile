.PHONY: build install test lint fmt

build:
	go build -o cctidy ./cmd/cctidy/

install:
	go install ./cmd/cctidy/

test:
	go test -tags integration ./...

lint:
	golangci-lint run ./...

fmt:
	golangci-lint fmt ./...
