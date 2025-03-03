#!/bin/bash

# Script to help set up Google OAuth credentials for Atlantis Trading
# This script will guide you through the process of creating and configuring
# Google OAuth credentials for your application

echo "===== Google OAuth Setup Guide for Atlantis Trading ====="
echo "This script will help you set up Google OAuth for your application."
echo ""

# Generate a strong JWT secret if needed
if [ "$1" == "--generate-jwt" ]; then
  JWT_SECRET=$(openssl rand -base64 32)
  echo "Generated JWT_SECRET: $JWT_SECRET"
  echo "Add this as a secret named 'JWT_SECRET' in your GitHub repository."
  echo ""
fi

# Instructions for setting up Google OAuth
echo "===== Step 1: Create Google OAuth Credentials ====="
echo "1. Go to https://console.cloud.google.com/"
echo "2. Create a new project or select an existing one"
echo "3. Navigate to 'APIs & Services' > 'Credentials'"
echo "4. Click 'Create Credentials' > 'OAuth client ID'"
echo "5. Set up the OAuth consent screen if prompted"
echo "   - Choose 'External' user type for public apps"
echo "   - Fill in the required information (app name, user support email, developer contact)"
echo ""
echo "===== Step 2: Configure OAuth Client ====="
echo "6. For 'Application type', select 'Web application'"
echo "7. Add your domain as an authorized JavaScript origin:"
echo "   - https://atlantis.trading"
echo "8. Add your callback URL as an authorized redirect URI:"
echo "   - https://atlantis.trading/auth/google/callback"
echo "9. Click 'Create' to generate your credentials"
echo "10. Save the Client ID and Client Secret"
echo ""
echo "===== Step 3: Add Secrets to GitHub ====="
echo "11. Go to your GitHub repository"
echo "12. Navigate to 'Settings' > 'Secrets and variables' > 'Actions'"
echo "13. Click 'New repository secret'"
echo "14. Add the following secrets:"
echo "    - GOOGLE_CLIENT_ID: Your Google OAuth client ID"
echo "    - GOOGLE_CLIENT_SECRET: Your Google OAuth client secret"
echo "    - JWT_SECRET: A strong random string (use --generate-jwt flag with this script to generate one)"
echo ""
echo "===== Step 4: Deploy Your Application ====="
echo "15. After adding the secrets, trigger a new deployment:"
echo "    - Push a small change to your repository, or"
echo "    - Use the manual trigger in GitHub Actions"
echo ""
echo "===== Step 5: Verify the Configuration ====="
echo "16. After deploying, check your backend logs to ensure it's picking up the environment variables"
echo "17. Test the 'Sign in with Google' button on your login page"
echo "18. Monitor the network requests to see if the OAuth flow is working correctly"
echo ""
echo "For more information, refer to the Google OAuth documentation:"
echo "https://developers.google.com/identity/protocols/oauth2/web-server" 