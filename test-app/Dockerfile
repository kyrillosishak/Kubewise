# Multi-stage Dockerfile for kubewise-test-app components
# Usage: docker build --build-arg COMPONENT=<component-name> -t kubewise-test/<component-name>:latest .

ARG COMPONENT

# Build stage
FROM golang:1.22-alpine AS builder

ARG COMPONENT

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Build the component
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/bin/${COMPONENT} ./cmd/${COMPONENT}

# Runtime stage
FROM alpine:3.19

ARG COMPONENT

# Install ca-certificates for HTTPS
RUN apk add --no-cache ca-certificates

# Create non-root user
RUN adduser -D -g '' appuser

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/bin/${COMPONENT} /app/server

# Use non-root user
USER appuser

# Expose default port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/server"]
