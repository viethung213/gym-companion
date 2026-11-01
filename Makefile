.PHONY: proto-gen proto-lint build-prod build-core build-ai build-gateway build-test run-prod run-prod-d stop-prod run-gateway stop-gateway run-core stop-core run-ai stop-ai logs-prod test-env-up test-env-down lint test-all test-unit test-integration test-module test-unit-module test-integration-module clean install-hooks proto-docker-build proto-update db-init-postgres db-init-go db-init-all

# Lệnh chạy docker compose của Buf CLI
BUF_COMPOSE = docker compose -f infra/buf/docker-compose.yml

# =====================================================================
# 1. API Contract & Protobuf Commands (Buf CLI)
# =====================================================================

# Lấy Docker Image Buf Generator thống nhất từ GitHub Container Registry (GHCR)
proto-docker-pull:
	@echo "Pulling unified Buf custom Docker image from GHCR..."
	docker pull ghcr.io/viethung213/gym-companion/buf-generator:latest || true

# Bước 1: Build Docker image chứa Buf CLI và các plugins (nếu muốn tự build local)
proto-docker-build:
	@echo "Building Buf custom Docker image..."
	docker build -t ghcr.io/viethung213/gym-companion/buf-generator:latest -f infra/buf/buf.Dockerfile .

# Bước 2: Sinh mã nguồn Go/TS (stubs) và tài liệu OpenAPI từ Proto (Chạy qua Docker 100% với image từ GHCR)
proto-gen: clean proto-docker-pull
	@echo "Generating API contracts via Buf using Docker (GHCR Image)..."
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
# 2. Docker Build & Run Environments
# =====================================================================

# Build API Gateway Docker image độc lập (<10MB)
build-gateway:
	@echo "Building API Gateway Docker Image..."
	docker build -f infra/gateway/Dockerfile -t fitai-gateway:latest .

# Build Core Fitness Service Docker image độc lập
build-core:
	@echo "Building Core Fitness Service Docker Image..."
	docker build --target prod-core -t fitai-core:latest .

# Build AI & Notification Service Docker image độc lập
build-ai:
	@echo "Building AI & Notification Service Docker Image..."
	docker build --target prod-ai -t fitai-ai:latest .

# Build toàn bộ production target images (cả Backend Services và API Gateway)
build-prod: build-gateway build-core build-ai

# Khởi chạy riêng API Gateway Container trên port 8000 & 8001
run-gateway:
	@-docker network create fitai-network 2>/dev/null
	@echo "Starting API Gateway Container on ports 8000 & 8001..."
	docker compose -f infra/gateway/docker-compose.yml up -d --build

# Dừng riêng API Gateway Container
stop-gateway:
	@echo "Stopping API Gateway Container..."
	docker compose -f infra/gateway/docker-compose.yml down

# Khởi chạy riêng Core Fitness Service Container trên port 8080 & 9090
run-core:
	@-docker network create fitai-network 2>/dev/null
	@-cp -n .env.core.example .env.core 2>/dev/null
	@echo "Starting Core Fitness Service Container in background..."
	docker run -d --name fitai-core --network fitai-network --network-alias core --network-alias app --env-file .env.core -p 8080:8080 -p 9090:9090 fitai-core:latest

# Dừng riêng Core Fitness Service Container
stop-core:
	@echo "Stopping Core Fitness Service Container..."
	docker stop fitai-core || true
	docker rm fitai-core || true

# Khởi chạy riêng AI & Notification Service Container trên port 8082 & 9092
run-ai:
	@-docker network create fitai-network 2>/dev/null
	@-cp -n .env.ai.example .env.ai 2>/dev/null
	@echo "Starting AI & Notification Service Container in background..."
	docker run -d --name fitai-ai --network fitai-network --network-alias ai -e GATEWAY_URL=http://fitai-gateway:8000 --env-file .env.ai -p 8082:8082 -p 9092:9092 fitai-ai:latest

# Dừng riêng AI & Notification Service Container
stop-ai:
	@echo "Stopping AI & Notification Service Container..."
	docker stop fitai-ai || true
	docker rm fitai-ai || true

# Khởi chạy toàn bộ hệ thống Production (cả 3: Gateway, Core Service và AI Service) chạy ngầm dưới nền
run-prod: run-gateway run-core run-ai
	@echo "======================================================================"
	@echo "🟢 Hệ thống đã khởi chạy thành công cả 3 dịch vụ dưới nền:"
	@echo "   - API Gateway Data Plane:    http://localhost:8000"
	@echo "   - API Gateway Control Plane: http://localhost:8001"
	@echo "   - Core Fitness Service:      http://localhost:8080"
	@echo "   - AI & Notification Service: http://localhost:8082"
	@echo "======================================================================"
	@echo "👉 Xem logs Gateway: docker logs -f fitai-gateway"
	@echo "👉 Xem logs Core:    docker logs -f fitai-core"
	@echo "👉 Xem logs AI:      docker logs -f fitai-ai"
	@echo "👉 Dừng toàn bộ:     make stop-prod"

