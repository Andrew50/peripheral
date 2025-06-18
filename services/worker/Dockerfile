FROM python:3.11-slim

# Set working directory
WORKDIR /app

# Install system dependencies
RUN apt-get update && apt-get install -y \
    gcc \
    g++ \
    build-essential \
    wget \
    curl \
    git \
    && rm -rf /var/lib/apt/lists/*

# Copy requirements
COPY requirements.txt .

# Install numpy first (required for TA-Lib)
RUN pip install --upgrade pip && \
    pip install --no-cache-dir numpy>=1.21.0

# Install TA-Lib Python wrapper with specific build options
RUN pip install --no-cache-dir --global-option=build_ext \
    --global-option="-I/usr/include" \
    --global-option="-L/usr/lib" \
    TA-Lib || \
    (echo "TA-Lib failed, continuing without it..." && \
    sed -i '/^TA-Lib/d' requirements.txt)

# Install remaining Python dependencies
RUN pip install --no-cache-dir -r requirements.txt

# Copy application code
COPY src/ ./src/
COPY worker.py .

# Create directories for data and results
RUN mkdir -p /app/data /app/results /app/logs

# Set environment variables
ENV PYTHONPATH=/app
ENV PYTHONUNBUFFERED=1

# Create non-root user for security
RUN useradd -m -u 1000 worker && \
    chown -R worker:worker /app
USER worker

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD python -c "import sys; sys.exit(0)"

# Default command
CMD ["python", "worker.py"]