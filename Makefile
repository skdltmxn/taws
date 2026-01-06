.PHONY: build run clean vet fmt test install

BINARY_NAME=taws
BUILD_DIR=bin
GO=go

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short=12 HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || echo unknown)

LDFLAGS=-s -w \
	-X 'github.com/skdltmxn/taws/internal/buildinfo.Version=$(VERSION)' \
	-X 'github.com/skdltmxn/taws/internal/buildinfo.Commit=$(COMMIT)' \
	-X 'github.com/skdltmxn/taws/internal/buildinfo.Date=$(DATE)'

build:
	@mkdir -p $(BUILD_DIR)
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/taws/

run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

clean:
	rm -rf $(BUILD_DIR)
	$(GO) clean

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

test:
	$(GO) test -v ./...

install:
	$(GO) install -ldflags "$(LDFLAGS)" ./cmd/taws/
