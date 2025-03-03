#!/bin/bash

# Script to help set up required GitHub secrets for the application
# This script will guide you through creating the necessary secrets

echo "===== GitHub Secrets Setup Guide ====="
echo "This script will help you set up the required secrets for your GitHub repository."
echo "You will need to manually add these secrets in your GitHub repository settings."
echo ""

# Generate a strong JWT secret
JWT_SECRET=$(openssl rand -base64 32)
echo "Generated JWT_SECRET: $JWT_SECRET"
echo "Add this as a secret named 'JWT_SECRET' in your GitHub repository."
echo ""

# Prompt for Google OAuth credentials
echo "===== Google OAuth Setup ====="
echo "You need to set up a Google OAuth client in the Google Cloud Console:"
echo "1. Go to https://console.cloud.google.com/"
echo "2. Create a new project or select an existing one"
echo "3. Navigate to 'APIs & Services' > 'Credentials'"
echo "4. Click 'Create Credentials' > 'OAuth client ID'"
echo "5. Set up the OAuth consent screen if prompted"
echo "6. For 'Application type', select 'Web application'"
echo "7. Add authorized JavaScript origins (your app's domain)"
echo "8. Add authorized redirect URIs (e.g., https://yourdomain.com/auth/google/callback)"
echo "9. Click 'Create'"
echo ""

read -p "Enter your Google Client ID: " GOOGLE_CLIENT_ID
read -p "Enter your Google Client Secret: " GOOGLE_CLIENT_SECRET

echo ""
echo "===== Summary of Secrets to Add ====="
echo "Add the following secrets to your GitHub repository:"
echo "1. JWT_SECRET: $JWT_SECRET"
echo "2. GOOGLE_CLIENT_ID: $GOOGLE_CLIENT_ID"
echo "3. GOOGLE_CLIENT_SECRET: $GOOGLE_CLIENT_SECRET"
echo ""
echo "Make sure you also have these existing secrets set up:"
echo "- DOCKER_USERNAME"
echo "- DOCKER_TOKEN"
echo "- DB_ROOT_PASSWORD"
echo "- REDIS_PASSWORD"
echo "- POLYGON_API_KEY"
echo ""
echo "To add these secrets:"
echo "1. Go to your repository on GitHub"
echo "2. Navigate to Settings > Secrets and variables > Actions"
echo "3. Click 'New repository secret'"
echo "4. Add each secret with its name and value"
echo ""
echo "After adding all secrets, your GitHub Actions workflow will be able to deploy with proper authentication." 