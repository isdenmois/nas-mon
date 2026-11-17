# AGENTS.md

`mon` is a Linux homelab monitor: a Go backend that serves an embedded Vue 3 +
Chart.js dashboard and Prometheus metrics. Frontend sources live in `web/`; the
build is copied into `internal/httpapi/dist/` and embedded with `//go:embed`.

## Commands

Use Bun, not npm — `bun.lock` is the source of truth. First-time setup:
`bun install && just install-hooks`.

- `just unit` — frontend unit tests (Vitest). `just test` is **Go only**.
- `just e2e` — Playwright; mocks the whole API, so no Go backend, root, or Docker.
- `just lint`, `just format`, `just vet`, `bun run typecheck`, `just build`.
- Single Go test: `go test ./internal/collect -run TestName`
- Single Vitest file: `bunx vitest run web/src/shared/lib/format.test.ts`
- Single e2e case: `bunx playwright test e2e/overview.spec.ts -g "renders the current snapshot"`

Pre-commit runs format (writes and stages fixes) → lint → typecheck → Go tests →
Vitest; e2e runs on pre-push. Do not commit with hooks skipped without a reason.

## Build artifact gotcha

`bun run build` / `just web` / `just build` rewrites the tracked stub
`internal/httpapi/dist/index.html`. Restore it before committing:

```sh
git checkout -- internal/httpapi/dist/index.html
```

Everything else under `internal/httpapi/dist/` is gitignored. A plain `go build`
works before the first frontend build because the stub is embedded.

## Structure

- Go flow: `cmd/mon` → `internal/app` (composition root) → `internal/collect`
  (hwmon, host, docker, volumes, SMART) → `internal/monitor` (ingest, minute
  buckets, wear policy, queries) → `internal/store` (SQLite). `internal/httpapi`
  exposes UI, `/api/*`, `/metrics`, `/healthz`. `internal/domain` holds the
  persisted metric kind names — do not rename them casually. `internal/api` is
  the JSON contract.
- Frontend uses Feature-Sliced Design: `app` → `pages` → `entities` → `shared`.
  Import only downward, and through each slice's `index.ts`. `@` maps to
  `web/src`. Do not add a `components/` dump or import across slices.
- `web/src/shared/api/contracts.ts` mirrors JSON produced by the Go API. When
  adding/changing a field, update the Go response, the contract, and
  `e2e/fixtures.ts` together.

## Testing quirks

- The e2e mock must match real API paths only (`url.pathname.startsWith('/api/')`).
  A `**/api/**` glob also intercepts dev modules under `/src/shared/api/` and
  serves them a 404.
- Playwright freezes the clock at `2026-01-02T03:04:05Z`, locale `en-US`, timezone
  `UTC`. Chart screenshots are committed; regenerate with
  `bun run test:e2e:update` only for an intended visual change and inspect the
  image diff.
- Vitest uses jsdom, colocated `web/src/**/*.test.ts`, and the `@` alias from
  `vitest.config.ts`. Tests follow AAA (`// Arrange`, `// Act`, `// Assert`).

## Runtime / env

- Collectors are Linux-only (`/proc`, `/sys/class/hwmon`, Docker socket). SMART
  needs root; run locally without it:
  `MON_DB_PATH=./data/mon.db MON_SMART_INTERVAL=0 go run ./cmd/mon`.
- All configuration comes from `MON_*` env vars parsed in `internal/config`;
  the table (and API) is documented in `README.md`.
- SQLite is single-writer; `data/` is local state and gitignored. The Docker
  image is privileged with `network_mode: host` (needed for host network stats).

## Style

- UnoCSS (`web/uno.config.ts`) provides atomic utilities; its theme colors map to
  the CSS variables in `web/src/app/styles/global.css`. Use utilities for new
  layout and leaf components, and keep shared component classes in `global.css`.
- Biome covers `web/`, `e2e/`, `playwright.config.ts`, and `package.json`; use
  `bun run lint:fix`. `noUnusedImports`/`noUnusedVariables` are disabled for
  `.vue` because Biome cannot see template usage — `vue-tsc` is the authority.
