#!/bin/bash

echo "🚀 Testing Claude CLI in Container with Auth"
echo "==========================================="

# Test with mounted claude.json
docker run --rm \
    -v ~/.claude.json:/root/.claude.json:ro \
    -v $(pwd)/shared/workspaces/test:/workspace \
    -w /workspace \
    ubuntu:22.04 bash -c "
        # Install dependencies
        apt-get update && apt-get install -y curl nodejs npm > /dev/null 2>&1
        
        # Install Claude CLI
        npm install -g @anthropic-ai/claude-code > /dev/null 2>&1
        
        # Test Claude
        echo 'Testing Claude CLI...'
        claude --version
        
        echo -e '\nTesting simple calculation:'
        echo 'What is 2+2?' | claude --print
        
        echo -e '\nTesting file creation:'
        echo 'Create a simple hello.sh script that prints Hello from Container' | claude --print
    "

echo -e "\n✅ Test complete!"