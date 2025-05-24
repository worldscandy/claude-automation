#!/bin/bash

echo "🧪 Claude CLI Test with Non-Root User"
echo "===================================="

# Check if auth files exist
if [ ! -f auth/.claude.json ] || [ ! -f auth/.credentials.json ]; then
    echo "❌ Auth files not found. Please run ./setup.sh first"
    exit 1
fi

# Clean test workspace
rm -rf test-workspace/*

# Test with non-root user
echo "📝 Running test as non-root user..."
docker run --rm \
    -v $(pwd)/auth/.claude.json:/home/node/.claude.json \
    -v $(pwd)/auth:/home/node/.claude \
    -v $(pwd)/test-workspace:/workspace \
    -w /workspace \
    -e SHELL=/bin/bash \
    --user node \
    node:20 bash -c '
        echo "1. Current user:"
        whoami
        echo "Home: $HOME"
        
        echo -e "\n2. Installing Claude CLI..."
        npm install -g @anthropic-ai/claude-code
        
        echo -e "\n3. Testing calculation:"
        echo "What is 555 + 444? Reply with only the number." | claude --print
        
        echo -e "\n4. Testing file creation with permission skip:"
        echo "Create a file named success.txt with the content: Claude is working in Docker as non-root user!" | \
            claude --print --dangerously-skip-permissions
        
        echo -e "\n5. Check result:"
        if [ -f success.txt ]; then
            echo "✅ File created:"
            cat success.txt
        else
            echo "❌ File not created"
            
            # Try without dangerous flag
            echo -e "\n6. Retry without skip-permissions flag:"
            echo "Create a file named retry.txt with: Testing without dangerous flag" | claude --print
        fi
        
        echo -e "\n7. Final check:"
        ls -la
    '