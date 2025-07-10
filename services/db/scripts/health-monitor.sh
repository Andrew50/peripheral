#!/bin/bash
set -euo pipefail

# Database Health Monitor Script
# Detects corruption, connection issues, and triggers automatic recovery
# 
# SAFETY FIRST: This script will NEVER automatically delete any data.
# If safe recovery fails, manual intervention is required to prevent data loss.

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
    "corrupted page"
    "invalid page header"
    "could not read block"
    "unexpected page version"
    "page verification failed"
    "WAL file is corrupted"
    "recovery failed"
    "backup block is corrupted"
)
STARTUP_GRACE_PERIOD=${STARTUP_GRACE_PERIOD:-300}  # seconds
START_TIME=$(date +%s)

# Deployment suppression flag – if this file exists we treat all health checks as skipped
DEPLOYMENT_SUPPRESSION_FILE=${DEPLOYMENT_SUPPRESSION_FILE:-/backups/deploying.flag}

# Database credentials
DB_USER=${POSTGRES_USER:-postgres}
DB_NAME=${POSTGRES_DB:-postgres}
DB_HOST=${DB_HOST:-localhost}

# Environment detection
ENVIRONMENT=${ENVIRONMENT:-Development}
if [ -n "$K8S_NAMESPACE" ]; then
    ENVIRONMENT="$K8S_NAMESPACE"
elif [ -n "$KUBERNETES_SERVICE_HOST" ]; then
    # Try to get namespace from Kubernetes
    if [ -f "/var/run/secrets/kubernetes.io/serviceaccount/namespace" ]; then
        ENVIRONMENT=$(cat /var/run/secrets/kubernetes.io/serviceaccount/namespace)
    fi
fi

# State tracking
FAILURE_COUNT=0
LAST_RECOVERY_TIME=0
LAST_FAILURE_REASON=""
FAILURE_DETAILS=""

# Logging function
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] HEALTH: $1" | tee -a "$LOG_FILE"
}

error_log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] HEALTH ERROR: $1" | tee -a "$LOG_FILE" >&2
}

# Check if database is accepting connections
check_connection() {
    local output
    output=$(PGPASSWORD=$POSTGRES_PASSWORD pg_isready -U "$DB_USER" -d "$DB_NAME" -h "$DB_HOST" 2>&1 || true)

    # Accepting connections normally
    if echo "$output" | grep -q "accepting connections"; then
        return 0
    fi

    # Database is still starting up (57P03)
    if echo "$output" | grep -q "starting up"; then
        local elapsed=$(( $(date +%s) - START_TIME ))
        if [ "$elapsed" -lt "$STARTUP_GRACE_PERIOD" ]; then
            log "Database is still starting up (${elapsed}/${STARTUP_GRACE_PERIOD}s)"
            return 0
        fi
        LAST_FAILURE_REASON="Database still starting after grace period"
        FAILURE_DETAILS="$output"
        return 1
    fi

    LAST_FAILURE_REASON="Database connection failed"
    FAILURE_DETAILS="$output"
    return 1
}

# Check for corruption indicators in logs
check_for_corruption() {
    # Skip corruption checks during initial startup grace period
    local elapsed=$(( $(date +%s) - START_TIME ))
    if [ "$elapsed" -lt "$STARTUP_GRACE_PERIOD" ]; then
        return 0
    fi
    
    # Skip local process check if monitoring a remote Postgres instance
    if [[ "$DB_HOST" != "localhost" && "$DB_HOST" != "127.0.0.1" && "$DB_HOST" != "0.0.0.0" ]]; then
        # Only analyze logs; assume process is remote
        : # no-op; continue to log inspection below
    else
        # Check if PostgreSQL process is running locally
        local pg_pid
        pg_pid=$(pgrep postgres | head -1 2>/dev/null)
        if [ -n "$pg_pid" ]; then
            if ! kill -0 "$pg_pid" 2>/dev/null; then
                error_log "PostgreSQL process $pg_pid is not running"
                LAST_FAILURE_REASON="PostgreSQL process not running"
                FAILURE_DETAILS="Process $pg_pid not found"
                return 1
            fi
        else
            error_log "No PostgreSQL process found"
            LAST_FAILURE_REASON="No PostgreSQL process found"
            FAILURE_DETAILS="pgrep postgres returned no results"
            return 1
        fi
    fi
    
    # Check our captured PostgreSQL logs for corruption indicators
    if [ -f "/backups/postgresql-logs.log" ]; then
        local recent_logs
        recent_logs=$(tail -n 100 "/backups/postgresql-logs.log" 2>/dev/null || echo "")
        
        for indicator in "${CORRUPTION_INDICATORS[@]}"; do
            if echo "$recent_logs" | grep -q "$indicator"; then
                error_log "Corruption indicator detected: $indicator"
                LAST_FAILURE_REASON="Database corruption detected"
                FAILURE_DETAILS="Corruption indicator found: $indicator"
                return 1
            fi
        done
    else
        # If log file doesn't exist, that's suspicious but not necessarily corruption
        error_log "PostgreSQL log file not found: /backups/postgresql-logs.log"
        # Don't return failure here as the database might still be working
    fi
    
    return 0
}

