# Kubewise Test Application

A multi-component Kubernetes application for validating Kubewise's ML-powered resource prediction and recommendation capabilities. The test app generates realistic workload patterns including steady-state usage, memory leaks, CPU spikes, and time-based patterns.

## Quick Start

### Prerequisites

- Docker
- kubectl
- Helm 3.x
- kind (for local testing)
- Go 1.21+ (for building from source)

### Local Testing with kind

```bash
# 1. Create a kind cluster with Prometheus
./hack/setup-kind.sh

# 2. Install Kubewise
./hack/install-kubewise.sh

# 3. Build and deploy the test application
make docker-build
./hack/install-test-app.sh

# 4. Run E2E tests
./hack/run-e2e-tests.sh
```

Or run everything in one command:

```bash
make e2e-full
```

## Components

| Component | Description | Port |
|-----------|-------------|------|
| Pattern Controller | Central orchestrator for test scenarios | 8080 |
| Memory Hog | Generates configurable memory usage patterns | 8082 |
| CPU Burster | Generates configurable CPU usage patterns | 8083 |
| Steady Worker | HTTP service with predictable resource usage | 8084 |
| Load Generator | HTTP traffic generator with configurable patterns | 8081 |
| Metrics Validator | Validates Kubewise predictions against actual usage | 8080 |

## Test Scenarios

Start a scenario via the Pattern Controller API:

```bash
kubectl exec -n kubewise-test deploy/pattern-controller -- \
  wget -q -O - --post-data='{"name":"baseline"}' \
  --header="Content-Type: application/json" \
  http://localhost:8080/api/v1/scenarios/start
```

### Available Scenarios

| Scenario | Duration | Description |
|----------|----------|-------------|
| `baseline` | 2 hours | Steady resource usage for prediction accuracy validation |
| `stress` | 1 hour | High load with burst traffic and CPU spikes |
| `anomaly` | 2 hours | Memory leaks and CPU spikes for anomaly detection testing |
| `full-validation` | 4 hours | Comprehensive test combining all patterns |

### Scenario Details

**baseline**
- Steady Worker: constant CPU/memory usage
- Memory Hog: steady 256MB allocation
- CPU Burster: steady 20% CPU
- Load Generator: constant 50 RPS

**stress**
- Load Generator: burst mode (100 RPS baseline, 500 RPS bursts every 5m)
- CPU Burster: spike mode (30% baseline, 90% spikes every 5m)

**anomaly**
- Phase 1 (0-30m): Memory leak at 10MB/min
- Phase 2 (30-60m): CPU spikes at 95% every 2m
- Phase 3 (60m+): Memory spikes of 200MB every 3m

**full-validation**
- Combines baseline, ramp-up, leak, spike, burst, and wave patterns
- Tests all Kubewise capabilities in sequence


## Component Configuration

### Memory Hog Modes

```bash
# Steady mode - maintain constant memory
kubectl exec -n kubewise-test deploy/memory-hog -- \
  wget -q -O - --post-data='{"mode":"steady","targetMB":256}' \
  --header="Content-Type: application/json" \
  http://localhost:8082/api/v1/config

# Leak mode - simulate memory leak
kubectl exec -n kubewise-test deploy/memory-hog -- \
  wget -q -O - --post-data='{"mode":"leak","leakRateMBMin":10}' \
  --header="Content-Type: application/json" \
  http://localhost:8082/api/v1/config

# Spike mode - periodic memory spikes
kubectl exec -n kubewise-test deploy/memory-hog -- \
  wget -q -O - --post-data='{"mode":"spike","spikeSizeMB":128,"spikeInterval":"5m"}' \
  --header="Content-Type: application/json" \
  http://localhost:8082/api/v1/config
```

### CPU Burster Modes

```bash
# Steady mode - constant CPU usage
kubectl exec -n kubewise-test deploy/cpu-burster -- \
  wget -q -O - --post-data='{"mode":"steady","targetPercent":30}' \
  --header="Content-Type: application/json" \
  http://localhost:8083/api/v1/config

# Spike mode - periodic CPU bursts
kubectl exec -n kubewise-test deploy/cpu-burster -- \
  wget -q -O - --post-data='{"mode":"spike","spikePercent":90,"spikeInterval":"5m"}' \
  --header="Content-Type: application/json" \
  http://localhost:8083/api/v1/config

# Wave mode - sinusoidal CPU pattern
kubectl exec -n kubewise-test deploy/cpu-burster -- \
  wget -q -O - --post-data='{"mode":"wave","waveMin":10,"waveMax":80,"wavePeriod":"10m"}' \
  --header="Content-Type: application/json" \
  http://localhost:8083/api/v1/config

# Random mode - unpredictable CPU usage
kubectl exec -n kubewise-test deploy/cpu-burster -- \
  wget -q -O - --post-data='{"mode":"random"}' \
  --header="Content-Type: application/json" \
  http://localhost:8083/api/v1/config
```

