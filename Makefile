BINARY := gitlab-tui
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X github.com/arturoburigo/gitlab-tui/internal/version.Version=$(VERSION) \
           -X github.com/arturoburigo/gitlab-tui/internal/version.Commit=$(COMMIT) \
           -X github.com/arturoburigo/gitlab-tui/internal/version.Date=$(DATE)

.PHONY: build
build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/gitlab-tui

.PHONY: install
install:
	go install -ldflags "$(LDFLAGS)" ./cmd/gitlab-tui

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
	go run ./cmd/gitlab-tui
