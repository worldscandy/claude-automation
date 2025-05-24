.PHONY: build test clean run-test

build:
	@echo "Building orchestrator..."
	go build -o bin/orchestrator ./cmd/orchestrator
	@echo "Building agent..."
	go build -o bin/agent ./cmd/agent
	@echo "Build complete!"

agent-static:
	@echo "Building static agent for containers..."
	CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o cmd/agent/agent ./cmd/agent

test-setup:
	@echo "Creating test directories..."
	mkdir -p shared/workspaces

run-test: build agent-static test-setup
	@echo "Running test..."
	./bin/orchestrator

clean:
	rm -rf bin/
	rm -f cmd/agent/agent
	rm -rf shared/workspaces/*

docker-test:
	docker build -t claude-agent-test -f Dockerfile.test .
	docker run --rm -v /var/run/docker.sock:/var/run/docker.sock claude-agent-test