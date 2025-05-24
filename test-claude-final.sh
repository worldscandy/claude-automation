#!/bin/bash

echo "🧪 Claude CLI Container Test (Final Version)"
echo "==========================================="

# Check if auth files exist
if [ ! -f auth/.claude.json ] || [ ! -f auth/.credentials.json ]; then
    echo "❌ Auth files not found. Please run ./setup.sh first"
    exit 1
fi

# Clean test workspace
rm -rf test-workspace/*

# Main test
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
        
        echo -e "\n2. Testing calculation (no permissions needed):"
        echo "What is 999 + 888? Reply with only the number." | claude --print
        
        echo -e "\n3. Testing file creation with permission skip:"
        echo "Create a file named hello.txt with the content: Hello from Claude in Docker! This test is working!" | \
            claude --print --dangerously-skip-permissions
        
        echo -e "\n4. Checking created file:"
        if [ -f hello.txt ]; then
            echo "✅ File created successfully:"
            cat hello.txt
        else
            echo "❌ File not created"
        fi
        
        echo -e "\n5. Testing script creation and execution:"
        echo "Create a bash script named info.sh that prints the current date, hostname, and working directory. Then make it executable." | \
            claude --print --dangerously-skip-permissions
        
        echo -e "\n6. Check and run the script:"
        if [ -f info.sh ]; then
            echo "✅ Script created:"
            cat info.sh
            echo -e "\nExecuting script:"
            chmod +x info.sh 2>/dev/null
            ./info.sh 2>/dev/null || bash info.sh
        else
            echo "❌ Script not created"
        fi
        
        echo -e "\n7. Final workspace contents:"
        ls -la
    '

echo -e "\n✅ Test complete!"
echo "Check test-workspace/ for results:"
ls -la test-workspace/