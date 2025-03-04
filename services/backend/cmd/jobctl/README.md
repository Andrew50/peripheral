# JobCTL - Backend Job Control CLI

JobCTL is a command-line interface for managing and monitoring backend jobs. It allows you to:

- List all available jobs
- Check job status
- Run jobs manually
- Monitor the job queue

## Usage

### From Outside the Container

Use the provided `jobctl` script in the backend directory:

```bash
./backend/jobctl [command] [arguments]
```

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

### Check Queue Status

Check the status of the job queue:

```bash
jobctl queue
```

### Help

Show help information:

```bash
jobctl help
```

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