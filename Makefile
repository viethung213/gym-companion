.PHONY: proto-gen proto-lint build-prod build-test test-env-up test-env-down lint test-all test-unit test-integration test-module test-unit-module test-integration-module clean install-hooks proto-docker-build proto-update db-init-postgres db-init-go db-init-all

# Lệnh chạy docker compose của Buf CLI
BUF_COMPOSE = docker compose -f infra/buf/docker-compose.yml

# =====================================================================
# 1. API Contract & Protobuf Commands (Buf CLI)
# =====================================================================

# Bước 1: Build Docker image chứa Buf CLI và các plugins Go sinh stubs
proto-docker-build:
	@echo "Building Buf custom Docker image..."
	$(BUF_COMPOSE) build

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


# Khởi chạy toàn bộ môi trường kiểm thử (App + DBs + Kafka) cục bộ dưới Docker
test-env-up:
	docker network create fitai-network || true
	cp -n .env.example .env || true
	docker compose -f infra/db/postgres/docker-compose.yml up -d
	docker compose -f infra/kafka/docker-compose.yml up -d
	docker compose -f docker-compose.test.yml up -d --build

# Dừng toàn bộ môi trường kiểm thử và xóa toàn bộ dữ liệu rác (volumes) để tránh đầy bộ nhớ
test-env-down:
	@echo "Stopping application container..."
	docker compose -f docker-compose.test.yml down -v || true
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

# Chạy toàn bộ các bài test (cả unit và integration) của toàn dự án
test-all:
	@echo "Running all tests..."
	go test -v ./...

# Chạy tất cả các Unit Tests của toàn dự án (sử dụng tag build: unit)
test-unit:
	@echo "Running all Unit Tests (Domain & Application)..."
	go test -v -tags=unit ./...

# Chạy tất cả các Integration Tests của toàn dự án (Chạy trong Docker để kết nối hạ tầng)
test-integration: build-test
	docker network create fitai-network || true
	cp -n .env.example .env || true
	@echo "Running Integration Tests inside Docker..."
	docker run --rm --network fitai-network --env-file .env fitai-app:test go test -v -race -tags=integration ./...

# --- Lệnh Chạy Theo Từng Module (Yêu cầu truyền biến MODULE=...) ---

# Chạy toàn bộ test của một module cụ thể (Ví dụ: make test-module MODULE=workout)
test-module:
ifndef MODULE
	$(error Lỗi: Vui lòng khai báo MODULE. Ví dụ: make test-module MODULE=workout)
endif
	@echo "Running tests for module $(MODULE)..."
	go test -v ./internal/$(MODULE)/...

# Chạy chỉ Unit Tests của một module cụ thể (Ví dụ: make test-unit-module MODULE=workout)
test-unit-module:
ifndef MODULE
	$(error Lỗi: Vui lòng khai báo MODULE. Ví dụ: make test-unit-module MODULE=workout)
endif
	@echo "Running Unit Tests for module $(MODULE)..."
	go test -v -tags=unit ./internal/$(MODULE)/...

# Chạy chỉ Integration Tests của một module cụ thể (Chạy trong Docker để kết nối hạ tầng)
test-integration-module: build-test
ifndef MODULE
	$(error Lỗi: Vui lòng khai báo MODULE. Ví dụ: make test-integration-module MODULE=workout)
endif
	docker network create fitai-network || true
	cp -n .env.example .env || true
	@echo "Running Integration Tests for module $(MODULE) inside Docker..."
	docker run --rm --network fitai-network --env-file .env fitai-app:test go test -v -race -tags=integration ./internal/$(MODULE)/...

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
