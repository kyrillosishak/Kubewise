# Multi-stage build for recommendation-api
# Supports linux/amd64 and linux/arm64

FROM --platform=$BUILDPLATFORM golang:1.22-bookworm AS builder

ARG TARGETPLATFORM
ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

# Copy go mod files first for dependency caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -o recommendation-api ./cmd/server

# Runtime image
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /app/recommendation-api /usr/local/bin/recommendation-api

USER nonroot:nonroot

EXPOSE 8080 9090

ENTRYPOINT ["/usr/local/bin/recommendation-api"]
