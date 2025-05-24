#!/bin/bash

echo "🧪 Claude CLI Execution Test in Container"
echo "========================================"

# Create minimal test
docker run --rm \
    -v ~/.claude.json:/root/.claude.json:ro \
    -v $(pwd):/workspace \
    node:20-alpine sh -c "
        echo 'Installing Claude CLI...'
        npm install -g @anthropic-ai/claude-code --silent
        
        echo -e '\n1. Version check:'
        claude --version || echo 'Version check failed'
        
        echo -e '\n2. Simple calculation test:'
        echo 'What is 10 + 20?' | claude --print --no-conversation || echo 'Calculation failed'
        
        echo -e '\n3. File creation test:'
        cd /workspace && echo 'Create a file called test.txt with content: Hello from Claude in Container' | claude --print --no-conversation
        
        echo -e '\n4. Check if file was created:'
        ls -la test.txt 2>/dev/null || echo 'File not created'
        cat test.txt 2>/dev/null || echo 'Cannot read file'
    "

echo -e "\n✅ Test complete!"