### Load Generator Modes

```bash
# Constant RPS
kubectl exec -n kubewise-test deploy/load-generator -- \
  wget -q -O - --post-data='{"mode":"constant","rps":100}' \
  --header="Content-Type: application/json" \
  http://localhost:8081/api/v1/config

# Ramp up traffic
kubectl exec -n kubewise-test deploy/load-generator -- \
  wget -q -O - --post-data='{"mode":"ramp-up","rampStartRPS":10,"rampEndRPS":200,"rampDuration":"10m"}' \
  --header="Content-Type: application/json" \
  http://localhost:8081/api/v1/config

# Burst mode
kubectl exec -n kubewise-test deploy/load-generator -- \
  wget -q -O - --post-data='{"mode":"burst","rps":50,"burstRPS":500,"burstInterval":"5m"}' \
  --header="Content-Type: application/json" \
  http://localhost:8081/api/v1/config

# Start/stop load generation
kubectl exec -n kubewise-test deploy/load-generator -- \
  wget -q -O - --post-data='' http://localhost:8081/api/v1/start

kubectl exec -n kubewise-test deploy/load-generator -- \
  wget -q -O - --post-data='' http://localhost:8081/api/v1/stop
```

## Validation Reports

### Triggering Validation

```bash
# Trigger a validation cycle
kubectl exec -n kubewise-test deploy/metrics-validator -- \
  wget -q -O - --post-data='' http://localhost:8080/api/v1/validate

# Get the latest report
kubectl exec -n kubewise-test deploy/metrics-validator -- \
  wget -q -O - http://localhost:8080/api/v1/reports/latest
```

### Report Structure

```json
{
  "generated_at": "2024-01-15T10:30:00Z",
  "test_duration": "2h30m",
  "scenario_name": "full-validation",
  "overall_cpu_accuracy": 85.5,
  "overall_mem_accuracy": 88.2,
  "confidence_correlation": 0.72,
  "total_predictions": 150,
  "accurate_predictions": 128,
  "anomaly_stats": {
    "total": 10,
    "detected": 9,
    "detection_rate": 0.90,
    "false_positive_rate": 0.05,
    "by_type": {
      "memory_leak": {"total": 5, "detected": 5, "avg_detection_time": "12m"},
      "cpu_spike": {"total": 5, "detected": 4, "avg_detection_time": "3m"}
    }
  },
  "cost_stats": {
    "total_validations": 50,
    "average_accuracy": 0.85
  },
  "pass_criteria": [
    {"name": "CPU Prediction Accuracy", "expected": ">= 70%", "actual": "85.5%", "pass": true},
    {"name": "Memory Prediction Accuracy", "expected": ">= 70%", "actual": "88.2%", "pass": true},
    {"name": "Anomaly Detection Rate", "expected": ">= 90%", "actual": "90.0%", "pass": true},
    {"name": "False Positive Rate", "expected": "<= 10%", "actual": "5.0%", "pass": true},
    {"name": "Cost Estimation Accuracy", "expected": ">= 80%", "actual": "85.0%", "pass": true}
  ],
  "overall_pass": true
}
```

### Pass Criteria

| Metric | Threshold | Description |
|--------|-----------|-------------|
| CPU Prediction Accuracy | ≥ 70% | Kubewise CPU recommendations vs actual usage |
| Memory Prediction Accuracy | ≥ 70% | Kubewise memory recommendations vs actual usage |
| Anomaly Detection Rate | ≥ 90% | Percentage of triggered anomalies detected |
| False Positive Rate | ≤ 10% | Percentage of false anomaly alerts |
| Cost Estimation Accuracy | ≥ 80% | Kubewise cost estimates vs calculated costs |
| Memory Leak Detection Time | ≤ 30 min | Time to detect memory leak anomalies |
| CPU Spike Detection Time | ≤ 5 min | Time to detect CPU spike anomalies |


## Running E2E Tests

### Test Scenarios

```bash
# Run baseline tests (default)
./hack/run-e2e-tests.sh

# Run specific scenario
./hack/run-e2e-tests.sh -s stress
./hack/run-e2e-tests.sh -s anomaly
./hack/run-e2e-tests.sh -s full

# Verbose output
./hack/run-e2e-tests.sh -s full -v

# Custom timeout
./hack/run-e2e-tests.sh -s full -t 2h
```

### Test Output

