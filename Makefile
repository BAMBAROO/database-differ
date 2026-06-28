.PHONY: build build-all verify-cgo test lint clean up down

BINARY_NAME=db-schema-differ
VERSION=1.0.0
BUILD_DATE=$(shell date -u +%Y-%m-%d)
LDFLAGS=-ldflags "-X github.com/bryanathallah/db-schema-differ/cmd.Version=$(VERSION) -X github.com/bryanathallah/db-schema-differ/cmd.BuildDate=$(BUILD_DATE)"

build:
	go build $(LDFLAGS) -o bin/$(BINARY_NAME) main.go

build-all:
	mkdir -p dist
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64  go build $(LDFLAGS) -o dist/$(BINARY_NAME)-macos-amd64 main.go
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64  go build $(LDFLAGS) -o dist/$(BINARY_NAME)-macos-arm64 main.go
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64  go build $(LDFLAGS) -o dist/$(BINARY_NAME)-windows-amd64.exe main.go
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64  go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-amd64 main.go

verify-cgo: build-all
	@go tool nm dist/$(BINARY_NAME)-linux-amd64 | grep -q "x_cgo_init" && (echo "ERROR: CGO found in binary" && exit 1) || echo "OK: no CGO"

test:
	go test -v ./...

lint:
	golangci-lint run

clean:
	rm -rf bin/ dist/

up:
	docker-compose up -d

down:
	docker-compose down
