#!/bin/bash
set -e

UBUNTU_VERSION="${UBUNTU_VERSION:-26.04}"
UBUNTU_ISO="ubuntu-${UBUNTU_VERSION}-live-server-amd64.iso"
ISO_DIR_UBUNTU="assets/http/ubuntu/iso"

CENTOS_VERSION="${CENTOS_VERSION:-10-stream}"
CENTOS_ISO="CentOS-Stream-10-latest-x86_64-dvd1.iso"
ISO_URL="https://mirror.stream.centos.org/${CENTOS_VERSION}/BaseOS/x86_64/iso/${CENTOS_ISO}"
ISO_PATH="/tmp/centos-stream-${CENTOS_VERSION}.iso"
REPO_DIR="assets/http/centos/mirror"

mkdir -p "$ISO_DIR_UBUNTU"
mkdir -p "$REPO_DIR"

# ----------------------------
# CentOS
# ----------------------------
echo "[INFO] downloading CentOS ${CENTOS_VERSION} DVD ISO"
if [ -f "$ISO_PATH" ]; then
    echo "[INFO] ISO already exists: $ISO_PATH"
else
    wget -q --show-progress "$ISO_URL" -O "$ISO_PATH"
fi
echo "[SUCCESS] ISO ready"

echo "[INFO] extracting ISO contents"
# Use a container with 7z (same as before, but added -y for overwrite)
podman run --rm \
    --privileged \
    -v "$ISO_PATH:/tmp/centos.iso:ro" \
    -v "$(pwd)/$REPO_DIR:/mnt/repo" \
    fedora:latest \
    bash -c "dnf install -y 7zip && 7z x -y /tmp/centos.iso -o/mnt/repo/"
echo "[SUCCESS] ISO extracted"

# Remove ISO after extraction (optional, but keeps /tmp clean)
rm -f "$ISO_PATH"
echo "[SUCCESS] CentOS local repo ready at $REPO_DIR"

# ----------------------------
# Ubuntu
# ----------------------------
echo "[INFO] fetching Ubuntu ${UBUNTU_VERSION} live server ISO"
if [ -f "${ISO_DIR_UBUNTU}/${UBUNTU_ISO}" ]; then
    echo "[INFO] Ubuntu ISO already exists"
else
    wget -q --show-progress \
        "https://releases.ubuntu.com/${UBUNTU_VERSION}/${UBUNTU_ISO}" \
        -O "${ISO_DIR_UBUNTU}/${UBUNTU_ISO}"
fi
echo "[SUCCESS] Ubuntu ISO fetched"
