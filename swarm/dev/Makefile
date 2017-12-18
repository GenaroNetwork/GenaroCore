.PHONY: build cluster test

default: build

build:
	go build -o bin/swarm    github.com/GenaroNetwork/GenaroCore/cmd/swarm
	go build -o bin/geth     github.com/GenaroNetwork/GenaroCore/cmd/geth
	go build -o bin/bootnode github.com/GenaroNetwork/GenaroCore/cmd/bootnode

cluster: build
	scripts/boot-cluster.sh

test:
	go test -v github.com/GenaroNetwork/GenaroCore/swarm/...
