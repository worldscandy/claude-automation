#!/bin/bash

echo "🧪 Testing Claude Automation Image"
echo "================================="

# Clean workspace
rm -rf test-workspace/*

# Run test with custom image
echo "📝 Running test with claude-automation image..."
docker run --rm \
    -v $(pwd)/auth/.claude.json:/home/claude/.claude.json:ro \
    -v $(pwd)/auth/.claude:/home/claude/.claude:ro \
    -v $(pwd)/test-workspace:/home/claude/workspace \
    claude-automation:latest bash -c '
        echo "1. User info:"
        echo "User: $(whoami)"
        echo "Home: $HOME"
        echo "PWD: $(pwd)"
        
        echo -e "\n2. Check Claude installation:"
        which claude
        claude --version
        
        echo -e "\n3. Test calculation:"
        echo "What is 111 + 222? Reply with only the number." | claude --print
        
        echo -e "\n4. Test file creation (with permission skip):"
        echo "Create a file named docker-test.txt with: Successfully running Claude as non-root user!" | \
            claude --print --dangerously-skip-permissions
        
        echo -e "\n5. Check result:"
        ls -la
        if [ -f docker-test.txt ]; then
            echo "✅ File created:"
            cat docker-test.txt
        else
            echo "❌ File not created"
        fi
        
        echo -e "\n6. Test code generation:"
        echo "Create a Python script called hello.py that prints Hello from Docker Claude" | \
            claude --print --dangerously-skip-permissions
            
        echo -e "\n7. Final workspace:"
        ls -la
        
        # Test Python execution if script was created
        if [ -f hello.py ]; then
            echo -e "\n8. Run generated script:"
            python3 hello.py 2>/dev/null || echo "Python not available"
        fi
    '

echo -e "\n✅ Test complete!"
echo "Host workspace contents:"
ls -la test-workspace/