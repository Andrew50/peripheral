# Adding SSH Access to a Docker Container (For Development/Debugging Only)

## 1. Modify the Dockerfile for the service

For example, to add SSH to your backend container, add this to backend/Dockerfile.prod:

```Dockerfile
# ... existing Dockerfile content ...

# Install SSH server
RUN apk add --no-cache openssh-server \
    && mkdir -p /run/sshd \
    && echo "root:password" | chpasswd \
    && sed -i 's/#PermitRootLogin prohibit-password/PermitRootLogin yes/' /etc/ssh/sshd_config

# Start SSH server along with your application
COPY entrypoint-with-ssh.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
```

## 2. Create an entrypoint script

Create a file at backend/entrypoint-with-ssh.sh:

```bash
#!/bin/sh
# Start SSH daemon
/usr/sbin/sshd

# Start your original application
exec "$@"
```

## 3. Update your docker-compose file to expose the SSH port

In docker-compose.registry.yaml:

```yaml
services:
  backend:
    image: yourrepo/backend:latest
    restart: unless-stopped
    ports:
      - "2222:22"  # Map container's SSH port to host port 2222
    networks:
      - compose_network
```

## Important Security Considerations

This approach is NOT recommended for production environments because:

1. It increases the attack surface of your application
2. Container SSH access should typically only be needed for debugging
3. For production, consider using container orchestration tools' built-in capabilities:
   - `docker exec` for Docker
   - `kubectl exec` for Kubernetes

For production management, always prefer SSH access to the host, not to individual containers. 