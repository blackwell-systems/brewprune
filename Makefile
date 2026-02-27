.PHONY: build test lint vet clean install

build:
	go build -o bin/brewprune ./cmd/brewprune

test:
	go test -v -race -cover ./...

lint:
	golangci-lint run

vet:
	go vet ./...

clean:
	rm -rf bin/ dist/

install:
	go install ./cmd/brewprune
