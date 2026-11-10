# =====================================================================
# Base Stage (Môi trường build chung)
# =====================================================================
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git make build-base

WORKDIR /app

# Copy go.mod & go.sum trước để tận dụng Docker layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy mã nguồn
COPY . .

# =====================================================================
# Trạng thái 1: Tester Stage (Chạy test bên trong container)
# =====================================================================
FROM builder AS tester

# Mặc định khi chạy target này sẽ thực thi go test
CMD ["go", "test", "-v", "-race", "./..."]

# =====================================================================
# Build Binary cho Core Service
# =====================================================================
FROM builder AS core-builder

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/fitai-core ./cmd/core/main.go

# =====================================================================
# Target: Core Production Stage
# =====================================================================
FROM alpine:3.20 AS prod-core

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=core-builder /bin/fitai-core /app/fitai-core

EXPOSE 8080
EXPOSE 8443/udp
EXPOSE 8443/tcp

CMD ["/app/fitai-core"]

# =====================================================================
# Build Binary cho AI & Notification Service
# =====================================================================
FROM builder AS ai-builder

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/fitai-ai ./cmd/ai/main.go

# =====================================================================
# Target: AI Production Stage
# =====================================================================
FROM alpine:3.20 AS prod-ai

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=ai-builder /bin/fitai-ai /app/fitai-ai

# Copy ADK prompts và skills (cần thiết cho coaching và nutrition agent)
COPY internal/coaching/infrastructure/ai/adk/prompts ./internal/coaching/infrastructure/ai/adk/prompts
COPY internal/coaching/infrastructure/ai/adk/skills ./internal/coaching/infrastructure/ai/adk/skills
COPY internal/nutrition/infrastructure/ai/adk/prompts ./internal/nutrition/infrastructure/ai/adk/prompts

EXPOSE 8082

CMD ["/app/fitai-ai"]


