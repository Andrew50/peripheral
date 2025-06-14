#!/bin/bash
set -euo pipefail

# Database Health Monitor Script
# Detects corruption, connection issues, and triggers automatic recovery

# Configuration
LOG_FILE="/backups/health-monitor.log"
HEALTH_CHECK_INTERVAL=60  # seconds
MAX_FAILURE_COUNT=3
RECOVERY_COOLDOWN=3600    # 1 hour between recovery attempts
CORRUPTION_INDICATORS=(
    "database system was interrupted while in recovery"
    "segmentation fault"
    "startup process.*was terminated by signal"
    "could not open file.*No such file or directory"
    "invalid page header"
    "checksum verification failed"
)

# Database credentials
DB_USER=${POSTGRES_USER:-postgres}
DB_NAME=${POSTGRES_DB:-postgres}
DB_HOST=${DB_HOST:-localhost}

# State tracking
FAILURE_COUNT=0
LAST_RECOVERY_TIME=0

# Logging function
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] HEALTH: $1" | tee -a "$LOG_FILE"
}

error_log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] HEALTH ERROR: $1" | tee -a "$LOG_FILE" >&2
}

# Check if database is accepting connections
check_connection() {
    if PGPASSWORD=$POSTGRES_PASSWORD pg_isready -U "$DB_USER" -d "$DB_NAME" -h "$DB_HOST" >/dev/null 2>&1; then
        return 0
    else
        return 1
    fi
}

# Check for corruption indicators in logs
check_for_corruption() {
    local recent_logs
    recent_logs=$(tail -n 100 "$LOG_FILE" 2>/dev/null || echo "")
    
    for indicator in "${CORRUPTION_INDICATORS[@]}"; do
        if echo "$recent_logs" | grep -q "$indicator"; then
            error_log "Corruption indicator detected: $indicator"
            return 1
        fi
    done
    
    return 0
}

# Test basic database functionality
test_database_functionality() {
    # Test simple query
    if ! PGPASSWORD=$POSTGRES_PASSWORD psql -U "$DB_USER" -d "$DB_NAME" -h "$DB_HOST" -c "SELECT 1;" >/dev/null 2>&1; then
        error_log "Basic database query failed"
        return 1
    fi
    
    # Test schema_versions table
    if ! PGPASSWORD=$POSTGRES_PASSWORD psql -U "$DB_USER" -d "$DB_NAME" -h "$DB_HOST" -c "SELECT COUNT(*) FROM schema_versions;" >/dev/null 2>&1; then
        error_log "Schema_versions table query failed"
        return 1
    fi
    
    # Test write capability
    if ! PGPASSWORD=$POSTGRES_PASSWORD psql -U "$DB_USER" -d "$DB_NAME" -h "$DB_HOST" -c "CREATE TEMP TABLE health_test (id INTEGER); DROP TABLE health_test;" >/dev/null 2>&1; then
        error_log "Database write test failed"
        return 1
    fi
    
    return 0
}

# Get the most recent valid backup
get_latest_backup() {
    local latest_backup
    latest_backup=$(find /backups -name "backup_*.sql.gz" -type f -exec stat -c '%Y %n' {} \; | sort -nr | head -1 | cut -d' ' -f2-)
    
    if [ -n "$latest_backup" ] && [ -f "$latest_backup" ]; then
        # Check backup size
        local backup_size=$(stat -c%s "$latest_backup")
        if [ "$backup_size" -gt 1000 ]; then
            echo "$latest_backup"
            return 0
        fi
    fi
    
    return 1
}

# Trigger automatic recovery
trigger_recovery() {
    local current_time=$(date +%s)
    
    # Check cooldown period
    if [ $((current_time - LAST_RECOVERY_TIME)) -lt $RECOVERY_COOLDOWN ]; then
        error_log "Recovery attempted too recently, waiting for cooldown period"
        return 1
    fi
    
    log "=== TRIGGERING AUTOMATIC RECOVERY ==="
    LAST_RECOVERY_TIME=$current_time
    
    # Try to get latest backup
    local latest_backup
    if latest_backup=$(get_latest_backup); then
        log "Found valid backup: $latest_backup"
        
        # Create recovery flag for external monitoring
        echo "$(date): Auto-recovery triggered with backup $latest_backup" > /backups/recovery-in-progress
        
        # Call the backup restore script
        if /app/recovery-restore.sh "$latest_backup"; then
            log "Automatic recovery completed successfully"
            rm -f /backups/recovery-in-progress
            FAILURE_COUNT=0
            return 0
        else
            error_log "Automatic recovery failed"
            return 1
        fi
    else
        error_log "No valid backup found for recovery"
        
        # Create recovery flag for external monitoring
        echo "$(date): Auto-recovery triggered but no valid backup found" > /backups/recovery-in-progress
        
        # Call the fresh reset script as last resort
        if /app/recovery-reset.sh; then
            log "Automatic fresh database reset completed"
            rm -f /backups/recovery-in-progress
            FAILURE_COUNT=0
            return 0
        else
            error_log "Automatic fresh reset failed"
            return 1
        fi
    fi
}