# Dừng toàn bộ hệ thống Production (cả 3 containers: Gateway, Core và AI)
stop-prod: stop-core stop-ai stop-gateway
	@docker stop fitai-backend 2>/dev/null || true
	@docker rm fitai-backend 2>/dev/null || true
	@echo "======================================================================"
	@echo "🛑 Đã dừng toàn bộ 3 dịch vụ thành công."
	@echo "======================================================================"

# Build test target image (chứa toàn bộ môi trường Go để chạy test)
build-test:
	@echo "Building Test/Tester Docker Image..."
	docker build --target tester -t fitai-app:test .


# Khởi chạy toàn bộ môi trường hạ tầng kiểm thử (Postgres + Kafka + Redis) dưới Docker
test-env-up:
	@-docker network create fitai-network 2>/dev/null
	@-cat .env.core .env.ai > .env 2>/dev/null || cat .env.core.example .env.ai.example > .env 2>/dev/null
	docker compose -f infra/db/postgres/docker-compose.yml up -d
	docker compose -f infra/kafka/docker-compose.yml up -d
	docker compose -f infra/redis/docker-compose.yml up -d

# Dừng toàn bộ môi trường hạ tầng kiểm thử và xóa toàn bộ dữ liệu rác (volumes) để tránh đầy bộ nhớ
test-env-down:
	@echo "Stopping postgres container..."
	docker compose -f infra/db/postgres/docker-compose.yml down -v || true
	@echo "Stopping kafka container..."
	docker compose -f infra/kafka/docker-compose.yml down -v || true
	@echo "Stopping redis container..."
	docker compose -f infra/redis/docker-compose.yml down -v || true
	@echo "Removing shared network..."
	docker network rm fitai-network || true

# =====================================================================
# 3. Code Quality & Testing (Local commands wrapping Docker)
# =====================================================================

# Chạy golangci-lint tĩnh cục bộ để kiểm tra chất lượng mã nguồn (sử dụng go run để biên dịch tự động bằng phiên bản Go của máy host, hỗ trợ Go 1.25+ và đồng bộ với CI)
lint:
	@echo "Linting Go source files (including unit, integration, and e2e tagged files)..."
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.5 run --build-tags "unit,integration,e2e" -v

# --- Lệnh Chạy Toàn Bộ Dự Án ---

# Chạy toàn bộ các bài test (Unit, Integration, E2E) của toàn dự án bên trong Docker
test-all: build-test
	@-docker network create fitai-network 2>/dev/null
	@-cat .env.core .env.ai > .env 2>/dev/null || cat .env.core.example .env.ai.example > .env 2>/dev/null
	@echo "Running all tests (Unit, Integration, E2E) inside Docker..."
	@docker run --rm --network fitai-network --env-file .env fitai-app:test sh -c ' \
		OUT=$$(go test -v -race -p=1 -tags="unit,integration,e2e" ./... 2>&1); \
		STATUS=$$?; \
		echo "$$OUT"; \
		PASSED=$$(echo "$$OUT" | grep -c -e "--- PASS:"); \
		FAILED=$$(echo "$$OUT" | grep -c -e "--- FAIL:"); \
		echo "=================================================="; \
		echo "📊 TỔNG HỢP KẾT QUẢ KIỂM THỬ (TEST SUMMARY):"; \
		echo "🟢 ĐẠT (PASSED): $$PASSED"; \
		echo "🔴 THẤT BẠI (FAILED): $$FAILED"; \
		echo "=================================================="; \
		if [ $$STATUS -ne 0 ] || [ $$FAILED -gt 0 ]; then exit 1; fi'

# Chạy tất cả các Unit Tests của toàn dự án bên trong Docker
test-unit: build-test
	@-docker network create fitai-network 2>/dev/null
	@-cat .env.core .env.ai > .env 2>/dev/null || cat .env.core.example .env.ai.example > .env 2>/dev/null
	@echo "Running all Unit Tests inside Docker..."
	docker run --rm --network fitai-network --env-file .env fitai-app:test go test -v -race -tags=unit ./...

# Chạy tất cả các Integration Tests của toàn dự án bên trong Docker
test-integration: build-test
	@-docker network create fitai-network 2>/dev/null
	@-cat .env.core .env.ai > .env 2>/dev/null || cat .env.core.example .env.ai.example > .env 2>/dev/null
	@echo "Running all Integration Tests inside Docker..."
	docker run --rm --network fitai-network --env-file .env fitai-app:test go test -v -race -tags=integration ./...

# --- Lệnh Chạy Theo Từng Module (Yêu cầu truyền biến MODULE=...) ---

