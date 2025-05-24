#!/bin/bash

echo "🔧 Claude Automation Setup Script"
echo "================================"
echo ""

# Function to extract values from host Claude config
extract_from_host() {
    echo "📂 Checking host Claude configuration..."
    
    if [ ! -f ~/.claude.json ]; then
        echo "❌ ~/.claude.json not found on host"
        return 1
    fi
    
    if [ ! -f ~/.claude/.credentials.json ]; then
        echo "❌ ~/.claude/.credentials.json not found on host"
        return 1
    fi
    
    echo "✅ Found host Claude configuration files"
    echo ""
    
    # Extract values using jq
    echo "📝 Extracting values from host configuration..."
    
    # From ~/.claude.json
    export CLAUDE_USER_ID=$(jq -r '.userID' ~/.claude.json 2>/dev/null || echo "")
    export CLAUDE_ACCOUNT_UUID=$(jq -r '.oauthAccount.accountUuid' ~/.claude.json 2>/dev/null || echo "")
    export CLAUDE_EMAIL=$(jq -r '.oauthAccount.emailAddress' ~/.claude.json 2>/dev/null || echo "")
    export CLAUDE_ORG_UUID=$(jq -r '.oauthAccount.organizationUuid' ~/.claude.json 2>/dev/null || echo "")
    export CLAUDE_ORG_NAME=$(jq -r '.oauthAccount.organizationName' ~/.claude.json 2>/dev/null || echo "")
    export CLAUDE_ORG_ROLE=$(jq -r '.oauthAccount.organizationRole' ~/.claude.json 2>/dev/null || echo "")
    export CLAUDE_MAX_TIER=$(jq -r '.claudeMaxTier' ~/.claude.json 2>/dev/null || echo "")
    
    # From ~/.claude/.credentials.json
    export CLAUDE_ACCESS_TOKEN=$(jq -r '.claudeAiOauth.accessToken' ~/.claude/.credentials.json 2>/dev/null || echo "")
    export CLAUDE_TOKEN_EXPIRES_AT=$(jq -r '.claudeAiOauth.expiresAt' ~/.claude/.credentials.json 2>/dev/null || echo "")
    
    # Validate extraction
    if [ -z "$CLAUDE_ACCESS_TOKEN" ] || [ "$CLAUDE_ACCESS_TOKEN" == "null" ]; then
        echo "❌ Failed to extract authentication data from host"
        return 1
    fi
    
    echo "✅ Successfully extracted authentication data"
    
    # Show extracted data (hiding sensitive parts)
    echo ""
    echo "📋 Extracted information:"
    echo "  Email: $CLAUDE_EMAIL"
    echo "  Organization: $CLAUDE_ORG_NAME"
    echo "  Max Tier: $CLAUDE_MAX_TIER"
    echo "  Access Token: ${CLAUDE_ACCESS_TOKEN:0:10}..."
    echo ""
    
    return 0
}

# Function to save to .env file
save_to_env() {
    echo "💾 Saving configuration to .env file..."
    
    cat > .env << EOF
# Claude Authentication Configuration
# Generated on $(date)

# OAuth Access Token from ~/.claude/.credentials.json
CLAUDE_ACCESS_TOKEN=$CLAUDE_ACCESS_TOKEN

# Token expiration timestamp (Unix milliseconds)
CLAUDE_TOKEN_EXPIRES_AT=$CLAUDE_TOKEN_EXPIRES_AT

# User information from ~/.claude.json
CLAUDE_USER_ID=$CLAUDE_USER_ID
CLAUDE_ACCOUNT_UUID=$CLAUDE_ACCOUNT_UUID
CLAUDE_EMAIL=$CLAUDE_EMAIL
CLAUDE_ORG_UUID=$CLAUDE_ORG_UUID
CLAUDE_ORG_NAME="$CLAUDE_ORG_NAME"
CLAUDE_ORG_ROLE=$CLAUDE_ORG_ROLE
CLAUDE_MAX_TIER=$CLAUDE_MAX_TIER

# GitHub Configuration
GITHUB_TOKEN=${GITHUB_TOKEN:-your_github_token_here}
GITHUB_OWNER=${GITHUB_OWNER:-worldscandy}
GITHUB_REPO=${GITHUB_REPO:-claude-automation}

# Optional: LINE Integration
# LINE_CHANNEL_ACCESS_TOKEN=your_line_token_here
# LINE_CHANNEL_SECRET=your_line_secret_here
EOF

    chmod 600 .env
    echo "✅ Configuration saved to .env"
}

# Main script
echo "Select setup method:"
echo "1) Auto-detect from host Claude installation"
echo "2) Manual configuration"
echo ""
read -p "Enter your choice (1-2): " choice

