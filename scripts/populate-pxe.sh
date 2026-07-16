#!/bin/bash
set -e

BIOS_DIR="assets/tftp/bios"
EFI_DIR="assets/tftp/efi"

mkdir -p "$BIOS_DIR/pxelinux.cfg"
mkdir -p "$EFI_DIR/grub/x86_64-efi"

echo "[INFO] extracting BIOS PXE files"
podman run --name pxe-bios debian:stable bash -c "
    apt-get update -q && apt-get install -y pxelinux syslinux-common
    cp \$(find /usr/lib -name 'pxelinux.0'  | head -1) /tmp/pxelinux.0
    cp \$(find /usr/lib -name 'ldlinux.c32' | head -1) /tmp/ldlinux.c32
    cp \$(find /usr/lib -name 'menu.c32'    | head -1) /tmp/menu.c32
    cp \$(find /usr/lib -name 'libutil.c32' | head -1) /tmp/libutil.c32
"

podman cp pxe-bios:/tmp/pxelinux.0  "$BIOS_DIR/pxelinux.0"
podman cp pxe-bios:/tmp/ldlinux.c32 "$BIOS_DIR/ldlinux.c32"
podman cp pxe-bios:/tmp/menu.c32    "$BIOS_DIR/menu.c32"
podman cp pxe-bios:/tmp/libutil.c32 "$BIOS_DIR/libutil.c32"
podman rm pxe-bios
echo "[SUCCESS] BIOS files extracted"

echo "[INFO] extracting EFI PXE files"
podman run --name pxe-efi debian:stable bash -c "
    apt-get update -q && apt-get install -y grub-efi-amd64-bin grub-efi-amd64-signed shim-signed
    mkdir -p /tmp/netboot
    grub-mknetdir --net-directory=/tmp/netboot --subdir=/efi/grub
    cp \$(find /usr/lib -name 'shimx64.efi.signed' | head -1) /tmp/shimx64.efi.signed
"

podman cp pxe-efi:/tmp/netboot/efi/grub/.   "$EFI_DIR/grub/"
podman cp pxe-efi:/tmp/shimx64.efi.signed    "$EFI_DIR/shimx64.efi.signed"
podman cp pxe-efi:/tmp/shimx64.efi.signed    "$EFI_DIR/shimx64.efi"
podman rm pxe-efi
echo "[SUCCESS] EFI files extracted"

restorecon -Rv assets/tftp/ 2>/dev/null || true
echo "[SUCCESS] PXE assets populated"
