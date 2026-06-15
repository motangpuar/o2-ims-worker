# PXE Installer Agent 

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
