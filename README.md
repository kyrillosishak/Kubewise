# Kubewise

**Intelligent Kubernetes Resource Optimization with Machine Learning**

Kubewise automatically analyzes your container workloads and provides ML-powered resource recommendations that reduce costs while preventing OOM kills and CPU throttling.

[![CI](https://github.com/example/kubewise/actions/workflows/ci.yaml/badge.svg)](https://github.com/example/kubewise/actions/workflows/ci.yaml)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

## Why Kubewise?

Most Kubernetes clusters run with 2-3x over-provisioned resources because teams set conservative limits to avoid outages. This wastes money and capacity.

Kubewise solves this by:
- Learning actual resource usage patterns per workload
- Predicting future needs using time-series ML models
- Recommending right-sized resources with confidence scores
- Detecting anomalies like memory leaks before they cause incidents

**Typical results:** 20-40% reduction in resource costs with improved reliability.

## How It Works

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Kubernetes Cluster                          │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                          │
│  │  Node 1  │  │  Node 2  │  │  Node 3  │                          │
│  │ ┌──────┐ │  │ ┌──────┐ │  │ ┌──────┐ │                          │
│  │ │Agent │ │  │ │Agent │ │  │ │Agent │ │  Collect metrics         │
│  │ └──┬───┘ │  │ └──┬───┘ │  │ └──┬───┘ │  Run local inference     │
│  └────┼─────┘  └────┼─────┘  └────┼─────┘  Detect anomalies        │
│       │             │             │                                 │
│       └─────────────┼─────────────┘                                 │
│                     │ gRPC streaming                                │
│                     ▼                                               │
│            ┌────────────────┐                                       │
│            │ Kubewise API   │  Aggregate predictions                │
│            │                │  Federated learning                   │
│            │  ┌──────────┐  │  Cost analysis                        │
│            │  │TimescaleDB│  │  Safety controls                     │
│            │  └──────────┘  │                                       │
│            └───────┬────────┘                                       │
│                    │                                                │
│                    ▼                                                │
│         ┌─────────────────────┐                                     │
│         │ResourceRecommendation│  Kubernetes CRD                    │
│         │        CRDs          │  GitOps friendly                   │
│         └─────────────────────┘                                     │
└─────────────────────────────────────────────────────────────────────┘
```

1. **Lightweight agents** run on each node, collecting cgroup metrics every 10 seconds
2. **Edge inference** generates predictions locally using an embedded ML model
3. **Federated learning** improves the model over time without centralizing raw data
4. **Recommendations** are created as Kubernetes CRDs for GitOps workflows
5. **Safety controls** include dry-run mode, approval workflows, and automatic rollback

## Features

- **ML-Powered Predictions**: LSTM-based model trained on real workload patterns
- **Edge Computing**: Predictions run on each node, minimizing network overhead
- **Federated Learning**: Model improves from your cluster's data without privacy concerns
- **Anomaly Detection**: Memory leak detection, CPU spike alerts, OOM risk warnings
- **Cost Analysis**: Track actual vs recommended costs with cloud provider pricing
- **Safety First**: Dry-run mode, approval workflows, automatic rollback on issues
- **Time-Aware**: Separate recommendations for peak, off-peak, and weekly patterns
- **GitOps Ready**: Recommendations as CRDs, CLI tool, REST & gRPC APIs

## Quick Start

### Prerequisites

- Kubernetes 1.23+
- Helm 3.8+
- 3+ nodes recommended (for federated learning)

### Installation

```bash
# Add the Helm repository
helm repo add kubewise https://example.github.io/kubewise
helm repo update

# Install with default settings (dry-run mode enabled)
helm install kubewise kubewise/kubewise \
  --namespace kubewise-system \
  --create-namespace

# Verify installation
kubectl get pods -n kubewise-system
kubectl get daemonset -n kubewise-system
```

### View Recommendations

```bash
# Wait 5-10 minutes for initial data collection, then:
kubectl get resourcerecommendations --all-namespaces

# Example output:
# NAMESPACE   NAME              TARGET      CPU-REQ   MEM-REQ   CONFIDENCE   PHASE
# default     nginx-rec         nginx       50m       64Mi      0.87         Pending
# backend     api-server-rec    api-server  200m      256Mi     0.92         Pending
```

### Apply a Recommendation

```bash
# Dry-run first
kubectl patch resourcerecommendation nginx-rec -n default \
  --type=merge -p '{"spec":{"dryRun":true}}'

# Review the generated patch
kubectl get resourcerecommendation nginx-rec -n default -o jsonpath='{.status.generatedPatch}'

# Apply when ready
kubectl patch resourcerecommendation nginx-rec -n default \
  --type=merge -p '{"spec":{"autoApply":true}}'
```

## Architecture

| Component | Language | Description |
|-----------|----------|-------------|
| `resource-agent` | Rust | DaemonSet that collects metrics and runs inference |
| `recommendation-api` | Go | Central API for aggregation, storage, and management |
| `ml-model` | Python | Training pipeline for the prediction model |
| `charts/` | Helm | Kubernetes deployment manifests |

### Resource Agent (Rust)

Ultra-lightweight agent (~10MB memory) that:
- Reads cgroup v1/v2 metrics directly (no kubelet dependency)
- Runs ONNX inference locally
- Buffers data during network issues
- Streams metrics via gRPC

### Recommendation API (Go)

Central service that:
- Aggregates predictions from all agents
- Performs federated learning aggregation (FedAvg)
- Manages model versioning and distribution
- Provides REST/gRPC APIs
- Tracks costs and savings

### ML Model (Python)

LSTM-based model that predicts:
- CPU request/limit recommendations
- Memory request/limit recommendations
- Confidence scores
- Time-window specific predictions

## Configuration

### Basic Configuration

```yaml
# values.yaml
resourceAgent:
  config:
    collectionInterval: 10      # Metrics collection interval (seconds)
    predictionInterval: 300     # Prediction interval (seconds)
    logLevel: info

recommendationApi:
  config:
    dryRunMode: true            # Start in dry-run mode
    modelUpdateSchedule: "0 2 * * *"  # Daily model updates at 2 AM
  
  autoscaling:
    enabled: true
    minReplicas: 2
    maxReplicas: 10

costEstimation:
  provider: aws                 # aws, gcp, azure, or custom
```

### Production Configuration

See [docs/examples/](docs/examples/) for production-ready configurations:
- `values-small-cluster.yaml` - For clusters < 20 nodes
- `values-medium-cluster.yaml` - For clusters 20-100 nodes  
- `values-large-cluster.yaml` - For clusters 100+ nodes

## CLI Tool

```bash
# Install the CLI
brew install kubewise/tap/kubewise-cli

# Or download directly
curl -LO https://github.com/example/kubewise/releases/latest/download/kw-darwin-amd64
chmod +x kw-darwin-amd64 && sudo mv kw-darwin-amd64 /usr/local/bin/kw

# Usage
kw get recommendations                    # List all recommendations
kw get recommendations -n production      # Filter by namespace
kw apply recommendation nginx-rec --dry-run  # Dry-run apply
kw costs                                  # View cluster costs
kw savings --since 30d                    # View savings report
```

## API Reference

### REST API

```bash
# List recommendations
curl http://kubewise-api:8080/api/v1/recommendations

# Get cost analysis
curl http://kubewise-api:8080/api/v1/costs

# Apply recommendation (dry-run)
curl -X POST http://kubewise-api:8080/api/v1/recommendation/{id}/dry-run
```

### gRPC API

Used for agent-to-API communication:
- `Register` - Agent registration
- `SyncMetrics` - Stream metrics and predictions
- `GetModelUpdate` - Check for model updates
- `UploadGradients` - Federated learning

See [docs/api-reference.md](docs/api-reference.md) for complete documentation.

## Safety Features

Kubewise is designed with safety as a priority:

| Feature | Description |
|---------|-------------|
| Dry-run mode | Preview changes without applying |
| Approval workflow | Require human approval for high-risk changes |
| Confidence thresholds | Only apply recommendations above a confidence level |
| Automatic rollback | Revert if OOM kills or throttling increase |
| Gradual rollout | Apply to subset of replicas first |
| Audit logging | Track all changes and approvals |

### Configure Safety per Namespace

```bash
# Require approval for production namespace
curl -X PUT http://kubewise-api:8080/api/v1/safety/config/production \
  -H "Content-Type: application/json" \
  -d '{
    "dryRunEnabled": true,
    "requireApproval": true,
    "autoApplyEnabled": false
  }'

# Allow auto-apply for development namespace
curl -X PUT http://kubewise-api:8080/api/v1/safety/config/development \
  -H "Content-Type: application/json" \
  -d '{
    "dryRunEnabled": false,
    "requireApproval": false,
    "autoApplyEnabled": true,
    "autoApplyMinConfidence": 0.9
  }'
