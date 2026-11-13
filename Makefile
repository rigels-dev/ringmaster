# Releases are named v{major}.{mid}.{minor}-{codename}; the VERSION file is the
# source of truth (see internal/version for the codename rules)
VERSION ?= $(shell cat VERSION)
LDFLAGS := -s -w -X ringmaster/internal/version.Version=$(VERSION)

.PHONY: all build web test test-web tag release dist lockfile clean

all: build web

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/ringmaster ./cmd/ringmaster

web:
	cd web && npm ci && npx nuxt build

test:
	go vet ./...
	go test ./...

# Unit tests for the web server's own logic; needs only Node.js, no npm install
# The web tests import npm packages, so install them when they are missing
test-web:
	cd web && { [ -d node_modules ] || npm ci; } && npm test

# Point web/package-lock.json at the public npm registry (after npm install via a mirror)
lockfile:
	scripts/normalize-lockfile.py web/package-lock.json

# Release archives for every platform, plus checksums, in dist/
dist: web
	scripts/dist.sh $(VERSION)

# Tag the current commit with the release name from VERSION
# Refuses while anything is uncommitted, so the tagged commit carries this VERSION
tag:
	@git diff --quiet HEAD || { echo "Commit your changes (including VERSION) before tagging" >&2; exit 1; }
	git tag -a $(VERSION) -m "Ringmaster $(VERSION)"

# Tag the release named in VERSION and push the tag, which starts the Release
# workflow. Run on main once the release PR is merged and pulled. It checks
# that first, so it is safe to run when unsure
release:
	@scripts/release.sh $(VERSION)

clean:
	rm -rf bin dist web/.output
