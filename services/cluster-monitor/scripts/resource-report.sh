#!/bin/bash
set -euo pipefail

# Resource Report Script
# Generates comprehensive resource usage reports and sends to Telegram

# Configuration
ENVIRONMENT="${ENVIRONMENT:-Development}"
K8S_NAMESPACE="${K8S_NAMESPACE:-default}"
MINIKUBE_PROFILE="${MINIKUBE_PROFILE:-minikube}"

# Logging function
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

# Format bytes to human readable
format_bytes() {
    local bytes=$1
    if [ "$bytes" -ge 1073741824 ]; then
        echo "$(awk "BEGIN {printf \"%.1f\", $bytes/1073741824}")GiB"
    elif [ "$bytes" -ge 1048576 ]; then
        echo "$(awk "BEGIN {printf \"%.1f\", $bytes/1048576}")MiB"
    elif [ "$bytes" -ge 1024 ]; then
        echo "$(awk "BEGIN {printf \"%.1f\", $bytes/1024}")KiB"
    else
        echo "${bytes}B"
    fi
}

# Format CPU millicores to cores
format_cpu() {
    local millicores=$1
    if [ "$millicores" -ge 1000 ]; then
        echo "$(awk "BEGIN {printf \"%.2f\", $millicores/1000}")cores"
    else
        echo "${millicores}m"
    fi
}

# Get cluster resource information
get_cluster_resources() {
    local report=""
    
    # Node resources
    if kubectl get nodes --context="$MINIKUBE_PROFILE" >/dev/null 2>&1; then
        local node_info
        node_info=$(kubectl describe nodes --context="$MINIKUBE_PROFILE" | grep -E "(Name:|cpu:|memory:|Allocated resources:|Resource|Requests|Limits)" | head -20)
        
        # Parse node capacity
        local node_cpu=$(kubectl get nodes --context="$MINIKUBE_PROFILE" -o jsonpath='{.items[0].status.capacity.cpu}' 2>/dev/null || echo "unknown")
        local node_memory=$(kubectl get nodes --context="$MINIKUBE_PROFILE" -o jsonpath='{.items[0].status.capacity.memory}' 2>/dev/null || echo "unknown")
        
        report+="🖥️ *Node Resources:*\n"
        report+="• CPU Capacity: $node_cpu cores\n"
        report+="• Memory Capacity: $node_memory\n"
        report+="\n"
    fi
    
    echo -e "$report"
}

# Get pod resource usage
get_pod_resources() {
    local report=""
    
    if kubectl top pods -n "$K8S_NAMESPACE" --context="$MINIKUBE_PROFILE" >/dev/null 2>&1; then
        report+="📊 *Pod Resource Usage:*\n"
        
        # Get top resource consuming pods
        local pod_metrics
        pod_metrics=$(kubectl top pods -n "$K8S_NAMESPACE" --context="$MINIKUBE_PROFILE" --no-headers 2>/dev/null | sort -k2 -nr | head -5)
        
        if [ -n "$pod_metrics" ]; then
            report+="\`\`\`\n"
            report+="POD NAME                CPU    MEMORY\n"
            report+="$pod_metrics\n"
            report+="\`\`\`\n"
        else
            report+="• No metrics available\n"
        fi
        report+="\n"
    else
        report+="📊 *Pod Resource Usage:*\n"
        report+="• Metrics server not available\n\n"
    fi
    
    echo -e "$report"
}

# Get deployment status and resource requests/limits
get_deployment_resources() {
    local report=""
    
    if kubectl get deployments -n "$K8S_NAMESPACE" --context="$MINIKUBE_PROFILE" >/dev/null 2>&1; then
        report+="🚀 *Deployment Status:*\n"
        
        local deployments
        deployments=$(kubectl get deployments -n "$K8S_NAMESPACE" --context="$MINIKUBE_PROFILE" --no-headers 2>/dev/null)
        
        if [ -n "$deployments" ]; then
            while IFS= read -r deployment_line; do
                local dep_name=$(echo "$deployment_line" | awk '{print $1}')
                local ready=$(echo "$deployment_line" | awk '{print $2}')
                local available=$(echo "$deployment_line" | awk '{print $4}')
                
                # Get resource requests and limits
                local resources
                resources=$(kubectl describe deployment "$dep_name" -n "$K8S_NAMESPACE" --context="$MINIKUBE_PROFILE" 2>/dev/null | grep -A 4 -E "(Requests|Limits):" | grep -E "(cpu|memory):" | tr '\n' ' ')
                
                report+="• *$dep_name*: $ready ready, $available available\n"
                if [ -n "$resources" ]; then
                    report+="  Resources: $resources\n"
                fi
            done <<< "$deployments"
        else
            report+="• No deployments found\n"
        fi
        report+="\n"
    fi
    
    echo -e "$report"
}

