.PHONY: build test lint vet clean install \
        test-container test-shell docker-build

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

# ── Container targets ────────────────────────────────────────────────────────

docker-build:
	docker compose build

# Run the full integration test suite inside the container
test-container: docker-build
	docker compose run --rm test

# Drop into an interactive shell with brewprune + mock brew pre-installed
# Useful for manual exploration: run 'brewprune scan', poke the DB, etc.
test-shell: docker-build
	docker compose run --rm shell
