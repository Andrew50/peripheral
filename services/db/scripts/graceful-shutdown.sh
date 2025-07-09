#!/bin/bash
set -e

# Graceful PostgreSQL Shutdown Script
# This script is called by the Kubernetes preStop hook to prepare PostgreSQL for shutdown

LOG_PREFIX="[$(date '+%Y-%m-%d %H:%M:%S')] GRACEFUL_SHUTDOWN:"

log() {
    echo "$LOG_PREFIX $1"
}

error_log() {
    echo "$LOG_PREFIX ERROR: $1" >&2
}

# Telegram alert function (optional)
SEND_TELEGRAM() {
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
trap 'SEND_TELEGRAM "🚨 Graceful-shutdown failed in $DB_NAME on line $LINENO"' ERR

log "Starting graceful shutdown process..."

# Database credentials
DB_USER=${POSTGRES_USER:-postgres}
DB_NAME=${POSTGRES_DB:-postgres}

# Step 1: Prevent new non-superuser connections
log "Disabling new connections (ALTER DATABASE … CONNECTION LIMIT 0)…"
PGPASSWORD=$POSTGRES_PASSWORD psql -U "$DB_USER" -d "$DB_NAME" -c "ALTER DATABASE \"$DB_NAME\" CONNECTION LIMIT 0;" 2>/dev/null || { log "Could not disable new connections"; SEND_TELEGRAM "⚠️ Failed to disable new DB connections during graceful shutdown"; }

# Step 2: Wait for current transactions to complete (up to 90 seconds)
log "Waiting for active transactions to complete..."
for i in {1..90}; do
    ACTIVE_CONNECTIONS=$(PGPASSWORD=$POSTGRES_PASSWORD psql -U "$DB_USER" -d "$DB_NAME" -t -c "
        SELECT COUNT(*) FROM pg_stat_activity 
        WHERE state = 'active' 
        AND datname = '$DB_NAME' 
        AND pid <> pg_backend_pid()
    " 2>/dev/null | tr -d ' ' || echo "0")
    
    if [ "$ACTIVE_CONNECTIONS" -eq 0 ]; then
        log "All transactions completed"
        break
    fi
    
    log "Waiting for $ACTIVE_CONNECTIONS active transactions... ($i/90)"
    sleep 1
done

# After waiting loop check again
if [[ "$ACTIVE_CONNECTIONS" -gt 0 ]]; then
    SEND_TELEGRAM "⚠️ Graceful shutdown timed out – $ACTIVE_CONNECTIONS active connections still running after 90s";
fi

# Step 3: Terminate remaining idle connections
log "Terminating idle connections..."
PGPASSWORD=$POSTGRES_PASSWORD psql -U "$DB_USER" -d "$DB_NAME" -c "
    SELECT pg_terminate_backend(pid) 
    FROM pg_stat_activity 
    WHERE datname = '$DB_NAME' 
    AND pid <> pg_backend_pid()
    AND state IN ('idle', 'idle in transaction', 'idle in transaction (aborted)')
" 2>/dev/null || { log "Could not terminate idle connections"; SEND_TELEGRAM "⚠️ Failed to terminate idle connections during graceful shutdown"; }

# Step 4: Force checkpoint to ensure data is written to disk
log "Forcing checkpoint..."
PGPASSWORD=$POSTGRES_PASSWORD psql -U "$DB_USER" -d "$DB_NAME" -c "CHECKPOINT;" 2>/dev/null || { log "Could not force checkpoint"; SEND_TELEGRAM "⚠️ Failed to force checkpoint during graceful shutdown"; }

# Step 5: Wait additional time for any remaining cleanup
log "Waiting for cleanup processes..."
sleep 45

log "Graceful shutdown preparation completed. Ready for SIGTERM."
SEND_TELEGRAM "✅ Graceful shutdown preparation completed for $DB_NAME" || true 