.PHONY: install-deps buf-update buf-gen buf-lint clean

# Step 1: Install required protoc/buf plugins into GOPATH/bin
install-deps:
	@echo "Installing Go protobuf, gRPC-gateway and Buf plugins..."
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest
	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@latest
	go install github.com/bufbuild/buf/cmd/buf@latest
	@echo "Dependencies installed successfully. Ensure your GOPATH/bin is in your system PATH."

# Step 2: Update Buf dependencies (googleapis, grpc-gateway)
buf-update:
	@echo "Updating Buf schema dependencies..."
	cd proto && buf dep update

# Step 3: Generate Go stubs and Swagger OpenAPI specifications
buf-gen: clean
	@echo "Generating API contracts via Buf..."
	cd proto && buf generate

# Verify: Lint proto schemas to ensure they match style guides
buf-lint:
	@echo "Linting Protobuf files..."
	@cd proto && buf lint

# Install Git hooks locally to prevent invalid commits
install-hooks:
	@echo "Installing Git commit-msg hook..."
	@cp scripts/verify-commit-msg.sh .git/hooks/commit-msg
	@chmod +x .git/hooks/commit-msg
	@echo "Git commit-msg hook installed successfully!"

# Clean: Remove previously generated stubs
clean:
	@echo "Cleaning up generated stubs..."
	@rm -rf internal/gen/go
	@rm -rf docs/swagger
	@echo "Cleanup completed."
