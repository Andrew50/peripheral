FROM alpine:latest

# Install required packages
RUN apk update && apk add --no-cache \
    bash \
    curl \
    jq \
    docker-cli \
    coreutils \
    findutils \
    procps \
    util-linux \
    net-tools \
    && rm -rf /var/cache/apk/*

# Install kubectl
RUN curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" \
    && chmod +x kubectl \
    && mv kubectl /usr/local/bin/

# Create app directory
WORKDIR /app

# Copy monitoring scripts
COPY scripts/ /app/
COPY config/ /app/config/

# Make scripts executable
RUN chmod +x /app/*.sh

# Create log directory
RUN mkdir -p /var/log/monitor

# Set default command
CMD ["/app/cluster-monitor.sh"] 