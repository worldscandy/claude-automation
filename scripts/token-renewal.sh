#!/bin/bash
set -e

echo "🔐 Claude CLI Token Extraction Tool"
echo "=================================="

# Check if Claude CLI is authenticated
if ! claude --version >/dev/null 2>&1; then
    echo "❌ Claude CLI not found or not working"
    exit 1
fi

# Check if .claude.json exists
if [ ! -f "$HOME/.claude.json" ]; then
    echo "❌ Claude CLI not authenticated. Please run 'claude login' first."
    exit 1
fi

# Check if .credentials.json exists
if [ ! -f "$HOME/.claude/.credentials.json" ]; then
    echo "❌ Credentials file not found. Please run 'claude login' first."
    exit 1
fi

echo "✅ Claude CLI authentication files found"
echo

# Extract values from .claude.json
echo "📋 Extracting user information from .claude.json..."

# Use jq if available, otherwise use basic text processing
if command -v jq >/dev/null 2>&1; then
    USER_ID=$(jq -r '.userID' "$HOME/.claude.json")
    ACCOUNT_UUID=$(jq -r '.oauthAccount.accountUuid' "$HOME/.claude.json")
    EMAIL=$(jq -r '.oauthAccount.emailAddress' "$HOME/.claude.json")
    ORG_UUID=$(jq -r '.oauthAccount.organizationUuid' "$HOME/.claude.json")
    ORG_ROLE=$(jq -r '.oauthAccount.organizationRole' "$HOME/.claude.json")
    ORG_NAME=$(jq -r '.oauthAccount.organizationName' "$HOME/.claude.json")
    MAX_TIER=$(jq -r '.claudeMaxTier' "$HOME/.claude.json")
else
    echo "⚠️  jq not available, using basic text extraction..."
    USER_ID=$(grep '"userID"' "$HOME/.claude.json" | cut -d'"' -f4)
    ACCOUNT_UUID=$(grep '"accountUuid"' "$HOME/.claude.json" | cut -d'"' -f4)
    EMAIL=$(grep '"emailAddress"' "$HOME/.claude.json" | cut -d'"' -f4)
    ORG_UUID=$(grep '"organizationUuid"' "$HOME/.claude.json" | cut -d'"' -f4)
    ORG_ROLE=$(grep '"organizationRole"' "$HOME/.claude.json" | cut -d'"' -f4)
    ORG_NAME=$(grep '"organizationName"' "$HOME/.claude.json" | cut -d'"' -f4)
    MAX_TIER=$(grep '"claudeMaxTier"' "$HOME/.claude.json" | cut -d'"' -f4)
fi

# Extract values from .credentials.json
echo "🔑 Extracting token information from .credentials.json..."

if command -v jq >/dev/null 2>&1; then
    ACCESS_TOKEN=$(jq -r '.claudeAiOauth.accessToken' "$HOME/.claude/.credentials.json")
    EXPIRES_AT=$(jq -r '.claudeAiOauth.expiresAt' "$HOME/.claude/.credentials.json")
else
    ACCESS_TOKEN=$(grep '"accessToken"' "$HOME/.claude/.credentials.json" | cut -d'"' -f4)
    EXPIRES_AT=$(grep '"expiresAt"' "$HOME/.claude/.credentials.json" | cut -d':' -f2 | tr -d ' ,')
fi

# Validate extracted values
if [ -z "$ACCESS_TOKEN" ] || [ -z "$EXPIRES_AT" ] || [ -z "$USER_ID" ]; then
    echo "❌ Failed to extract required values. Please check authentication files."
    exit 1
fi

# Convert timestamp to human readable date
if command -v date >/dev/null 2>&1; then
    EXPIRES_DATE=$(date -d "@$((EXPIRES_AT / 1000))" 2>/dev/null || echo "Unknown")
else
    EXPIRES_DATE="Unknown"
fi

echo "✅ Token extraction completed"
echo

# Display extracted information
echo "📊 Token Information:"
echo "  Access Token: ${ACCESS_TOKEN:0:20}..."
echo "  Expires At: $EXPIRES_AT ($EXPIRES_DATE)"
echo "  User ID: $USER_ID"
echo "  Account UUID: $ACCOUNT_UUID"
echo "  Email: $EMAIL"
echo

# Generate .env-secret format
echo "📝 Copy the following to your .env-secret file:"
echo "================================================"
cat << EOF
# Claude Authentication Configuration
# Generated on $(date)

# OAuth Access Token from ~/.claude/.credentials.json
CLAUDE_ACCESS_TOKEN=$ACCESS_TOKEN

# Token expiration timestamp (Unix milliseconds)
CLAUDE_TOKEN_EXPIRES_AT=$EXPIRES_AT

# User information from ~/.claude.json
CLAUDE_USER_ID=$USER_ID
CLAUDE_ACCOUNT_UUID=$ACCOUNT_UUID
CLAUDE_EMAIL=$EMAIL
CLAUDE_ORG_UUID=$ORG_UUID
CLAUDE_ORG_NAME="$ORG_NAME"
CLAUDE_ORG_ROLE=$ORG_ROLE
CLAUDE_MAX_TIER=$MAX_TIER

# GitHub Configuration (keep existing values)
# GITHUB_TOKEN=your_existing_github_token
# GITHUB_OWNER=worldscandy
# GITHUB_REPO=claude-automation
EOF
echo "================================================"
echo

echo "🚀 Next Steps:"
echo "1. Copy the above configuration to your host .env-secret file"
echo "2. Keep existing GITHUB_* variables unchanged"
echo "3. Restart your Container Orchestration system"
echo "4. Verify with: docker run claude-automation-claude claude --version"
echo

echo "✅ Token renewal process completed!"