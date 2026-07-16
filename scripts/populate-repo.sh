#!/bin/bash
set -e

UBUNTU_VERSION="${UBUNTU_VERSION:-26.04}"
UBUNTU_ISO="ubuntu-${UBUNTU_VERSION}-live-server-amd64.iso"
ISO_DIR_UBUNTU="assets/http/ubuntu/iso" MIRROR_DIR_CENTOS="assets/http/centos/mirror"
CENTOS_VERSION="${CENTOS_VERSION:-10-stream}"
ISO_URL="https://mirror.stream.centos.org/${CENTOS_VERSION}/BaseOS/x86_64/iso/CentOS-Stream-10-latest-x86_64-dvd1.iso"
ISO_PATH="/tmp/centos-stream-${CENTOS_VERSION}.iso"
REPO_DIR="assets/http/centos/mirror"

mkdir -p "$ISO_DIR_UBUNTU"
mkdir -p "$MIRROR_DIR_CENTOS"



mkdir -p "$REPO_DIR"

echo "[INFO] downloading CentOS ${CENTOS_VERSION} DVD ISO"
wget -q --show-progress "$ISO_URL" -O "$ISO_PATH"
echo "[SUCCESS] ISO downloaded"

echo "[INFO] extracting ISO contents"
podman run --rm \
    --privileged \
    -v "$ISO_PATH:/tmp/centos.iso:ro" \
    -v "$(pwd)/$REPO_DIR:/mnt/repo" \
    fedora:latest \
    bash -c "dnf install 7z -y && mount -o loop /tmp/centos.iso /mnt/cdrom 2>/dev/null || \
             7z x -y /tmp/centos.iso -o/mnt/repo/"
echo "[SUCCESS] ISO extracted"

rm -f "$ISO_PATH"
echo "[SUCCESS] CentOS local repo ready at $REPO_DIR"

echo "[INFO] fetching Ubuntu ${UBUNTU_VERSION} live server ISO"
wget -q --show-progress \
    "https://releases.ubuntu.com/${UBUNTU_VERSION}/${UBUNTU_ISO}" \
    -O "${ISO_DIR_UBUNTU}/${UBUNTU_ISO}"
echo "[SUCCESS] Ubuntu ISO fetched"
