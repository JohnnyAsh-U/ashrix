#!/usr/bin/env bash
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"
: "${DIRECT_URL:=http://127.0.0.1:8081}"
: "${ASHRIX_URL:=http://grafana.ashrix.io:8080}"

export DIRECT_URL ASHRIX_URL

# Sequential execution prevents the two runs from competing for CPU/network resources.
"$ROOT/benchmarks/scripts/run.sh" direct
"$ROOT/benchmarks/scripts/run.sh" ashrix

echo "Both benchmark runs completed. Compare timestamped directories under benchmarks/results/."
