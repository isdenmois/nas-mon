# syntax=docker/dockerfile:1

FROM oven/bun:1-alpine AS web
WORKDIR /src
COPY package.json bun.lock ./
RUN bun install --frozen-lockfile
COPY web ./web
RUN bun run build

FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY --from=web /src/internal/httpapi/dist ./internal/httpapi/dist
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/mon ./cmd/mon

FROM alpine:3.22
RUN apk add --no-cache smartmontools tzdata ca-certificates
COPY --from=build /out/mon /usr/local/bin/mon
EXPOSE 8090
VOLUME ["/data"]
HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8090/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/mon"]
