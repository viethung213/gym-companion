# =====================================================================
# Base Stage (Môi trường build chung)
# =====================================================================
FROM golang:1.25-alpine AS builder

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
# Build Binary cho Production
# =====================================================================
FROM builder AS prod-builder

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/fitai-api ./cmd/api/main.go

# =====================================================================
# Trạng thái 2: Production Stage (Image chạy thực tế siêu nhẹ)
# =====================================================================
FROM alpine:3.20 AS prod

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binary từ prod-builder
COPY --from=prod-builder /bin/fitai-api /app/fitai-api

EXPOSE 8080
EXPOSE 9090

CMD ["/app/fitai-api"]