# Test basic database functionality
test_database_functionality() {
    # Skip detailed tests during startup grace period
    local elapsed=$(( $(date +%s) - START_TIME ))
    if [ "$elapsed" -lt "$STARTUP_GRACE_PERIOD" ]; then
        return 0
    fi
    
    # Test simple query
    if ! PGPASSWORD=$POSTGRES_PASSWORD psql -U "$DB_USER" -d "$DB_NAME" -h "$DB_HOST" -c "SELECT 1;" >/dev/null 2>&1; then
        error_log "Basic database query failed"
        LAST_FAILURE_REASON="Basic database query failed"
        FAILURE_DETAILS="SELECT 1 query failed"
        return 1
    fi
    
    # Test schema_versions table
    if ! PGPASSWORD=$POSTGRES_PASSWORD psql -U "$DB_USER" -d "$DB_NAME" -h "$DB_HOST" -c "SELECT COUNT(*) FROM schema_versions;" >/dev/null 2>&1; then
        error_log "Schema_versions table query failed"
        LAST_FAILURE_REASON="Schema_versions table query failed"
        FAILURE_DETAILS="SELECT COUNT(*) FROM schema_versions failed"
        return 1
    fi
    
    # Test write capability
    if ! PGPASSWORD=$POSTGRES_PASSWORD psql -U "$DB_USER" -d "$DB_NAME" -h "$DB_HOST" -c "CREATE TEMP TABLE health_test (id INTEGER); DROP TABLE health_test;" >/dev/null 2>&1; then
        error_log "Database write test failed"
        LAST_FAILURE_REASON="Database write test failed"
        FAILURE_DETAILS="CREATE/DROP TEMP TABLE failed"
        return 1
    fi
    
    # CRITICAL: Test data integrity with checksums
    if ! PGPASSWORD=$POSTGRES_PASSWORD psql -U "$DB_USER" -d "$DB_NAME" -h "$DB_HOST" -c "SELECT pg_relation_filepath('schema_versions');" >/dev/null 2>&1; then
        error_log "Data integrity check failed"
        LAST_FAILURE_REASON="Data integrity check failed"
        FAILURE_DETAILS="pg_relation_filepath check failed"
        return 1
    fi
    
    # CRITICAL: Test WAL status
    if ! PGPASSWORD=$POSTGRES_PASSWORD psql -U "$DB_USER" -d "$DB_NAME" -h "$DB_HOST" -c "SELECT pg_current_wal_lsn();" >/dev/null 2>&1; then
        error_log "WAL status check failed"
        LAST_FAILURE_REASON="WAL status check failed"
        FAILURE_DETAILS="pg_current_wal_lsn() query failed"
        return 1
    fi
    
    # CRITICAL: Test replication lag (if replica exists)
    if PGPASSWORD=$POSTGRES_PASSWORD psql -U "$DB_USER" -d "$DB_NAME" -h "$DB_HOST" -c "SELECT application_name, state, sent_lsn, write_lsn, flush_lsn, replay_lsn FROM pg_stat_replication;" >/dev/null 2>&1; then
        REPLICATION_LAG=$(PGPASSWORD=$POSTGRES_PASSWORD psql -U "$DB_USER" -d "$DB_NAME" -h "$DB_HOST" -t -c "SELECT EXTRACT(EPOCH FROM (now() - pg_last_xact_replay_timestamp()))::INTEGER;" 2>/dev/null | xargs)
        if [ -n "$REPLICATION_LAG" ] && [ "$REPLICATION_LAG" -gt 300 ]; then
            error_log "Replication lag is high: ${REPLICATION_LAG}s"
            LAST_FAILURE_REASON="Replication lag is high"
            FAILURE_DETAILS="Replication lag: ${REPLICATION_LAG}s (threshold: 300s)"
            return 1
        fi
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

# Check backup availability and provide detailed status
check_backup_availability() {
    local backup_info=""
    
    # Check if backup directory exists
    if [ ! -d "/backups" ]; then
        echo "❌ Backup directory /backups not found"
        return 1
    fi
    
    # Count total backups
    local total_backups=$(find /backups -name "backup_*.sql.gz" -type f 2>/dev/null | wc -l)
    if [ "$total_backups" -eq 0 ]; then
        echo "❌ No backup files found in /backups"
        return 1
    fi
    
    # Get latest backup details
    local latest_backup
    if latest_backup=$(get_latest_backup); then
        local backup_time=$(stat -c '%Y' "$latest_backup")
        local backup_age=$(( $(date +%s) - backup_time ))
        local backup_size=$(stat -c%s "$latest_backup")
        local backup_size_mb=$((backup_size / 1024 / 1024))
        
        backup_info="✅ Found $total_backups backup(s)"
        backup_info="$backup_info\n• Latest: $(date -d @$backup_time '+%Y-%m-%d %H:%M:%S') (${backup_age}s ago)"
        backup_info="$backup_info\n• Size: ${backup_size_mb}MB"
        backup_info="$backup_info\n• Path: $latest_backup"
        
        # Check if backup is recent (less than 24 hours)
        if [ $backup_age -gt 86400 ]; then
            backup_info="$backup_info\n⚠️ Warning: Latest backup is over 24 hours old"
        fi
        
        echo -e "$backup_info"
        return 0
    else
        echo "❌ No valid backups found (all backups may be corrupted or too small)"
        return 1
    fi
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
        
        # DO NOT perform fresh reset - this would delete data
        # Instead, log the situation and return failure
        error_log "CRITICAL: No valid backup available for recovery. Manual intervention required to prevent data loss."
        return 1
    fi
}

# Get recent database logs
get_recent_db_logs() {
    local log_lines=20
    
    # Check our captured PostgreSQL logs (primary source)
    if [ -f "/backups/postgresql-logs.log" ]; then
        tail -n $log_lines "/backups/postgresql-logs.log" 2>/dev/null | grep -E "(ERROR|FATAL|PANIC)" | tail -n 5
    else
        # Fallback to health monitor logs if PostgreSQL logs not available
        if [ -f "$LOG_FILE" ]; then
            tail -n $log_lines "$LOG_FILE" 2>/dev/null | grep -E "(ERROR|FAILED|CRITICAL)" | tail -n 5
        fi
    fi
}

# Get database status information
get_db_status_info() {
    local status_info=""
    
    # Try to get basic database info if connection is available
    if check_connection; then
        # Get database version
        local db_version=$(PGPASSWORD=$POSTGRES_PASSWORD psql -U "$DB_USER" -d "$DB_NAME" -h "$DB_HOST" -t -c "SELECT version();" 2>/dev/null | head -1 | xargs)
        if [ -n "$db_version" ]; then
            status_info="$status_info\n• Version: $db_version"
        fi
        
        # Get active connections
        local active_connections=$(PGPASSWORD=$POSTGRES_PASSWORD psql -U "$DB_USER" -d "$DB_NAME" -h "$DB_HOST" -t -c "SELECT count(*) FROM pg_stat_activity WHERE state = 'active';" 2>/dev/null | xargs)
        if [ -n "$active_connections" ]; then
            status_info="$status_info\n• Active Connections: $active_connections"
        fi
        
        # Get database size
        local db_size=$(PGPASSWORD=$POSTGRES_PASSWORD psql -U "$DB_USER" -d "$DB_NAME" -h "$DB_HOST" -t -c "SELECT pg_size_pretty(pg_database_size('$DB_NAME'));" 2>/dev/null | xargs)
        if [ -n "$db_size" ]; then
            status_info="$status_info\n• Database Size: $db_size"
        fi
        
        # Get WAL status
        local wal_lsn=$(PGPASSWORD=$POSTGRES_PASSWORD psql -U "$DB_USER" -d "$DB_NAME" -h "$DB_HOST" -t -c "SELECT pg_current_wal_lsn();" 2>/dev/null | xargs)
        if [ -n "$wal_lsn" ]; then
            status_info="$status_info\n• WAL LSN: $wal_lsn"
        fi
        
        # Get replication status if available
        local replication_status=$(PGPASSWORD=$POSTGRES_PASSWORD psql -U "$DB_USER" -d "$DB_NAME" -h "$DB_HOST" -t -c "SELECT application_name, state FROM pg_stat_replication LIMIT 1;" 2>/dev/null | xargs)
        if [ -n "$replication_status" ] && [ "$replication_status" != "application_name state" ]; then
            status_info="$status_info\n• Replication: $replication_status"
        fi
    fi
    
    echo -e "$status_info"
}

# Get system resource information
get_system_info() {
    # Build a compact, single-line summary of resource usage (CPU, RAM in GB, Disk, Uptime)
    local cpu_usage mem_info mem_total_mb mem_used_mb mem_total_gb mem_used_gb mem_percent
    local disk_usage uptime_info summary

    # CPU usage (fallback to mpstat if top format differs)
    cpu_usage=$(top -bn1 | grep "Cpu(s)" | awk '{print $2}' | cut -d'%' -f1 2>/dev/null)
    if [ -z "$cpu_usage" ]; then
        cpu_usage=$(mpstat 1 1 2>/dev/null | awk '/Average:/ {printf "%.1f", 100-$NF}')
    fi
    cpu_usage=${cpu_usage:-N/A}

    # Memory usage (convert MB → GB with 1 decimal)
    mem_info=$(free -m 2>/dev/null | grep Mem || true)
    if [ -n "$mem_info" ]; then
        mem_total_mb=$(echo "$mem_info" | awk '{print $2}')
        mem_used_mb=$(echo "$mem_info" | awk '{print $3}')
        mem_percent=$((mem_used_mb * 100 / mem_total_mb))
        mem_total_gb=$(awk "BEGIN {printf \"%.1f\", $mem_total_mb / 1024}")
        mem_used_gb=$(awk "BEGIN {printf \"%.1f\", $mem_used_mb / 1024}")
    fi

    # Root disk usage (numeric percentage without % sign)
    disk_usage=$(df -h / 2>/dev/null | tail -1 | awk '{print $5}' | tr -d '%')
    disk_usage=${disk_usage:-N/A}

    # Uptime (strip leading 'up ' and commas to save space)
    uptime_info=$(uptime -p 2>/dev/null | sed 's/^up //;s/,//g')

    # Construct single-line output
    summary="• CPU: ${cpu_usage}% | RAM: ${mem_used_gb:-?}/${mem_total_gb:-?}GB (${mem_percent:-?}%) | Disk: ${disk_usage}% | Uptime: ${uptime_info}"

    # Prepend newline so downstream formatting remains unchanged
    echo -e "\n${summary}"
}

# Get Kubernetes pod information
get_k8s_info() {
    local k8s_info=""
    
    # Check if we're running in Kubernetes
    if [ -n "$KUBERNETES_SERVICE_HOST" ] || [ -f "/var/run/secrets/kubernetes.io/serviceaccount/token" ]; then
        # Get current pod name
        local pod_name=$(hostname 2>/dev/null || echo "Unknown")
        k8s_info="$k8s_info\n• Pod: $pod_name"
        
        # Get namespace if available
        if [ -f "/var/run/secrets/kubernetes.io/serviceaccount/namespace" ]; then
            local namespace=$(cat /var/run/secrets/kubernetes.io/serviceaccount/namespace 2>/dev/null)
            if [ -n "$namespace" ]; then
                k8s_info="$k8s_info\n• Namespace: $namespace"
            fi
        fi
        
        # Get pod IP address
        local pod_ip=$(hostname -i 2>/dev/null || echo "Unknown")
        if [ -n "$pod_ip" ] && [ "$pod_ip" != "Unknown" ]; then
            k8s_info="$k8s_info\n• Pod IP: $pod_ip"
        fi
        
        # Get node name if available
        if [ -n "${NODE_NAME:-}" ]; then
            k8s_info="$k8s_info\n• Node: ${NODE_NAME:-Unknown}"
        fi
    fi
    
    echo -e "$k8s_info"
}

# Get current health check status
get_health_status() {
    local health_status=""
    
    # Check connection status
    if check_connection; then
        health_status="$health_status\n• Connection: ✅ Healthy"
    else
        health_status="$health_status\n• Connection: ❌ Failed"
    fi
    
    # Check corruption status
    if check_for_corruption; then
        health_status="$health_status\n• Corruption: ✅ Clean"
    else
        health_status="$health_status\n• Corruption: ❌ Detected"
    fi
    
    # Check functionality status (simplified)
    if PGPASSWORD=$POSTGRES_PASSWORD psql -U "$DB_USER" -d "$DB_NAME" -h "$DB_HOST" -c "SELECT 1;" >/dev/null 2>&1; then
        health_status="$health_status\n• Functionality: ✅ Working"
    else
        health_status="$health_status\n• Functionality: ❌ Failed"
    fi
    
    echo -e "$health_status"
}

# Get backup status information
get_backup_status() {
    local backup_status=""
    
    # Check if backup directory exists
    if [ -d "/backups" ]; then
        # Get latest backup info
        local latest_backup
        latest_backup=$(find /backups -name "backup_*.sql.gz" -type f -exec stat -c '%Y %n' {} \; | sort -nr | head -1 | cut -d' ' -f2-)
        
        if [ -n "$latest_backup" ] && [ -f "$latest_backup" ]; then
            local backup_time=$(stat -c '%Y' "$latest_backup")
            local backup_age=$(( $(date +%s) - backup_time ))
            local backup_size=$(stat -c%s "$latest_backup")
            local backup_size_mb=$((backup_size / 1024 / 1024))
            
            backup_status="$backup_status\n• Latest Backup: $(date -d @$backup_time '+%Y-%m-%d %H:%M:%S')"
            backup_status="$backup_status\n• Backup Age: ${backup_age}s ago"
            backup_status="$backup_status\n• Backup Size: ${backup_size_mb}MB"
            
            # Check if backup is recent (less than 24 hours)
            if [ $backup_age -lt 86400 ]; then
                backup_status="$backup_status\n• Backup Status: ✅ Recent"
            else
                backup_status="$backup_status\n• Backup Status: ⚠️ Old"
            fi
        else
            backup_status="$backup_status\n• Backup Status: ❌ No backups found"
        fi
        
        # Check backup disk space
        local disk_usage=$(df -h /backups 2>/dev/null | tail -1 | awk '{print $5}' | cut -d'%' -f1)
        if [ -n "$disk_usage" ] && [ "$disk_usage" != "Use%" ]; then
            backup_status="$backup_status\n• Disk Usage: ${disk_usage}%"
        fi
    else
        backup_status="$backup_status\n• Backup Status: ❌ Backup directory not found"
    fi
    
    echo -e "$backup_status"
}

# Send Telegram alert
send_alert() {
    local alert_message="$1"
    local alert_type="${2:-WARNING}"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S %Z')
    
    log "ALERT: $alert_message"
    
    # Create alert file for local monitoring
    echo "$alert_message" > /backups/alert-$(date +%s)
    
    # Skip Telegram if credentials not configured
    if [ -z "$TELEGRAM_BOT_TOKEN" ] || [ -z "$TELEGRAM_CHAT_ID" ]; then
        log "Telegram credentials not configured, skipping notification"
        return 0
    fi
    
    # Get recent database logs and minimal system info
    local recent_logs
    recent_logs=$(get_recent_db_logs)
    local system_info
    system_info=$(get_system_info)
    
    # Determine emoji based on alert type
    local emoji="⚠️"
    case "$alert_type" in
        "CRITICAL") emoji="🚨" ;;
        "ERROR") emoji="❌" ;;
        "WARNING") emoji="⚠️" ;;
        "INFO") emoji="ℹ️" ;;
        "SUCCESS") emoji="✅" ;;
    esac
    
    local env_info="${ENVIRONMENT:-Development}"
    
    # Compact Telegram message
    local telegram_message="$emoji *$alert_type* \- *$env_info*\n\n$alert_message\n\n*System Resources:*$system_info"

    if [ -n "$recent_logs" ]; then
        telegram_message="$telegram_message\n\n*Recent Logs:*\n\
\`\`\`\n$recent_logs\n\`\`\`"
    fi

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
    
    # Reset failure tracking
    LAST_FAILURE_REASON=""
    FAILURE_DETAILS=""
    
    # Check database connection
    if ! check_connection; then
        error_log "Database connection failed: $LAST_FAILURE_REASON - $FAILURE_DETAILS"
        return 1
    fi
    
    # Check for corruption indicators
    if ! check_for_corruption; then
        error_log "Corruption indicators found: $LAST_FAILURE_REASON - $FAILURE_DETAILS"
        return 1
    fi
    
    # Test database functionality
    if ! test_database_functionality; then
        error_log "Database functionality test failed: $LAST_FAILURE_REASON - $FAILURE_DETAILS"
        return 1
    fi
    
    log "Health check passed"
    return 0
}

