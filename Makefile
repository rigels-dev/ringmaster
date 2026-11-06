# Releases are named v{major}.{mid}.{minor}-{codename}; the VERSION file is the
# source of truth (see internal/version for the codename rules)
VERSION ?= $(shell cat VERSION)
LDFLAGS := -s -w -X ringmaster/internal/version.Version=$(VERSION)

.PHONY: all build web test tag dist clean

all: build web

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/ringmaster ./cmd/ringmaster

web:
	cd web && npm ci && npx nuxt build

test:
	go vet ./...
	go test ./...

# Release archives for every platform, plus checksums, in dist/
dist: web
	scripts/dist.sh $(VERSION)

# Tag the current commit with the release name from VERSION
tag:
	git tag -a $(VERSION) -m "Ringmaster $(VERSION)"

clean:
	rm -rf bin dist web/.output
