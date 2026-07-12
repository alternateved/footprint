.DEFAULT_GOAL := build

fmt:
		go fmt ./...

lint: fmt
		golangci-lint run

test:
		go test ./...

clean:
		go clean

build: lint
		go build .

run: build
		go run .

install: build
		go install .

.PHONY: fmt lint test clean build run