```
==========================================
Running E2E Tests
==========================================

Test namespace:     kubewise-test
Kubewise namespace: kubewise-system
Scenario:           baseline

[TEST] Testing component health...
  ✓ pattern-controller: healthy
  ✓ memory-hog: healthy
  ✓ cpu-burster: healthy
  ✓ steady-worker: healthy
  ✓ load-generator: healthy
  ✓ metrics-validator: healthy
[INFO] All components healthy

[TEST] Testing Kubewise API accessibility...
  ✓ Kubewise API is accessible

[TEST] Running baseline scenario...
[INFO] Starting baseline scenario...
  ✓ Baseline scenario started
[INFO] Waiting for data collection (5m)...
  ✓ Baseline scenario completed

[TEST] Testing validation report generation...
  ✓ Validation report generated

==========================================
Test Summary
==========================================

Total:   5
Passed:  5
Failed:  0
Skipped: 0

[INFO] All tests passed! ✓
```

## Helm Chart Configuration

### Custom Values

```yaml
# custom-values.yaml
namespace: kubewise-test

patternController:
  config:
    timeAcceleration: "24.0"  # Compress 24h into 1h

memoryHog:
  config:
    mode: "leak"
    leakRateMBMin: "20"
  resources:
    limits:
      memory: 1Gi

cpuBurster:
  config:
    mode: "spike"
    spikePercent: "95"
  resources:
    limits:
      cpu: "4"

loadGenerator:
  config:
    rps: "200"
    autoStart: "true"

metricsValidator:
  config:
    validationInterval: "2m"
    webhookURL: "http://my-webhook:8080/notify"
```

Install with custom values:

```bash
helm upgrade --install kubewise-test ./charts/kubewise-test \
  --namespace kubewise-test \
  --create-namespace \
  --values custom-values.yaml
```

## Prometheus Metrics

All components expose metrics at `/metrics` in Prometheus format.

### Key Metrics

```promql
# Component health
kubewise_test_component_up{component="memory-hog"}

# Resource usage
kubewise_test_cpu_usage_percent{component="cpu-burster"}
kubewise_test_memory_usage_bytes{component="memory-hog"}

# Prediction accuracy
kubewise_test_prediction_accuracy{component="steady-worker", resource_type="cpu"}
kubewise_test_prediction_accuracy{component="steady-worker", resource_type="memory"}

# Anomaly detection timing
histogram_quantile(0.95, kubewise_test_anomaly_detection_seconds{anomaly_type="memory_leak"})

# Validation results
kubewise_test_validation_total{component="steady-worker", result="accurate"}
kubewise_test_validation_total{component="steady-worker", result="inaccurate"}

# Load generator stats
kubewise_test_load_requests_total{status="success"}
kubewise_test_load_latency_seconds{quantile="0.99"}
```

## Troubleshooting

### Check Component Logs

```bash
# Pattern Controller
kubectl logs -n kubewise-test -l app=pattern-controller -f

# Memory Hog
kubectl logs -n kubewise-test -l app=memory-hog -f

# CPU Burster
kubectl logs -n kubewise-test -l app=cpu-burster -f

# Metrics Validator
kubectl logs -n kubewise-test -l app=metrics-validator -f
```

### Check Component Status

```bash
# Get all pods
kubectl get pods -n kubewise-test -o wide

# Describe a specific pod
kubectl describe pod -n kubewise-test -l app=memory-hog

# Check resource usage
kubectl top pods -n kubewise-test
```

### Common Issues

**Pods not starting**
- Check image availability: `docker images | grep kubewise-test`
- For kind clusters, ensure images are loaded: `make kind-load`

**Metrics Validator can't reach Kubewise**
- Verify Kubewise is running: `kubectl get pods -n kubewise-system`
- Check service connectivity: `kubectl exec -n kubewise-test deploy/metrics-validator -- wget -q -O - http://kubewise-api.kubewise-system:8080/healthz`

**Memory Hog hitting limits**
- The component auto-pauses near container limits
- Increase limits in values.yaml or reduce leak rate

**E2E tests timing out**
- Increase timeout: `./hack/run-e2e-tests.sh -t 1h`
- Check if all pods are ready: `kubectl get pods -n kubewise-test`

## Development

### Building from Source

```bash
# Build all binaries
make build

# Build specific component
make cpu-burster

# Run tests
make test

# Run linter
make lint
```

### Building Docker Images

```bash
# Build all images
make docker-build

# Build specific image
make docker-build-memory-hog

# Push to registry
DOCKER_REGISTRY=myregistry.io make docker-push
```

## License

See the [main Kubewise repository](https://github.com/kyrillosishak/Kubewise) for license information.
