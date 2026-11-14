# =====================================================================
# Build Stage: Compile API Gateway Binary
# =====================================================================
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git make build-base

WORKDIR /app

# Cache Go modules layer
COPY go.mod go.sum ./
RUN go mod download

# Copy ONLY what API Gateway depends on (Zero backend coupling)
COPY cmd/gateway ./cmd/gateway
COPY internal/gateway ./internal/gateway
COPY internal/gen/go/contracts ./internal/gen/go/contracts
COPY internal/shared/internaltoken ./internal/shared/internaltoken
COPY internal/shared/telemetry ./internal/shared/telemetry

# Build Gateway binary with stripped debugging symbols
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/fitai-gateway ./cmd/gateway/main.go

# =====================================================================
# Tester Stage: Chạy Unit & E2E tests độc lập trên Docker
# =====================================================================
FROM builder AS tester

CMD ["go", "test", "-v", "./internal/gateway/..."]

# =====================================================================
# Production Stage: Lightweight Alpine Gateway Image (<15MB)
# =====================================================================
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /bin/fitai-gateway /app/fitai-gateway

# Copy gateway routing configurations
COPY configs ./configs

# Data Plane (8000) & Control Plane (8001)
EXPOSE 8000
EXPOSE 8001

CMD ["/app/fitai-gateway"]
