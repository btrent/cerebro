.PHONY: build run test setup fmt vet tidy clean

BINARY := cerebro
PKG := ./cmd/cerebro

build:
	go build -o bin/$(BINARY) $(PKG)

run:
	go run $(PKG)

test:
	go test ./...

# One-time: create the dedicated venv and install MLX runtimes.
setup:
	bash scripts/setup.sh

fmt:
	gofmt -w .

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -rf bin
