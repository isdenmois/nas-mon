#!/usr/bin/env bash
# Measures mon's own CPU time for a fixed window and a given load scenario.
#
#   scripts/bench-cpu.sh [idle|dashboard|scrape] [seconds]
#
# Numbers come from wait4 rusage of the child process, so they are exact even
# inside a container where /proc/<pid>/stat may be unreliable. The default
# window is 5 minutes: rusage is reported with 10ms resolution, so shorter
# windows can round a small result to zero.
set -euo pipefail

SCENARIO=${1:-idle}
DURATION=${2:-300}
BIN=${BIN:-./bin/mon}
PORT=${PORT:-18090}

if [[ ! -x $BIN ]]; then
  echo "missing $BIN; run 'just build' first" >&2
  exit 1
fi

WORK=$(mktemp -d)
DB=$WORK/mon.db
LOG=$WORK/mon.log
TIME_LOG=$WORK/time

load_pid=
cleanup() {
  if [[ -n $load_pid ]]; then
    kill "$load_pid" 2>/dev/null || true
    wait "$load_pid" 2>/dev/null || true
  fi
  rm -rf "$WORK"
}
trap cleanup EXIT

base=http://127.0.0.1:$PORT
case $SCENARIO in
idle) ;;
dashboard)
  # One tab: overview every 10s plus the eight history charts every minute.
  (
    end=$((SECONDS + DURATION))
    while ((SECONDS < end)); do
      curl -s -o /dev/null "$base/api/overview" || true
      for ids in 1,2,3 4,5 6,7,8 1,2,3,4,5,6,7,8; do
        curl -s -o /dev/null "$base/api/history?ids=$ids&range=24h" || true
      done
      sleep 10
    done
  ) &
  load_pid=$!
  ;;
scrape)
  (
    end=$((SECONDS + DURATION))
    while ((SECONDS < end)); do
      curl -s -o /dev/null "$base/metrics" || true
      sleep 15
    done
  ) &
  load_pid=$!
  ;;
*)
  echo "unknown scenario: $SCENARIO (want idle, dashboard or scrape)" >&2
  exit 2
  ;;
esac

MON_DB_PATH=$DB MON_PORT=$PORT MON_SMART_INTERVAL=0 MON_VOLUMES_DIR=$WORK/volumes \
  /usr/bin/time -f 'user=%U sys=%S maxrss=%M' -o "$TIME_LOG" \
  timeout --signal=TERM $((DURATION + 3)) "$BIN" >"$LOG" 2>&1 || true

kill "$load_pid" 2>/dev/null || true
wait "$load_pid" 2>/dev/null || true
load_pid=

read -r user sys rss <<<"$(grep -o "user=[0-9.]* sys=[0-9.]* maxrss=[0-9]*" "$TIME_LOG" | sed -e "s/user=//" -e "s/sys=//" -e "s/maxrss=//")"
awk -v u="$user" -v s="$sys" -v d="$DURATION" -v rss="$rss" -v sc="$SCENARIO" 'BEGIN {
  cpu = u + s
  printf "%-10s cpu=%.2fs over %ss (%.4f%%) peak_rss=%sKB\n", sc, cpu, d, cpu / d * 100, rss
}'