```

## Monitoring

### Prometheus Metrics

```yaml
# Enable ServiceMonitor
recommendationApi:
  metrics:
    serviceMonitor:
      enabled: true
```

Key metrics:
- `kubewise_recommendations_total` - Total recommendations generated
- `kubewise_recommendations_applied` - Recommendations applied
- `kubewise_savings_dollars` - Estimated savings
- `kubewise_agent_collection_duration_seconds` - Metric collection latency
- `kubewise_prediction_confidence` - Model confidence distribution

### Grafana Dashboard

Import the included dashboard: [docs/examples/grafana-dashboard.json](docs/examples/grafana-dashboard.json)

## Development

### Building from Source

```bash
# Build the agent (Rust)
cd resource-agent
cargo build --release

# Build the API (Go)
cd recommendation-api
make build

# Train the model (Python)
cd ml-model
pip install -r requirements.txt
python generate_data.py
python train.py
```

### Running Tests

```bash
# Agent tests
cd resource-agent && cargo test

# API tests
cd recommendation-api && make test

# Model validation
cd ml-model && python validate.py
```

### Local Development

```bash
# Start local Kubernetes (kind/minikube)
kind create cluster --config hack/kind-config.yaml

# Deploy with local images
make deploy-local
```

## Roadmap

- [ ] GPU resource recommendations
- [ ] Vertical Pod Autoscaler integration
- [ ] Multi-cluster support
- [ ] Custom model training UI
- [ ] Slack/PagerDuty integrations
- [ ] FinOps dashboard

## Contributing

Contributions are welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

Apache License 2.0 - see [LICENSE](LICENSE) for details.

## Acknowledgments

- Inspired by [Kubernetes VPA](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler)
- ML architecture influenced by [Autopilot](https://research.google/pubs/pub49174/)
- Federated learning based on [FedAvg](https://arxiv.org/abs/1602.05629)
