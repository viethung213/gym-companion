.PHONY: proto-gen proto-lint build-prod build-test test-env-up test-env-down lint test-all test-unit test-integration test-module test-unit-module test-integration-module clean install-hooks proto-docker-build proto-update db-init-postgres db-init-go db-init-all db-migrate-exercise

# Lệnh chạy docker compose của Buf CLI
BUF_COMPOSE = docker compose -f infra/buf/docker-compose.yml
POSTGRES_CONTAINER ?= fitai-postgres-test
POSTGRES_DATABASE ?= fitai_test
POSTGRES_USER ?= postgres

# =====================================================================
# 1. API Contract & Protobuf Commands (Buf CLI)
# =====================================================================

# Bước 1: Build Docker image chứa Buf CLI và các plugins Go sinh stubs
proto-docker-build:
	@echo "Building Buf custom Docker image..."
	docker build -t ghcr.io/viethung213/gym-companion/buf-generator:latest -f infra/buf/buf.Dockerfile .

# Bước 2: Sinh mã nguồn Go (stubs) và tài liệu Swagger OpenAPI từ các hợp đồng Proto
proto-gen: clean
	@echo "Generating API contracts via Buf using Docker..."
	$(BUF_COMPOSE) run --rm buf generate proto --template proto/buf.gen.yaml

# Cập nhật các phụ thuộc bên ngoài của Protobuf (như googleapis, grpc-gateway) từ buf.lock
proto-update:
	@echo "Updating Buf schema dependencies using Docker..."
	$(BUF_COMPOSE) run --rm buf dep update proto

# Lint các tệp Protobuf để đảm bảo tuân thủ chuẩn style guide của dự án
proto-lint:
	@echo "Linting Protobuf files using Docker..."
	$(BUF_COMPOSE) run --rm buf lint proto


# =====================================================================
# 2. Docker Build & Test Environments
# =====================================================================

# Build production target image (tối ưu dung lượng, chỉ chứa binary chạy thực tế)
build-prod:
	@echo "Building Production Docker Image..."
	docker build --target prod -t fitai-app:latest .

# Build test target image (chứa toàn bộ môi trường Go để chạy test)
build-test:
	@echo "Building Test/Tester Docker Image..."
	docker build --target tester -t fitai-app:test .


# Khởi chạy toàn bộ môi trường hạ tầng kiểm thử (Postgres + Kafka) dưới Docker
test-env-up:
	@-docker network create fitai-network 2>/dev/null
	@-cp -n .env.example .env 2>/dev/null
	docker compose -f infra/db/postgres/docker-compose.yml up -d --wait
	docker compose -f infra/kafka/docker-compose.yml up -d --wait

# Dừng toàn bộ môi trường hạ tầng kiểm thử và xóa toàn bộ dữ liệu rác (volumes) để tránh đầy bộ nhớ
test-env-down:
	@echo "Stopping postgres container..."
	docker compose -f infra/db/postgres/docker-compose.yml down -v || true
	@echo "Stopping kafka container..."
	docker compose -f infra/kafka/docker-compose.yml down -v || true
	@echo "Removing shared network..."
	docker network rm fitai-network || true

# =====================================================================
# 3. Code Quality & Testing (Local commands wrapping Docker)
# =====================================================================

# Chạy golangci-lint tĩnh cục bộ để kiểm tra chất lượng mã nguồn (sử dụng go run để biên dịch tự động bằng phiên bản Go của máy host, hỗ trợ Go 1.25+ và đồng bộ với CI)
lint:
	@echo "Linting Go source files..."
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.5 run -v

# --- Lệnh Chạy Toàn Bộ Dự Án ---

# Chạy toàn bộ các bài test (Unit, Integration, E2E) của toàn dự án bên trong Docker
test-all: test-env-up build-test
	@-docker network create fitai-network 2>/dev/null
	@-cp -n .env.example .env 2>/dev/null
	@echo "Running all tests (Unit, Integration, E2E) inside Docker..."
	@docker run --rm --network fitai-network --env-file .env fitai-app:test sh -c ' \
		OUT=$$(go test -v -race -p=1 -tags="unit,integration,e2e" ./...); \
		echo "$$OUT"; \
		PASSED=$$(echo "$$OUT" | grep -c -e "--- PASS:"); \
		FAILED=$$(echo "$$OUT" | grep -c -e "--- FAIL:"); \
		echo "=================================================="; \
		echo "📊 TỔNG HỢP KẾT QUẢ KIỂM THỬ (TEST SUMMARY):"; \
		echo "🟢 ĐẠT (PASSED): $$PASSED"; \
		echo "🔴 THẤT BẠI (FAILED): $$FAILED"; \
		echo "=================================================="; \
		if [ $$FAILED -gt 0 ]; then exit 1; fi'

# Chạy tất cả các Unit Tests của toàn dự án bên trong Docker
test-unit: build-test
	@-docker network create fitai-network 2>/dev/null
	@-cp -n .env.example .env 2>/dev/null
	@echo "Running all Unit Tests inside Docker..."
	docker run --rm --network fitai-network --env-file .env fitai-app:test go test -v -race -tags=unit ./...

# Chạy tất cả các Integration Tests của toàn dự án bên trong Docker
test-integration: test-env-up build-test
	@-docker network create fitai-network 2>/dev/null
	@-cp -n .env.example .env 2>/dev/null
	@echo "Running all Integration Tests inside Docker..."
	docker run --rm --network fitai-network --env-file .env fitai-app:test go test -v -race -tags=integration ./...

