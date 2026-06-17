# PXE Installer Agent 
[![wakatime](https://wakatime.com/badge/user/bbc34f7e-9b97-4141-9f4c-64ed61b82c61/project/1b641e6e-4b98-40c8-b667-8ff44b7ad006.svg)](https://wakatime.com/badge/user/bbc34f7e-9b97-4141-9f4c-64ed61b82c61/project/1b641e6e-4b98-40c8-b667-8ff44b7ad006)

## Prerequisites

### Netboot Artifacts 

You will need the following files for minimal PXE boot 

1. pxelinux.0
2. libutil.c32
3. ldlinux.c32
4. menu.c32

If you are using Fedora as Host you can get these packages from `syslinux-tftpboot`

```bash
mkdir -p assets/generic
sudo dnf install syslinux-tftpboot -y
cp /usr/share/syslinux/menu.c32 assets/generic/
cp /usr/share/syslinux/libutil.c32 assets/generic/
cp /usr/share/syslinux/ldlinux.c32 assets/generic/

# Copy pxelinux.cfg files to assets
cp -r pxelinux.cfg/ assets/generic
tree assets/
assets/
└── generic
    ├── ldlinux.c32
    ├── libutil.c32
    ├── menu.c32
    ├── pxelinux.0
    └── pxelinux.cfg
        ├── 01-52-54-00-22-e5-fc
        ├── 01-52-54-00-8f-c1-32
        ├── 01-de-ea-db-ee-ee-ff
        ├── 01-e2-32-36-e8-63-b5
        ├── 01-e2-33-36-e8-63-b5
        ├── 01-e2-37-26-f8-63-b5
        ├── 01-e2-37-36-e8-12-b7
        ├── 01-e2-37-36-e8-63-b5
        ├── 01-XXX
        └── default

3 directories, 14 files

# Download vmlinuz and initrd
wget http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/images/pxeboot/vmlinuz -O assets/generic/stream10
wget http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/images/pxeboot/initrd.img -O assets/generic/stream10
```

Populate the `assets/` path according to the following structure

```
assets/
├── boot
│   ├── rhel
│   └── ubuntu
│       └── jammy
│           ├── initrd
│           ├── metadata
│           │   ├── meta-data
│           │   └── user-data
│           ├── ubuntu-24.04-latest-live-server-amd64.iso
│           └── vmlinuz
```


## Main Function

```
-> Call Config
-> Spawn TFTP
-> Spawn DHCP
-> Start Metric Agent
```

## Packages
This projecet will be separated into packages
1. config: Handle config parameters
2. tftp: Handle TFTP Function
3. dhcp: Hanlde DHCP Function

### Config

Variables are exposed as immutable reference via shadow function

```
Gather() -> Get information of current values
```

### TFTP Server


```
*)
```


## Testing Module

### TFTP Client

```
go run cmd/worker/client.go
go run cmd/worker/clientTFTP.go  -h
#   -a string
#     	Server Address (default "127.0.0.1")
#   -f string
#     	File to Download (default "pxelinux.0")
#   -p string
#     	Server Port (default "69")
#   -t int
#     	Timeout (default 10)
# infidel at bmw-nuc in ~/o2-ims-worker on dev*
```

## Scripts

#### Create DHCP Interfaces

```
sudo scripts/createDummyInterface.sh create
# Will create br0 and dummy-client
sudo scripts/createDummyInterface.sh show
# List the interfaces
sudo scripts/createDummyInterface.sh clean
# Clean the interfaces
```
