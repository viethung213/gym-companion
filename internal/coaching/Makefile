.PHONY: help build run-webui run-cli test test-unit test-coverage

# Default configurations
WEB_UI_PORT ?= 8000

help:
	@echo "========================================================================"
	@echo "  🏋️ GYM COMPANION - COACHING BOUNDED CONTEXT MAKEFILE"
	@echo "========================================================================"
	@echo "  make run-webui     : Launch Google ADK Web UI Playground (http://localhost:8000/ui/)"
	@echo "  make run-cli       : Run CLI coaching test generator"
	@echo "  make build         : Compile Coaching module & CLI binaries"
	@echo "  make test          : Execute all unit & integration tests in Coaching context"
	@echo "  make test-coverage : Run tests with HTML coverage report generation"
	@echo "========================================================================"

run-webui:
	@echo "🚀 Starting Google ADK Web UI on http://localhost:$(WEB_UI_PORT)/ui/ ..."
	cd ../.. && go run ./cmd/test-coaching/main.go web -port $(WEB_UI_PORT) api -webui_address http://localhost:$(WEB_UI_PORT) webui -api_server_address http://localhost:$(WEB_UI_PORT)/api

run-cli:
	@echo "🧪 Running Coaching CLI roadmap generator..."
	cd ../.. && go run ./cmd/test-coaching/main.go -user test-user-123

build:
	@echo "🔨 Compiling Coaching module..."
	cd ../.. && go build ./internal/coaching/... ./cmd/test-coaching/...

test:
	@echo "🧪 Running Coaching unit & integration tests..."
	cd ../.. && go test -v ./internal/coaching/...

test-coverage:
	@echo "📊 Running Coaching coverage report..."
	cd ../.. && go test -v -coverprofile=coaching_coverage.out ./internal/coaching/... && go tool cover -html=coaching_coverage.out -o coaching_coverage.html
	@echo "✅ Coverage report generated at coaching_coverage.html"
