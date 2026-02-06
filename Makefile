VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

.PHONY: build test test-race lint fmt clean coverage coverage-html

build:
	go build -ldflags="-X github.com/GainForest/hypercerts-cli/cmd.version=$(VERSION)" -o hc ./cmd/hc

test:
	go test ./...

test-race:
	go test -race ./...

lint:
	golangci-lint run

fmt:
	go fmt ./...

clean:
	rm -f hc coverage.out coverage.html

coverage:
	go test -coverprofile=coverage.out ./...

coverage-html: coverage
	go tool cover -html=coverage.out -o coverage.html
