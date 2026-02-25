default: build

build:
    go build -o gjoll ./cmd/gjoll

test:
    go test ./...

lint:
    go vet ./...
    golangci-lint run

fmt:
    gofmt -w .

tidy:
    go mod tidy

clean:
    rm -f gjoll

all: fmt lint test build
