.PHONY: web build test vet run docker clean e2e

e2e:
	cd web && npm run test:e2e

web:
	cd web && npm ci && npm run build

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
