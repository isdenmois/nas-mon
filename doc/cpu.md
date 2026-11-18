# CPU cost

This is the measurement record for "mon eats CPU in the background". If the code
and these numbers disagree, re-run the commands below instead of trusting either.

## What runs 24/7

Nothing on the dashboard side (the tab pauses itself when hidden, and `/metrics`
is only as expensive as the scrape rate). What remains is seven loops:

| Loop | Default | Work per tick |
|---|---|---|
| aggregate | 5m | one write transaction: upsert minutes, roll up touched hours and days, prune at most hourly |
| temp | 30s | `ReadDir(/sys/class/hwmon)` plus a few file reads |
| host | 30s | four `/proc` files |
| docker | 60s | one container list plus one stats request per container, 4 in flight |
| volumes | 5m | `mountinfo` plus one `statfs` per mount under `/volumes` |
| smart | 12h | one `smartctl -j -a` per disk |
| healthcheck | 30s | `GET /healthz` from Docker |

## Measured

Hardware: Ryzen 7 9800X3D, local disk, no dockerd and no hwmon on this box, so
these are component measurements rather than a whole-host number.

### Docker tick — 25 containers (`BenchmarkRead25Containers`)

`GET /containers/json` plus 25 `GET /containers/{id}/stats?stream=false` calls
against an HTTP server serving a realistic ~8 KB stats document per container.

| | value |
|---|---|
| client CPU per tick | **0.86 ms** (780 KB decoded, 4 300 allocs) |
| at the old 15s cadence | 4 ticks/min ≈ 3.4 ms/min |
| at 60s | 1 tick/min ≈ 0.86 ms/min |

The client is not the expensive half: the other half is **dockerd** computing
cgroup stats for every container, which also drops 4×. That cost lives outside
this process, so it does not appear in any profile of `mon`.

### Store (120 series)

| Benchmark | original | first pass | now |
|---|---|---|---|
| flush one minute of samples | 3.0 ms, 5 680 allocs | 2.2 ms, 1 612 allocs | **1.1 ms**, 3 187 allocs |
| the same samples batched as five minutes | 11.4 ms, 28 244 allocs | 7.5 ms, 7 858 allocs | **2.3 ms**, 8 056 allocs |
| prune over 219 k day rows | 5.5 ms | 1.1 ms | 1.1 ms |

Per hour of runtime, with day retention enabled:

| | original | now |
|---|---|---|
| flushes | 60 × 3.0 ms = 180 ms | 12 × 2.3 ms = 28 ms |
| prunes | 60 × 5.5 ms = 330 ms | 1 × 1.1 ms = 1.1 ms |
| total | **510 ms/h** | **29 ms/h** |

Two things made the second pass, both visible in the CPU traces:

- **statements are prepared once** instead of `fmt.Sprintf`-built and parsed per
  row. The trace showed `_exprDup`, `_sqlite3ExprListFlags` and
  `_sqlite3PrimaryKeyIndex` — SQLite's *compiler*, not its executor.
- **the rollup runs per series** (`WHERE series_id = ? AND ts >= ? AND ts < ?`)
  instead of one `GROUP BY` over the whole bucket. The old plan read
  `points_minute_ts` and then sorted every series through a temp B-tree
  (`_sqlite3BtreeIndexMoveto`); the new one seeks 5–60 rows per series on the
  primary key and sorts nothing.

Allocation count rose while bytes per op fell (710 kB → 470 kB): each rollup is
now its own prepared exec, which costs a few small allocations and saves far
more time. The store is not allocation-bound at 12 flushes per hour.

### Syscalls

`Syscall6` was 19% of the second trace. It is not a separate cost to fix: it is
the write/read work of everything above — WAL page writes during the flush,
`socket read` for the Docker stats payloads, `openat`/`read` for `/proc` and
`/sys`. Cutting the DB work by 17× cut its share of syscalls with it; the
remainder tracks scrape-free work that only more container polling would grow.

To split the remainder by syscall, run on the host:

```sh
strace -c -f -p $(docker inspect -f '{{.State.Pid}}' nas-mon)   # 60s, then Ctrl-C
```


That is 0.014% of one core originally and 0.0008% now, on this CPU. A NAS core
is slower, but the order of magnitude does not change: the store is a rounding
error either way.

### Whole process, idle

`scripts/bench-cpu.sh idle 120` on a box with no dockerd and no hwmon measures
less than the 10 ms resolution of `wait4` rusage, i.e. **under 0.01% of a core**.

## Reproduce

```sh
go test ./internal/collect -run XXX -bench BenchmarkRead25Containers -benchtime 20x
go test ./internal/store   -run XXX -bench .                       -benchtime 30x

just build
scripts/bench-cpu.sh idle 300        # also: dashboard, scrape
```

On the real host there is a pprof listener instead of a benchmark:

```sh
MON_PPROF_ADDR=127.0.0.1:6061 docker compose up -d
curl -s -o /tmp/cpu.pprof 'http://127.0.0.1:6061/debug/pprof/profile?seconds=60'
go tool pprof -top -nodecount=15 /tmp/cpu.pprof
```

`docker stats nas-mon` shows the same thing over wall clock.

## Reading the numbers

`mon`'s own background cost is a few milliseconds per minute. If a CPU graph
shows something larger, it is one of:

* **dockerd** computing container stats — 4× less work per minute now;
* a **dashboard tab**, which polls `/api/overview` every 10s and eight history
  charts per minute — now paused while the tab is hidden;
* a **scrape loop** on `/metrics`, which rebuilds the whole exposition per
  request;
* the 12-hourly `smartctl` forks, which are spiky but average to nothing.

Measure before changing defaults again: `MON_DOCKER_INTERVAL=15s` restores the
old cadence, and the benchmarks above say what that costs.

After both store passes the traces read 0.03–0.04% of a core over 10- and
30-minute windows. What is left is kernel-side write/read work for the WAL and
the Docker socket, spread across every statement and request — there is no
single call left worth optimising.
