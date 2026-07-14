#!/bin/sh
set -e

# sr-email entrypoint: starts the API server in the background
# and the cron worker on a loop. To break them into separate
# containers later, change the Docker command to /go/bin/api
# or /go/bin/cron respectively — no code changes needed.

echo "[sr-email] starting API server"
/go/bin/api &

# Determine cron interval from env, default 30s.
INTERVAL="${CRON_INTERVAL:-30s}"
echo "[sr-email] starting cron worker [interval=${INTERVAL}]"

while true; do
    /go/bin/cron
    sleep "${INTERVAL}"
done