# Database Backup & Auto-Recovery System

This document describes the comprehensive backup and automatic recovery system implemented for the PostgreSQL database.

## 🎯 Overview

The system provides:
- **Automated backups** twice daily (6 AM and 6 PM UTC)
- **Continuous health monitoring** with corruption detection
- **Automatic recovery** when database issues are detected
- **Backup verification** and retention management
- **Comprehensive logging** and alerting

## 🏗️ Architecture

### Components

1. **Main Database Pod** (`db`)
   - TimescaleDB with PostgreSQL 17
   - Enhanced with backup and recovery scripts
   - Connected to persistent storage

2. **Health Monitor** (`db-health-monitor`)
   - Continuous monitoring (every 60 seconds)
   - Corruption detection algorithms
   - Automatic recovery triggering

3. **Backup CronJobs**
   - `db-backup-cronjob`: Creates backups twice daily
   - `db-backup-verification`: Verifies backup integrity daily
   - `db-backup-cleanup`: Manages retention (30 days)

4. **Storage**
   - `db-pvc`: Main database storage (250Gi)
   - `db-backups-pvc`: Backup storage (100Gi)

## 📋 Backup System

### Schedule
- **Main Backups**: 6:00 AM and 6:00 PM UTC daily
- **Verification**: 3:00 AM UTC daily
- **Cleanup**: 2:00 AM UTC daily

### Backup Features
- Full database dumps with schema and data
- Compression (gzip) for space efficiency
- Integrity verification before storage
- Metadata manifests with backup information
- 30-day retention policy

### Backup Process
1. Database connectivity check
2. Schema and data extraction via `pg_dump`
3. Content verification
4. Compression and storage
5. Manifest creation
6. Old backup cleanup

## 🔍 Health Monitoring

### Detection Mechanisms
- **Connection Tests**: `pg_isready` checks
- **Functionality Tests**: Basic SQL operations
- **Corruption Indicators**: Log pattern matching
- **Schema Integrity**: Migration system checks

### Monitored Indicators
- Database system interruption during recovery
- Segmentation faults
- Startup process termination
- File system errors
- Invalid page headers
- Checksum verification failures

### Failure Thresholds
- **Health Check Interval**: 60 seconds
- **Failure Threshold**: 3 consecutive failures
- **Recovery Cooldown**: 1 hour between attempts

## 🚨 Auto-Recovery System

### Recovery Strategies

#### 1. Backup Restoration
- **Trigger**: Database corruption detected with valid backup available
- **Process**:
  1. Find most recent valid backup
  2. Create temporary database
  3. Restore backup to temporary database
  4. Verify restored database integrity
  5. Replace corrupted database
  6. Re-enable connections

#### 2. Fresh Reset
- **Trigger**: No valid backups available
- **Process**:
  1. Initialize fresh PostgreSQL cluster
  2. Run initialization scripts
  3. Apply database migrations
  4. Create clean schema

### Recovery Flow
```
Health Check Failure
        ↓
Increment Failure Count
        ↓
Reached Threshold? (3 failures)
        ↓
Find Latest Valid Backup
        ↓
Backup Available?
   ↓         ↓
  Yes        No
   ↓         ↓
Restore    Fresh Reset
Backup     Database
   ↓         ↓
Verify Recovery Success
        ↓
Reset Failure Count
        ↓
Resume Monitoring
```

## 🚀 Deployment

### Prerequisites
- Kubernetes cluster with kubectl access
- Docker for building images
- Environment variables set:
  - `DOCKER_USERNAME`
  - `DOCKER_TAG`

### Installation
```bash
# From project root directory
./config/dev/scripts/deploy-backup-system.sh
```

This script will:
1. Build updated database image with backup scripts
2. Deploy backup system infrastructure
3. Set up persistent storage
4. Start health monitoring
5. Create initial test backup

## 🔧 Management

### Monitoring Commands
```bash
# Check backup job status
kubectl get cronjobs
kubectl get jobs

# View health monitor logs
kubectl logs deployment/db-health-monitor --tail=20

# Check recent backups
kubectl exec -it $(kubectl get pods -l app=db-health-monitor -o jsonpath='{.items[0].metadata.name}') -- ls -la /backups/

# View backup logs
kubectl logs job/db-backup-cronjob-[timestamp]
```

### Manual Operations
```bash
# Trigger manual backup
kubectl create job manual-backup-$(date +%s) --from=cronjob/db-backup-cronjob

# Manual recovery from specific backup
kubectl exec -it $(kubectl get pods -l app=db -o jsonpath='{.items[0].metadata.name}') -- /app/recovery-restore.sh /backups/backup_[timestamp].sql.gz

# Check health monitor status
kubectl exec $(kubectl get pods -l app=db-health-monitor -o jsonpath='{.items[0].metadata.name}') -- tail -20 /backups/health-monitor.log

# View recovery alerts
kubectl exec $(kubectl get pods -l app=db-health-monitor -o jsonpath='{.items[0].metadata.name}') -- ls /backups/alert-*
```

