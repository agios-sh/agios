# agios - AGI OS CLI
BINARY    := agios
MODULE    := github.com/agios-sh/agios
VERSION   ?= dev
LDFLAGS   := -s -w -X main.version=$(VERSION)
PLATFORMS := darwin/arm64 darwin/amd64 linux/arm64 linux/amd64 windows/arm64 windows/amd64

.PHONY: all build run clean test fmt vet lint check install mock-app dist help

## —— Development ——————————————————————————————————————————

all: check build  ## Run checks then build

build:  ## Build the binary
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

run: build  ## Build and run (passes ARGS, e.g. make run ARGS="status")
	./$(BINARY) $(ARGS)

clean:  ## Remove build artifacts
	rm -f $(BINARY) $(BINARY).exe
	rm -rf dist/

install: build  ## Install to $GOPATH/bin
	go install -ldflags "$(LDFLAGS)" .

## —— Quality ——————————————————————————————————————————————

fmt:  ## Format all Go source files
	gofmt -w .

vet:  ## Run go vet
	go vet ./...

lint: fmt vet  ## Format + vet

check:  ## CI-equivalent checks (format, vet, test)
	@echo "==> Checking formatting..."
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "Files not formatted:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi
	@echo "==> Running vet..."
	go vet ./...
	@echo "==> Running tests..."
	go test ./...
	@echo "==> All checks passed."

## —— Testing —————————————————————————————————————————————

test:  ## Run all tests
	go test ./...

test-v:  ## Run all tests (verbose)
	go test -v ./...

test-race:  ## Run tests with race detector
	go test -race ./...

cover:  ## Run tests with coverage report
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	@rm -f coverage.out

cover-html:  ## Open coverage report in browser
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out
	@rm -f coverage.out

mock-app:  ## Build the mock app for integration testing
	go build -o testdata/mock-app/mock-app ./testdata/mock-app/

## —— Cross-compilation ———————————————————————————————————

dist: clean  ## Build release binaries for all platforms
	@mkdir -p dist
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; \
		arch=$${platform#*/}; \
		ext=""; \
		if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
		echo "==> Building $$os/$$arch..."; \
		GOOS=$$os GOARCH=$$arch go build -ldflags "$(LDFLAGS)" -o "dist/$(BINARY)_$${os}_$${arch}$${ext}" .; \
	done
	@echo "==> Done. Binaries in dist/"

## —— Help ————————————————————————————————————————————————

help:  ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | awk -F ':.*## ' '{printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'
