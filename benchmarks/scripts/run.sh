#!/usr/bin/env bash
set -Eeuo pipefail
set -x


# Run the same workload against the direct application and the Ashrix gateway.
# Usage: ./benchmarks/scripts/run.sh direct|ashrix

MODE="${1:-}"
if [[ "$MODE" != "direct" && "$MODE" != "ashrix" ]]; then
  echo "usage: $0 direct|ashrix" >&2
  exit 2
fi

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

: "${DURATION:=20s}"
: "${WARMUP:=5s}"
: "${REPEATS:=2}"
: "${WORKERS:=100}"

if [[ "$MODE" == "direct" ]]; then
  BASE_URL="${DIRECT_URL:-http://127.0.0.1:8081}"
else
  BASE_URL="${ASHRIX_URL:-http://grafana.ashrix.io:8080}"
fi

if ! command -v vegeta >/dev/null 2>&1; then
  echo "vegeta is required: go install github.com/tsenart/vegeta@latest" >&2
  exit 1
fi

STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
OUT="benchmarks/results/${STAMP}-${MODE}"
mkdir -p "$OUT" benchmarks/payloads

for size in 1024 16384 262144 1048576 10485760; do
  file="benchmarks/payloads/upload-${size}.bin"
  if [[ ! -f "$file" || "$(wc -c < "$file")" -ne "$size" ]]; then
    head -c "$size" /dev/zero > "$file"
  fi
done

{
  echo "timestamp_utc=$STAMP"
  echo "mode=$MODE"
  echo "base_url=$BASE_URL"
  echo "duration=$DURATION"
  echo "warmup=$WARMUP"
  echo "repeats=$REPEATS"
  echo "workers=$WORKERS"
  echo "go_version=$(go version 2>/dev/null || true)"
  echo "kernel=$(uname -srmo)"
  echo "cpu=$(lscpu 2>/dev/null | awk -F: '/Model name/ {gsub(/^ +/,"",$2); print $2; exit}' || true)"
  echo "memory=$(awk '/MemTotal/ {print $2 " kB"}' /proc/meminfo 2>/dev/null || true)"
  echo "vegeta=$(vegeta -version 2>&1 | head -1)"
  echo "git_commit=$(git rev-parse HEAD 2>/dev/null || echo unknown)"
} > "$OUT/environment.txt"
# echo "Running"

run_case() {
  local label="$1" method="$2" path="$3" rate="$4" body="${5:-}"
  local target="$method $BASE_URL$path"
  local i
  for ((i=1; i<=REPEATS; i++)); do
    local raw="$OUT/${label}-r${i}.bin"
    local args=(attack -rate="$rate" -workers="$WORKERS" -timeout=60s)
    if [[ -n "$body" ]]; then args+=(-body="$body" -header="Content-Type: application/octet-stream"); fi

    echo "Running"
    # Warm-up is deliberately discarded and kept separate from measured samples.
    echo  "$target" | vegeta "${args[@]}" -duration="$WARMUP" > /dev/null
    echo "$target" | vegeta "${args[@]}" -duration="$DURATION" | tee "$raw" >/dev/null
    vegeta report -type=text "$raw" > "$OUT/${label}-r${i}.txt"
    vegeta report -type=json "$raw" > "$OUT/${label}-r${i}.json"
    echo "Running"

  done
}

# Fixed offered rates by size to avoid overwhelming the host with 10 MiB requests.
# Each rate is used identically in direct and Ashrix modes.
declare -A RATE=( [1024]=50 [16384]=50 [262144]=20 [1048576]=5 [10485760]=1 )

run_case health GET /health 50
for size in 1024 16384 262144 1048576 10485760; do
  run_case "download-${size}" GET "/payload?size=${size}" "${RATE[$size]}"
  run_case "upload-${size}" POST /echo "${RATE[$size]}" "benchmarks/payloads/upload-${size}.bin"
done

# Capacity curve: increasing offered request rate on the small health endpoint.
for rate in 100 500 1000 1500 2000 2500 3000; do
  run_case "capacity-${rate}rps" GET /health "$rate"
done

# Machine-readable index and compact aggregate report.
{
  echo "# ${MODE} benchmark results (${STAMP})"
  echo
  echo "Base URL: ${BASE_URL}  "
  echo "Duration: ${DURATION}; warm-up: ${WARMUP}; repetitions: ${REPEATS}"
  echo
  for f in "$OUT"/*.txt; do
    echo "## $(basename "$f" .txt)"
    cat "$f"
    echo
  done
} > "$OUT/summary.md"

echo "Completed: $OUT"
echo "Review summary: $OUT/summary.md"
