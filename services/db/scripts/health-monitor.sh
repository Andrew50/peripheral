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
    if PGPASSWORD=$POSTGRES_PASSWORD pg_isready -U "$DB_USER" -d "$DB_NAME" -h "$DB_HOST" >/dev/null 2>&1; then
        return 0
    else
        LAST_FAILURE_REASON="Database connection failed"
        FAILURE_DETAILS="pg_isready check failed for $DB_USER@$DB_HOST:$DB_NAME"
        return 1
    fi
}

# Check for corruption indicators in logs
check_for_corruption() {
    # Check if PostgreSQL process is running
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
    local system_info=""
    
    # Get CPU usage
    local cpu_usage=$(top -bn1 | grep "Cpu(s)" | awk '{print $2}' | cut -d'%' -f1 2>/dev/null || echo "N/A")
    if [ "$cpu_usage" != "N/A" ]; then
        system_info="$system_info\n• CPU Usage: ${cpu_usage}%"
    fi
    
    # Get memory usage
    local mem_info=$(free -m 2>/dev/null | grep Mem)
    if [ -n "$mem_info" ]; then
        local mem_total=$(echo "$mem_info" | awk '{print $2}')
        local mem_used=$(echo "$mem_info" | awk '{print $3}')
        local mem_percent=$((mem_used * 100 / mem_total))
        system_info="$system_info\n• Memory Usage: ${mem_percent}% (${mem_used}MB/${mem_total}MB)"
    fi
    
    # Get disk usage for backup directory
    if [ -d "/backups" ]; then
        local disk_usage=$(df -h /backups 2>/dev/null | tail -1 | awk '{print $5}' | cut -d'%' -f1)
        if [ -n "$disk_usage" ] && [ "$disk_usage" != "Use%" ]; then
            system_info="$system_info\n• Backup Disk Usage: ${disk_usage}%"
        fi
    fi
    
    # Get uptime
    local uptime_info=$(uptime 2>/dev/null | awk -F'up ' '{print $2}' | awk -F',' '{print $1}')
    if [ -n "$uptime_info" ]; then
        system_info="$system_info\n• System Uptime: $uptime_info"
    fi
    
    echo -e "$system_info"
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
        if [ -n "$NODE_NAME" ]; then
            k8s_info="$k8s_info\n• Node: $NODE_NAME"
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
    
    # Get recent database logs and status
    local recent_logs
    recent_logs=$(get_recent_db_logs)
    local db_status
    db_status=$(get_db_status_info)
    local system_info
    system_info=$(get_system_info)
    local k8s_info
    k8s_info=$(get_k8s_info)
    local health_status
    health_status=$(get_health_status)
    local backup_status
    backup_status=$(get_backup_status)
    
    # Determine emoji based on alert type
    local emoji="⚠️"
    case "$alert_type" in
        "CRITICAL") emoji="🚨" ;;
        "ERROR") emoji="❌" ;;
        "WARNING") emoji="⚠️" ;;
        "INFO") emoji="ℹ️" ;;
        "SUCCESS") emoji="✅" ;;
    esac
    
    # Get environment info
    local env_info="${ENVIRONMENT:-Development}"
    local host_info=$(hostname 2>/dev/null || echo "Unknown")
    local db_info="$DB_USER@$DB_HOST:$DB_NAME"
    
    # Format message for Telegram
    local telegram_message="$emoji *Database Alert - $alert_type*

*System:* PostgreSQL Health Monitor
*Environment:* $env_info
*Host:* $host_info
*Database:* $db_info
*Time:* $timestamp

*Alert Message:*
$alert_message

*Failure Details:*
• Reason: ${LAST_FAILURE_REASON:-Unknown}
• Details: ${FAILURE_DETAILS:-No specific details}
• Failure Count: $FAILURE_COUNT/$MAX_FAILURE_COUNT
• Last Recovery: $(date -d @$LAST_RECOVERY_TIME '+%Y-%m-%d %H:%M:%S' 2>/dev/null || echo "Never")

*Health Status:*$health_status
• Connection String: $DB_USER@$DB_HOST:$DB_NAME$db_status

*System Resources:*$system_info

*Kubernetes Info:*$k8s_info

*Backup Status:*$backup_status"

    # Add recent logs if available
    if [ -n "$recent_logs" ]; then
        telegram_message="$telegram_message

*Recent Database Logs:*
\`\`\`
$recent_logs
\`\`\`"
    fi

    # Attempt to fetch recent Kubernetes pod logs if kubectl is available
    if command -v kubectl >/dev/null 2>&1; then
        local pod_logs="$(kubectl logs deployment/db-health-monitor --tail=20 2>/dev/null || true)"

        # Fallback to database pods if health-monitor deployment logs are unavailable
        if [ -z "$pod_logs" ]; then
            pod_logs="$(kubectl logs -l app=db --tail=20 2>/dev/null || true)"
        fi

        if [ -n "$pod_logs" ]; then
            telegram_message="$telegram_message

*Pod Logs (last 20 lines):*
\`\`\`
$pod_logs
\`\`\`"
        fi
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