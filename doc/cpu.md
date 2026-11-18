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

| Benchmark | before | after |
|---|---|---|
| flush one minute of samples | 3.0 ms, 5 680 allocs | 2.2 ms, 1 612 allocs |
| the same samples batched as five minutes | 11.4 ms, 28 244 allocs | 7.5 ms, 7 858 allocs |
| prune over 219 k day rows | 5.5 ms | 1.1 ms |

Per hour of runtime, with day retention enabled:

| | before | after |
|---|---|---|
| flushes | 60 × 3.0 ms = 180 ms | 12 × 7.5 ms = 90 ms |
| prunes | 60 × 5.5 ms = 330 ms | 1 × 1.1 ms = 1.1 ms |
| total | **510 ms/h** | **91 ms/h** |

That is 0.014% of one core before and 0.003% after, on this CPU. A NAS core is
slower, but the order of magnitude does not change: the store is a rounding
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
