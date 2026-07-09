.PHONY: buf-docker-build buf-update buf-gen buf-lint clean install-hooks

DOCKER_IMAGE = fitai-buf-gen
# $(CURDIR) works portably on Windows, Linux, and macOS in Makefiles
DOCKER_RUN = docker run --rm -v "$(CURDIR):/workspace" -w /workspace $(DOCKER_IMAGE)

# Step 1: Build the custom Docker image containing Buf and the Go plugins
buf-docker-build:
	@echo "Building Buf custom Docker image..."
	docker build -t $(DOCKER_IMAGE) -f buf.Dockerfile .

# Step 2: Generate Go stubs and Swagger OpenAPI specifications
buf-gen: clean
	@echo "Generating API contracts via Buf using Docker..."
	$(DOCKER_RUN) generate proto --template proto/buf.gen.yaml


# Update Buf dependencies (googleapis, grpc-gateway) when buf.lock changes
buf-update:
	@echo "Updating Buf schema dependencies using Docker..."
	$(DOCKER_RUN) dep update proto

# Verify: Lint proto schemas to ensure they match style guides
buf-lint:
	@echo "Linting Protobuf files using Docker..."
	$(DOCKER_RUN) lint proto

# Install Git hooks locally to prevent invalid commits
install-hooks:
	@echo "Installing Git hooks..."
	@chmod +x scripts/install-hooks.sh
	@./scripts/install-hooks.sh
	@echo "Git hooks installed successfully!"

# Clean: Remove previously generated stubs
clean:
	@echo "Cleaning up generated stubs..."
	-docker run --rm --entrypoint rm -v "$(CURDIR):/workspace" -w /workspace $(DOCKER_IMAGE) -rf internal/gen/go docs/swagger
	@echo "Cleanup completed."