# Send Telegram alert
send_alert() {
    local alert_message="$1"
    local alert_type="${2:-WARNING}"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S UTC')
    
    log "ALERT: $alert_message"
    
    # Create alert file for local monitoring
    echo "$alert_message" > /backups/alert-$(date +%s)
    
    # Skip Telegram if credentials not configured
    if [ -z "$TELEGRAM_BOT_TOKEN" ] || [ -z "$TELEGRAM_CHAT_ID" ]; then
        log "Telegram credentials not configured, skipping notification"
        return 0
    fi
    
    # Determine emoji based on alert type
    local emoji="⚠️"
    case "$alert_type" in
        "CRITICAL") emoji="🚨" ;;
        "ERROR") emoji="❌" ;;
        "WARNING") emoji="⚠️" ;;
        "INFO") emoji="ℹ️" ;;
        "SUCCESS") emoji="✅" ;;
    esac
    
    # Format message for Telegram
    local telegram_message="$emoji *Database Alert - $alert_type*

*System:* PostgreSQL Backup & Recovery
*Time:* $timestamp
*Environment:* ${ENVIRONMENT:-Development}

*Message:*
$alert_message

*Health Status:*
• Database Connection: $(check_connection && echo "✅ Healthy" || echo "❌ Failed")
• Failure Count: $FAILURE_COUNT/$MAX_FAILURE_COUNT
• Last Recovery: $(date -d @$LAST_RECOVERY_TIME '+%Y-%m-%d %H:%M:%S' 2>/dev/null || echo "Never")

*Quick Actions:*
\`kubectl logs deployment/db-health-monitor --tail=20\`
\`kubectl get pods -l app=db\`"

    # Send to Telegram
    local response
    response=$(curl -s -X POST "https://api.telegram.org/bot$TELEGRAM_BOT_TOKEN/sendMessage" \
        -H "Content-Type: application/json" \
        -d "{
            \"chat_id\": \"$TELEGRAM_CHAT_ID\",
            \"text\": \"$telegram_message\",
            \"parse_mode\": \"Markdown\",
            \"disable_web_page_preview\": true
        }" 2>/dev/null)
    
    if echo "$response" | grep -q '"ok":true'; then
        log "Telegram alert sent successfully"
    else
        log "Failed to send Telegram alert: $response"
    fi
}

# Main health check function
perform_health_check() {
    log "Performing health check..."
    
    # Check database connection
    if ! check_connection; then
        error_log "Database connection failed"
        return 1
    fi
    
    # Check for corruption indicators
    if ! check_for_corruption; then
        error_log "Corruption indicators found"
        return 1
    fi
    
    # Test database functionality
    if ! test_database_functionality; then
        error_log "Database functionality test failed"
        return 1
    fi
    
    log "Health check passed"
    return 0
}

# Main monitoring loop
main() {
    log "=== Database Health Monitor Started ==="
    
    while true; do
        if perform_health_check; then
            # Reset failure count on success
            if [ $FAILURE_COUNT -gt 0 ]; then
                log "Health restored after $FAILURE_COUNT failures"
                FAILURE_COUNT=0
            fi
        else
            FAILURE_COUNT=$((FAILURE_COUNT + 1))
            error_log "Health check failed (failure count: $FAILURE_COUNT/$MAX_FAILURE_COUNT)"
            
            if [ $FAILURE_COUNT -ge $MAX_FAILURE_COUNT ]; then
                error_log "Maximum failure count reached, triggering recovery"
                send_alert "Database health check failed $MAX_FAILURE_COUNT times, triggering automatic recovery" "CRITICAL"
                
                if trigger_recovery; then
                    log "Recovery successful, resuming monitoring"
                    send_alert "Database recovery completed successfully! System is back online." "SUCCESS"
                    FAILURE_COUNT=0
                else
                    error_log "Recovery failed, continuing to monitor"
                    send_alert "Automatic database recovery failed, manual intervention required" "CRITICAL"
                    # Wait longer before next attempt
                    sleep $((HEALTH_CHECK_INTERVAL * 5))
                    continue
                fi
            fi
        fi
        
        sleep $HEALTH_CHECK_INTERVAL
    done
}

# Handle signals gracefully
trap 'log "Health monitor shutting down"; exit 0' SIGTERM SIGINT

# Run main function
main "$@" 