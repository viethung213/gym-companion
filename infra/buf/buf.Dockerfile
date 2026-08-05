# Sử dụng Go làm builder để tải và biên dịch các plugin Protobuf Go
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git

# Cài đặt các plugin Go với phiên bản mới nhất thông dụng nhất
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11 && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.2 && \
    go install connectrpc.com/connect/cmd/protoc-gen-connect-go@v1.18.1 && \
    go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@v2.29.0

# Sử dụng image chính thức của Buf làm runtime base
FROM bufbuild/buf:1.34.0

# Cài Node.js và npm trong runtime của buf
RUN apk add --no-cache nodejs npm

# Cài đặt các plugin TypeScript toàn cục trực tiếp trong runtime image
RUN npm install -g @bufbuild/protobuf @bufbuild/protoplugin @bufbuild/protoc-gen-es@^1.10.0 @connectrpc/protoc-gen-connect-es@^1.6.0

# Copy các binary plugin Go đã được biên dịch sang
COPY --from=builder /go/bin/protoc-gen-go /usr/local/bin/
COPY --from=builder /go/bin/protoc-gen-go-grpc /usr/local/bin/
COPY --from=builder /go/bin/protoc-gen-connect-go /usr/local/bin/
COPY --from=builder /go/bin/protoc-gen-openapiv2 /usr/local/bin/

WORKDIR /workspace



