BINARY    := snap
VERSION   := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS   := -X github.com/CodeOne45/snap/internal/adapters/cli.version=$(VERSION)
GOLANGCI  := golangci-lint

.PHONY: all build install test test-golden update-golden vet fmt fmt-check lint check clean help

all: build ## Build the snap binary (default target)

build: ## Build ./snap with the version stamped in
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/snap

install: ## Install snap to $GOBIN (or $GOPATH/bin) with the version stamped in
	go install -ldflags "$(LDFLAGS)" ./cmd/snap

test: ## Run the full test suite
	go test ./...

test-golden: ## Run only the M6 golden round-trip tests, verbosely
	go test ./internal/golden/... -run Golden -v

update-golden: ## Regenerate golden fixtures' testdata/golden/*/expected/ output
	go test ./internal/golden/... -run Golden -update

vet: ## Run go vet
	go vet ./...

fmt: ## Format all source files in place
	gofmt -w .

fmt-check: ## Fail if any file is not gofmt-formatted
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt needed on:"; echo "$$unformatted"; exit 1; \
	fi

lint: ## Run golangci-lint
	$(GOLANGCI) run

check: fmt-check vet test lint ## Run everything CI runs

clean: ## Remove build artifacts
	rm -f $(BINARY)

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*## "}; {printf "  %-15s %s\n", $$1, $$2}'
