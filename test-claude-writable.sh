#!/bin/bash

echo "🧪 Testing Claude with Writable Config"
echo "====================================="

# Clean workspace
rm -rf test-workspace/*

# Run test with writable config
echo "📝 Setting up writable auth configuration..."
docker run --rm \
    -v $(pwd)/test-workspace:/home/claude/workspace \
    claude-automation:latest bash -c '
        # Copy auth files to writable location
        echo "1. Setting up authentication..."
        cat > /home/claude/.claude.json << '\''EOF'\''
'"$(cat auth/.claude.json)"'
EOF
        
        mkdir -p /home/claude/.claude
        cat > /home/claude/.claude/.credentials.json << '\''EOF'\''
'"$(cat auth/.credentials.json)"'
EOF
        
        echo "✅ Auth files created"
        
        echo -e "\n2. Test calculation:"
        echo "What is 333 + 666? Reply with only the number." | claude --print
        
        echo -e "\n3. Test file creation:"
        echo "Create a file named success.txt containing: Claude is working perfectly in Docker!" | \
            claude --print --dangerously-skip-permissions
        
        echo -e "\n4. Check results:"
        ls -la
        if [ -f success.txt ]; then
            echo -e "\n✅ File content:"
            cat success.txt
        fi
        
        echo -e "\n5. Test script generation:"
        echo "Create a bash script named info.sh that shows the date, user, and working directory" | \
            claude --print --dangerously-skip-permissions
        
        if [ -f info.sh ]; then
            echo -e "\n✅ Generated script:"
            cat info.sh
            echo -e "\nRunning script:"
            bash info.sh
        fi
    '

echo -e "\n✅ Test complete!"
echo "Results in test-workspace:"
ls -la test-workspace/