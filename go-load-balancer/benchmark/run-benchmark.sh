#!/bin/sh
# Fires sustained HTTP load at the load balancer and prints a vegeta
# report (throughput, success ratio, latency percentiles).
#
# Usage: run-benchmark.sh [RATE] [DURATION] [TARGET_URL]
set -eu

RATE="${1:-200}"
DURATION="${2:-30s}"
TARGET="${3:-http://loadbalancer:8080/}"

echo "attacking ${TARGET} at ${RATE} req/s for ${DURATION}"
echo "GET ${TARGET}" | vegeta attack -rate="${RATE}" -duration="${DURATION}" \
  | tee /tmp/results.bin \
  | vegeta report

vegeta report -type=hist[0,10ms,50ms,100ms,200ms,500ms,1s] /tmp/results.bin
