# Setting Up a Bastion Host for Secure SSH Access

A bastion host is a server specifically designed for SSH access, adding an extra layer of security between external users and your production servers.

## Architecture

```
Internet → Bastion Host → Production Server with Docker
```

## Steps to Set Up a Bastion Host

1. **Create a dedicated instance** for the bastion host (small instance type is sufficient)

2. **Harden the bastion host**:
   ```bash
   # Minimal OS installation
   # Only install essentials
   sudo apt-get update
   sudo apt-get install -y openssh-server fail2ban ufw

   # Configure SSH hardening as described in the SSH security guide
   ```

3. **Configure firewall on the bastion host**:
   ```bash
   # Allow SSH from your development IPs only
   sudo ufw allow from your-ip-address to any port 22
   
   # Enable firewall
   sudo ufw enable
   ```

4. **Configure your production server firewall**:
   ```bash
   # Only allow SSH from the bastion host
   sudo ufw allow from bastion-host-ip to any port 22
   
   # Continue allowing web traffic
   sudo ufw allow 80/tcp
   sudo ufw allow 443/tcp
   ```

5. **Set up SSH key forwarding** on your local machine:
   
   Add this to your ~/.ssh/config:
   ```
   Host bastion
       HostName bastion-ip-address
       User username
       IdentityFile ~/.ssh/id_ed25519
       ForwardAgent yes

   Host production
       HostName production-ip-address
       User produser
       ProxyJump bastion
   ```

6. **Update your GitHub Actions workflow**:
   ```yaml
   - name: Setup SSH agent for key forwarding
     run: |
       eval $(ssh-agent -s)
       echo "${{ secrets.PROD_SSH_KEY }}" | ssh-add -

   - name: Deploy to production server via bastion
     run: |
       ssh -o ProxyJump=user@bastion-ip -o StrictHostKeyChecking=no produser@prod-ip '
         cd /path/to/your/app
         git pull origin prod
         docker-compose -f docker-compose.registry.yaml pull
         docker-compose -f docker-compose.registry.yaml down
         docker-compose -f docker-compose.registry.yaml up -d
       '
   ```

## Benefits

1. Reduced attack surface on your production server
2. Centralized point for SSH access and logs
3. Can implement additional security measures like 2FA on the bastion host
4. Allows for more restrictive firewall rules on production servers 