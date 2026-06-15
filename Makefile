all: build build_clients

build:
	go build -o bin/worker cmd/worker/main.go 

build_client:
	go build -o bin/clientDHCP cmd/worker/clientDHCP.go 
	go build -o bin/clientTFTP cmd/worker/clientTFTP.go 

clean:
	rm -rf bin/*

.PHONY: build
