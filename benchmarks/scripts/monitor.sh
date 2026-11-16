#!/usr/bin/env bash
set -Eeuo pipefail

# Usage: monitor.sh OUTPUT_DIR GATEWAY_PID CONNECTOR_PID [SECONDS]
if [[ $# -lt 3 ]]; then
  echo "usage: $0 OUTPUT_DIR GATEWAY_PID CONNECTOR_PID [SECONDS]" >&2
  exit 2
fi
OUT="$1"
GATEWAY_PID="$2"
CONNECTOR_PID="$3"
SECONDS_TO_RUN="${4:-300}"
mkdir -p "$OUT"

for pid in "$GATEWAY_PID" "$CONNECTOR_PID"; do
  if ! kill -0 "$pid" 2>/dev/null; then
    echo "PID $pid is not running" >&2
    exit 1
  fi
done

if ! command -v pidstat >/dev/null 2>&1; then
  echo "pidstat missing; install with: sudo dnf install sysstat" >&2
  exit 1
fi

(
  timeout "$SECONDS_TO_RUN" pidstat -h -u -r -d -p "$GATEWAY_PID,$CONNECTOR_PID" 1
) > "$OUT/process-metrics.txt" || status=$?
if [[ "${status:-0}" -gt 1 && "${status:-0}" -ne 124 ]]; then
  exit "$status"
fi

ss -s > "$OUT/socket-summary.txt"
ss -u -a -n > "$OUT/udp-sockets.txt"
for pid in "$GATEWAY_PID" "$CONNECTOR_PID"; do
  printf '%s ' "$pid" >> "$OUT/file-descriptors.txt"
  find "/proc/$pid/fd" -maxdepth 1 -type l 2>/dev/null | wc -l >> "$OUT/file-descriptors.txt"
done
