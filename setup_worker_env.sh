#!/bin/bash

# Create directories for our project
mkdir -p worker_env
cd worker_env

# Create a requirements.txt file
cat > requirements.txt << 'REQ'
# Core dependencies
requests==2.31.0
numpy==1.26.3
pandas==2.2.0

# Worker-specific dependencies
celery==5.3.6
redis==5.0.1

# Development tools
pytest==7.4.3
black==23.12.1
REQ

# Create a basic worker module
mkdir -p worker
cat > worker/__init__.py << 'INIT'
# Worker package
INIT

cat > worker/tasks.py << 'TASKS'
# Define your worker tasks here
import time

def process_task(data):
    """Process a task with the given data."""
    print(f"Processing task with data: {data}")
    # Simulate work
    time.sleep(1)
    return {"status": "completed", "result": f"Processed {data}"}
TASKS

# Create a simple script to run the worker
cat > run_worker.py << 'WORKER'
#!/usr/bin/env python3
"""
Simple worker script.
"""
from worker.tasks import process_task

def main():
    test_data = {"id": 123, "action": "test"}
    result = process_task(test_data)
    print(f"Task completed with result: {result}")

if __name__ == "__main__":
    main()
WORKER

chmod +x run_worker.py

# Create a README
cat > README.md << 'README'
# Worker Environment

This is a basic worker environment setup.

## Installation

To install dependencies in your environment:

```bash
pip install -r requirements.txt
```

## Running

Execute the worker script:

```bash
python run_worker.py
```
README

echo "Worker environment created in $(pwd)"
echo "To use it:"
echo "1. Create a virtual environment outside Cursor if possible"
echo "2. Install the dependencies: pip install -r requirements.txt"
echo "3. Run the worker: python run_worker.py" 