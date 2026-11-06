# Version comes from the nearest git tag (e.g. v0.1.0), or "dev" without one
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X ringmaster/internal/cli.Version=$(VERSION)

.PHONY: all build web test clean

all: build web

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/ringmaster ./cmd/ringmaster

web:
	cd web && npm ci && npx nuxt build

test:
	go vet ./...
	go test ./...

clean:
	rm -rf bin web/.output
