# Protocol Buffer Definitions

This directory contains the shared protobuf definitions for the Container Resource Predictor system.

## Structure

- `predictor/v1/predictor.proto` - Main service and message definitions

## Code Generation

### Using buf (recommended)

```bash
# Install buf
# macOS: brew install bufbuild/buf/buf
# Linux: see https://buf.build/docs/installation

# Lint protos
buf lint

# Check for breaking changes
buf breaking --against '.git#branch=main'

# Generate code
buf generate
```

### Manual generation

#### Go
```bash
protoc --go_out=. --go-grpc_out=. predictor/v1/predictor.proto
```

#### Rust
The Rust code is generated at build time using `tonic-build` in the `resource-agent` crate.

## Services

### PredictorSync

gRPC service for agent-API communication:

- `Register` - Agent registration with the API
- `SyncMetrics` - Stream metrics from agent to API
- `GetModelUpdate` - Check for and download model updates
- `UploadGradients` - Upload federated learning gradients

## Message Types

### Core Types
- `ContainerMetrics` - Resource usage metrics for a container
- `ResourceProfile` - Predicted resource recommendations
- `Anomaly` - Detected anomaly information

### Request/Response Types
- `RegisterRequest/Response` - Agent registration
- `MetricsBatch` - Batch of metrics for streaming
- `ModelRequest/Response` - Model update handling
- `GradientsRequest/Response` - Federated learning