# Get storage information
get_storage_info() {
    local report=""
    
    report+="💾 *Storage Information:*\n"
    
    # PVC usage
    if kubectl get pvc -n "$K8S_NAMESPACE" --context="$MINIKUBE_PROFILE" >/dev/null 2>&1; then
        local pvcs
        pvcs=$(kubectl get pvc -n "$K8S_NAMESPACE" --context="$MINIKUBE_PROFILE" --no-headers 2>/dev/null)
        
        if [ -n "$pvcs" ]; then
            report+="*Persistent Volume Claims:*\n"
            while IFS= read -r pvc_line; do
                local pvc_name=$(echo "$pvc_line" | awk '{print $1}')
                local status=$(echo "$pvc_line" | awk '{print $2}')
                local capacity=$(echo "$pvc_line" | awk '{print $4}')
                
                report+="• $pvc_name: $status ($capacity)\n"
            done <<< "$pvcs"
        else
            report+="• No PVCs found\n"
        fi
    fi
    
    # Node disk usage (if available)
    if command -v minikube >/dev/null 2>&1; then
        local disk_usage
        disk_usage=$(minikube ssh -p "$MINIKUBE_PROFILE" "df -h /" 2>/dev/null | tail -n1 | awk '{print $3"/"$2" ("$5" used)"}' || echo "unknown")
        report+="*Node Disk Usage:* $disk_usage\n"
    fi
    
    report+="\n"
    echo -e "$report"
}

# Get network information
get_network_info() {
    local report=""
    
    report+="🌐 *Network Information:*\n"
    
    # Service status
    if kubectl get services -n "$K8S_NAMESPACE" --context="$MINIKUBE_PROFILE" >/dev/null 2>&1; then
        local service_count
        service_count=$(kubectl get services -n "$K8S_NAMESPACE" --context="$MINIKUBE_PROFILE" --no-headers 2>/dev/null | wc -l)
        report+="• Services: $service_count active\n"
        
        # Check ingress
        if kubectl get ingress -n "$K8S_NAMESPACE" --context="$MINIKUBE_PROFILE" >/dev/null 2>&1; then
            local ingress_count
            ingress_count=$(kubectl get ingress -n "$K8S_NAMESPACE" --context="$MINIKUBE_PROFILE" --no-headers 2>/dev/null | wc -l)
            if [ "$ingress_count" -gt 0 ]; then
                report+="• Ingress: $ingress_count configured\n"
                
                # Get ingress hosts
                local hosts
                hosts=$(kubectl get ingress -n "$K8S_NAMESPACE" --context="$MINIKUBE_PROFILE" -o jsonpath='{.items[*].spec.rules[*].host}' 2>/dev/null | tr ' ' '\n' | sort -u | tr '\n' ' ')
                if [ -n "$hosts" ]; then
                    report+="• Hosts: $hosts\n"
                fi
            fi
        fi
    fi
    
    # Minikube IP
    if command -v minikube >/dev/null 2>&1; then
        local minikube_ip
        minikube_ip=$(minikube ip -p "$MINIKUBE_PROFILE" 2>/dev/null || echo "unknown")
        report+="• Minikube IP: $minikube_ip\n"
    fi
    
    report+="\n"
    echo -e "$report"
}

# Get event information
get_recent_events() {
    local report=""
    
    report+="📋 *Recent Events:*\n"
    
    # Get recent events
    if kubectl get events -n "$K8S_NAMESPACE" --context="$MINIKUBE_PROFILE" >/dev/null 2>&1; then
        local warning_events
        warning_events=$(kubectl get events -n "$K8S_NAMESPACE" --context="$MINIKUBE_PROFILE" --field-selector type=Warning --no-headers 2>/dev/null | head -3)
        
        if [ -n "$warning_events" ]; then
            report+="*Recent Warnings:*\n"
            while IFS= read -r event_line; do
                local reason=$(echo "$event_line" | awk '{print $4}')
                local object=$(echo "$event_line" | awk '{print $5}')
                local message=$(echo "$event_line" | awk '{for(i=6;i<=NF;i++) printf "%s ", $i; print ""}')
                report+="• $reason ($object): ${message:0:50}...\n"
            done <<< "$warning_events"
        else
            report+="• No recent warnings\n"
        fi
    fi
    
    report+="\n"
    echo -e "$report"
}

