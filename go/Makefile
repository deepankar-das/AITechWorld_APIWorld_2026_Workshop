
BINARY_DAEMON   = bin/enforcer-daemon
BINARY_HOOK     = bin/enforcer-hook
BINARY_CENTRAL  = bin/enforcer-central
BINARY_CLIENT   = bin/enforcer-client
BINARY_AUTHSEED = bin/enforcer-authseed
VERSION        ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS         = -s -w -X main.Version=$(VERSION)
GOFLAGS         = CGO_ENABLED=0
DIST_TARBALL    = dist/enforcer-$(VERSION)-$(shell go env GOOS)-$(shell go env GOARCH).tar.gz
SBOM_FILE       = dist/enforcer-$(VERSION)-sbom.spdx.json
PROVENANCE_FILE = dist/enforcer-$(VERSION)-provenance.json
SIGNED_ARTIFACTS = $(DIST_TARBALL) $(SBOM_FILE) $(PROVENANCE_FILE)

.PHONY: all build build-go test test-v test-cover console copy-console clean package vet fmt build-linux \
	sbom sign provenance release-integrity verify-tools verify-signatures

all: build

# Full build: rebuild console from source + compile Go binaries (standalone use)
build: vet console
	$(GOFLAGS) go build -ldflags "$(LDFLAGS)" -o $(BINARY_DAEMON)  ./cmd/daemon
	$(GOFLAGS) go build -ldflags "$(LDFLAGS)" -o $(BINARY_HOOK)    ./cmd/hookhandler
	$(GOFLAGS) go build -ldflags "$(LDFLAGS)" -o $(BINARY_CENTRAL) ./cmd/central
	$(GOFLAGS) go build -ldflags "$(LDFLAGS)" -o $(BINARY_CLIENT)  ./cmd/client
	$(GOFLAGS) go build -ldflags "$(LDFLAGS)" -o $(BINARY_AUTHSEED) ./cmd/authseed
	@echo "✅ Built 5 binaries in bin/"
	@ls -lh bin/

# Quick build: assumes console is already built (used by scripts/build.sh)
build-go: vet copy-console
	$(GOFLAGS) go build -ldflags "$(LDFLAGS)" -o $(BINARY_DAEMON)  ./cmd/daemon
	$(GOFLAGS) go build -ldflags "$(LDFLAGS)" -o $(BINARY_HOOK)    ./cmd/hookhandler
	$(GOFLAGS) go build -ldflags "$(LDFLAGS)" -o $(BINARY_CENTRAL) ./cmd/central
	$(GOFLAGS) go build -ldflags "$(LDFLAGS)" -o $(BINARY_CLIENT)  ./cmd/client
	$(GOFLAGS) go build -ldflags "$(LDFLAGS)" -o $(BINARY_AUTHSEED) ./cmd/authseed
	@echo "✅ Built 5 binaries in bin/"
	@ls -lh bin/

# Copy already-built console assets into go/internal/console/ (no npm rebuild)
copy-console:
	rm -rf internal/console/out-hub internal/console/out-sentinel
	cp -r ../console/out-hub internal/console/out-hub
	cp -r ../console/out-sentinel internal/console/out-sentinel
	@echo "✅ Console static exports copied to internal/console/out-hub + out-sentinel/"

test:
	go test -count=1 ./internal/...

test-v:
	go test -v -count=1 ./internal/...

test-cover:
	go test -coverprofile=coverage.out ./internal/...
	go tool cover -html=coverage.out -o coverage.html
	@echo "✅ Coverage report: coverage.html"

vet:
	go vet ./...

fmt:
	gofmt -w .

console:
	cd ../console && npm ci && npm run build:all
	rm -rf internal/console/out-hub internal/console/out-sentinel
	cp -r ../console/out-hub internal/console/out-hub
	cp -r ../console/out-sentinel internal/console/out-sentinel
	@echo "✅ Console static exports copied to internal/console/out-hub + out-sentinel/"

clean:
	rm -rf bin/ coverage.out coverage.html

package: all
	mkdir -p dist
	tar czf $(DIST_TARBALL) \
		-C bin enforcer-daemon enforcer-hook enforcer-central enforcer-client enforcer-authseed
	@echo "✅ Package: $(DIST_TARBALL)"

