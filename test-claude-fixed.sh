#!/bin/bash

echo "🧪 Fixed Claude CLI Container Test"
echo "=================================="

# Check if auth files exist
if [ ! -f auth/.claude.json ] || [ ! -f auth/.credentials.json ]; then
    echo "❌ Auth files not found. Please run ./setup.sh first"
    exit 1
fi

# Test with proper credential mounting
echo "📝 Running comprehensive test..."
docker run --rm \
    -v $(pwd)/auth/.claude.json:/root/.claude.json \
    -v $(pwd)/auth:/root/.claude \
    -v $(pwd)/test-workspace:/workspace \
    -w /workspace \
    -e SHELL=/bin/bash \
    node:20 bash -c '
        echo "1. Installing Claude CLI..."
        npm install -g @anthropic-ai/claude-code --silent
        
        echo -e "\n2. Testing calculation:"
        echo "What is 789 + 321? Reply with only the number." | claude --print
        
        echo -e "\n3. Testing file creation with auto-approval:"
        # Use environment variable for auto-approval
        export CLAUDE_AUTO_APPROVE=true
        echo "Create a file named test-output.txt with the content: Successfully created by Claude!" | claude --print --dangerously-skip-permissions
        
        echo -e "\n4. Checking created file:"
        if [ -f test-output.txt ]; then
            echo "✅ File created successfully:"
            cat test-output.txt
        else
            echo "❌ File not created"
        fi
        
        echo -e "\n5. Testing code execution:"
        echo "Create a simple bash script that prints the current date and save it as date.sh" | claude --print --dangerously-skip-permissions
        
        echo -e "\n6. List workspace contents:"
        ls -la
    '