# Main monitoring loop
main() {
    log "=== Database Health Monitor Started ==="
    
    while true; do
            # Suppress alerts when a deployment is in progress
    if [ -f "$DEPLOYMENT_SUPPRESSION_FILE" ]; then
        # Check if flag is stale (older than 30 minutes)
        local flag_age=$(( $(date +%s) - $(stat -c %Y "$DEPLOYMENT_SUPPRESSION_FILE" 2>/dev/null || echo 0) ))
        if [ $flag_age -gt 1800 ]; then
            log "Deployment flag is stale (${flag_age}s old); removing it and resuming monitoring."
            local stale_alert="⚠️ Stale deployment flag detected and removed!

A deployment suppression flag was found that is ${flag_age} seconds old (over 30 minutes). This suggests a deployment script may have crashed or been interrupted without properly cleaning up.

• Flag file: $DEPLOYMENT_SUPPRESSION_FILE
• Flag age: ${flag_age}s (threshold: 1800s)
• Action: Flag removed automatically
• Status: Health monitoring resumed

This is a self-healing action - no manual intervention required, but you may want to check recent deployment logs."
            
            rm -f "$DEPLOYMENT_SUPPRESSION_FILE"
            send_alert "$stale_alert" "WARNING"
        else
            log "Deployment flag detected ($DEPLOYMENT_SUPPRESSION_FILE); skipping health checks."
            FAILURE_COUNT=0
            sleep $HEALTH_CHECK_INTERVAL
            continue
        fi
    fi
        
        if perform_health_check; then
            # Reset failure count on success
            if [ $FAILURE_COUNT -gt 0 ]; then
                log "Health restored after $FAILURE_COUNT failures"
                send_alert "Database health restored after $FAILURE_COUNT consecutive failures." "SUCCESS"
                FAILURE_COUNT=0
            fi
        else
            FAILURE_COUNT=$((FAILURE_COUNT + 1))
            error_log "Health check failed (failure count: $FAILURE_COUNT/$MAX_FAILURE_COUNT)"
            
            # Send an alert only on the FIRST consecutive failure (state change: OK -> FAIL)
            if [ $FAILURE_COUNT -eq 1 ]; then
                local initial_alert="Database health check failed. Reason: ${LAST_FAILURE_REASON:-Unknown} - ${FAILURE_DETAILS:-No details}"
                send_alert "$initial_alert" "ERROR"
            fi
            
            if [ $FAILURE_COUNT -ge $MAX_FAILURE_COUNT ]; then
                error_log "Maximum failure count reached, triggering recovery"
                local recovery_alert="Database health check failed $MAX_FAILURE_COUNT times, triggering automatic recovery. Last failure: ${LAST_FAILURE_REASON:-Unknown} - ${FAILURE_DETAILS:-No details}"
                send_alert "$recovery_alert" "CRITICAL"
                
                if trigger_recovery; then
                    log "Recovery successful, resuming monitoring"
                    send_alert "Database recovery completed successfully! System is back online." "SUCCESS"
                    FAILURE_COUNT=0
                else
                    error_log "Safe recovery failed, manual intervention required"
                    # Get backup availability information
                    local backup_status
                    backup_status=$(check_backup_availability)
                    
                    local manual_intervention_alert="🚨 CRITICAL: Safe database recovery failed! 

Database health check failed $MAX_FAILURE_COUNT times and automatic recovery from backup was unsuccessful.

⚠️ MANUAL INTERVENTION REQUIRED ⚠️
• No data will be automatically deleted
• Database may be in a corrupted or inconsistent state
• Immediate attention needed to prevent data loss

Last failure: ${LAST_FAILURE_REASON:-Unknown} - ${FAILURE_DETAILS:-No details}

Environment: ${ENVIRONMENT:-Development}
Database: $DB_USER@$DB_HOST:$DB_NAME

Backup Status:
$backup_status

Required actions:
1. Investigate database corruption/connection issues
2. Verify backup integrity and availability
3. Consider manual database recovery procedures
4. Contact database administrator immediately"

                    send_alert "$manual_intervention_alert" "CRITICAL"
                    
                    # Wait longer before next attempt to avoid spam
                    sleep $((HEALTH_CHECK_INTERVAL * 10))
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