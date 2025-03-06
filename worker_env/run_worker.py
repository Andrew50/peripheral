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
