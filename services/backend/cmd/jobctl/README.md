# JobCTL - Backend Job Control CLI

JobCTL is a command-line interface for managing and monitoring backend jobs. It allows you to:

- List all available jobs
- Check job status
- Run jobs manually
- Monitor the job queue
- Monitor worker task execution

## Usage

### From Outside the Container

Use the provided `jobctl` script in the backend directory:

```bash
./services/backend/jobctl [command] [arguments]
```

This script sets the necessary environment variables to connect to the database and Redis on localhost. If your Docker setup uses different port mappings, you may need to modify the script.

By default, it assumes:
- PostgreSQL is accessible on localhost:5432
- Redis is accessible on localhost:6379

If your Docker Compose setup uses different port mappings, edit the `services/backend/jobctl` script to match your configuration.

### From Inside the Container

If you're already inside the backend container, you can use the `jobctl` command directly:

```bash
jobctl [command] [arguments]
```

## Available Commands

### List Jobs

List all available jobs:

```bash
jobctl list
```

### Check Job Status

Check the status of all jobs:

```bash
jobctl status
```

Check the status of a specific job:

```bash
jobctl status [job_name]
```

### Run a Job

Run a specific job:

```bash
jobctl run [job_name]
```

When running a job that queues tasks for the worker, the CLI will automatically:
1. Detect if new tasks were added to the queue
2. Display information about the queued tasks
3. Monitor the tasks until they complete or timeout
4. Show the results or errors from the worker

### Check Queue Status

Check the status of the job queue:

```bash
jobctl queue
```

### Monitor a Task

Monitor a specific task by its ID:

```bash
jobctl monitor [task_id]
```

This is useful when you have a task ID from a previous operation and want to check its status and output.

### Help

Show help information:

```bash
jobctl help
```

## Troubleshooting

If you encounter connection issues when running outside the container:

1. Verify that your Docker containers are running:
   ```bash
   docker ps | grep db
   docker ps | grep cache
   ```

2. Check that the ports are correctly mapped in your Docker Compose configuration:
   ```bash
   docker-compose ps
   ```

3. Ensure the environment variables in the `jobctl` script match your Docker setup.

## Examples

List all available jobs:
```bash
./backend/jobctl list
```

Run the market metrics update job:
```bash
./backend/jobctl run updateMarketMetrics
```

Check the status of the sector update job:
```bash
./backend/jobctl status updateSectors
```

Check the job queue:
```bash
./backend/jobctl queue
```

Monitor a specific task:
```bash
./backend/jobctl monitor 123e4567-e89b-12d3-a456-426614174000
``` 
