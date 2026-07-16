#!/bin/bash
set -e

IMAGES_DIR="assets/tftp/images"

mkdir -p "$IMAGES_DIR/debian"
mkdir -p "$IMAGES_DIR/centos"
mkdir -p "$IMAGES_DIR/ubuntu"

echo "[INFO] fetching Debian bookworm"
wget -q --show-progress \
    https://deb.debian.org/debian/dists/bookworm/main/installer-amd64/current/images/netboot/debian-installer/amd64/linux \
    -O "$IMAGES_DIR/debian/linux"
wget -q --show-progress \
    https://deb.debian.org/debian/dists/bookworm/main/installer-amd64/current/images/netboot/debian-installer/amd64/initrd.gz \
    -O "$IMAGES_DIR/debian/initrd.gz"
echo "[SUCCESS] Debian images fetched"

echo "[INFO] fetching CentOS Stream 10"
wget -q --show-progress \
    https://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/images/pxeboot/vmlinuz \
    -O "$IMAGES_DIR/centos/vmlinuz"
wget -q --show-progress \
    https://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/images/pxeboot/initrd.img \
    -O "$IMAGES_DIR/centos/initrd.img"
echo "[SUCCESS] CentOS images fetched"

echo "[INFO] fetching Ubuntu noble"
wget -q --show-progress \
    https://releases.ubuntu.com/24.04/netboot/amd64/linux \
    -O "$IMAGES_DIR/ubuntu/linux"
wget -q --show-progress \
    https://releases.ubuntu.com/24.04/netboot/amd64/initrd \
    -O "$IMAGES_DIR/ubuntu/initrd"
echo "[SUCCESS] Ubuntu images fetched"

restorecon -Rv assets/tftp/images/ 2>/dev/null || true
echo "[SUCCESS] all images populated"
