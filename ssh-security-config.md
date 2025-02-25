# Secure SSH Configuration Guide

Edit your SSH configuration file at `/etc/ssh/sshd_config`:

```bash
# Disable root login
PermitRootLogin no

# Use key authentication only
PasswordAuthentication no
PubkeyAuthentication yes

# Restrict SSH access to specific users
AllowUsers yourdeployuser

# Change default port (optional but adds security by obscurity)
# Port 2222

# Limit login attempts
MaxAuthTries 3

# Enable logging
LogLevel VERBOSE
```

After making changes, restart the SSH service:
```bash
sudo systemctl restart ssh
```

## Setup SSH Keys for Deployment

1. Generate SSH keys on your development machine if you haven't already:
   ```bash
   ssh-keygen -t ed25519 -C "deployment-key"
   ```

2. Add your public key to the server's authorized_keys:
   ```bash
   ssh-copy-id -i ~/.ssh/id_ed25519.pub yourdeployuser@your-server-ip
   ```

3. For GitHub Actions, add the private key as a secret in your GitHub repository:
   - Go to your repository → Settings → Secrets
   - Add a new secret called `PROD_SSH_KEY` with the contents of your private key file
   - Make sure the host and username are also added as secrets (PROD_HOST, PROD_USERNAME) 