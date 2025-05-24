#!/bin/bash

echo "🧪 Claude CLI Test with Ubuntu (Full Compatibility)"
echo "================================================="

# Use Ubuntu for better compatibility
docker run --rm \
    -v ~/.claude.json:/root/.claude.json:ro \
    -v $(pwd)/test-workspace:/workspace \
    -w /workspace \
    node:20 bash -c "
        echo '1. Installing Claude CLI...'
        npm install -g @anthropic-ai/claude-code --silent
        
        echo -e '\n2. Version check:'
        claude --version
        
        echo -e '\n3. Simple test (non-interactive):'
        echo 'Reply with just the number: What is 15 + 25?' | claude --print
        
        echo -e '\n4. Test completed!'
    "

echo "✅ Done!"