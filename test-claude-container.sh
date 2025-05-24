#!/bin/bash

echo "🧪 Testing Claude CLI with Generated Auth Files"
echo "=============================================="

# Check if auth files exist
if [ ! -f auth/.claude.json ] || [ ! -f auth/.credentials.json ]; then
    echo "❌ Auth files not found. Please run ./setup.sh first"
    exit 1
fi

echo "✅ Found auth files:"
ls -la auth/

# Test 1: Simple calculation
echo -e "\n📝 Test 1: Simple Calculation"
docker run --rm \
    -v $(pwd)/auth/.claude.json:/root/.claude.json \
    -v $(pwd)/auth/.credentials.json:/root/.claude/.credentials.json \
    -v $(pwd)/test-workspace:/workspace \
    -w /workspace \
    -e SHELL=/bin/bash \
    node:20 bash -c "
        # Install Claude CLI
        npm install -g @anthropic-ai/claude-code --silent
        
        # Test simple calculation
        echo 'What is 123 + 456? Reply with just the number.' | claude --print
    "

# Test 2: File creation
echo -e "\n📝 Test 2: File Creation"
docker run --rm \
    -v $(pwd)/auth/.claude.json:/root/.claude.json \
    -v $(pwd)/auth/.credentials.json:/root/.claude/.credentials.json \
    -v $(pwd)/test-workspace:/workspace \
    -w /workspace \
    -e SHELL=/bin/bash \
    node:20 bash -c "
        # Install Claude CLI
        npm install -g @anthropic-ai/claude-code --silent
        
        # Create directory for credentials
        mkdir -p /root/.claude
        cp /root/.claude/.credentials.json /root/.claude/.credentials.json
        
        # Test file creation
        echo 'Create a file called hello.txt with the content: Hello from Claude in Docker!' | claude --print --yes
        
        # Check if file was created
        echo -e '\nChecking created file:'
        ls -la hello.txt 2>/dev/null || echo 'File not found'
        cat hello.txt 2>/dev/null || echo 'Cannot read file'
    "

# Test 3: Code execution
echo -e "\n📝 Test 3: Code Execution"
docker run --rm \
    -v $(pwd)/auth/.claude.json:/root/.claude.json \
    -v $(pwd)/auth/.credentials.json:/root/.claude/.credentials.json \
    -v $(pwd)/test-workspace:/workspace \
    -w /workspace \
    -e SHELL=/bin/bash \
    node:20 bash -c "
        # Install Node.js tools
        npm install -g @anthropic-ai/claude-code --silent
        
        # Create directory for credentials  
        mkdir -p /root/.claude
        cp /root/.claude/.credentials.json /root/.claude/.credentials.json
        
        # Test code execution
        echo 'Create a simple Node.js script that prints the current date and time, then execute it' | claude --print --yes
        
        # List created files
        echo -e '\nFiles in workspace:'
        ls -la
    "

echo -e "\n✅ Test complete!"
echo "Check test-workspace/ directory for created files"