.DEFAULT_GOAL := build

fmt:
		go fmt ./...

test:
		go test ./...

clean:
		go clean

build: fmt
		go build .

run: build
		go run .

install: fmt
		go install .

.PHONY: fmt test clean build run install
