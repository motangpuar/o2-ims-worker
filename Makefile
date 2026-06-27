# Container Engine (defaults to podman, can be overridden with make ENGINE=docker)
ENGINE ?= podman

# Source Image (Official CentOS Stream 10 container)
IMAGE  ?= alpine:latest


# Local Inputs/Outputs
REPO_FILE ?= centos10-mirror.repo
DEST_DIR  ?= ./assets/mirros/
CENTOS_VERSION ?= stream10

# UID/GID Mapping (ensures files are owned by you, not root)
USER_ID := $(shell id -u)
GROUP_ID := $(shell id -g)

build:
	go build -o bin/worker cmd/worker/main.go 

build_client:
	go build -o bin/clientDHCP cmd/worker/clientDHCP.go 
	go build -o bin/clientTFTP cmd/worker/clientTFTP.go 

clean:
	rm -rf bin/*

buld_structure:
	mkdir -p assets/generic/pxelinux.cfg
	mkdir -p assets/generic/tftpboot
	mkdir -p assets/generic/boot/stream10
	wget https://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/images/pxeboot/initrd.img -O assets/generic/boot/stream10/
	wget https://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/images/pxeboot/vmlinuz -O assets/generic/boot/stream10/
	mkdir -p assets/generic/boot/ubuntu
	mkdir -p assets/http/

help:
	@echo "Usage:"
	@echo "  make sync      - Run the container to populate $(DEST_DIR)"
	@echo "  make clean     - Remove the local mirror directory"
	@echo "  make ENGINE=docker sync  - Use Docker instead of Podman"

# Extract the baseurl dynamically from your local repo file
SOURCE_URL := $(shell grep -E '^baseurl=' $(REPO_FILE) | head -n 1 | cut -d= -f2)

ifeq ($(ENGINE),docker)
    USER_FLAGS := -u $(USER_ID):$(GROUP_ID)
else
    # For rootless podman, we don't pass -u, but we ensure correct UID mapping
    # USER_FLAGS := --userns=keep-id
    USER_FLAGS :=
endif

sync: $(DEST_DIR)
	@if [ -z "$(SOURCE_URL)" ]; then echo "Error: Could not parse baseurl from $(REPO_FILE)"; exit 1; fi
	@echo "Found source URL: $(SOURCE_URL)"
	@echo "Cloning full boot installation tree using lftp via $(ENGINE)..."

	$(ENGINE) run --rm \
		--security-opt label=disable \
		$(USER_FLAGS) \
		-v $(PWD)/$(DEST_DIR):/mnt/mirror:z \
		$(IMAGE) \
		sh -c "apk add --no-cache lftp && \
		       lftp -c 'set http:use-propfind no; \
		                set net:timeout 10; \
		                set net:max-retries 3; \
		                set net:reconnect-interval-base 5; \
		                mirror --verbose=3 --no-perms --parallel=4 $(SOURCE_URL) /mnt/mirror; \
		                quit'"
	@echo "Sync complete! Clean bootable structure ready in $(DEST_DIR)"


# Ensure destination directory exists
$(DEST_DIR):
	mkdir -p $(DEST_DIR)

clean_repo:
	rm -rf $(DEST_DIR)

MIRROR_DIR := ./assets/http/mirror/debian12
DEBIAN_DIST := bookworm
DEBIAN_ARCH := amd64
DEBIAN_SECTIONS := main

mirror_debian:
	mkdir -p $(MIRROR_DIR)
	podman run --rm \
		--userns=keep-id \
		-v $(CURDIR)/$(MIRROR_DIR):/mirror:Z \
		--user root \
		debian:12 \
		bash -c " \
			apt-get update -qq && \
			apt-get install -y -qq debmirror > /dev/null && \
			chown -R $(shell id -u):$(shell id -g) /mirror && \
			debmirror \
				--host=deb.debian.org \
				--root=debian \
				--dist=$(DEBIAN_DIST) \
				--section=$(DEBIAN_SECTIONS) \
				--arch=$(DEBIAN_ARCH) \
				--method=http \
				--no-check-gpg \
				--progress \
				/mirror \
		"
	podman unshare chown -R $(shell id -u):$(shell id -g) $(CURDIR)/$(MIRROR_DIR)

.PHONY: build build_structure all sync clean help mirror_debian
all: build build_client build_structure
