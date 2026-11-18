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

### Reading an `strace -c` of an idle process

`strace -c` reports **time spent in syscalls**, including waiting, so on an idle
daemon it is dominated by things that cost no CPU:

| syscall | share | what it is |
|---|---|---|
| `futex` | 80% | Go parking and unparking OS threads |
| `nanosleep` | 8% | runtime `sysmon`, which wakes on a fixed ~10 ms loop |
| `epoll_pwait` | 7% | the network poller waiting for the next request |
| `pwrite64`/`pread64`/`read`/`write` | 0.4% | SQLite WAL pages, Docker socket, `/proc` and `/sys` |

Only the last row is CPU-shaped, and that is the row the two store passes cut.
The first three say the process is *idle and healthy*: one `futex` per thread
park is how a Go program waits.

### Scheduler size

`strace` also shows 19 OS threads for a monitor using 0.04% of a core. Without
a cgroup CPU limit (this compose file sets none), Go 1.25+ sizes `GOMAXPROCS`
from the **host's** core count, so a container on a big NAS builds a runtime for
the whole machine. Measured on a 16-core box:

| `GOMAXPROCS` | OS threads after startup (3 runs) |
|---|---|
| 16 (default here) | 10, 10, 10 |
| 2 | 8, 8, 7 |
| 1 | 7, 7, 7 |

`compose.yaml` therefore pins `GOMAXPROCS=2`: enough for the HTTP handler while
a collector is blocked, small enough to keep GC workers and spinning scheduler
threads down. It is logged on startup (`gomaxprocs=2`), so a profile can always
confirm what took effect. The saving here is modest (10 → 8 threads); on a
bigger NAS with more cores it is larger, and the next `pprof`/`strace` decides
whether `1` is better still.



That is 0.014% of one core originally and 0.0008% now, on this CPU. A NAS core
is slower, but the order of magnitude does not change: the store is a rounding
error either way.

### Whole process, idle

`scripts/bench-cpu.sh idle 120` on a box with no dockerd and no hwmon measures
less than the 10 ms resolution of `wait4` rusage, i.e. **under 0.01% of a core**.

### The dashboard is the expensive part

`BenchmarkHistory24h` measures one charts-page request: 24 hours at minute
resolution for 25 series.

| | value |
|---|---|
| one /api/history request | **32 ms**, 10.4 MB allocated, 432 k allocs |
| charts page (8 requests/min) | **256 ms/min = 15 s/h** |

For comparison the whole background costs under 0.1 s/h. A visible charts tab
is therefore ~17x more expensive than every collector and flush combined --
which is why the tab pauses itself when hidden, and why any further CPU work
belongs here, not in the loops.

### Per-tick costs, measured

Benchmarks turn a profile into arithmetic. Frequencies are the defaults:

| Component | Benchmark | Per tick | Per hour |
|---|---|---|---|
| Docker read (25 containers) | `BenchmarkRead25Containers` | 0.86 ms | 52 ms (60×) |
| Store flush (120 series) | `BenchmarkFlushFiveMinutes` | 2.3 ms | 28 ms (12×) |
| Day prune (219 k rows) | `BenchmarkPruneDay` | 1.1 ms | 1.1 ms (1×) |
| Host read (4 `/proc` files) | `BenchmarkHostRead` | 0.06 ms | 7 ms (120×) |

Everything nameable adds up to under 0.1 s per hour. The rest of a 0.025%
profile is Go's idle machinery — `futex`, `sysmon`'s `nanosleep`,
`epoll_pwait` — which is what waiting looks like, not work that can be removed.

At that level a profile is also statistically thin: 600 s at 0.025% is 15
samples, so a node showing "40% of total" can be six samples. `BenchmarkHostRead`
exists for that reason — the `/proc` parser looked like 40% of one trace and
measured 0.8% of the real cost. Measure before refactoring a parser.

## Reproduce

```sh
go test ./internal/collect -run XXX -bench BenchmarkRead25Containers -benchtime 20x
go test ./internal/collect -run XXX -bench BenchmarkHostRead           -benchtime 200x
go test ./internal/store   -run XXX -bench .                           -benchtime 30x

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
