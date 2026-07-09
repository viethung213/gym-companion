# Sử dụng Go làm builder để tải và biên dịch các plugin Protobuf
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

# Cài đặt các plugin với phiên bản mới nhất thông dụng nhất
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11 && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.2 && \
    go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@v2.29.0 && \
    go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@v2.29.0

# Sử dụng image chính thức của Buf làm runtime base (bản mới ổn định)
FROM bufbuild/buf:1.34.0

# Copy các binary plugin đã được biên dịch sang
COPY --from=builder /go/bin/protoc-gen-go /usr/local/bin/
COPY --from=builder /go/bin/protoc-gen-go-grpc /usr/local/bin/
COPY --from=builder /go/bin/protoc-gen-grpc-gateway /usr/local/bin/
COPY --from=builder /go/bin/protoc-gen-openapiv2 /usr/local/bin/

WORKDIR /workspace
