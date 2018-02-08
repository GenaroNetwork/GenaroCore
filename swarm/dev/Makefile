.PHONY: build cluster test

default: build

build:
	go build -o bin/swarm    github.com/GenaroNetwork/Genaro/cmd/swarm
	go build -o bin/geth     github.com/GenaroNetwork/Genaro/cmd/geth
	go build -o bin/bootnode github.com/GenaroNetwork/Genaro/cmd/bootnode

cluster: build
	scripts/boot-cluster.sh

test:
	go test -v github.com/GenaroNetwork/Genaro/swarm/...
