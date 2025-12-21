# User Guide

This guide explains how to use the Smart Container Resource Predictor to optimize your Kubernetes workloads.

## Getting Started

### Understanding the System

The Container Resource Predictor consists of:

1. **Resource Agents**: Run on each node, collecting metrics and generating predictions
2. **Recommendation API**: Aggregates predictions and serves recommendations
3. **CLI Tool**: Command-line interface for interacting with recommendations

### Viewing Your First Recommendations

After installation, the system needs time to collect data. Recommendations become available after:
- Minimum 10 metric samples per container (~2 minutes at default 10s interval)
- Higher confidence after 24+ hours of data collection

Check if recommendations are available:

```bash
# Using kubectl
kubectl get resourcerecommendations --all-namespaces

# Using the CLI
crp get recommendations
```

## Interpreting Recommendations

### Recommendation Structure

Each recommendation includes:

| Field | Description |
|-------|-------------|
| `cpuRequest` | Recommended CPU request (e.g., "100m") |
| `cpuLimit` | Recommended CPU limit (e.g., "500m") |
| `memoryRequest` | Recommended memory request (e.g., "128Mi") |
| `memoryLimit` | Recommended memory limit (e.g., "256Mi") |
| `confidence` | Confidence score (0.0 - 1.0) |
| `timeWindow` | Time window: peak, off-peak, weekly, or all |
| `riskLevel` | Risk level: low, medium, or high |

### Understanding Confidence Scores

| Score Range | Meaning | Recommendation |
|-------------|---------|----------------|
| 0.9 - 1.0 | Very High | Safe to apply automatically |
| 0.7 - 0.9 | High | Review briefly, then apply |
| 0.5 - 0.7 | Medium | Review carefully before applying |
| < 0.5 | Low | Investigate why confidence is low |

Low confidence may indicate:
- Insufficient historical data
- Highly variable workload patterns
- Recent deployment changes

### Time Windows

Recommendations can be specific to time periods:

- **peak**: Optimized for high-traffic periods
- **off-peak**: Optimized for low-traffic periods  
- **weekly**: Accounts for weekly patterns (e.g., weekday vs weekend)
- **all**: General recommendation for all times

### Risk Levels

| Risk Level | Criteria | Action Required |
|------------|----------|-----------------|
| Low | Memory reduction < 10%, CPU reduction < 20% | Can apply directly |
| Medium | Memory reduction 10-30%, CPU reduction 20-40% | Review recommended |
| High | Memory reduction > 30% | Requires approval |

## Applying Recommendations Safely

### Step 1: Review the Recommendation

```bash
# View recommendation details
crp get recommendations --namespace my-app --deployment my-service

# Or via kubectl
kubectl get resourcerecommendation my-service-rec -n my-app -o yaml
```

Example output:
```yaml
apiVersion: predictor.io/v1
kind: ResourceRecommendation
metadata:
  name: my-service-rec
  namespace: my-app
spec:
  targetRef:
    kind: Deployment
    name: my-service
  recommendation:
    cpuRequest: "100m"
    cpuLimit: "500m"
    memoryRequest: "128Mi"
    memoryLimit: "256Mi"
    confidence: 0.87
    timeWindow: all
  costImpact:
    currentMonthlyCost: "$45.00"
    projectedMonthlyCost: "$28.00"
    monthlySavings: "$17.00"
  riskLevel: low
status:
  phase: Pending
```

### Step 2: Use Dry-Run Mode

Always test with dry-run first:

```bash
# CLI dry-run
crp apply recommendation my-service-rec --namespace my-app --dry-run

# API dry-run
curl -X POST http://predictor-api:8080/api/v1/recommendation/{id}/dry-run
```

Dry-run output shows:
- The YAML patch that would be applied
- Current vs recommended values
- Potential impact assessment

### Step 3: Approve High-Risk Recommendations

For high-risk recommendations:

```bash
# Approve via CLI
crp approve recommendation my-service-rec --namespace my-app --reason "Reviewed metrics, safe to apply"

# Approve via API
curl -X POST http://predictor-api:8080/api/v1/recommendation/{id}/approve \
  -H "Content-Type: application/json" \
  -d '{"approver": "user@example.com", "reason": "Reviewed metrics"}'
```

### Step 4: Apply the Recommendation

```bash
# Apply via CLI
crp apply recommendation my-service-rec --namespace my-app

# Apply via kubectl (generates and applies patch)
kubectl patch deployment my-service -n my-app --patch-file patch.yaml
```

### Step 5: Monitor the Outcome

After applying, monitor for issues:

```bash
# Check outcome tracking
crp get recommendation my-service-rec --namespace my-app --show-outcome

# Watch for OOM kills
kubectl get events -n my-app --field-selector reason=OOMKilled

# Check CPU throttling
kubectl top pods -n my-app
```

The system automatically tracks:
- OOM kills in the first hour
- CPU throttling changes
- Overall workload health

## Working with the CLI

### Installation

```bash
# Download the CLI
curl -LO https://github.com/example/container-resource-predictor/releases/latest/download/crp-darwin-amd64
chmod +x crp-darwin-amd64
sudo mv crp-darwin-amd64 /usr/local/bin/crp
```

### Configuration

```bash
# Configure API endpoint
crp config set api-endpoint http://predictor-api.predictor-system:8080

# Or use environment variable
export CRP_API_ENDPOINT=http://predictor-api.predictor-system:8080
```

### Common Commands

