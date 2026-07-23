.DEFAULT_GOAL := build

VERSION ?= $(shell git describe --tags)
COMMIT  ?= $(shell git rev-parse --short HEAD)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -ldflags "\
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.date=$(DATE)"

fmt:
		go fmt ./...

test:
		go test ./...

clean:
		go clean

build: fmt
		go build $(LDFLAGS) .

run: build
		go run $(LDFLAGS) .

install: fmt
		go install $(LDFLAGS) .

.PHONY: fmt test clean build run install
