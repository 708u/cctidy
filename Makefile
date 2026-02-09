.PHONY: build install test lint fmt

build:
	go build -o ccfmt ./cmd/

install:
	go install ./cmd/

test:
	go test -tags integration ./...

lint:
	golangci-lint run ./...

fmt:
	golangci-lint fmt ./...
