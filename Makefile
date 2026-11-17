.PHONY: web build test vet run docker clean e2e

e2e:
	bun run test:e2e

web:
	bun install --frozen-lockfile && bun run build

build: web
	go build -o bin/mon ./cmd/mon

test:
	go test ./...

vet:
	go vet ./...

run:
	go run ./cmd/mon

docker:
	docker compose up -d --build

clean:
	rm -rf bin
