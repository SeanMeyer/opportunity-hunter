.PHONY: build test lint run clean

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

build:
	CGO_ENABLED=0 go build -ldflags="-X main.version=$(VERSION)" -o opportunity-hunter ./cmd/opportunity-hunter/

test:
	go test ./...

lint:
	go vet ./...

run:
	go run ./cmd/opportunity-hunter/ run

clean:
	rm -f opportunity-hunter
