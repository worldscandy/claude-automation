# Multi-stage, multi-target Dockerfile for Claude Automation System
ARG BUILD_TARGET=production

# =============================================================================
# Builder Stage - Go applications
# =============================================================================
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build all binaries
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o monitor ./cmd/monitor
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o orchestrator ./cmd/orchestrator
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o agent ./cmd/agent

# =============================================================================
# Base Runtime Stage
# =============================================================================
FROM alpine:3.19 AS base

# Install common runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    git \
    bash \
    curl \
    && rm -rf /var/cache/apk/*

# Create non-root user
RUN addgroup -g 1001 appgroup && \
    adduser -D -u 1001 -G appgroup appuser

# Create application directory
WORKDIR /app

# Copy binaries from builder stage
COPY --from=builder /app/monitor /app/orchestrator /app/agent ./

# Create required directories
RUN mkdir -p /app/workspaces /app/sessions /app/auth /app/config && \
    chown -R appuser:appgroup /app

# =============================================================================
# Production Stage - Docker-based execution
# =============================================================================
FROM base AS production

# Install Docker CLI for container orchestration
RUN apk add --no-cache docker-cli

# Install basic Claude CLI mock for testing
RUN echo '#!/bin/bash\necho "🤖 Claude CLI Mock Implementation"\necho "Task: $@"\necho "Working directory: $(pwd)"\necho "Files in workspace:"\nls -la 2>/dev/null || echo "No files yet"\necho "✅ Task simulation completed successfully"\necho "Test result from Claude CLI mock" > claude-output.txt\necho "Task completed at $(date)"' > /usr/local/bin/claude && chmod +x /usr/local/bin/claude

# Copy configuration
COPY --chown=appuser:appgroup auth/ ./auth/
COPY --chown=appuser:appgroup config/ ./config/

# Switch to non-root user
USER appuser

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD ps aux | grep monitor || exit 1

# Default command
CMD ["./monitor"]

# =============================================================================
# Kubernetes Stage - Pod-based execution with real Claude CLI
# =============================================================================
FROM node:20-alpine AS kubernetes

# Install system dependencies including Go
RUN apk add --no-cache \
    git \
    bash \
    curl \
    wget \
    openssh-client \
    ca-certificates \
    go \
    && rm -rf /var/cache/apk/*

# Install Claude CLI globally
RUN npm install -g @anthropic-ai/claude-code

# Fix Claude CLI env compatibility for Alpine Linux
RUN if [ -f /usr/local/bin/claude ]; then \
    sed -i '1s|#!/usr/bin/env -S node|#!/usr/bin/env node|' /usr/local/bin/claude; \
    fi

# Create non-root user
RUN adduser -D -s /bin/bash claude

# Create directories
RUN mkdir -p /app/auth \
             /workspace \
             /tmp/claude \
    && chown -R claude:claude /app /workspace /tmp/claude

# Copy Go binaries from builder stage
COPY --from=builder --chown=claude:claude /app/monitor /app/orchestrator /app/agent /app/

# Switch to non-root user
USER claude
WORKDIR /workspace

# Health check for Kubernetes environment
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD claude --version || exit 1

# Default command - keep container alive for Pod execution
CMD ["sh", "-c", "while true; do sleep 30; done"]