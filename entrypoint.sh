#!/bin/bash
set -e

# Fix permissions for mounted volumes as claude user
sudo chown -R claude:claude /home/claude/workspace 2>/dev/null || echo "Workspace permission fix attempted"
sudo chown -R claude:claude /app/workspaces 2>/dev/null || echo "App workspace permission fix attempted"
sudo chown -R claude:claude /app/sessions 2>/dev/null || echo "Sessions permission fix attempted"

# Setup Claude CLI auth files and directories
# Create necessary directories for Claude CLI with proper permissions
sudo mkdir -p /home/claude/.claude/todos /home/claude/.claude/projects /home/claude/.claude/statsig 2>/dev/null || echo "Claude directories creation attempted"

# Ensure Claude CLI auth files have correct permissions if they exist
if [ -f "/home/claude/.claude.json" ]; then
    sudo chown claude:claude /home/claude/.claude.json 2>/dev/null || echo "Failed to set .claude.json ownership"
    sudo chmod 600 /home/claude/.claude.json 2>/dev/null || echo "Failed to set .claude.json permissions"
fi

if [ -f "/home/claude/.claude/.credentials.json" ]; then
    sudo chown claude:claude /home/claude/.claude/.credentials.json 2>/dev/null || echo "Failed to set .credentials.json ownership"
    sudo chmod 600 /home/claude/.claude/.credentials.json 2>/dev/null || echo "Failed to set .credentials.json permissions"
fi

# Ensure .claude directory is writable by claude user
sudo chown -R claude:claude /home/claude/.claude 2>/dev/null || echo "Claude auth permission fix attempted"
sudo chmod -R u+w /home/claude/.claude 2>/dev/null || echo "Claude write permission fix attempted"

# Execute the main command
exec "$@"