case $choice in
    1)
        echo ""
        echo "🔍 Auto-detecting from host..."
        if extract_from_host; then
            save_to_env
            
            # Ask for GitHub token
            echo ""
            echo "📝 Additional configuration needed:"
            read -p "Enter your GitHub Personal Access Token (or press Enter to set later): " github_token
            if [ ! -z "$github_token" ]; then
                sed -i "s/your_github_token_here/$github_token/" .env
                echo "✅ GitHub token saved"
            fi
        else
            echo ""
            echo "❌ Auto-detection failed. Please use manual setup."
            exit 1
        fi
        ;;
        
    2)
        echo ""
        echo "📝 Manual configuration"
        
        # Check if .env exists
        if [ ! -f .env ]; then
            echo "Creating .env from template..."
            cp .env.example .env
        fi
        
        echo ""
        echo "Please edit the .env file with your actual credentials:"
        echo "  1. Get access token from ~/.claude/.credentials.json"
        echo "  2. Get user info from ~/.claude.json"
        echo "  3. Create GitHub Personal Access Token"
        echo ""
        echo "Then run this script again to generate auth files."
        
        # Open editor if available
        if command -v nano &> /dev/null; then
            read -p "Open .env in nano editor? (y/n): " open_editor
            if [ "$open_editor" == "y" ]; then
                nano .env
            fi
        fi
        
        exit 0
        ;;
        
    *)
        echo "❌ Invalid choice"
        exit 1
        ;;
esac

# Load environment variables
source .env

# Validate required variables
echo ""
echo "🔍 Validating configuration..."
required_vars=(
    "CLAUDE_ACCESS_TOKEN"
    "CLAUDE_TOKEN_EXPIRES_AT" 
    "CLAUDE_USER_ID"
    "CLAUDE_ACCOUNT_UUID"
    "CLAUDE_EMAIL"
    "CLAUDE_ORG_UUID"
    "CLAUDE_ORG_NAME"
    "CLAUDE_MAX_TIER"
)

missing_vars=()
for var in "${required_vars[@]}"; do
    if [ -z "${!var}" ] || [ "${!var}" == *"your_"* ] || [ "${!var}" == *"TEMPLATE_"* ]; then
        missing_vars+=($var)
    fi
done

if [ ${#missing_vars[@]} -ne 0 ]; then
    echo "❌ Missing or invalid environment variables:"
    printf '%s\n' "${missing_vars[@]}"
    echo ""
    echo "Please check .env file"
    exit 1
fi

# Create auth directory
mkdir -p auth

# Generate .claude.json from template
echo ""
echo "📝 Generating authentication files..."
cat .claude.json.template | \
    sed "s/TEMPLATE_FIRST_START_TIME/$(date -u +%Y-%m-%dT%H:%M:%S.%3NZ)/" | \
    sed "s/TEMPLATE_USER_ID/$CLAUDE_USER_ID/" | \
    sed "s/TEMPLATE_ACCOUNT_UUID/$CLAUDE_ACCOUNT_UUID/" | \
    sed "s/TEMPLATE_EMAIL/$CLAUDE_EMAIL/" | \
    sed "s/TEMPLATE_ORG_UUID/$CLAUDE_ORG_UUID/" | \
    sed "s/TEMPLATE_ORG_ROLE/$CLAUDE_ORG_ROLE/" | \
    sed "s/TEMPLATE_ORG_NAME/$CLAUDE_ORG_NAME/" | \
    sed "s/TEMPLATE_MAX_TIER/$CLAUDE_MAX_TIER/" \
    > auth/.claude.json

# Generate .credentials.json from template
cat .claude.credentials.json.template | \
    sed "s/TEMPLATE_ACCESS_TOKEN/$CLAUDE_ACCESS_TOKEN/" | \
    sed "s/TEMPLATE_EXPIRES_AT/$CLAUDE_TOKEN_EXPIRES_AT/" \
    > auth/.credentials.json

# Set proper permissions
chmod 600 auth/.credentials.json
chmod 644 auth/.claude.json

echo "✅ Authentication files generated successfully!"
echo ""
echo "📁 Created files:"
echo "  - auth/.claude.json"
echo "  - auth/.credentials.json"
echo "  - .env (configuration)"
echo ""

# Check token expiration
current_time=$(date +%s)000
if [ "$CLAUDE_TOKEN_EXPIRES_AT" -lt "$current_time" ]; then
    echo "⚠️  WARNING: Your Claude access token appears to be expired!"
    echo "   Please re-authenticate with Claude CLI to get a new token."
else
    expires_date=$(date -d "@$((CLAUDE_TOKEN_EXPIRES_AT/1000))" 2>/dev/null || date -r "$((CLAUDE_TOKEN_EXPIRES_AT/1000))" 2>/dev/null || echo "unknown")
    echo "📅 Token expires: $expires_date"
fi

echo ""
echo "🚀 Setup complete! You can now run the Claude automation system."
echo ""
echo "⚠️  Security reminders:"
echo "  - Never commit .env or auth/ directory to git"
echo "  - Keep your access token secure"
echo "  - Rotate tokens before expiration"
echo ""
echo "Next steps:"
echo "  1. Review and update GitHub token in .env if needed"
echo "  2. Run 'make build' to build the project"
echo "  3. Start the automation system"