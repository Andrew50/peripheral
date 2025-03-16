# Define your worker tasks here
import time

def process_task(data):
    """Process a task with the given data."""
    print(f"Processing task with data: {data}")
    # Simulate work
    time.sleep(1)
    return {"status": "completed", "result": f"Processed {data}"}
