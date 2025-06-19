# Cluster Monitoring and Alerting System

This document describes the comprehensive cluster monitoring and alerting system that provides real-time monitoring, Telegram notifications, and automatic restart capabilities for your Kubernetes cluster.

## 🚀 Quick Start

### 1. Deploy the Monitoring System
```bash
# Deploy cluster monitoring
./config/deploy/scripts/deploy-monitoring.sh

# Setup Telegram alerts (optional but recommended)
./config/dev/scripts/setup-telegram-alerts.sh
```

### 2. Enable Auto-Start on Boot
```bash
# Enable systemd service for auto-restart
sudo systemctl enable minikube-auto-start.service
sudo systemctl start minikube-auto-start.service
```

### 3. Check Status
```bash
# Check monitoring pods
kubectl get pods -l app=cluster-monitor -n stage

# View monitor logs
kubectl logs deployment/cluster-monitor -n stage
```

## 📊 System Architecture

### Components

1. **Cluster Monitor** - Main monitoring service
   - Monitors cluster health every 30 seconds
   - Checks API server connectivity
   - Monitors node and pod health
   - Handles automatic cluster restart

2. **Node Monitor** - Host-level monitoring (DaemonSet)
   - Monitors host resources (CPU, memory, disk)
   - Tracks Docker daemon health
   - Provides system-level metrics

3. **Resource Reporter** - Periodic resource reports
   - Generates comprehensive reports every hour
   - Includes resource usage, deployment status, and recommendations
   - Sends detailed Telegram reports

4. **Auto-Start Service** - System boot integration
   - Automatically starts minikube on system boot
   - Uses systemd for reliable startup

## 🔔 Alert Types

The system sends various types of Telegram notifications:

### 🚨 CRITICAL Alerts
- Cluster API server unreachable
- Multiple consecutive health check failures
- Automatic restart triggers
- Critical resource exhaustion

### ❌ ERROR Alerts
- Service connectivity issues
- Pod failures
- Network problems
- Backup/recovery failures

### ⚠️ WARNING Alerts
- Resource pressure (memory, disk, CPU)
- High pod restart counts
- Node condition issues
- Performance degradation

### 🔄 RESTART Alerts
- Cluster restart initiated
- Recovery operations
- Service restarts

### ✅ SUCCESS Alerts
- Successful cluster restarts
- Recovery completions
- Health check recoveries

### 📊 REPORT Alerts (Hourly)
- Comprehensive resource usage
- Deployment status
- Storage information
- Performance recommendations

## 📋 Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `CHECK_INTERVAL` | 30 | Cluster health check interval (seconds) |
| `RESOURCE_CHECK_INTERVAL` | 300 | Resource report interval (seconds) |
| `MAX_FAILURE_COUNT` | 3 | Failures before restart trigger |
| `AUTO_RESTART_ENABLED` | true | Enable automatic cluster restart |
| `TELEGRAM_BOT_TOKEN` | - | Telegram bot token |
| `TELEGRAM_CHAT_ID` | - | Telegram chat ID |

### Thresholds (ConfigMap)

```yaml
# CPU usage thresholds
CPU_WARNING_THRESHOLD=70
CPU_CRITICAL_THRESHOLD=85

# Memory usage thresholds  
MEMORY_WARNING_THRESHOLD=75
MEMORY_CRITICAL_THRESHOLD=90

# Disk usage thresholds
DISK_WARNING_THRESHOLD=80
DISK_CRITICAL_THRESHOLD=95
```

## 🛠️ Management Commands

### Monitoring Operations
```bash
# Check monitor status
kubectl get pods -l app=cluster-monitor -n stage

# View real-time logs
kubectl logs -f deployment/cluster-monitor -n stage

# Check resource usage
kubectl top pods -n stage

# Get cluster events
kubectl get events -n stage --sort-by='.lastTimestamp'
```

### Manual Operations
```bash
# Trigger manual resource report
kubectl create job manual-report --from=cronjob/resource-report -n stage

# Restart cluster monitor
kubectl rollout restart deployment/cluster-monitor -n stage

# Check auto-start service
sudo systemctl status minikube-auto-start.service
```

### Troubleshooting
```bash
# Check monitor logs for errors
kubectl logs deployment/cluster-monitor -n stage | grep ERROR

# Verify Telegram configuration
kubectl get secret telegram-secret -n stage -o yaml

# Test cluster connectivity
kubectl cluster-info --context=minikube-stage

# Check node health
kubectl describe nodes
```

## 📱 Telegram Integration

### Setup Process

1. **Create Telegram Bot**
   - Message @BotFather on Telegram
   - Send `/newbot` and follow instructions
   - Save the bot token

2. **Get Chat ID**
   - Add bot to your chat/group
   - Send a message to the bot
   - Use the setup script to get chat ID