# Get performance recommendations
get_recommendations() {
    local report=""
    local recommendations=()
    
    # Check if metrics are available
    if ! kubectl top pods -n "$K8S_NAMESPACE" --context="$MINIKUBE_PROFILE" >/dev/null 2>&1; then
        recommendations+=("Install metrics-server for better monitoring")
    fi
    
    # Check for pods with high restart counts
    local high_restart_pods
    high_restart_pods=$(kubectl get pods -n "$K8S_NAMESPACE" --context="$MINIKUBE_PROFILE" --no-headers 2>/dev/null | awk '$4 > 5 {print $1 " (" $4 " restarts)"}' | head -3)
    
    if [ -n "$high_restart_pods" ]; then
        while IFS= read -r pod_info; do
            recommendations+=("Investigate high restart count: $pod_info")
        done <<< "$high_restart_pods"
    fi
    
    # Check for non-running pods
    local non_running_pods
    non_running_pods=$(kubectl get pods -n "$K8S_NAMESPACE" --context="$MINIKUBE_PROFILE" --no-headers 2>/dev/null | awk '$3 != "Running" && $3 != "Completed" {print $1 " (" $3 ")"}' | head -3)
    
    if [ -n "$non_running_pods" ]; then
        while IFS= read -r pod_info; do
            recommendations+=("Check pod status: $pod_info")
        done <<< "$non_running_pods"
    fi
    
    if [ ${#recommendations[@]} -gt 0 ]; then
        report+="💡 *Recommendations:*\n"
        for rec in "${recommendations[@]}"; do
            report+="• $rec\n"
        done
        report+="\n"
    fi
    
    echo -e "$report"
}

# Send Telegram alert
send_telegram_report() {
    local message="$1"
    
    # Skip if credentials not configured
    if [ -z "${TELEGRAM_BOT_TOKEN:-}" ] || [ -z "${TELEGRAM_CHAT_ID:-}" ]; then
        log "Telegram credentials not configured, skipping notification"
        return 0
    fi
    
    # Send to Telegram
    local response
    response=$(curl -s -X POST "https://api.telegram.org/bot$TELEGRAM_BOT_TOKEN/sendMessage" \
        -H "Content-Type: application/json" \
        -d "{
            \"chat_id\": \"$TELEGRAM_CHAT_ID\",
            \"text\": \"$message\",
            \"parse_mode\": \"Markdown\",
            \"disable_web_page_preview\": true
        }" 2>/dev/null)
    
    if echo "$response" | grep -q '"ok":true'; then
        log "Telegram resource report sent successfully"
    else
        log "Failed to send Telegram report: $response"
    fi
}

# Main function
main() {
    log "=== Generating Resource Report ==="
    
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S UTC')
    local report=""
    
    # Build comprehensive report
    report+="📊 *Cluster Resource Report*\n\n"
    report+="*Environment:* $ENVIRONMENT\n"
    report+="*Namespace:* $K8S_NAMESPACE\n"
    report+="*Time:* $timestamp\n\n"
    
    report+="$(get_cluster_resources)"
    report+="$(get_pod_resources)"
    report+="$(get_deployment_resources)"
    report+="$(get_storage_info)"
    report+="$(get_network_info)"
    report+="$(get_recent_events)"
    report+="$(get_recommendations)"
    
    # Cluster health summary
    local health_emoji="✅"
    local health_status="Healthy"
    
    # Quick health check
    if ! kubectl cluster-info --context="$MINIKUBE_PROFILE" >/dev/null 2>&1; then
        health_emoji="❌"
        health_status="API Unreachable"
    elif ! minikube status -p "$MINIKUBE_PROFILE" >/dev/null 2>&1; then
        health_emoji="⚠️"
        health_status="Cluster Issues"
    fi
    
    report+="$health_emoji *Overall Status:* $health_status\n\n"
    report+="*Next Report:* $(date -d '+1 hour' '+%H:%M UTC')\n"
    report+="Use \`kubectl get pods -n $K8S_NAMESPACE\` for real-time status"
    
    # Send the report
    send_telegram_report "$report"
    
    log "Resource report generation completed"
}

# Run main function
main "$@" 