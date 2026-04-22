BINARY := bin/helm-resources
PKG := github.com/gekart/helm-resources
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

GOFLAGS := -trimpath
LDFLAGS := -s -w -X $(PKG)/cmd.version=$(VERSION)

.PHONY: all build test lint install uninstall clean tidy

all: build

build:
	@mkdir -p bin
	go build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(BINARY) ./cmd

test:
	go test ./...

lint:
	golangci-lint run

tidy:
	go mod tidy

install: build
	helm plugin install . || helm plugin update resources

uninstall:
	helm plugin uninstall resources

clean:
	rm -rf bin
