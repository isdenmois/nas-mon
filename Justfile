# List available recipes
default:
    @just --list

# Run the Playwright e2e suite
e2e:
    bun run test:e2e

# Install web dependencies and build into internal/server/dist
web:
    bun install --frozen-lockfile
    bun run build

# Build the Go binary (includes the web frontend)
build: web
    go build -o bin/mon ./cmd/mon

# Run Go tests
test:
    go test ./...

# Run Go vet
vet:
    go vet ./...

# Run the app locally
run:
    go run ./cmd/mon

# Build and start the Docker Compose stack
docker:
    docker compose up -d --build

# Remove build artifacts
clean:
    rm -rf bin
