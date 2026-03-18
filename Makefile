.PHONY: build test vet lint bench smoke clean sync-public sync-public-pr

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-X github.com/thgrace/training-wheels/internal/cli.Version=$(VERSION)"

# Build the tw binary
build:
	go build $(LDFLAGS) -o tw ./cmd/tw/

# Run unit tests (safe to run locally)
test:
	go test ./...

# Run go vet
vet:
	go vet ./...

# Run golangci-lint
lint:
	golangci-lint run ./...

# Run benchmarks (use BENCH=<pattern> to filter, COUNT=N for iterations)
BENCH ?= .
COUNT ?= 1
bench:
	go test -bench=$(BENCH) -benchmem -count=$(COUNT) -run=^$$ ./...

# Build and run smoke tests in Docker container (NEVER run locally)
smoke:
	docker build --load -f Dockerfile.test -t tw-smoke-test .
	docker run --rm tw-smoke-test

# Run all checks (unit tests + vet + lint + smoke)
ci: test vet lint smoke

# Sync to public repo (squash-push to public/main)
sync-public:
	./scripts/sync-public.sh

# Sync to public repo via PR (for review before merging)
sync-public-pr:
	./scripts/sync-public.sh --pr

# Build the hook-lab Docker image (Linux)
lab-build:
	docker build -t tw-hook-lab labs/

# Run the hook lab (Linux — .env supplies OPEN_ROUTER_TOKEN for Claude)
lab-run:
	docker run --rm \
		--env-file .env \
		-e GITHUB_TOKEN \
		-e GEMINI_API_KEY \
		-v $(PWD)/labs/output:/lab/output \
		tw-hook-lab

# Build + run (Linux)
lab: lab-build lab-run

# Build the hook-lab Docker image (Windows — requires Windows container mode)
lab-build-win:
	docker build -f labs/Dockerfile.windows -t tw-hook-lab-win labs/

# Run the hook lab (Windows)
lab-run-win:
	docker run --rm \
		--env-file .env \
		-e GITHUB_TOKEN \
		-e GEMINI_API_KEY \
		-v "$(PWD)/labs/output:C:\lab\output" \
		tw-hook-lab-win

# Build + run (Windows)
lab-win: lab-build-win lab-run-win

# Run the hook lab locally (uses agents already installed on host)
# Usage: make lab-local [AGENT=claude|gemini|copilot]
AGENT ?= all
lab-local:
	./labs/run-local.sh $(AGENT)

# Clean build artifacts
clean:
	rm -f tw
