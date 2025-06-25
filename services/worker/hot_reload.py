#!/usr/bin/env python3
"""
Hot reload script for the worker service
Starts the worker and can restart it when files change in development
"""

import os
import sys
import time
import subprocess  # nosec B404 - subprocess needed for hot reload functionality
from watchdog.observers import Observer
from watchdog.events import FileSystemEventHandler

class WorkerReloadHandler(FileSystemEventHandler):
    """Handler for file system events that triggers worker restarts"""
    
    def __init__(self, worker_process):
        self.worker_process = worker_process
        self.restart_pending = False
        
    def on_modified(self, event):
        if event.is_directory:
            return
            
        # Only restart for Python files
        if event.src_path.endswith('.py'):
            print(f"File changed: {event.src_path}")
            self.restart_worker()
    
    def restart_worker(self):
        if self.restart_pending:
            return
            
        self.restart_pending = True
        print("Restarting worker...")
        
        # Terminate current worker
        if self.worker_process and self.worker_process.poll() is None:
            self.worker_process.terminate()
            self.worker_process.wait()
        
        # Start new worker
        self.worker_process = start_worker()
        self.restart_pending = False

def start_worker():
    """Start the worker process"""
    print("Starting worker...")
    # Validate that we're running the expected Python executable and script
    python_exe = sys.executable
    worker_script = "worker.py"
    
    # Ensure the worker script exists and is a file
    if not os.path.isfile(worker_script):
        raise FileNotFoundError(f"Worker script {worker_script} not found")
    
    # Use secure subprocess call with validated arguments
    # Don't capture stdout/stderr so logs are visible
    return subprocess.Popen(  # nosec B603 - controlled subprocess call with validated arguments
        [python_exe, worker_script],
        # Remove stdout and stderr capture to allow logs to show
        # stdout=subprocess.PIPE,
        # stderr=subprocess.PIPE
    )

def main():
    """Main hot reload loop"""
    print("Starting worker with hot reload...")
    
    # Check if hot reload is enabled
    hot_reload_enabled = os.environ.get("HOT_RELOAD", "false").lower() == "true"
    
    if not hot_reload_enabled:
        print("Hot reload disabled, starting worker normally...")
        # Validate arguments before subprocess call
        python_exe = sys.executable
        worker_script = "worker.py"
        
        if not os.path.isfile(worker_script):
            raise FileNotFoundError(f"Worker script {worker_script} not found")
        
        # Use secure subprocess call with validated arguments
        subprocess.run(  # nosec B603 - controlled subprocess call with validated arguments
            [python_exe, worker_script],
            check=True
        )
        return
    
    # Start initial worker process
    worker_process = start_worker()
    
    # Set up file watcher
    event_handler = WorkerReloadHandler(worker_process)
    observer = Observer()
    observer.schedule(event_handler, "/app", recursive=True)
    observer.start()
    
    try:
        while True:
            # Check if worker process is still running
            if worker_process.poll() is not None:
                print("Worker process died, restarting...")
                worker_process = start_worker()
                event_handler.worker_process = worker_process
            
            time.sleep(1)
    except KeyboardInterrupt:
        print("Shutting down...")
        observer.stop()
        if worker_process and worker_process.poll() is None:
            worker_process.terminate()
            worker_process.wait()
    
    observer.join()

if __name__ == "__main__":
    main() 