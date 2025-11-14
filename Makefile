BUILD_DIR=./build
BINARY_NAME=ims-worker
BINARY ?= cmd/boothandler/$(BINARY_NAME)

# Default value for BUILDER
BUILDER := $(shell if command -v podman > /dev/null 2>&1; then echo podman; \
                elif command -v docker > /dev/null 2>&1; then echo docker; \
                else echo none; fi)
# Target to check and use BUILDER
check-builder:
ifeq ($(BUILDER),none)
	@echo "Error: Neither podman nor docker is installed." >&2
	@exit 1
else
	@echo "Using $(BUILDER) as container runtime."
endif

build: 
	mkdir -p $(BUILD_DIR)
	@echo "Building GO Binary..."
	@go mod tidy
	@go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/boothandler/
	@echo "Binary build as $(BINARY)"

IMAGE_TAG ?= ims-worker:latest
# Example target using BUILDER
#image: $(BINARY) check-builder
image: check-builder
	@echo "Building container image with $(BUILDER)..."
	$(BUILDER) build -t $(IMAGE_TAG) .

# Check if binary is already built
$(BINARY):
	@$(MAKE) build

# Clean target
clean:
	@echo "Cleaning up..."
	rm -f $(BINARY) || true
	$(BUILDER) rmi $(IMAGE_TAG) || true

.PHONY: check-buider build image clean