verify-tools:
	@command -v syft >/dev/null 2>&1 || (echo "❌ syft not found. Install: https://github.com/anchore/syft" && exit 1)
	@command -v cosign >/dev/null 2>&1 || (echo "❌ cosign not found. Install: https://github.com/sigstore/cosign" && exit 1)

sbom: package
	@command -v syft >/dev/null 2>&1 || (echo "❌ syft not found. Install: https://github.com/anchore/syft" && exit 1)
	syft $(DIST_TARBALL) -o spdx-json=$(SBOM_FILE)
	@echo "✅ SBOM: $(SBOM_FILE)"

# Keyless signing by default (OIDC in CI). For key-based signing, set COSIGN_KEY.
sign: package sbom provenance
	@command -v cosign >/dev/null 2>&1 || (echo "❌ cosign not found. Install: https://github.com/sigstore/cosign" && exit 1)
	@for artifact in $(SIGNED_ARTIFACTS); do \
		cosign sign-blob --yes --bundle "$$artifact.sig.bundle" "$$artifact"; \
		echo "✅ Signature bundle: $$artifact.sig.bundle"; \
	done

provenance: package sbom
	@mkdir -p dist
	@tar_sha=$$(shasum -a 256 $(DIST_TARBALL) | awk '{print $$1}'); \
	sbom_sha=$$(shasum -a 256 $(SBOM_FILE) | awk '{print $$1}'); \
	ts=$$(date -u +"%Y-%m-%dT%H:%M:%SZ"); \
	printf '{\n  "_type": "https://in-toto.io/Statement/v1",\n  "subject": [\n    { "name": "$(DIST_TARBALL)", "digest": { "sha256": "%s" } },\n    { "name": "$(SBOM_FILE)", "digest": { "sha256": "%s" } }\n  ],\n  "predicateType": "https://slsa.dev/provenance/v1",\n  "predicate": {\n    "buildDefinition": {\n      "buildType": "https://github.com/anthropics/enforcer/go/Makefile#release-integrity",\n      "externalParameters": { "version": "$(VERSION)" },\n      "internalParameters": {},\n      "resolvedDependencies": []\n    },\n    "runDetails": {\n      "builder": { "id": "enforcer-local-build" },\n      "metadata": { "invocationId": "$(VERSION)", "startedOn": "%s", "finishedOn": "%s" },\n      "byproducts": []\n    }\n  }\n}\n' "$$tar_sha" "$$sbom_sha" "$$ts" "$$ts" > $(PROVENANCE_FILE)
	@echo "✅ Provenance: $(PROVENANCE_FILE)"

verify-signatures: sign
	@command -v cosign >/dev/null 2>&1 || (echo "❌ cosign not found. Install: https://github.com/sigstore/cosign" && exit 1)
	@for artifact in $(SIGNED_ARTIFACTS); do \
		cosign verify-blob --bundle "$$artifact.sig.bundle" "$$artifact" >/dev/null; \
		echo "✅ Verified: $$artifact"; \
	done

release-integrity: package sbom provenance sign verify-signatures
	@echo "✅ Release integrity artifacts ready:"
	@ls -lh $(DIST_TARBALL) $(SBOM_FILE) $(PROVENANCE_FILE) \
		$(DIST_TARBALL).sig.bundle $(SBOM_FILE).sig.bundle $(PROVENANCE_FILE).sig.bundle

# Cross-compile for Linux
build-linux:
	GOOS=linux GOARCH=amd64 $(GOFLAGS) go build -ldflags "$(LDFLAGS)" -o bin/linux-amd64/enforcer-daemon  ./cmd/daemon
	GOOS=linux GOARCH=amd64 $(GOFLAGS) go build -ldflags "$(LDFLAGS)" -o bin/linux-amd64/enforcer-hook    ./cmd/hookhandler
	GOOS=linux GOARCH=amd64 $(GOFLAGS) go build -ldflags "$(LDFLAGS)" -o bin/linux-amd64/enforcer-central ./cmd/central
	GOOS=linux GOARCH=amd64 $(GOFLAGS) go build -ldflags "$(LDFLAGS)" -o bin/linux-amd64/enforcer-client  ./cmd/client
	GOOS=linux GOARCH=amd64 $(GOFLAGS) go build -ldflags "$(LDFLAGS)" -o bin/linux-amd64/enforcer-authseed ./cmd/authseed
	@echo "✅ Linux amd64 binaries in bin/linux-amd64/"
