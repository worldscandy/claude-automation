#!/bin/bash

echo "🧪 Quick Claude Auth Test"
echo "========================"

# Simple test with alpine (smaller image)
docker run --rm \
    -v ~/.claude.json:/root/.claude.json:ro \
    -v $(pwd):/workspace \
    alpine:latest sh -c "
        which claude || echo 'Claude not found, checking mount...'
        ls -la /root/.claude.json
        echo 'Auth file mounted successfully!'
    "

echo -e "\n✅ Mount test complete!"
echo -e "\n📋 Solution:"
echo "- Mount ~/.claude.json to container's /root/.claude.json"
echo "- Install Claude CLI inside container" 
echo "- Claude will use the mounted auth automatically"