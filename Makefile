BINARY := gzlab
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
PLATFORMS := darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64
LDFLAGS := -X github.com/arturoburigo/gzlab/internal/version.Version=$(VERSION) \
           -X github.com/arturoburigo/gzlab/internal/version.Commit=$(COMMIT) \
           -X github.com/arturoburigo/gzlab/internal/version.Date=$(DATE)

.PHONY: build
build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/gzlab

.PHONY: install
install:
	go install -ldflags "$(LDFLAGS)" ./cmd/gzlab

.PHONY: test
test:
	go test ./...

.PHONY: lint
lint:
	golangci-lint run ./...

.PHONY: fmt
fmt:
	gofmt -l -w .

.PHONY: run
run:
	go run ./cmd/gzlab

.PHONY: release
release: clean-dist
	@set -e; \
	for platform in $(PLATFORMS); do \
		os=$${platform%/*}; arch=$${platform#*/}; ext=""; archive_ext="tar.gz"; \
		if [ "$$os" = "windows" ]; then ext=".exe"; archive_ext="zip"; fi; \
		outdir="dist/$(BINARY)_$(VERSION)_$${os}_$${arch}"; \
		mkdir -p "$$outdir"; \
		echo "building $$os/$$arch"; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o "$$outdir/$(BINARY)$$ext" ./cmd/gzlab; \
		cp README.md LICENSE "$$outdir/"; \
		if [ "$$archive_ext" = "zip" ]; then \
			(cd dist && zip -qr "$(BINARY)_$(VERSION)_$${os}_$${arch}.zip" "$(BINARY)_$(VERSION)_$${os}_$${arch}"); \
		else \
			(cd dist && tar -czf "$(BINARY)_$(VERSION)_$${os}_$${arch}.tar.gz" "$(BINARY)_$(VERSION)_$${os}_$${arch}"); \
		fi; \
	done
	(cd dist && shasum -a 256 *.tar.gz *.zip > checksums.txt)

.PHONY: clean-dist
clean-dist:
	rm -rf dist
