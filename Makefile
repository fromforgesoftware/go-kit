.PHONY: help test test-race test-integration bench vet lint mock clean

GO_TEST_TIMEOUT ?= 180s
GO_PKG ?= ./...
BENCH_RUN ?= .
BENCH_TIME ?= 1s

help:
	@echo "Available targets:"
	@echo "  make test             - Run all unit tests"
	@echo "  make test-race        - Run all unit tests under -race (slower)"
	@echo "  make test-integration - Run integration tests (requires Docker)"
	@echo "  make bench            - Run Benchmark* tests across the kit (BENCH_RUN=Marshal narrows)"
	@echo "  make vet              - go vet ./..."
	@echo "  make lint             - golangci-lint run ./..."
	@echo "  make mock             - regenerate mockery mocks"

test:
	go test -count=1 -timeout $(GO_TEST_TIMEOUT) $(GO_PKG)

# test-race is the default CI gate — race detector catches the class of
# concurrency bugs the audit highlighted (TCP session ctx races, UDP
# session pending-map mutation, websocket reconnect channel races).
test-race:
	go test -count=1 -race -timeout $(GO_TEST_TIMEOUT) $(GO_PKG)

# Integration tests build behind `//go:build integration` and spin up
# gnomock-backed containers (RabbitMQ, Postgres, Redis). Slow; run
# locally or on a nightly job.
test-integration:
	go test -count=1 -tags=integration -timeout 600s $(GO_PKG)

# Benchmarks. Race detector intentionally off — race-instrumentation
# dominates timings and would mask the changes we want to measure.
# Use BENCH_RUN to narrow ("make bench BENCH_RUN=Marshal") and
# BENCH_TIME for longer runs ("make bench BENCH_TIME=5s") when
# investigating a regression.
bench:
	go test -run=^$$ -bench=$(BENCH_RUN) -benchmem -benchtime=$(BENCH_TIME) -count=1 $(GO_PKG)

vet:
	go vet $(GO_PKG)

lint:
	golangci-lint run $(GO_PKG)

mock:
	mockery --config .mockery.yaml