# --- Lệnh Chạy Theo Từng Module (Yêu cầu truyền biến MODULE=...) ---

# Chạy toàn bộ test (Unit, Integration, E2E) của một module cụ thể bên trong Docker
test-module: test-env-up build-test
ifndef MODULE
	$(error Lỗi: Vui lòng khai báo MODULE. Ví dụ: make test-module MODULE=auth)
endif
	@-docker network create fitai-network 2>/dev/null
	@-cp -n .env.example .env 2>/dev/null
	@echo "Running all tests (Unit, Integration, E2E) for module $(MODULE) sequentially inside Docker..."
	@docker run --rm --network fitai-network --env-file .env fitai-app:test sh -c ' \
		OUT=$$(go test -v -race -p=1 -tags="unit,integration,e2e" ./internal/$(MODULE)/...); \
		echo "$$OUT"; \
		PASSED=$$(echo "$$OUT" | grep -c -e "--- PASS:"); \
		FAILED=$$(echo "$$OUT" | grep -c -e "--- FAIL:"); \
		echo "=================================================="; \
		echo "📊 TỔNG HỢP KẾT QUẢ KIỂM THỬ MODULE $(MODULE):"; \
		echo "🟢 ĐẠT (PASSED): $$PASSED"; \
		echo "🔴 THẤT BẠI (FAILED): $$FAILED"; \
		echo "=================================================="; \
		if [ $$FAILED -gt 0 ]; then exit 1; fi'

# Chạy chỉ Unit Tests của một module cụ thể bên trong Docker
test-unit-module: build-test
ifndef MODULE
	$(error Lỗi: Vui lòng khai báo MODULE. Ví dụ: make test-unit-module MODULE=auth)
endif
	@-docker network create fitai-network 2>/dev/null
	@-cp -n .env.example .env 2>/dev/null
	@echo "Running Unit Tests for module $(MODULE) inside Docker..."
	docker run --rm --network fitai-network --env-file .env fitai-app:test go test -v -race -tags=unit ./internal/$(MODULE)/...

# Chạy chỉ Integration Tests của một module cụ thể bên trong Docker
test-integration-module: test-env-up build-test
ifndef MODULE
	$(error Lỗi: Vui lòng khai báo MODULE. Ví dụ: make test-integration-module MODULE=auth)
endif
	@-docker network create fitai-network 2>/dev/null
	@-cp -n .env.example .env 2>/dev/null
	@echo "Running Integration Tests for module $(MODULE) inside Docker..."
	docker run --rm --network fitai-network --env-file .env fitai-app:test go test -v -race -tags=integration ./internal/$(MODULE)/...

# Chạy chỉ E2E Tests của một module cụ thể bên trong Docker
test-e2e-module: build-test
ifndef MODULE
	$(error Lỗi: Vui lòng khai báo MODULE. Ví dụ: make test-e2e-module MODULE=auth)
endif
	@-docker network create fitai-network 2>/dev/null
	@-cp -n .env.example .env 2>/dev/null
	@echo "Running E2E Tests for module $(MODULE) inside Docker..."
	docker run --rm --network fitai-network --env-file .env fitai-app:test go test -v -race -tags=e2e ./internal/$(MODULE)/...



# =====================================================================
# 4. Data Initialization & Seeding (Khởi tạo dữ liệu)
# =====================================================================

# Tìm và chạy tất cả các file SQL khởi tạo dữ liệu (*init*.sql) ở các module trong internal/ vào Postgres
db-init-postgres:
	@echo "Running PostgreSQL init scripts found in internal/..."
	@for file in $$(find internal -name "*init*.sql" 2>/dev/null); do \
		echo "Executing $$file in Postgres..."; \
		docker exec -i fitai-postgres-test psql -U postgres -d fitai < $$file; \
	done

# Tìm và chạy tất cả các file Go khởi tạo dữ liệu (*init*.go) ở các module trong internal/
db-init-go:
	@echo "Running Go init scripts found in internal/..."
	@for file in $$(find internal -name "*init*.go" 2>/dev/null); do \
		echo "Running Go script $$file..."; \
		go run $$file; \
	done

# Chạy tất cả các loại script khởi tạo dữ liệu trong dự án
db-init-all: db-init-postgres db-init-go
	@echo "All database initialization scripts completed."

db-migrate-exercise:
	@echo "Applying Exercise database migrations..."
	docker exec -i $(POSTGRES_CONTAINER) psql -v ON_ERROR_STOP=1 -U $(POSTGRES_USER) -d $(POSTGRES_DATABASE) < scripts/postgres-migrations/exercise/001_add_exercise_archive_status.sql

# =====================================================================
# 5. Utilities
# =====================================================================

# Xóa các thư mục mã nguồn tự sinh (Go stubs & Swagger API Docs) để dọn dẹp môi trường trước khi sinh code mới.
# Chạy việc xóa này qua Docker để tránh lỗi phân quyền (permission errors) khi mount volume (do container chạy dưới quyền root).
clean:
	@echo "Cleaning up generated stubs..."
	-docker compose -f infra/buf/docker-compose.yml run --rm --entrypoint rm buf -rf internal/gen/go docs/swagger
	@echo "Cleanup completed."

# Đăng ký Git hooks tự động kiểm soát format commit message chuẩn Conventional Commits
install-hooks:
	@echo "Installing Git hooks..."
	@chmod +x scripts/install-hooks.sh
	@./scripts/install-hooks.sh
	@echo "Git hooks installed successfully!"