3. **Configure Secrets**
   ```bash
   ./config/dev/scripts/setup-telegram-alerts.sh
   ```

### Message Examples

**Cluster Down Alert:**
```
🚨 Cluster Monitor - CRITICAL

System: Kubernetes Cluster Monitor
Time: 2025-06-16 23:45:00 UTC
Environment: Stage
Namespace: stage

Message:
Cluster health check failed 3 times, triggering automatic restart

Cluster Status:
• Minikube: ❌ Stopped
• API Server: ❌ Unreachable
• Nodes: 0/1 Ready
• Pods: 0/8 Running

Monitor Stats:
• Cluster Failures: 3/3
• API Failures: 3/3
• Node Failures: 2/3
• Last Restart: Never

Quick Actions:
kubectl get nodes --context=minikube-stage
kubectl get pods -n stage --context=minikube-stage
minikube status -p minikube-stage
```

**Hourly Resource Report:**
```
📊 Cluster Resource Report

Environment: Stage
Namespace: stage
Time: 2025-06-16 23:00:00 UTC

🖥️ Node Resources:
• CPU Capacity: 8 cores
• Memory Capacity: 32Gi

📊 Pod Resource Usage:
POD NAME                CPU    MEMORY
backend-abc123          245m   1247Mi
worker-def456           156m   892Mi
frontend-ghi789         89m    234Mi

🚀 Deployment Status:
• backend: 1/1 ready, 1 available
• worker: 3/3 ready, 3 available
• frontend: 1/1 ready, 1 available
• db: 1/1 ready, 1 available

💾 Storage Information:
• db-data-pvc: Bound (100Gi)
• cache-pvc: Bound (50Gi)
Node Disk Usage: 15.2GiB/63.9GiB (25% used)

✅ Overall Status: Healthy

Next Report: 00:00 UTC
```

## 🔄 Auto-Restart Functionality

### How It Works

1. **Health Monitoring**
   - Checks cluster every 30 seconds
   - Monitors API server, nodes, and pods
   - Tracks failure counts

2. **Failure Detection**
   - API server connectivity
   - Minikube process status
   - Node readiness
   - Pod health

3. **Restart Process**
   - Triggered after 3 consecutive failures
   - 1-hour cooldown between restarts
   - Graceful stop → wait → start
   - Health verification after restart

4. **Recovery Verification**
   - Checks API connectivity
   - Verifies node readiness
   - Monitors pod startup
   - Sends success/failure notifications

### Restart Scenarios

- Minikube process crash
- API server unreachable
- Node not ready
- Docker daemon issues
- Network connectivity problems

## 📈 Performance Monitoring

### Metrics Tracked

- **Resource Usage**: CPU, memory, disk per pod
- **Cluster Health**: API latency, node conditions
- **Application Health**: Pod restarts, readiness
- **Storage**: PVC usage, disk space
- **Network**: Service endpoints, ingress status

### Reports Include

- Top resource-consuming pods
- Deployment readiness status
- Storage utilization
- Recent warning events
- Performance recommendations

## 🛡️ Security Considerations

- Monitor runs with minimal required permissions
- Telegram tokens stored in Kubernetes secrets
- Host access limited to read-only mounts
- Auto-restart cooldown prevents abuse
- Audit logs for all operations

## 🔧 Customization

### Adding Custom Checks

1. Extend `cluster-monitor.sh` with new functions
2. Add alert types in `send_alert()` function
3. Configure thresholds in ConfigMap
4. Update Telegram message templates

### Integration Points

- Existing database backup system
- Application-specific health checks
- External monitoring systems
- Custom alert channels

## 📚 Related Documentation

- [Database Backup & Recovery System](backup-recovery-system.md)
- [Telegram Alerts Setup](../config/dev/scripts/setup-telegram-alerts.sh)
- [Deployment Scripts](../config/deploy/scripts/)
- [Kubernetes Configurations](../config/deploy/k8s/)

## ❓ Troubleshooting

### Common Issues

**Monitor not starting:**
```bash
# Check image availability
kubectl describe pod cluster-monitor-xxx -n stage

# Verify secrets
kubectl get secret telegram-secret -n stage

# Check permissions
kubectl describe clusterrolebinding cluster-monitor
```

**No Telegram alerts:**
```bash
# Test credentials
curl -s "https://api.telegram.org/bot$BOT_TOKEN/getMe"

# Check secret values
kubectl get secret telegram-secret -n stage -o jsonpath='{.data.bot-token}' | base64 -d
```

**Auto-restart not working:**
```bash
# Check systemd service
sudo systemctl status minikube-auto-start.service

# View service logs
sudo journalctl -u minikube-auto-start.service
```

**Resource reports missing:**
```bash
# Check cronjob
kubectl get cronjobs -n stage

# View job logs
kubectl logs job/resource-report-xxx -n stage
```

For additional support, check the logs and use the provided troubleshooting commands. 