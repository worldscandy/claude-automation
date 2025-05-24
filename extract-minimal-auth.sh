#!/bin/bash

echo "📋 Creating Minimal Claude Auth for Containers"
echo "============================================="

# Extract only essential auth info
echo "Extracting essential fields from .claude.json..."

# Check what we actually need
jq '{
    userID: .userID,
    oauthAccount: .oauthAccount,
    claudeMaxTier: .claudeMaxTier,
    hasAvailableMaxSubscription: .hasAvailableMaxSubscription
}' ~/.claude.json > claude-auth-minimal.json 2>/dev/null

if [ $? -eq 0 ]; then
    echo "✅ Minimal auth file created: claude-auth-minimal.json"
    echo "File size comparison:"
    ls -lh ~/.claude.json claude-auth-minimal.json | awk '{print $5, $9}'
    
    echo -e "\nMinimal config preview:"
    jq 'keys' claude-auth-minimal.json
else
    echo "❌ Failed to extract auth info"
fi