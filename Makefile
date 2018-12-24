# This Makefile is meant to be used by people that do not usually work
# with Go source code. If you know what GOPATH is then you probably
# don't need to bother with make.

.PHONY: go-genaro android ios go-genaro-cross swarm evm all test clean
.PHONY: go-genaro-linux go-genaro-linux-386 go-genaro-linux-amd64 go-genaro-linux-mips64 go-genaro-linux-mips64le
.PHONY: go-genaro-linux-arm go-genaro-linux-arm-5 go-genaro-linux-arm-6 go-genaro-linux-arm-7 go-genaro-linux-arm64
.PHONY: go-genaro-darwin go-genaro-darwin-386 go-genaro-darwin-amd64
.PHONY: go-genaro-windows go-genaro-windows-386 go-genaro-windows-amd64

GOBIN = $(shell pwd)/build/bin
GO ?= latest

go-genaro:
	build/env.sh go run build/ci.go install ./cmd/go-genaro
	@echo "Done building."
	@echo "Run \"$(GOBIN)/go-genaro\" to launch go-genaro."

swarm:
	build/env.sh go run build/ci.go install ./cmd/swarm
	@echo "Done building."
	@echo "Run \"$(GOBIN)/swarm\" to launch swarm."

all:
	build/env.sh go run build/ci.go install

android:
	build/env.sh go run build/ci.go aar --local
	@echo "Done building."
	@echo "Import \"$(GOBIN)/go-genaro.aar\" to use the library."

ios:
	build/env.sh go run build/ci.go xcode --local
	@echo "Done building."
	@echo "Import \"$(GOBIN)/Geth.framework\" to use the library."

test: all
	build/env.sh go run build/ci.go test

clean:
	rm -fr build/_workspace/pkg/ $(GOBIN)/*

# The devtools target installs tools required for 'go generate'.
# You need to put $GOBIN (or $GOPATH/bin) in your PATH to use 'go generate'.

devtools:
	env GOBIN= go get -u golang.org/x/tools/cmd/stringer
	env GOBIN= go get -u github.com/kevinburke/go-bindata/go-bindata
	env GOBIN= go get -u github.com/fjl/gencodec
	env GOBIN= go get -u github.com/golang/protobuf/protoc-gen-go
	env GOBIN= go install ./cmd/abigen
	@type "npm" 2> /dev/null || echo 'Please install node.js and npm'
	@type "solc" 2> /dev/null || echo 'Please install solc'
	@type "protoc" 2> /dev/null || echo 'Please install protoc'

# Cross Compilation Targets (xgo)

go-genaro-cross: go-genaro-linux go-genaro-darwin go-genaro-windows go-genaro-android go-genaro-ios
	@echo "Full cross compilation done:"
	@ls -ld $(GOBIN)/go-genaro-*

go-genaro-linux: go-genaro-linux-386 go-genaro-linux-amd64 go-genaro-linux-arm go-genaro-linux-mips64 go-genaro-linux-mips64le
	@echo "Linux cross compilation done:"
	@ls -ld $(GOBIN)/go-genaro-linux-*

go-genaro-linux-386:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/386 -v ./cmd/go-genaro
	@echo "Linux 386 cross compilation done:"
	@ls -ld $(GOBIN)/go-genaro-linux-* | grep 386

go-genaro-linux-amd64:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/amd64 -v ./cmd/go-genaro
	@echo "Linux amd64 cross compilation done:"
	@ls -ld $(GOBIN)/go-genaro-linux-* | grep amd64

go-genaro-linux-arm: go-genaro-linux-arm-5 go-genaro-linux-arm-6 go-genaro-linux-arm-7 go-genaro-linux-arm64
	@echo "Linux ARM cross compilation done:"
	@ls -ld $(GOBIN)/go-genaro-linux-* | grep arm

go-genaro-linux-arm-5:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/arm-5 -v ./cmd/go-genaro
	@echo "Linux ARMv5 cross compilation done:"
	@ls -ld $(GOBIN)/go-genaro-linux-* | grep arm-5

go-genaro-linux-arm-6:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/arm-6 -v ./cmd/go-genaro
	@echo "Linux ARMv6 cross compilation done:"
	@ls -ld $(GOBIN)/go-genaro-linux-* | grep arm-6

go-genaro-linux-arm-7:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/arm-7 -v ./cmd/go-genaro
	@echo "Linux ARMv7 cross compilation done:"
	@ls -ld $(GOBIN)/go-genaro-linux-* | grep arm-7

go-genaro-linux-arm64:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/arm64 -v ./cmd/go-genaro
	@echo "Linux ARM64 cross compilation done:"
	@ls -ld $(GOBIN)/go-genaro-linux-* | grep arm64

go-genaro-linux-mips:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/mips --ldflags '-extldflags "-static"' -v ./cmd/go-genaro
	@echo "Linux MIPS cross compilation done:"
	@ls -ld $(GOBIN)/go-genaro-linux-* | grep mips

go-genaro-linux-mipsle:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/mipsle --ldflags '-extldflags "-static"' -v ./cmd/go-genaro
	@echo "Linux MIPSle cross compilation done:"
	@ls -ld $(GOBIN)/go-genaro-linux-* | grep mipsle

go-genaro-linux-mips64:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/mips64 --ldflags '-extldflags "-static"' -v ./cmd/go-genaro
	@echo "Linux MIPS64 cross compilation done:"
	@ls -ld $(GOBIN)/go-genaro-linux-* | grep mips64

go-genaro-linux-mips64le:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/mips64le --ldflags '-extldflags "-static"' -v ./cmd/go-genaro
	@echo "Linux MIPS64le cross compilation done:"
	@ls -ld $(GOBIN)/go-genaro-linux-* | grep mips64le

go-genaro-darwin: go-genaro-darwin-386 go-genaro-darwin-amd64
	@echo "Darwin cross compilation done:"
	@ls -ld $(GOBIN)/go-genaro-darwin-*

go-genaro-darwin-386:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=darwin/386 -v ./cmd/go-genaro
	@echo "Darwin 386 cross compilation done:"
	@ls -ld $(GOBIN)/go-genaro-darwin-* | grep 386

go-genaro-darwin-amd64:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=darwin/amd64 -v ./cmd/go-genaro
	@echo "Darwin amd64 cross compilation done:"
	@ls -ld $(GOBIN)/go-genaro-darwin-* | grep amd64

go-genaro-windows: go-genaro-windows-386 go-genaro-windows-amd64
	@echo "Windows cross compilation done:"
	@ls -ld $(GOBIN)/go-genaro-windows-*

go-genaro-windows-386:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=windows/386 -v ./cmd/go-genaro
	@echo "Windows 386 cross compilation done:"
	@ls -ld $(GOBIN)/go-genaro-windows-* | grep 386

go-genaro-windows-amd64:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=windows/amd64 -v ./cmd/go-genaro
	@echo "Windows amd64 cross compilation done:"
	@ls -ld $(GOBIN)/go-genaro-windows-* | grep amd64
