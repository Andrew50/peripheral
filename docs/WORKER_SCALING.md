# Worker Scaling Guide

This document explains how to scale Python strategy execution workers in different environments.

## Overview

The system uses a Redis-based task queue pattern where multiple worker instances process Python strategy execution jobs concurrently. Each worker:

- Listens to the same Redis queue (`python_execution_queue`)
- Processes jobs sequentially within each worker instance
- Scales horizontally by adding more worker instances
- Provides fault tolerance and load distribution

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Backend API   │    │   Redis Queue   │    │   Worker Pool   │
│                 │───▶│                 │───▶│                 │
│ Submits Jobs    │    │ python_execution│    │ Worker 1        │
└─────────────────┘    │ _queue          │    │ Worker 2        │
                       │                 │    │ Worker 3        │
                       └─────────────────┘    │ ...             │
                                              └─────────────────┘
```

## Development Environment

### Current Setup
- **3 workers** by default (using Docker Compose `replicas: 3`)
- Each worker has 1GB memory limit and 1 CPU limit
- All workers share the same Redis queue
- Uses Docker Compose scaling feature

### Scaling Workers in Development

1. **View current workers:**
   ```bash
   docker-compose -f config/dev/docker-compose.yaml ps worker
   ```

2. **Scale workers using the script:**
   ```bash
   ./scripts/scale-workers.sh dev 5    # Scale to 5 workers
   ./scripts/scale-workers.sh dev 2    # Scale down to 2 workers
   ```

3. **Scale workers manually:**
   ```bash
   # Scale to 5 workers
   docker-compose -f config/dev/docker-compose.yaml up -d --scale worker=5
   
   # Scale down to 2 workers
   docker-compose -f config/dev/docker-compose.yaml up -d --scale worker=2
   ```

4. **Monitor workers:**
   ```bash
   ./scripts/monitor-workers.sh dev
   ```

## Production/Staging Environment

### Current Setup
- **Kubernetes Deployment** with 3 initial replicas
- **Horizontal Pod Autoscaler (HPA)** for automatic scaling
- **Resource limits:** 2GB memory, 2 CPU per worker
- **Auto-scaling range:** 2-10 workers

### Auto-Scaling Configuration

The HPA automatically scales workers based on:
- **CPU utilization:** Target 70%
- **Memory utilization:** Target 80%
- **Scale up:** 100% increase every 15 seconds (when needed)
- **Scale down:** 50% decrease every 60 seconds (with 5-minute stabilization)

### Manual Scaling

1. **Scale to specific number:**
   ```bash
   ./scripts/scale-workers.sh prod 8    # Scale to 8 workers
   ./scripts/scale-workers.sh stage 4   # Scale to 4 workers
   ```

2. **Check current status:**
   ```bash
   ./scripts/monitor-workers.sh prod
   ./scripts/monitor-workers.sh stage
   ```

3. **Direct kubectl commands:**
   ```bash
   # Scale deployment
   kubectl scale deployment worker --replicas=5 -n prod --context=minikube-prod
   
   # Check HPA status
   kubectl get hpa worker-hpa -n prod --context=minikube-prod
   
   # View worker pods
   kubectl get pods -l app=worker -n prod --context=minikube-prod
   ```

## Monitoring and Troubleshooting

### Key Metrics to Monitor

1. **Queue Length:**
   - Redis queue: `python_execution_queue`
   - Target: < 10 pending jobs under normal load

2. **Worker Performance:**
   - CPU usage per worker
   - Memory usage per worker
   - Job completion time

3. **Scaling Events:**
   - HPA scaling decisions
   - Pod creation/termination events

### Monitoring Commands

```bash
# Development
./scripts/monitor-workers.sh dev

# Production/Staging
./scripts/monitor-workers.sh prod
./scripts/monitor-workers.sh stage

# Check Redis queue directly (development)
docker exec $(docker-compose -f config/dev/docker-compose.yaml ps -q cache) \
  redis-cli LLEN python_execution_queue

# Check worker logs (development)
docker-compose -f config/dev/docker-compose.yaml logs -f worker

# Check worker logs (production)
kubectl logs -l app=worker -n prod --context=minikube-prod -f
```

### Troubleshooting Common Issues

1. **Workers not processing jobs:**
   - Check Redis connectivity
   - Verify queue name configuration
   - Check worker logs for errors

2. **High queue length:**
   - Scale up workers manually
   - Check if workers are stuck on long-running jobs
   - Verify HPA is functioning

3. **Workers crashing:**
   - Check memory limits
   - Review error logs
   - Verify resource constraints

4. **Slow job processing:**
   - Monitor individual job execution times
   - Check for resource contention
   - Consider increasing worker resources

## Performance Tuning

### Resource Allocation

**Development:**
- Memory: 1GB per worker (suitable for testing)
- CPU: 1 core per worker

**Production:**
- Memory: 2GB per worker (handles complex strategies)
- CPU: 2 cores per worker (parallel processing within jobs)

### Scaling Guidelines

**When to scale up:**
- Queue length consistently > 5
- Average job wait time > 30 seconds
- CPU/Memory utilization > 80%

**When to scale down:**
- Queue length consistently = 0
- CPU/Memory utilization < 30%
- Cost optimization needed

### Optimal Worker Count

- **Minimum:** 2 workers (fault tolerance)
- **Typical:** 3-5 workers (balanced cost/performance)
- **Peak load:** 8-10 workers (high throughput)
- **Maximum:** 10 workers (resource limits)

## Configuration Files

- **Development:** `config/dev/docker-compose.yaml`
- **Kubernetes:** `config/deploy/k8s/worker.yaml`
- **Scaling script:** `scripts/scale-workers.sh`
- **Monitoring script:** `scripts/monitor-workers.sh`
- **Deployment:** `.github/workflows/deploy.yml`

## Best Practices

1. **Always maintain at least 2 workers** for fault tolerance
2. **Monitor queue length** to detect scaling needs
3. **Use HPA in production** for automatic scaling
4. **Set appropriate resource limits** to prevent resource starvation
5. **Test scaling** in staging before production changes
6. **Monitor worker logs** for performance issues
7. **Use the monitoring scripts** for regular health checks 