#!/bin/bash
set -euo pipefail

# PostgreSQL Log Capture Script
# Captures logs from PostgreSQL stdout/stderr and makes them available for health monitoring

LOG_CAPTURE_FILE="/backups/postgresql-logs.log"
MAX_LOG_SIZE="10M"  # Maximum size before rotation
LOG_RETENTION_DAYS=7

# Create log capture file if it doesn't exist
touch "$LOG_CAPTURE_FILE"

# Create archive directory if it doesn't exist
mkdir -p /backups/archive

# Function to rotate logs if they get too large
rotate_logs() {
    if [ -f "$LOG_CAPTURE_FILE" ]; then
        local file_size=$(stat -c%s "$LOG_CAPTURE_FILE" 2>/dev/null || echo "0")
        local max_size_bytes=$(numfmt --from=iec "$MAX_LOG_SIZE" 2>/dev/null || echo "10485760")
        
        if [ "$file_size" -gt "$max_size_bytes" ]; then
            # Rotate the log file
            mv "$LOG_CAPTURE_FILE" "${LOG_CAPTURE_FILE}.$(date +%Y%m%d-%H%M%S)"
            
            # Remove old log files
            find /backups -name "postgresql-logs.log.*" -type f -mtime +$LOG_RETENTION_DAYS -delete 2>/dev/null || true
            
            # Create new log file
            touch "$LOG_CAPTURE_FILE"
        fi
    fi
}

# Function to capture logs from stdin
capture_logs() {
    while IFS= read -r line; do
        # Add timestamp if not already present
        if [[ "$line" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2} ]]; then
            echo "$line" >> "$LOG_CAPTURE_FILE"
        else
            echo "[$(date '+%Y-%m-%d %H:%M:%S')] $line" >> "$LOG_CAPTURE_FILE"
        fi
        
        # Also output to stdout for normal container logging
        echo "$line"
        
        # Rotate logs if needed
        rotate_logs
    done
}

# If this script is called with arguments, treat them as a command to run
if [ $# -gt 0 ]; then
    # Execute the command and capture its output
    "$@" 2>&1 | capture_logs
else
    # If no arguments, just capture from stdin
    capture_logs
fi 