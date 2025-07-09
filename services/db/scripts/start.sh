#!/bin/bash
# This script starts a temporary Postgres instance, runs migrations,
# stops the temporary instance, and then execs the final Postgres instance.
set -e

# Function to log messages with timestamps
log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] STARTUP: $1"
}

# Function to log errors
error_log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] STARTUP ERROR: $1" >&2
}

# Telegram alert helper (optional)
send_alert() {
  local MSG="$1"
  if [[ -z "${TELEGRAM_BOT_TOKEN:-}" || -z "${TELEGRAM_CHAT_ID:-}" ]]; then
    log "Telegram credentials not configured – skipping alert"
    return 0
  fi
  local PREFIX=""
  if [[ -n "${ENVIRONMENT:-}" ]]; then PREFIX="[$ENVIRONMENT] "; fi
  curl -s -X POST "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/sendMessage" \
    -d chat_id="${TELEGRAM_CHAT_ID}" \
    -d text="${PREFIX}${MSG}" \
    -d disable_web_page_preview=true >/dev/null 2>&1 || true
}

# Send alert on any unhandled error
trap 'send_alert "🚨 DB start-up script failed on line $LINENO"' ERR

# Ensure WAL archive directory exists inside PGDATA (matches archive_command path)
mkdir -p /home/postgres/pgdata/wal_archive
chown postgres:postgres /home/postgres/pgdata/wal_archive

# Determine the location of migrate.sh dynamically
MIGRATE_SCRIPT=""
if [ -f "/app/migrate.sh" ]; then
  MIGRATE_SCRIPT="/app/migrate.sh"
elif [ -f "/usr/local/bin/migrate.sh" ]; then
  MIGRATE_SCRIPT="/usr/local/bin/migrate.sh"
fi

if [ -z "$MIGRATE_SCRIPT" ]; then
  error_log "migrate.sh not found in expected locations (/app/migrate.sh or /usr/local/bin/migrate.sh)"
  exit 1
fi
log "Using migration script: $MIGRATE_SCRIPT"

# === Temporary Postgres Start for Migrations ===
log "Starting temporary PostgreSQL instance for migrations..."
# We use the official entrypoint to ensure the data directory is initialized
# correctly on the very first run if needed.
# Start it in the background.
docker-entrypoint.sh postgres -c config_file=/etc/postgresql/postgresql.conf &
PG_PID=$!
log "Temporary PostgreSQL PID: $PG_PID"

# Wait for the temporary PostgreSQL instance to start
# Use pg_isready which is designed for this. Ensure PGHOST, PGUSER etc. are set
# appropriately if defaults (localhost, current user) aren't sufficient.
# The official postgres images often set PGUSER=postgres.
log "Waiting for temporary PostgreSQL instance to become available..."
WAIT_TIMEOUT=${WAIT_TIMEOUT:-600} # Maximum wait time in seconds; overridable via WAIT_TIMEOUT env var
WAIT_INTERVAL=1 # Check interval
SECONDS=0 # Start timer

# Loop until pg_isready succeeds or timeout occurs
until pg_isready -U postgres -h localhost -q; do
  if ! kill -0 "$PG_PID" 2>/dev/null; then
    error_log "Temporary PostgreSQL process (PID: $PG_PID) exited unexpectedly during startup wait."
    # Optional: attempt to show recent postgres logs if possible/needed for debugging
    exit 1 # Exit if the process died
  fi
  if [ "$SECONDS" -ge "$WAIT_TIMEOUT" ]; then
    error_log "Timed out waiting for temporary PostgreSQL instance to start after ${WAIT_TIMEOUT} seconds."
    # Attempt to gracefully stop the potentially stuck process before exiting
    log "Attempting to stop potentially stuck temporary PostgreSQL (PID: $PG_PID)..."
    kill -TERM "$PG_PID" || true
    sleep 2
    kill -KILL "$PG_PID" || true
    exit 1
  fi
  log "Temporary PostgreSQL is unavailable - sleeping ${WAIT_INTERVAL}s (elapsed: ${SECONDS}s)"
  sleep "$WAIT_INTERVAL"
done
log "Temporary PostgreSQL is up and running (PID: $PG_PID)"

# Run migrations
log "Running migrations..."
# Ensure the migrate script has necessary DB connection info (e.g., via env vars PGDATABASE, PGUSER, PGPASSWORD, PGHOST)
if "$MIGRATE_SCRIPT" postgres; then
  log "Migrations completed successfully."
else
  # If migrations fail, log the error, stop the temp instance, and exit non-zero
  error_log "Migrations failed. Stopping temporary instance and exiting."
  send_alert "❌ Database migrations failed during startup – container will exit"
  kill -TERM "$PG_PID" || true # Send SIGTERM
  # Wait a bit for graceful shutdown before exiting container
  wait "$PG_PID" || true
  exit 1
fi


# Stop the temporary PostgreSQL instance cleanly
log "Stopping temporary PostgreSQL instance (PID: $PG_PID)..."
# Send SIGTERM for graceful shutdown
kill -TERM "$PG_PID"
# Wait for the process to terminate
wait "$PG_PID"
log "Temporary PostgreSQL instance stopped."

# === Final Postgres Start using exec ===
log "Starting final PostgreSQL instance with exec (making it PID 1)..."
# Use exec to replace the current script process with the postgres process.
# The official entrypoint script will itself use 'exec postgres ...' at its end.
# This ensures signals sent to the container (like SIGTERM from 'docker stop')
# go directly to the postgres process run by the entrypoint.
# We wrap the postgres command with our log capture script to ensure logs are available for health monitoring
exec /app/capture-logs.sh docker-entrypoint.sh postgres -c config_file=/etc/postgresql/postgresql.conf

# Note: Anything after 'exec' will not run unless 'exec' fails.
error_log "Exec failed! Could not start final PostgreSQL instance."
send_alert "❌ Exec failed – database container could not start final Postgres instance"
exit 1