#!/bin/bash
set -euo pipefail

# Main Cluster Monitor Script
# Monitors cluster health, sends alerts, and handles auto-restart

# Configuration
LOG_FILE="/var/log/monitor/cluster-monitor.log"
CHECK_INTERVAL="${CHECK_INTERVAL:-30}"
RESOURCE_CHECK_INTERVAL="${RESOURCE_CHECK_INTERVAL:-300}"
MAX_FAILURE_COUNT="${MAX_FAILURE_COUNT:-3}"
AUTO_RESTART_ENABLED="${AUTO_RESTART_ENABLED:-true}"
ENVIRONMENT="${ENVIRONMENT:-Development}"
K8S_NAMESPACE="${K8S_NAMESPACE:-default}"
MINIKUBE_PROFILE="${MINIKUBE_PROFILE:-minikube}"

# Failure tracking
CLUSTER_FAILURE_COUNT=0
API_FAILURE_COUNT=0
NODE_FAILURE_COUNT=0
LAST_RESTART_TIME=0
RESTART_COOLDOWN=3600  # 1 hour

# Ensure log directory exists
mkdir -p "$(dirname "$LOG_FILE")"

# Logging functions
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_FILE"
}

error_log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERROR: $1" | tee -a "$LOG_FILE" >&2
}

# Send Telegram alert
send_alert() {
    local message="$1"
    local alert_type="${2:-WARNING}"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S UTC')
    
    log "ALERT: $message"
    
    # Skip Telegram if credentials not configured
    if [ -z "${TELEGRAM_BOT_TOKEN:-}" ] || [ -z "${TELEGRAM_CHAT_ID:-}" ]; then
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
        "RESTART") emoji="🔄" ;;
    esac
    
    # Get cluster status
    local cluster_status=$(get_cluster_status_summary)
    
    # Format message for Telegram
    local telegram_message="$emoji *Cluster Monitor - $alert_type*

*System:* Kubernetes Cluster Monitor
*Time:* $timestamp
*Environment:* $ENVIRONMENT
*Namespace:* $K8S_NAMESPACE

*Message:*
$message

*Cluster Status:*
$cluster_status

*Monitor Stats:*
• Cluster Failures: $CLUSTER_FAILURE_COUNT/$MAX_FAILURE_COUNT
• API Failures: $API_FAILURE_COUNT/$MAX_FAILURE_COUNT
• Node Failures: $NODE_FAILURE_COUNT/$MAX_FAILURE_COUNT
• Last Restart: $(date -d @$LAST_RESTART_TIME '+%Y-%m-%d %H:%M:%S' 2>/dev/null || echo "Never")

