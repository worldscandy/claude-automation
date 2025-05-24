.PHONY: build test clean run-test k8s-build k8s-deploy k8s-clean monitor

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

docker-test:
	docker build -t claude-agent-test -f Dockerfile.test .
	docker run --rm -v /var/run/docker.sock:/var/run/docker.sock claude-agent-test

# Kubernetes targets
k8s-build: monitor
	@echo "Building Kubernetes image..."
	docker build -t worldscandy/claude-automation:latest -f Dockerfile.k8s .

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