# o2-ims-worker

A bare metal provisioning agent that bridges the O-RAN O2 IMS with physical
infrastructure. It registers with the IMS API, pulls machine inventory by
cluster, and runs a DHCP/PXE/TFTP stack to boot those machines. Resource
metrics from the Kubernetes cluster are collected via Prometheus and reported
back to the IMS.

## What it does

On startup the worker registers itself with the IMS API using its hostname
and outbound IP. It then runs three concurrent services:

- DHCP server that issues leases only to MAC addresses registered in the IMS.
  Unregistered machines are rejected. PXE boot options are set automatically
  for registered clients.
- TFTP server that serves the PXE boot files from a configurable root
  directory.
- Heartbeat loop that reports service status, system memory and CPU usage, and
  DHCP/TFTP metric counts back to the IMS API every 30 seconds.

The IMS is the single source of truth for which machines exist. The worker
syncs machine state from the IMS and keeps lease records in a local PostgreSQL
database.

## Requirements

- Go 1.21+
- PostgreSQL (for DHCP lease state)
- Host network privileges (ports 67, 69)
- A running O2 IMS API instance
- PXE boot files at `/var/lib/tftpboot/`

## Environment variables

| Variable | Description |
|---|---|
| `IMS_API_URL` | Base URL of the O2 IMS API |
| `IMS_API_USERNAME` | IMS API username |
| `IMS_API_PASSWORD` | IMS API password |
| `IMS_METRICS_URL` | Metrics endpoint (defaults to `IMS_API_URL/api/clusters/6/update_metrics/`) |
| `KUBECONFIG` | Path to kubeconfig for Prometheus queries (optional) |

## Development

Install Go on Fedora:

```bash
sudo dnf install golang
```

Build and run locally:

```bash
# Download dependencies
go mod tidy

# Build binary
go build ./cmd/boothandler/...

# Run (requires host network and PostgreSQL)
IMS_API_URL=http://localhost:8000 \
IMS_API_USERNAME=admin \
IMS_API_PASSWORD=secret \
./boothandler
```

Run with Docker Compose (mounts local binary):

```bash
# Build binary first
go build -o build/ims-worker ./cmd/boothandler/...

# Uncomment the volume mount in docker-compose.yaml:
# volumes:
#   - ./build/ims-worker:/usr/bin/ims-worker:z

podman-compose up
```

The container requires privileged mode for DHCP (port 67) and TFTP (port 69).

## Production

Build the image:

```bash
make image
```

The container expects all environment variables to be set at runtime and a
TFTP root directory mounted at `/var/lib/tftpboot/` containing at minimum
`pxelinux.0`.

## Stack

Go, github.com/insomniacslk/dhcp, github.com/pin/tftp, k8s client-go,
pgxpool (PostgreSQL), gopsutil
