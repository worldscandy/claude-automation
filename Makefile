.PHONY: build test clean run-test docker-build docker-build-k8s k8s-build k8s-deploy k8s-clean monitor token-renewal auth-test test-auth-k8s test-orchestrator integration-tests

build:
	@echo "Building orchestrator..."
	go build -o bin/orchestrator ./cmd/orchestrator
	@echo "Building agent..."
	go build -o bin/agent ./cmd/agent
	@echo "Building monitor..."
	go build -o bin/monitor ./cmd/monitor
	@echo "Build complete!"

monitor:
	@echo "Building monitor for Kubernetes..."
	CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o bin/monitor ./cmd/monitor

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

# Docker targets with new structure
docker-build:
	@echo "Building production Docker image..."
	docker build -t worldscandy/claude-automation:latest --target production -f docker/Dockerfile .

docker-build-k8s:
	@echo "Building Kubernetes Docker image..."
	docker build -t worldscandy/claude-automation:k8s --target kubernetes -f docker/Dockerfile .

# Kubernetes targets
k8s-build: monitor
	@echo "Building Kubernetes image..."
	docker build -t worldscandy/claude-automation:k8s --target kubernetes -f docker/Dockerfile .

k8s-deploy:
	@echo "Deploying to Kubernetes..."
	kubectl apply -f deployments/monitor-deployment.yaml

k8s-clean:
	@echo "Cleaning up Kubernetes resources..."
	kubectl delete -f deployments/monitor-deployment.yaml --ignore-not-found=true

k8s-test: k8s-build k8s-deploy
	@echo "Testing Kubernetes deployment..."
	kubectl wait --for=condition=ready pod -l app=claude-automation --timeout=300s
	kubectl logs -l app=claude-automation --tail=50

# Authentication and Token Management
token-renewal:
	@echo "Starting Claude CLI token renewal container..."
	go run ./cmd/token-renewal

auth-test:
	@echo "Running authentication system tests..."
	go run ./test/integration/auth

# Issue #13 - Claude CLI Kubernetes Integration
claude-integration-test:
	@echo "Testing Claude CLI Kubernetes Integration..."
	@echo "Setting ORCHESTRATOR_MODE=kubernetes..."
	ORCHESTRATOR_MODE=kubernetes go run ./cmd/orchestrator -issue 13 -task "Create a simple test file with Hello World content" -repo worldscandy/claude-automation

# Integration Tests
test-auth-k8s:
	@echo "Running Kubernetes authentication integration tests..."
	go run ./test/integration/auth-k8s

test-orchestrator:
	@echo "Running Orchestrator integration tests..."
	go run ./test/integration/orchestrator

integration-tests: auth-test test-auth-k8s test-orchestrator
	@echo "All integration tests completed!"

# Issue #13 Development Workflow  
issue-13-dev: build
	@echo "Starting Issue #13 development environment..."
	@echo "Building all components..."
	@make k8s-build
	@echo "Running integration test..."
	@make claude-integration-test