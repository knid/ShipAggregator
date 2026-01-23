# =============================================================================
# Dockerfile for TR-Only Shipping Aggregation Backend
# =============================================================================
# Multi-stage build for minimal production images
# Supports both API server and background worker

# =============================================================================
# Stage 1: Build
# =============================================================================
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache \
    git \
    ca-certificates \
    tzdata \
    && update-ca-certificates

# Set working directory
WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build arguments
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown

# Build flags for smaller binary
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

# Build API server
RUN go build \
    -ldflags="-s -w -X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildTime=${BUILD_TIME}" \
    -o /app/bin/api \
    ./cmd/api

# Build worker
RUN go build \
    -ldflags="-s -w -X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildTime=${BUILD_TIME}" \
    -o /app/bin/worker \
    ./cmd/worker

# Build migration tool
RUN go build \
    -ldflags="-s -w" \
    -o /app/bin/migrate \
    ./cmd/migrate

# =============================================================================
# Stage 2: API Server
# =============================================================================
FROM alpine:3.19 AS api

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    && update-ca-certificates

# Create non-root user
RUN addgroup -g 1000 -S shipping && \
    adduser -u 1000 -S shipping -G shipping

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/bin/api /app/api
COPY --from=builder /app/bin/migrate /app/migrate

# Copy migrations
COPY --from=builder /app/migrations /app/migrations

# Set ownership
RUN chown -R shipping:shipping /app

# Switch to non-root user
USER shipping

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health/live || exit 1

# Default command
ENTRYPOINT ["/app/api"]

# =============================================================================
# Stage 3: Worker
# =============================================================================
FROM alpine:3.19 AS worker

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    && update-ca-certificates

# Create non-root user
RUN addgroup -g 1000 -S shipping && \
    adduser -u 1000 -S shipping -G shipping

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/bin/worker /app/worker

# Set ownership
RUN chown -R shipping:shipping /app

# Switch to non-root user
USER shipping

# Default command
ENTRYPOINT ["/app/worker"]

# =============================================================================
# Stage 4: Migration Tool (for CI/CD)
# =============================================================================
FROM alpine:3.19 AS migrate

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    && update-ca-certificates

# Create non-root user
RUN addgroup -g 1000 -S shipping && \
    adduser -u 1000 -S shipping -G shipping

# Set working directory
WORKDIR /app

# Copy binary and migrations from builder
COPY --from=builder /app/bin/migrate /app/migrate
COPY --from=builder /app/migrations /app/migrations

# Set ownership
RUN chown -R shipping:shipping /app

# Switch to non-root user
USER shipping

# Default command
ENTRYPOINT ["/app/migrate"]

# =============================================================================
# Stage 5: All-in-one (for development/testing)
# =============================================================================
FROM alpine:3.19 AS all

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    bash \
    && update-ca-certificates

# Create non-root user
RUN addgroup -g 1000 -S shipping && \
    adduser -u 1000 -S shipping -G shipping

# Set working directory
WORKDIR /app

# Copy all binaries from builder
COPY --from=builder /app/bin/api /app/api
COPY --from=builder /app/bin/worker /app/worker
COPY --from=builder /app/bin/migrate /app/migrate

# Copy migrations
COPY --from=builder /app/migrations /app/migrations

# Copy entrypoint script
COPY scripts/docker-entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

# Set ownership
RUN chown -R shipping:shipping /app

# Switch to non-root user
USER shipping

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health/live || exit 1

# Default command (can be overridden)
ENTRYPOINT ["/app/entrypoint.sh"]
CMD ["api"]