*Quick Actions:*
\`kubectl get nodes --context=$MINIKUBE_PROFILE\`
\`kubectl get pods -n $K8S_NAMESPACE --context=$MINIKUBE_PROFILE\`
\`minikube status -p $MINIKUBE_PROFILE\`"

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

# Get cluster status summary
get_cluster_status_summary() {
    local status=""
    
    # Check minikube status
    local minikube_status="Unknown"
    if command -v minikube >/dev/null 2>&1; then
        if minikube status -p "$MINIKUBE_PROFILE" >/dev/null 2>&1; then
            minikube_status="✅ Running"
        else
            minikube_status="❌ Stopped"
        fi
    fi
    
    # Check API server
    local api_status="Unknown"
    if kubectl cluster-info --context="$MINIKUBE_PROFILE" >/dev/null 2>&1; then
        api_status="✅ Accessible"
    else
        api_status="❌ Unreachable"
    fi
    
    # Check nodes
    local node_count=0
    local ready_nodes=0
    if kubectl get nodes --context="$MINIKUBE_PROFILE" >/dev/null 2>&1; then
        node_count=$(kubectl get nodes --context="$MINIKUBE_PROFILE" --no-headers | wc -l)
        ready_nodes=$(kubectl get nodes --context="$MINIKUBE_PROFILE" --no-headers | grep -c " Ready " || echo "0")
    fi
    
    # Check pods in namespace
    local pod_count=0
    local running_pods=0
    if kubectl get pods -n "$K8S_NAMESPACE" --context="$MINIKUBE_PROFILE" >/dev/null 2>&1; then
        pod_count=$(kubectl get pods -n "$K8S_NAMESPACE" --context="$MINIKUBE_PROFILE" --no-headers | wc -l)
        running_pods=$(kubectl get pods -n "$K8S_NAMESPACE" --context="$MINIKUBE_PROFILE" --no-headers | grep -c " Running " || echo "0")
    fi
    
    echo "• Minikube: $minikube_status
• API Server: $api_status  
• Nodes: $ready_nodes/$node_count Ready
• Pods: $running_pods/$pod_count Running"
}

# Check if minikube cluster is running
check_minikube_status() {
    if ! command -v minikube >/dev/null 2>&1; then
        error_log "minikube command not found"
        return 1
    fi
    
    if minikube status -p "$MINIKUBE_PROFILE" >/dev/null 2>&1; then
        log "Minikube cluster '$MINIKUBE_PROFILE' is running"
        CLUSTER_FAILURE_COUNT=0
        return 0
    else
        error_log "Minikube cluster '$MINIKUBE_PROFILE' is not running"
        CLUSTER_FAILURE_COUNT=$((CLUSTER_FAILURE_COUNT + 1))
        return 1
    fi
}

# Check Kubernetes API server connectivity
check_api_connectivity() {
    if kubectl cluster-info --context="$MINIKUBE_PROFILE" >/dev/null 2>&1; then
        log "Kubernetes API server is accessible"
        API_FAILURE_COUNT=0
        return 0
    else
        error_log "Cannot connect to Kubernetes API server"
        API_FAILURE_COUNT=$((API_FAILURE_COUNT + 1))
        return 1
    fi
}

# Check node health
check_node_health() {
    local node_issues=0
    
    # Get all nodes
    local nodes
    if ! nodes=$(kubectl get nodes --context="$MINIKUBE_PROFILE" --no-headers 2>/dev/null); then
        error_log "Failed to get node list"
        NODE_FAILURE_COUNT=$((NODE_FAILURE_COUNT + 1))
        return 1
    fi
    
    while IFS= read -r node_line; do
        local node_name=$(echo "$node_line" | awk '{print $1}')
        local node_status=$(echo "$node_line" | awk '{print $2}')
        
        if [ "$node_status" != "Ready" ]; then
            error_log "Node $node_name is not ready (status: $node_status)"
            node_issues=$((node_issues + 1))
        else
            log "Node $node_name is ready"
        fi
        
        # Check node conditions
        check_node_conditions "$node_name"
        
    done <<< "$nodes"
    
    if [ $node_issues -eq 0 ]; then
        NODE_FAILURE_COUNT=0
        return 0
    else
        NODE_FAILURE_COUNT=$((NODE_FAILURE_COUNT + 1))
        return 1
    fi
}

# Check specific node conditions
check_node_conditions() {
    local node_name="$1"
    
    # Get node conditions
    local conditions
    if conditions=$(kubectl describe node "$node_name" --context="$MINIKUBE_PROFILE" 2>/dev/null | grep -A 20 "Conditions:"); then
        
        # Check for pressure conditions
        if echo "$conditions" | grep -q "MemoryPressure.*True"; then
            send_alert "Node $node_name has memory pressure!" "WARNING"
        fi
        
        if echo "$conditions" | grep -q "DiskPressure.*True"; then
            send_alert "Node $node_name has disk pressure!" "WARNING"
        fi
        
        if echo "$conditions" | grep -q "PIDPressure.*True"; then
            send_alert "Node $node_name has PID pressure!" "WARNING"
        fi
        
        if echo "$conditions" | grep -q "NetworkUnavailable.*True"; then
            send_alert "Node $node_name has network issues!" "ERROR"
        fi
    fi
}

# Check pod health in namespace
check_pod_health() {
    local pod_issues=0
    
    # Get pods in namespace
    local pods
    if ! pods=$(kubectl get pods -n "$K8S_NAMESPACE" --context="$MINIKUBE_PROFILE" --no-headers 2>/dev/null); then
        error_log "Failed to get pod list"
        return 1
    fi
    
    while IFS= read -r pod_line; do
        local pod_name=$(echo "$pod_line" | awk '{print $1}')
        local pod_ready=$(echo "$pod_line" | awk '{print $2}')
        local pod_status=$(echo "$pod_line" | awk '{print $3}')
        local pod_restarts=$(echo "$pod_line" | awk '{print $4}')
        
        # Check for non-running pods
        if [ "$pod_status" != "Running" ] && [ "$pod_status" != "Completed" ]; then
            error_log "Pod $pod_name is in $pod_status state"
            pod_issues=$((pod_issues + 1))
        fi
        
        # Check for excessive restarts
        if [ "$pod_restarts" -gt 5 ]; then
            send_alert "Pod $pod_name has $pod_restarts restarts (excessive)" "WARNING"
        fi
        
        # Check readiness
        if [[ "$pod_ready" == *"/"* ]]; then
            local ready_count=$(echo "$pod_ready" | cut -d'/' -f1)
            local total_count=$(echo "$pod_ready" | cut -d'/' -f2)
            if [ "$ready_count" != "$total_count" ]; then
                log "Pod $pod_name is not fully ready ($pod_ready)"
            fi
        fi
        
    done <<< "$pods"
    
    return $pod_issues
}

# Restart minikube cluster
restart_cluster() {
    local current_time=$(date +%s)
    
    # Check cooldown period
    if [ $((current_time - LAST_RESTART_TIME)) -lt $RESTART_COOLDOWN ]; then
        error_log "Cluster restart attempted too recently, waiting for cooldown period"
        return 1
    fi
    
    log "=== TRIGGERING CLUSTER RESTART ==="
    LAST_RESTART_TIME=$current_time
    
    send_alert "Initiating cluster restart due to repeated failures" "RESTART"
    
    # Stop minikube
    log "Stopping minikube cluster..."
    if minikube stop -p "$MINIKUBE_PROFILE"; then
        log "Minikube stopped successfully"
    else
        error_log "Failed to stop minikube"
    fi
    
    # Wait a moment
    sleep 10
    
    # Start minikube
    log "Starting minikube cluster..."
    local cpu_count=8
    local mem_size=32768
    
    if [[ "$MINIKUBE_PROFILE" == "minikube-prod" ]]; then
        cpu_count=16
        mem_size=65536
    fi
    
    if minikube start -p "$MINIKUBE_PROFILE" --cpus="$cpu_count" --memory="$mem_size"; then
        log "Minikube started successfully"
        
        # Wait for cluster to be ready
        sleep 30
        
        # Verify restart was successful
        if check_minikube_status && check_api_connectivity; then
            send_alert "Cluster restart completed successfully! System is back online." "SUCCESS"
            CLUSTER_FAILURE_COUNT=0
            API_FAILURE_COUNT=0
            NODE_FAILURE_COUNT=0
            return 0
        else
            send_alert "Cluster restart failed - manual intervention required" "CRITICAL"
            return 1
        fi
    else
        error_log "Failed to start minikube"
        send_alert "Automatic cluster restart failed - manual intervention required" "CRITICAL"
        return 1
    fi
}

# Main monitoring loop
main() {
    log "=== Starting Cluster Monitor ==="
    log "Environment: $ENVIRONMENT"
    log "Namespace: $K8S_NAMESPACE"
    log "Minikube Profile: $MINIKUBE_PROFILE"
    log "Check Interval: ${CHECK_INTERVAL}s"
    log "Auto-restart: $AUTO_RESTART_ENABLED"
    
    send_alert "Cluster monitoring started" "INFO"
    
    local last_resource_check=0
    
    while true; do
        log "=== Cluster Health Check ==="
        
        local cluster_healthy=true
        
        # Check minikube status
        if ! check_minikube_status; then
            cluster_healthy=false
        fi
        
        # Check API connectivity
        if ! check_api_connectivity; then
            cluster_healthy=false
        fi
        
        # Check node health
        if ! check_node_health; then
            cluster_healthy=false
        fi
        
        # Check pod health
        check_pod_health
        
        # Handle failures
        if [ "$cluster_healthy" = false ]; then
            if [ $CLUSTER_FAILURE_COUNT -ge $MAX_FAILURE_COUNT ] || 
               [ $API_FAILURE_COUNT -ge $MAX_FAILURE_COUNT ] || 
               [ $NODE_FAILURE_COUNT -ge $MAX_FAILURE_COUNT ]; then
                
                error_log "Maximum failure count reached, triggering recovery"
                send_alert "Cluster health check failed $MAX_FAILURE_COUNT times, triggering automatic restart" "CRITICAL"
                
                if [ "$AUTO_RESTART_ENABLED" = "true" ]; then
                    if restart_cluster; then
                        log "Cluster restart successful, resuming monitoring"
                    else
                        error_log "Cluster restart failed"
                        # Wait longer before next attempt
                        sleep $((CHECK_INTERVAL * 5))
                        continue
                    fi
                else
                    send_alert "Auto-restart is disabled, manual intervention required" "CRITICAL"
                fi
            fi
        else
            log "Cluster health check passed"
        fi
        
        # Periodic resource check
        local current_time=$(date +%s)
        if [ $((current_time - last_resource_check)) -ge $RESOURCE_CHECK_INTERVAL ]; then
            log "Running periodic resource check..."
            /app/resource-check.sh || true
            last_resource_check=$current_time
        fi
        
        sleep $CHECK_INTERVAL
    done
}

# Handle signals gracefully
trap 'log "Cluster monitor shutting down"; exit 0' SIGTERM SIGINT

# Run main function
main "$@" 