# Backend architecture

This document is the design of record for the Go backend. If code and this file
disagree, one of them is a bug.

## Shape

Six packages, one arrow. Everything below points down; nothing points sideways.

| Package | May import | Responsibility |
|---|---|---|
| `internal/domain` | nothing | Metric vocabulary and the rules over it. Pure Go: no I/O, no clock, no JSON, no SQL. |
| `internal/api` | `domain` | The HTTP/JSON contract. Wire structs only, with `json` tags. |
| `internal/collect` | nothing | Linux/Docker adapters. Reads `/proc`, `/sys`, the Docker socket, `smartctl`. Returns plain snapshots. |
| `internal/monitor` | `domain`, `api`, `collect` (ingest mapping only) | Use cases: ingest a snapshot, bucket samples, flush, wear policy, query history, serve the overview snapshot. Declares the ports it needs. |
| `internal/store` | `domain`, `api` | The only package that contains SQL. Implements monitor's storage port. |
| `internal/httpapi` | `api`, `domain`, `monitor` | HTTP adapters: routing, parameter parsing, JSON encoding, Prometheus exposition, static files. |
| `internal/app` | everything | Composition root: construct adapters, schedule collector loops, own startup and shutdown. |
| `cmd/mon` | `app`, `config` | Entry point: config, logger, signals. |
| `internal/config` | nothing | `MON_*` environment parsing and defaults. |

Rules:

1. **`domain` imports nothing internal.** If a rule needs the clock, it takes a
   `time.Time` argument; if it needs storage, the caller does the I/O.
2. **No adapter imports another adapter.** `collect` never sees `store`,
   `httpapi` never sees `store`, `store` never sees `httpapi`. The single
   exception is `monitor/ingest.go`, which reads collector snapshots to turn
   them into samples and wire types — that mapping is its job.
3. **Interfaces belong to the consumer**, not the implementer and not to a
   `ports` package. `monitor` declares what it needs from storage; `httpapi`
   declares what it needs from `monitor`. `app` passes the concrete types in.
4. **Only `app` knows every concrete type.** Wiring decisions live in one place.
5. **Wire types live only in `api`.** `domain`, `collect` and `store` carry no
   `json` tags. A change to a response field is a change to `api` plus
   `web/src/shared/api/contracts.ts`, in the same commit.
6. **SQL lives only in `store`**, and only through `domain`/`api` types. Table
   names are chosen by `domain.Resolution`, never built from request input.
7. **Validation happens where data enters**: collectors validate hardware data,
   `httpapi` parses request parameters, `store` scans rows. Inside, trust the
   types.

## Glossary

| Term | Meaning |
|---|---|
| **Kind** | What is measured: `temp`, `host_cpu`, `container_mem`, … Persisted in SQLite and used as a Prometheus label. Never renamed without a migration. |
| **Series** | A `Kind` applied to a `Name` (a host, a container, a sensor key, a disk). Identity is server-assigned: `Series.ID` is what the wire carries, so names containing `:` are not a parsing problem. |
| **Sample** | One observation: `(Kind, Name, value)` at one instant. Lives in memory only. |
| **Point** | An aggregated time bucket: `(TS, avg, min, max, n)`. The only thing written to SQLite. |
| **Resolution** | The bucket size of a point table: minute, hour or day. Owns its table name and step. |
| **Range** | A UI preset (`1h`, `24h`, `all`, …) resolved into `(from, to, Resolution)`. |
| **WearObservation** | One SSD wear row: wear percentage, TBW, power-on hours, error counters. |
| **WearReason** | Why a wear row is worth writing: `first`, `identity`, `errors`, `wear`, `tbw`, `tbw-reset`. |
| **Snapshot** | What the dashboard sees right now: latest temperatures, host, containers, volumes, SMART, plus series descriptors. |
| **CollectorStatus** | Health of one collector loop: last success, last error, run count, duration. |

## Invariants

* **Metric kind strings are permanent.** They are rows in `series.kind` and
  Prometheus label values. Changing one is a migration, not a rename.
* **Every write is an idempotent upsert.** A crash between bucket and rollup, or
  a retried flush, must not duplicate history.
* **The last partial minute is dropped.** Losing a few seconds on shutdown beats
  publishing a skewed average.
* **NaN and Inf never reach SQL.** `domain.Sanitize` runs at the ingest boundary.
* **One process per database.** SQLite is opened single-writer; this is a
  deployment rule, not a lock.
* **Retention per resolution**: minutes 48 h, hours 90 d, days 5 y (configurable
  through `MON_RETENTION_*`; `0` keeps a resolution forever).

## Recipes

**Add a metric.**
1. `domain/kind.go`: add the `Kind` constant (and its unit, if the
   exposition or the UI needs one).
2. The collector struct: add the field and parse it.
3. `monitor/ingest.go`: one mapping line from the snapshot to a `domain.Sample`.
4. `api`: add the field to the snapshot DTO; `contracts.ts` follows.
5. If it should be charted: the frontend asks the series registry for its id —
   never builds a `kind:name` string.

**Add an endpoint.**
1. Wire types in `api` (if new).
2. A use-case method on `monitor`, plus the port it needs (declared there).
3. Handler + route in `httpapi`; parse and validate parameters at the edge.
4. `web/src/shared/api/` fetcher, contract type and e2e fixture.

**Change the schema.**
1. New `store/migrations/NNNN_name.sql` with `-- +goose Up` (down migrations
   only when they can be written honestly).
2. Update the query in `store`, and `store_test.go` round-trip.
3. If a stored kind string changes, migrate the rows in the same migration.

**Add a collector.**
1. Adapter package function returning a snapshot (fake the filesystem in tests).
2. `MON_<X>_INTERVAL` in `config`, plus its row in the README table.
3. Mapping in `monitor/ingest.go`.
4. `app.Run`: one more `loop` call.

## Testing

| Layer | How |
|---|---|
| `domain` | Table tests over pure functions. No clock, no files, no DB. |
| `monitor` | Fakes for the declared ports; real time only via injected `now`. |
| `store` | Real SQLite in `t.TempDir()`; every migration applied on open. |
| `collect` | Fake `/proc`, `/sys` trees in `t.TempDir()`, in-process Docker fake, `httptest` for the socket client, canned `smartctl` output. |
| `httpapi` | `httptest` against a fake monitor; no real DB. |
| e2e | Playwright with a mocked API, frozen clock, committed chart screenshots. |

`just test` is Go only; `just unit` is Vitest; `just e2e` needs neither root nor
Docker.
