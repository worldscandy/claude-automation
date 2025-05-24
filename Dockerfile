# Multi-stage build for optimal container size
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the monitor binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o monitor ./cmd/monitor

# Build the orchestrator binary  
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o orchestrator ./cmd/orchestrator

# Final stage - minimal runtime image
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    docker-cli \
    git \
    bash \
    curl \
    && rm -rf /var/cache/apk/*

# Install Claude CLI (placeholder - will be added when available)
# RUN curl -fsSL https://claude.ai/install.sh | sh
# For now, create a placeholder claude command
RUN echo '#!/bin/bash\necho "Claude CLI placeholder - replace with actual installation"\nexit 1' > /usr/local/bin/claude && chmod +x /usr/local/bin/claude

# Create non-root user
RUN addgroup -g 1001 appgroup && \
    adduser -D -u 1001 -G appgroup appuser

# Create application directory
WORKDIR /app

# Copy binaries from builder stage
COPY --from=builder /app/monitor /app/orchestrator ./

# Create required directories
RUN mkdir -p /app/workspaces /app/sessions /app/auth /app/config && \
    chown -R appuser:appgroup /app

# Copy auth and config directories
COPY --chown=appuser:appgroup auth/ ./auth/
COPY --chown=appuser:appgroup config/ ./config/

# Switch to non-root user
USER appuser

# Expose health check port (optional)
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD ps aux | grep monitor || exit 1

# Default command - run monitor
CMD ["./monitor"]