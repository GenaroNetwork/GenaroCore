.PHONY: build cluster test

default: build

build:
	go build -o bin/swarm    github.com/GenaroNetwork/GenaroGNX/cmd/swarm
	go build -o bin/geth     github.com/GenaroNetwork/GenaroGNX/cmd/geth
	go build -o bin/bootnode github.com/GenaroNetwork/GenaroGNX/cmd/bootnode

cluster: build
	scripts/boot-cluster.sh

test:
	go test -v github.com/GenaroNetwork/GenaroGNX/swarm/...
