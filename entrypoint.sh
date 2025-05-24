#!/bin/bash
set -e

# Fix permissions for mounted volumes as claude user
sudo chown -R claude:claude /home/claude/workspace 2>/dev/null || echo "Workspace permission fix attempted"
sudo chown -R claude:claude /app/workspaces 2>/dev/null || echo "App workspace permission fix attempted"
sudo chown -R claude:claude /app/sessions 2>/dev/null || echo "Sessions permission fix attempted"

# Setup Claude CLI auth files in home directory
if [ -d "/home/claude/.claude" ]; then
    # Copy .claude.json to home root if it exists in .claude directory
    if [ -f "/home/claude/.claude/.claude.json" ]; then
        sudo cp /home/claude/.claude/.claude.json /home/claude/.claude.json 2>/dev/null || echo "Failed to copy .claude.json"
        sudo chown claude:claude /home/claude/.claude.json 2>/dev/null || echo "Failed to set .claude.json ownership"
    fi
    
    # Create necessary directories for Claude CLI with proper permissions
    sudo mkdir -p /home/claude/.claude/todos /home/claude/.claude/projects /home/claude/.claude/statsig 2>/dev/null || echo "Claude directories creation attempted"
    
    # Ensure .claude directory is writable by claude user (not read-only)
    sudo chown -R claude:claude /home/claude/.claude 2>/dev/null || echo "Claude auth permission fix attempted"
    sudo chmod -R u+w /home/claude/.claude 2>/dev/null || echo "Claude write permission fix attempted"
fi

# Execute the main command
exec "$@"