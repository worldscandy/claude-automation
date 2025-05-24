#!/bin/bash

echo "🚀 Claude Automation Remote Execution PoC Test"
echo "============================================"

# Check if claude command exists
if ! command -v claude &> /dev/null; then
    echo "❌ Error: 'claude' command not found. Please install Claude CLI first."
    exit 1
fi

# Check if Docker is running
if ! docker info &> /dev/null; then
    echo "❌ Error: Docker is not running"
    exit 1
fi

echo "✅ Prerequisites check passed"
echo ""

# Build the project
echo "📦 Building Go binaries..."
make build agent-static

if [ $? -ne 0 ]; then
    echo "❌ Build failed"
    exit 1
fi

echo "✅ Build successful"
echo ""

# Create test socket directory
SOCKET_PATH="/tmp/claude-agent.sock"
rm -rf $SOCKET_PATH
sudo rm -rf $SOCKET_PATH 2>/dev/null || true

# Run test
echo "🧪 Running integration test..."
echo "This will:"
echo "  1. Start a Python container"
echo "  2. Execute Claude command to create a Hello World script"
echo "  3. Run the script in the container"
echo ""

./bin/orchestrator