### Configuration
Key settings can be modified via environment variables:
- `HEALTH_CHECK_INTERVAL`: Monitoring frequency (default: 60s)
- `MAX_FAILURE_COUNT`: Failures before recovery (default: 3)
- `RECOVERY_COOLDOWN`: Time between recovery attempts (default: 3600s)

## 📊 Logging & Alerting

### Telegram Notifications
The system supports real-time Telegram alerts for all critical events:

#### Alert Types
- 🚨 **CRITICAL**: Database failures requiring immediate attention
- ❌ **ERROR**: Backup failures or system errors  
- ⚠️ **WARNING**: Potential issues or warnings
- ✅ **SUCCESS**: Backup completions and recovery successes
- ℹ️ **INFO**: General information and status updates

#### Setup Telegram Alerts
```bash
# Run the interactive setup script
./config/dev/scripts/setup-telegram-alerts.sh
```

This script will:
1. Guide you through creating a Telegram bot
2. Help you get the chat ID
3. Test the connection
4. Create Kubernetes secrets
5. Deploy the configuration

#### Telegram Alert Examples

**Backup Success:**
```
✅ Database Backup - SUCCESS

System: PostgreSQL Backup System
Time: 2025-06-13 18:00:02 UTC
Environment: Development

Message:
Database backup completed successfully!

📊 Backup Details:
• File: backup_20250613_180002.sql.gz
• Size: 201.8MiB
• Tables: 20
• Database Size: 1557 MB
• Compression: 71%

📅 Schedule: Next backup at 6 AM/6 PM UTC
```

**Health Alert:**
```
🚨 Database Alert - CRITICAL

System: PostgreSQL Backup & Recovery
Time: 2025-06-13 19:33:29 UTC
Environment: Development

Message:
Database health check failed 3 times, triggering automatic recovery

Health Status:
• Database Connection: ❌ Failed
• Failure Count: 3/3
• Last Recovery: Never

Quick Actions:
kubectl logs deployment/db-health-monitor --tail=20
kubectl get pods -l app=db
```

### Log Files
- `/backups/backup.log`: Backup operation logs
- `/backups/health-monitor.log`: Health monitoring logs
- `/backups/recovery.log`: Recovery operation logs
- `/backups/verification.log`: Backup verification logs
- `/backups/retention.log`: Cleanup operation logs

### Alert Files
- `/backups/alert-[timestamp]`: Alert notifications
- `/backups/recovery-in-progress`: Recovery status flag

### Log Retention
- **Backup logs**: 7 days
- **Alert files**: 3 days
- **Recovery logs**: Permanent

## 🛡️ Security Considerations

- Database passwords stored in Kubernetes secrets
- Backup files compressed and stored securely
- Health monitor has minimal required permissions
- Recovery scripts run with database user privileges

## 📈 Performance Impact

- **Backup Jobs**: Run during low-usage periods
- **Health Monitor**: Minimal CPU/memory footprint
- **Recovery**: Temporary performance impact during restoration
- **Storage**: Backup compression reduces space requirements

## 🔄 Disaster Recovery

### RTO (Recovery Time Objective)
- **Automatic Recovery**: 5-10 minutes
- **Manual Recovery**: 15-30 minutes
- **Fresh Reset**: 10-15 minutes

### RPO (Recovery Point Objective)
- **Maximum Data Loss**: 12 hours (between backups)
- **Typical Data Loss**: 6 hours (average between backups)

### Emergency Procedures
1. **Complete System Failure**: Use latest backup for restoration
2. **Corrupted Backups**: Use backup verification logs to find valid backups
3. **Storage Issues**: Scale up persistent volumes
4. **Health Monitor Failure**: Manual monitoring and recovery

## 🚨 Troubleshooting

### Common Issues

#### Backup Failures
```bash
# Check backup job logs
kubectl logs job/db-backup-cronjob-[timestamp]

# Verify database connectivity
kubectl exec $(kubectl get pods -l app=db -o jsonpath='{.items[0].metadata.name}') -- pg_isready -U postgres

# Check storage space
kubectl exec $(kubectl get pods -l app=db-health-monitor -o jsonpath='{.items[0].metadata.name}') -- df -h /backups
```

#### Health Monitor Issues
```bash
# Check monitor status
kubectl get deployment db-health-monitor

# View monitor logs
kubectl logs deployment/db-health-monitor --tail=50

# Restart monitor
kubectl rollout restart deployment/db-health-monitor
```

#### Recovery Failures
```bash
# Check recovery logs
kubectl exec $(kubectl get pods -l app=db-health-monitor -o jsonpath='{.items[0].metadata.name}') -- tail -50 /backups/recovery.log

# Manual recovery
./config/dev/scripts/db-quick-fix.sh
```

### Emergency Contacts
- Review health monitor alerts in `/backups/alert-*`
- Check Kubernetes events: `kubectl get events --sort-by=.metadata.creationTimestamp`
- Database logs: `kubectl logs $(kubectl get pods -l app=db -o jsonpath='{.items[0].metadata.name}')`

## 📚 Related Documentation
- [Database Configuration](../services/db/README.md)
- [Migration System](../services/db/README.md#migrations)
- [Kubernetes Deployment](../config/deploy/README.md) 