#!/usr/bin/env python3
"""
Hot reload script for the Python worker
Monitors Python files and restarts the worker when changes are detected
"""

import os
import signal
import subprocess
import sys
import time

from watchdog.events import FileSystemEventHandler
from watchdog.observers import Observer


class WorkerRestartHandler(FileSystemEventHandler):
    """Handler for file system events that restarts the worker"""
    
    def __init__(self):
        self.process = None
        self.last_restart = 0
        self.debounce_seconds = 1  # Prevent rapid restarts
        self.start_worker()
    
    def start_worker(self):
        """Start the worker process"""
        if self.process:
            self.stop_worker()
        
        print("🚀 Starting Python worker...")
        self.process = subprocess.Popen([sys.executable, 'worker.py'])
        print(f"✅ Worker started with PID: {self.process.pid}")
    
    def stop_worker(self):
        """Stop the worker process"""
        if self.process and self.process.poll() is None:
            print("🛑 Stopping worker...")
            self.process.terminate()
            try:
                self.process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                print("⚠️  Worker didn't stop gracefully, force killing...")
                self.process.kill()
                self.process.wait()
            print("✅ Worker stopped")
    
    def on_modified(self, event):
        """Handle file modification events"""
        if event.is_directory:
            return
        
        # Only restart on Python file changes
        if event.src_path.endswith('.py'):
            # Debounce rapid changes
            current_time = time.time()
            if current_time - self.last_restart < self.debounce_seconds:
                return
            
            self.last_restart = current_time
            print(f"📝 File changed: {event.src_path}")
            print("🔄 Restarting worker...")
            self.start_worker()

def main():
    """Main hot reload function"""
    print("🔥 Hot reload enabled for Python worker")
    print("📁 Monitoring directory: /app")
    print("🔍 Watching for: *.py files")
    
    # Create event handler and observer
    event_handler = WorkerRestartHandler()
    observer = Observer()
    observer.schedule(event_handler, '/app', recursive=True)
    
    # Handle graceful shutdown
    def signal_handler(signum, frame):
        print("\n🛑 Shutting down hot reload...")
        event_handler.stop_worker()
        observer.stop()
        sys.exit(0)
    
    signal.signal(signal.SIGINT, signal_handler)
    signal.signal(signal.SIGTERM, signal_handler)
    
    # Start monitoring
    observer.start()
    
    try:
        while True:
            time.sleep(1)
    except KeyboardInterrupt:
        pass
    finally:
        observer.stop()
        event_handler.stop_worker()
    
    observer.join()

if __name__ == "__main__":
    main() 