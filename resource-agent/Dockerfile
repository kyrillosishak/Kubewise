# Multi-stage build for resource-agent
# Supports linux/amd64 and linux/arm64

FROM --platform=$BUILDPLATFORM rust:1.83-bookworm AS builder

ARG TARGETPLATFORM
ARG BUILDPLATFORM

# Install cross-compilation tools
RUN apt-get update && apt-get install -y \
    protobuf-compiler \
    && rm -rf /var/lib/apt/lists/*

# Install target toolchains
RUN case "$TARGETPLATFORM" in \
    "linux/amd64") \
        rustup target add x86_64-unknown-linux-gnu \
        ;; \
    "linux/arm64") \
        apt-get update && apt-get install -y gcc-aarch64-linux-gnu && \
        rustup target add aarch64-unknown-linux-gnu \
        ;; \
    esac

WORKDIR /app

# Copy manifests first for dependency caching
COPY Cargo.toml Cargo.lock ./
COPY crates/agent/Cargo.toml crates/agent/
COPY crates/agent-lib/Cargo.toml crates/agent-lib/

# Create dummy source files for dependency compilation
RUN mkdir -p crates/agent/src crates/agent-lib/src && \
    echo "fn main() {}" > crates/agent/src/main.rs && \
    echo "pub fn dummy() {}" > crates/agent-lib/src/lib.rs

# Build dependencies
RUN case "$TARGETPLATFORM" in \
    "linux/amd64") \
        cargo build --release --target x86_64-unknown-linux-gnu \
        ;; \
    "linux/arm64") \
        cargo build --release --target aarch64-unknown-linux-gnu \
        ;; \
    esac || true

# Copy actual source code
COPY crates/ crates/

# Build the actual binary
RUN case "$TARGETPLATFORM" in \
    "linux/amd64") \
        cargo build --release --target x86_64-unknown-linux-gnu && \
        cp target/x86_64-unknown-linux-gnu/release/resource-agent /app/resource-agent \
        ;; \
    "linux/arm64") \
        cargo build --release --target aarch64-unknown-linux-gnu && \
        cp target/aarch64-unknown-linux-gnu/release/resource-agent /app/resource-agent \
        ;; \
    esac

# Runtime image
FROM gcr.io/distroless/cc-debian12:nonroot

COPY --from=builder /app/resource-agent /usr/local/bin/resource-agent

USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/resource-agent"]
