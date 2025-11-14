# O2 IMS WORKER

## What Language?

Need to decide what language and framework for this project:
- GO: Tinkerbell implementation shows, how its done.
- Python: Easier implementation, but no clear information regarding its current supported features.

Any programming language we choose, they must have the following features (preferably not to be recreated):

- DHCP Server
- PXE Server
- Netboot features
- TFTP features
- Argument Handler


## GO

- DHCP Server: github.com/insomniacslk/dhcp/dhcpv4
- PXE Server: ...
- HTTP Server: ...
- TFTP Server: ...

## Development Testing

1. Install golang at fedora

    ```bash
    sudo dnf install golang
    ```

2. Environment setup

    - The container need the following ports (67,69,514). Host privilege is a must.
    -
    -
3. Build Local and load binary to using docker-compose

```
# Download depencies
go mod tidy
# Build project
go build

```

4. Uncomment the following line 

```
    volumes:
        - /build/ims-worker:/usr/bin/ims-worker:z
```

5. Start the service usinge `podman-compose up`


## Production

1. Build image

```
make image
```





