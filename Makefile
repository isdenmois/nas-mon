.PHONY: web build test vet run docker clean

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
