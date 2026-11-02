.PHONY: help build test test-unit test-coverage

help:
	@echo "========================================================================"
	@echo "  🏋️ GYM COMPANION - COACHING BOUNDED CONTEXT MAKEFILE"
	@echo "========================================================================"
	@echo "  make build         : Compile Coaching module"
	@echo "  make test          : Execute all unit & integration tests in Coaching context"
	@echo "  make test-coverage : Run tests with HTML coverage report generation"
	@echo "========================================================================"

build:
	@echo "🔨 Compiling Coaching module..."
	cd ../.. && go build ./internal/coaching/...

test:
	@echo "🧪 Running Coaching unit & integration tests..."
	cd ../.. && go test -v ./internal/coaching/...

test-coverage:
	@echo "📊 Running Coaching coverage report..."
	cd ../.. && go test -v -coverprofile=coaching_coverage.out ./internal/coaching/... && go tool cover -html=coaching_coverage.out -o coaching_coverage.html
	@echo "✅ Coverage report generated at coaching_coverage.html"