```bash
# List all recommendations
crp get recommendations

# Filter by namespace
crp get recommendations --namespace production

# Filter by deployment
crp get recommendations --namespace production --deployment api-server

# Show detailed output
crp get recommendations --output yaml

# View costs
crp costs --namespace production

# View savings report
crp savings --since 30d

# Debug predictions for a deployment
crp debug predictions my-deployment --namespace my-app

# Export metrics
crp debug export --since 7d --output metrics.json
```

## Using the REST API

### Authentication

The API uses Kubernetes ServiceAccount tokens:

```bash
TOKEN=$(kubectl create token predictor-user -n predictor-system)
curl -H "Authorization: Bearer $TOKEN" http://predictor-api:8080/api/v1/recommendations
```

### Common Endpoints

```bash
# List recommendations
GET /api/v1/recommendations
GET /api/v1/recommendations/{namespace}
GET /api/v1/recommendations/{namespace}/{deployment}

# Recommendation actions
POST /api/v1/recommendation/{id}/apply
POST /api/v1/recommendation/{id}/approve
POST /api/v1/recommendation/{id}/dry-run

# Cost analysis
GET /api/v1/costs
GET /api/v1/costs/{namespace}
GET /api/v1/savings?since=30d

# Debug
GET /api/v1/debug/predictions/{deployment}
```

## Working with ResourceRecommendation CRDs

### Listing Recommendations

```bash
# All namespaces
kubectl get resourcerecommendations -A

# Specific namespace
kubectl get rr -n my-app

# With additional columns
kubectl get rr -n my-app -o wide
```

### Viewing Details

```bash
kubectl describe rr my-service-rec -n my-app
```

### Filtering by Status

```bash
# Pending recommendations
kubectl get rr -A --field-selector status.phase=Pending

# Applied recommendations
kubectl get rr -A --field-selector status.phase=Applied
```

## Namespace Configuration

### Configuring Dry-Run Mode

Enable dry-run for specific namespaces:

```bash
# Via API
curl -X PUT http://predictor-api:8080/api/v1/safety/config/production \
  -H "Content-Type: application/json" \
  -d '{"dryRunEnabled": true}'
```

### Configuring Approval Requirements

```bash
# Require approval for all recommendations in a namespace
curl -X PUT http://predictor-api:8080/api/v1/safety/config/production \
  -H "Content-Type: application/json" \
  -d '{"requireApproval": true, "approvalThreshold": 0.0}'
```

### Configuring Auto-Apply

For non-critical namespaces, enable auto-apply for low-risk recommendations:

```bash
curl -X PUT http://predictor-api:8080/api/v1/safety/config/development \
  -H "Content-Type: application/json" \
  -d '{
    "dryRunEnabled": false,
    "requireApproval": false,
    "autoApplyEnabled": true,
    "autoApplyMaxRisk": "low",
    "autoApplyMinConfidence": 0.9
  }'
```

## Handling Anomalies

### Memory Leak Alerts

When a memory leak is detected:

1. Check the alert details:
   ```bash
   kubectl get events -n my-app --field-selector reason=MemoryLeakDetected
   ```

2. Review the projected OOM time

3. Investigate the application for memory leaks

4. Consider applying the recommended memory increase as a temporary fix

### CPU Spike Alerts

When CPU spikes are detected:

1. Check if the spike correlates with expected traffic patterns

2. Review the z-score (how many standard deviations from normal)

3. Investigate if the spike indicates a problem or normal behavior

## Best Practices

### 1. Start with Dry-Run Mode

Always enable dry-run mode initially:
```yaml
recommendationApi:
  config:
    dryRunMode: true
```

### 2. Use Gradual Rollout

Apply recommendations to a subset of replicas first:
1. Scale down to 1 replica
2. Apply recommendation
3. Monitor for 1 hour
4. Scale back up if healthy

### 3. Set Appropriate Confidence Thresholds

For production workloads:
- Require confidence > 0.8 for auto-apply
- Require approval for confidence < 0.7

### 4. Monitor After Applying

Always monitor for at least 1 hour after applying:
- Watch for OOM kills
- Check CPU throttling metrics
- Verify application performance

### 5. Use Time-Window Specific Recommendations

For workloads with variable traffic:
- Apply peak recommendations during high-traffic periods
- Apply off-peak recommendations during maintenance windows

### 6. Review Low-Confidence Recommendations

Investigate why confidence is low:
- Check if workload is new (< 24 hours of data)
- Look for highly variable usage patterns
- Consider if the workload is appropriate for ML-based predictions

## Troubleshooting

### No Recommendations Available

1. Check agent status:
   ```bash
   kubectl get pods -l app.kubernetes.io/component=agent -n predictor-system
   ```

2. Verify metrics collection:
   ```bash
   crp debug agent <node-name>
   ```

3. Ensure minimum data requirements are met (10+ samples)

### Recommendations Seem Wrong

1. Check the confidence score - low confidence indicates uncertainty

2. Review the prediction history:
   ```bash
   crp debug predictions my-deployment --namespace my-app
   ```

3. Compare with actual usage:
   ```bash
   kubectl top pods -n my-app
   ```

### Applied Recommendation Caused Issues

1. Check if automatic rollback was triggered:
   ```bash
   kubectl get rr my-service-rec -n my-app -o jsonpath='{.status.phase}'
   ```

2. Manually rollback if needed:
   ```bash
   kubectl patch deployment my-service -n my-app --patch-file previous-resources.yaml
   ```

3. Report the issue for model improvement

## Next Steps

- [API Reference](api-reference.md) - Complete API documentation
- [Example Configurations](examples/) - Sample configurations
- [Installation Guide](installation.md) - Detailed installation options