# Chạy toàn bộ test (Unit, Integration, E2E) của một module cụ thể bên trong Docker
test-module: build-test
ifndef MODULE
	$(error Lỗi: Vui lòng khai báo MODULE. Ví dụ: make test-module MODULE=auth)
endif
	@-docker network create fitai-network 2>/dev/null
	@-cat .env.core .env.ai > .env 2>/dev/null || cat .env.core.example .env.ai.example > .env 2>/dev/null
	@echo "Running all tests (Unit, Integration, E2E) for module $(MODULE) sequentially inside Docker..."
	@docker run --rm --network fitai-network --env-file .env fitai-app:test sh -c ' \
		if [ ! -d "./internal/$(MODULE)" ] || ! find ./internal/$(MODULE) -name "*.go" 2>/dev/null | grep -q .; then \
			echo "⚠️  Module \"$(MODULE)\" chưa triển khai Go code hoặc không có test. Bỏ qua kiểm thử."; \
			exit 0; \
		fi; \
		OUT=$$(go test -v -race -p=1 -tags="unit,integration,e2e" ./internal/$(MODULE)/... 2>&1); \
		STATUS=$$?; \
		echo "$$OUT"; \
		PASSED=$$(echo "$$OUT" | grep -c -e "--- PASS:"); \
		FAILED=$$(echo "$$OUT" | grep -c -e "--- FAIL:"); \
		echo "=================================================="; \
		echo "📊 TỔNG HỢP KẾT QUẢ KIỂM THỬ MODULE $(MODULE):"; \
		echo "🟢 ĐẠT (PASSED): $$PASSED"; \
		echo "🔴 THẤT BẠI (FAILED): $$FAILED"; \
		echo "=================================================="; \
		if [ $$STATUS -ne 0 ] || [ $$FAILED -gt 0 ]; then exit 1; fi'

# Chạy chỉ Unit Tests của một module cụ thể bên trong Docker
test-unit-module: build-test
ifndef MODULE
	$(error Lỗi: Vui lòng khai báo MODULE. Ví dụ: make test-unit-module MODULE=auth)
endif
	@-docker network create fitai-network 2>/dev/null
	@-cat .env.core .env.ai > .env 2>/dev/null || cat .env.core.example .env.ai.example > .env 2>/dev/null
	@echo "Running Unit Tests for module $(MODULE) inside Docker..."
	docker run --rm --network fitai-network --env-file .env fitai-app:test go test -v -race -tags=unit ./internal/$(MODULE)/...

# Chạy chỉ Integration Tests của một module cụ thể bên trong Docker
test-integration-module: build-test
ifndef MODULE
	$(error Lỗi: Vui lòng khai báo MODULE. Ví dụ: make test-integration-module MODULE=auth)
endif
	@-docker network create fitai-network 2>/dev/null
	@-cat .env.core .env.ai > .env 2>/dev/null || cat .env.core.example .env.ai.example > .env 2>/dev/null
	@echo "Running Integration Tests for module $(MODULE) inside Docker..."
	docker run --rm --network fitai-network --env-file .env fitai-app:test go test -v -race -tags=integration ./internal/$(MODULE)/...

# Chạy chỉ E2E Tests của một module cụ thể bên trong Docker
test-e2e-module: build-test
ifndef MODULE
	$(error Lỗi: Vui lòng khai báo MODULE. Ví dụ: make test-e2e-module MODULE=auth)
endif
	@-docker network create fitai-network 2>/dev/null
	@-cat .env.core .env.ai > .env 2>/dev/null || cat .env.core.example .env.ai.example > .env 2>/dev/null
	@echo "Running E2E Tests for module $(MODULE) inside Docker..."
	docker run --rm --network fitai-network --env-file .env fitai-app:test go test -v -race -tags=e2e ./internal/$(MODULE)/...



# =====================================================================
# 4. Data Initialization & Seeding (Khởi tạo dữ liệu)
# =====================================================================

# Tìm và chạy tất cả các file SQL khởi tạo dữ liệu ở internal/shared/database/migrations/ vào Postgres
db-init-postgres:
	@echo "Running PostgreSQL init scripts found in internal/shared/database/migrations/..."
	@for file in $$(find internal/shared/database/migrations -name "*.sql" 2>/dev/null | sort); do \
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
	-docker compose -f infra/buf/docker-compose.yml run --rm --entrypoint rm buf -rf internal/gen/go docs/swagger gen/es/contracts
	@echo "Cleanup completed."

# Đăng ký Git hooks tự động kiểm soát format commit message chuẩn Conventional Commits
install-hooks:
	@echo "Installing Git hooks..."
	@chmod +x scripts/install-hooks.sh
	@./scripts/install-hooks.sh
	@echo "Git hooks installed successfully!"
