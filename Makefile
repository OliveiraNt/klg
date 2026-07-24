BINARY  ?= klg
PKG     := github.com/OliveiraNt/klg
VERSION ?= dev
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

GO      ?= go

.PHONY: all build install run test test-race cover vet fmt tidy clean help

all: build

## build: Build the binary with version metadata
build:
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BINARY) .

## install: Install the binary into GOPATH/bin
install:
	$(GO) install -ldflags "$(LDFLAGS)" .

## run: Build and run against testdata/mixed.log
run: build
	./$(BINARY) --json-pretty < testdata/mixed.log

## test: Run all tests
test:
	$(GO) test ./...

## test-race: Run all tests with the race detector
test-race:
	$(GO) test -race ./...

## cover: Run tests with coverage report
cover:
	$(GO) test -cover ./...

## vet: Run go vet
vet:
	$(GO) vet ./...

## fmt: Format the code
fmt:
	$(GO) fmt ./...

## tidy: Tidy go.mod / go.sum
tidy:
	$(GO) mod tidy

## clean: Remove built artifacts
clean:
	rm -f $(BINARY) $(BINARY).exe

## help: Show this help
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /'
