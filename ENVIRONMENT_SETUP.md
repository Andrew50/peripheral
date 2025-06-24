# Environment Setup Guide

This guide will help you set up the required environment variables to run the application properly.

## Quick Fix for Current Errors

The errors you're seeing are caused by missing environment variables. Here's how to fix them:

### 1. Create Environment File

Create a `.env` file in your project root with the following content:

```bash
# Google OAuth Configuration (Required for authentication)
GOOGLE_CLIENT_ID=your_google_client_id_here
GOOGLE_CLIENT_SECRET=your_google_client_secret_here
GOOGLE_REFRESH_TOKEN=your_google_refresh_token_here

# Email Configuration
EMAIL_FROM_ADDRESS=your_email@example.com
SMTP_HOST=smtp.gmail.com
SMTP_PORT=465

# API Keys (Get these from respective services)
GEMINI_FREE_KEYS=your_gemini_api_keys_here
POLYGON_API_KEY=your_polygon_api_key
X_API_KEY=your_x_api_key
TWITTER_API_IO_KEY=your_twitter_api_key
OPENAI_API_KEY=your_openai_api_key

# Platform (optional)
PLATFORM=linux/amd64
```

### 2. Set Up Google OAuth (Required)

To fix the "Google OAuth is not configured properly" error:

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select an existing one
3. Enable the Google+ API and Google OAuth2 API
4. Go to "Credentials" → "Create Credentials" → "OAuth 2.0 Client IDs"
5. Set application type to "Web application"
6. Add authorized redirect URIs:
   - `http://localhost:5173/auth/google/callback` (for development)
   - Your production domain callback URL
7. Copy the Client ID and Client Secret to your `.env` file

### 3. Start the Services

Make sure all services are running:

```bash
# From the project root
cd config/dev
docker-compose up -d
```

### 4. Verify Backend is Running

Check that the backend is accessible:

```bash
curl http://localhost:5058/health
```

## Error-Specific Fixes

### "Authentication required" Errors
- These occur when the app tries to make authenticated requests without valid tokens
- Fixed by setting up Google OAuth properly
- The improved error handling will now gracefully fall back to default values

### "Cannot read properties of null (reading 'focus')" Error
- Fixed by adding null checks before calling `.focus()` on DOM elements
- The chat input will now safely handle cases where the element isn't yet mounted

### "Google OAuth is not configured properly" Error
- Fixed by setting `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET` environment variables
- Make sure these are valid credentials from Google Cloud Console

## Testing the Fix

1. Set up the environment variables as described above
2. Restart the docker services: `docker-compose down && docker-compose up -d`
3. Open the application at `http://localhost:5173`
4. Try to sign in with Google - it should now work properly

## Additional Notes

- The application will work in read-only mode even without authentication
- Some features require valid API keys (Gemini, OpenAI, etc.)
- For production deployment, make sure to set these variables in your deployment environment 