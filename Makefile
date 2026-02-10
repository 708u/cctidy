.PHONY: build install test lint fmt

build:
	go build -o cctidy ./cmd/

install:
	go install ./cmd/

test:
	go test -tags integration ./...

lint:
	golangci-lint run ./...

fmt:
	golangci-lint fmt ./...
