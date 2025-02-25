# Firewall Configuration for Production Server

## Using UFW (Uncomplicated Firewall)

```bash
# Install UFW if not already installed
sudo apt-get install ufw

# Set default policies
sudo ufw default deny incoming
sudo ufw default allow outgoing

# Allow SSH
sudo ufw allow ssh
# If you changed the SSH port, use this instead:
# sudo ufw allow 2222/tcp

# Allow web traffic
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp

# Enable the firewall
sudo ufw enable

# Check status
sudo ufw status
```

## For Cloud Providers

If you're using a cloud provider like AWS, Azure, or GCP, you'll also need to configure their firewall/security groups to allow SSH traffic to your instance.

Example for AWS EC2:
1. Go to EC2 Dashboard → Security Groups
2. Select the security group attached to your instance
3. Add an inbound rule:
   - Type: SSH
   - Protocol: TCP
   - Port Range: 22 (or your custom port)
   - Source: Your IP address or range (for best security) 