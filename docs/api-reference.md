# API Reference

This document provides complete API documentation for the Container Resource Predictor.

## REST API

Base URL: `http://<api-service>:8080`

### Authentication

The API supports two authentication methods:

1. **Kubernetes ServiceAccount Token**
   ```bash
   TOKEN=$(kubectl create token predictor-user -n predictor-system)
   curl -H "Authorization: Bearer $TOKEN" http://api:8080/api/v1/recommendations
   ```

2. **OIDC Token** (if configured)
   ```bash
   curl -H "Authorization: Bearer $OIDC_TOKEN" http://api:8080/api/v1/recommendations
   ```

---

## Health Endpoints

### GET /healthz

Returns the health status of the API.

**Response**
```json
{
  "status": "healthy"
}
```

**Status Codes**
- `200`: Service is healthy
- `503`: Service is unhealthy

---

### GET /readyz

Returns the readiness status of the API.

**Response**
```json
{
  "status": "ready"
}
```

**Status Codes**
- `200`: Service is ready to accept traffic
- `503`: Service is not ready

---

## Recommendations

### GET /api/v1/recommendations

List all recommendations across all namespaces.

**Query Parameters**
| Parameter | Type | Description |
|-----------|------|-------------|
| `status` | string | Filter by status (pending, approved, applied) |
| `minConfidence` | float | Minimum confidence score (0-1) |
| `limit` | int | Maximum results (default: 100) |
| `offset` | int | Pagination offset |

**Response**
```json
{
  "recommendations": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "namespace": "production",
      "deployment": "api-server",
      "cpuRequestMillicores": 100,
      "cpuLimitMillicores": 500,
      "memoryRequestBytes": 134217728,
      "memoryLimitBytes": 268435456,
      "confidence": 0.87,
      "modelVersion": "v1.2.0",
      "status": "pending",
      "createdAt": "2024-12-21T10:30:00Z",
      "timeWindow": "peak"
    }
  ],
  "total": 1
}
```

---

### GET /api/v1/recommendations/{namespace}

List recommendations for a specific namespace.

**Path Parameters**
| Parameter | Type | Description |
|-----------|------|-------------|
| `namespace` | string | Kubernetes namespace |

**Response**: Same as GET /api/v1/recommendations

---

### GET /api/v1/recommendations/{namespace}/{name}

Get a specific recommendation by namespace and deployment name.

**Path Parameters**
| Parameter | Type | Description |
|-----------|------|-------------|
| `namespace` | string | Kubernetes namespace |
| `name` | string | Deployment name |

**Response**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "namespace": "production",
  "deployment": "api-server",
  "cpuRequestMillicores": 100,
  "cpuLimitMillicores": 500,
  "memoryRequestBytes": 134217728,
  "memoryLimitBytes": 268435456,
  "confidence": 0.87,
  "modelVersion": "v1.2.0",
  "status": "pending",
  "createdAt": "2024-12-21T10:30:00Z",
  "timeWindow": "peak",
  "costImpact": {
    "currentMonthlyCost": 45.00,
    "projectedMonthlyCost": 28.00,
    "monthlySavings": 17.00,
    "currency": "USD"
  }
}
```

**Status Codes**
- `200`: Success
- `404`: Recommendation not found

---

### POST /api/v1/recommendation/{id}/apply

Apply a recommendation to the target workload.

**Path Parameters**
| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | string | Recommendation ID (UUID) |

**Request Body**
```json
{
  "dryRun": false
}
```

**Response**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "applied",
  "message": "Recommendation applied successfully",
  "yamlPatch": "apiVersion: apps/v1\nkind: Deployment..."
}
```

**Status Codes**
- `200`: Successfully applied
- `400`: Invalid request
- `403`: Requires approval
- `404`: Recommendation not found

---

### POST /api/v1/recommendation/{id}/approve

Approve a recommendation for application.

**Path Parameters**
| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | string | Recommendation ID (UUID) |

**Request Body**
```json
{
  "approver": "user@example.com",
  "reason": "Reviewed metrics, safe to apply"
}
```

**Response**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "approved",
  "message": "Recommendation approved",
  "approver": "user@example.com",
  "approvedAt": "2024-12-21T11:00:00Z"
}
```

---

### POST /api/v1/recommendation/{id}/dry-run

Perform a dry-run of applying a recommendation.

**Response**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "wouldApply": true,
  "yamlPatch": "apiVersion: apps/v1\nkind: Deployment...",
  "currentResources": {
    "cpuRequest": "200m",
    "cpuLimit": "1000m",
    "memoryRequest": "256Mi",
    "memoryLimit": "512Mi"
  },
  "recommendedResources": {
    "cpuRequest": "100m",
    "cpuLimit": "500m",
    "memoryRequest": "128Mi",
    "memoryLimit": "256Mi"
  }
}
```

---

### GET /api/v1/recommendation/{id}/approval-history

Get the approval history for a recommendation.

**Response**
```json
{
  "history": [
    {
      "action": "approved",
      "user": "user@example.com",
      "reason": "Reviewed metrics",
      "timestamp": "2024-12-21T11:00:00Z"
    }
  ]
}
```

---

### GET /api/v1/recommendation/{id}/outcome

Get the outcome tracking data for an applied recommendation.

**Response**
```json
{
  "recommendationId": "550e8400-e29b-41d4-a716-446655440000",
  "appliedAt": "2024-12-21T11:00:00Z",
  "observationPeriod": "1h",
  "oomKills": 0,
  "cpuThrottleIncrease": 0.0,
  "healthy": true
}
```

---

## Cost Analysis

### GET /api/v1/costs

Get cluster-wide cost analysis.

**Response**
```json
{
  "currentMonthlyCost": 1500.00,
  "recommendedMonthlyCost": 1050.00,
  "potentialSavings": 450.00,
  "currency": "USD",
  "deploymentCount": 25,
  "lastUpdated": "2024-12-21T10:00:00Z"
}
```

---

### GET /api/v1/costs/{namespace}

Get cost analysis for a specific namespace.

**Path Parameters**
| Parameter | Type | Description |
|-----------|------|-------------|
| `namespace` | string | Kubernetes namespace |

**Response**
```json
{
  "namespace": "production",
  "currentMonthlyCost": 300.00,
  "recommendedMonthlyCost": 210.00,
  "potentialSavings": 90.00,
  "currency": "USD",
  "deploymentCount": 5,
  "lastUpdated": "2024-12-21T10:00:00Z"
}
```

---

### GET /api/v1/savings

Get savings report.

**Query Parameters**
| Parameter | Type | Description |
|-----------|------|-------------|
| `since` | string | Time period (e.g., "30d", "7d") |

**Response**
```json
{
  "totalSavings": 1350.00,
  "currency": "USD",
  "period": "30d",
  "savingsByMonth": [
    { "month": "2024-12", "savings": 450.00 },
    { "month": "2024-11", "savings": 450.00 }
  ],
  "savingsByTeam": [
    { "team": "platform", "savings": 500.00 },
    { "team": "backend", "savings": 450.00 }
  ]
}
```

---

## Model Management

### GET /api/v1/models

List all model versions.

**Response**
```json
{
  "models": [
    {
      "version": "v1.2.0",
      "createdAt": "2024-12-20T02:00:00Z",
      "validationAccuracy": 0.94,
      "isActive": true,
      "sizeBytes": 98304
    }
  ]
}
```

---

### GET /api/v1/models/{version}

Get details for a specific model version.

**Response**
```json
{
  "version": "v1.2.0",
  "createdAt": "2024-12-20T02:00:00Z",
  "validationAccuracy": 0.94,
  "isActive": true,
  "sizeBytes": 98304,
  "deployedAgents": 45,
  "totalAgents": 50
}
```

---

### POST /api/v1/models/rollback/{version}

Rollback to a previous model version.

**Response**
```json
{
  "success": true,
  "message": "Rolled back to version v1.1.0",
  "previousVersion": "v1.2.0",
  "newVersion": "v1.1.0"
}
```

---

## Safety Configuration

### GET /api/v1/safety/config

List safety configurations for all namespaces.

**Response**
```json
{
  "configs": [
    {
      "namespace": "production",
      "dryRunEnabled": true,
      "requireApproval": true,
      "autoApplyEnabled": false
    }
  ]
}
```

---

### GET /api/v1/safety/config/{namespace}

Get safety configuration for a namespace.

**Response**
```json
{
  "namespace": "production",
  "dryRunEnabled": true,
  "requireApproval": true,
  "approvalThreshold": 0.3,
  "autoApplyEnabled": false,
  "autoApplyMaxRisk": "low",
  "autoApplyMinConfidence": 0.9
}
```

---

### PUT /api/v1/safety/config/{namespace}

Update safety configuration for a namespace.

**Request Body**
```json
{
  "dryRunEnabled": false,
  "requireApproval": true,
  "approvalThreshold": 0.3,
  "autoApplyEnabled": false
}
```

---

### GET /api/v1/safety/rollbacks

List automatic rollback events.

**Response**
```json
{
  "rollbacks": [
    {
      "id": "rollback-123",
      "recommendationId": "550e8400-e29b-41d4-a716-446655440000",
      "namespace": "production",
      "deployment": "api-server",
      "reason": "OOM kills detected",
      "triggeredAt": "2024-12-21T12:00:00Z"
    }
  ]
}
```

---

## Debug Endpoints

### GET /api/v1/debug/predictions/{deployment}

Get prediction history for a deployment.

**Query Parameters**
| Parameter | Type | Description |
|-----------|------|-------------|
| `namespace` | string | Kubernetes namespace |
| `since` | string | Time period (e.g., "24h", "7d") |

**Response**
```json
{
  "deployment": "api-server",
  "namespace": "production",
  "predictions": [
    {
      "timestamp": "2024-12-21T10:00:00Z",
      "cpuRequestMillicores": 100,
      "memoryRequestBytes": 134217728,
      "confidence": 0.87,
      "modelVersion": "v1.2.0"
    }
  ]
}
```

---

## Audit Endpoints

### GET /api/v1/audit/logs

Get audit logs (requires admin permissions).

**Query Parameters**
| Parameter | Type | Description |
|-----------|------|-------------|
| `since` | string | Time period |
| `action` | string | Filter by action type |
| `user` | string | Filter by user |

**Response**
```json
{
  "logs": [
    {
      "timestamp": "2024-12-21T11:00:00Z",
      "user": "user@example.com",
      "action": "approve_recommendation",
      "resource": "recommendation/550e8400-e29b-41d4-a716-446655440000",
      "details": { "reason": "Reviewed metrics" }
    }
  ]
}
```

---

## Error Responses

All endpoints return errors in a consistent format:

```json
{
  "error": "Human-readable error message",
  "code": "ERROR_CODE",
  "details": {}
}
```

**Common Error Codes**
| Code | HTTP Status | Description |
|------|-------------|-------------|
| `INVALID_REQUEST` | 400 | Request validation failed |
| `UNAUTHORIZED` | 401 | Authentication required |
| `FORBIDDEN` | 403 | Insufficient permissions |
| `NOT_FOUND` | 404 | Resource not found |
| `RATE_LIMITED` | 429 | Too many requests |
| `INTERNAL_ERROR` | 500 | Internal server error |
| `SERVICE_UNAVAILABLE` | 503 | Service temporarily unavailable |

---

## gRPC API

The gRPC API is used for agent-to-API communication.

**Service Definition**: `predictor.v1.PredictorSync`

**Port**: 9000 (default)

### Service Methods

#### Register

Register an agent with the API.

```protobuf
rpc Register(RegisterRequest) returns (RegisterResponse);
```

**RegisterRequest**
| Field | Type | Description |
|-------|------|-------------|
| `agent_id` | string | Unique agent identifier |
| `node_name` | string | Kubernetes node name |
| `kubernetes_version` | string | Kubernetes version |
| `agent_version` | string | Agent software version |
| `model_version` | string | Current ML model version |

**RegisterResponse**
| Field | Type | Description |
|-------|------|-------------|
| `success` | bool | Registration success |
| `message` | string | Status message |
| `config` | AgentConfig | Configuration for the agent |

---

#### SyncMetrics

Stream metrics from agent to API.

```protobuf
rpc SyncMetrics(stream MetricsBatch) returns (SyncResponse);
```

**MetricsBatch**
| Field | Type | Description |
|-------|------|-------------|
| `agent_id` | string | Agent identifier |
| `node_name` | string | Node name |
| `timestamp` | Timestamp | Batch timestamp |
| `metrics` | ContainerMetrics[] | Container metrics |
| `predictions` | ResourceProfile[] | Generated predictions |
| `anomalies` | Anomaly[] | Detected anomalies |

**SyncResponse**
| Field | Type | Description |
|-------|------|-------------|
| `success` | bool | Sync success |
| `message` | string | Status message |
| `metrics_received` | int64 | Number of metrics received |
| `predictions_received` | int64 | Number of predictions received |

---

#### GetModelUpdate

Check for and download model updates.

```protobuf
rpc GetModelUpdate(ModelRequest) returns (ModelResponse);
```

**ModelRequest**
| Field | Type | Description |
|-------|------|-------------|
| `agent_id` | string | Agent identifier |
| `current_model_version` | string | Current model version |

**ModelResponse**
| Field | Type | Description |
|-------|------|-------------|
| `update_available` | bool | Whether update is available |
| `new_version` | string | New model version |
| `model_weights` | bytes | Model weights data |
| `checksum` | string | SHA256 checksum |
| `metadata` | ModelMetadata | Model metadata |

---

#### UploadGradients

Upload federated learning gradients.

```protobuf
rpc UploadGradients(GradientsRequest) returns (GradientsResponse);
```

**GradientsRequest**
| Field | Type | Description |
|-------|------|-------------|
| `agent_id` | string | Agent identifier |
| `model_version` | string | Model version used |
| `gradients` | bytes | Gradient data |
| `sample_count` | int64 | Number of samples |

**GradientsResponse**
| Field | Type | Description |
|-------|------|-------------|
| `success` | bool | Upload success |
| `message` | string | Status message |

---

### Message Types

#### ContainerMetrics

```protobuf
message ContainerMetrics {
  string container_id = 1;
  string pod_name = 2;
  string namespace = 3;
  string deployment = 4;
  Timestamp timestamp = 5;
  float cpu_usage_cores = 6;
  uint64 cpu_throttled_periods = 7;
  uint64 cpu_throttled_time_ns = 8;
  uint64 memory_usage_bytes = 9;
  uint64 memory_working_set_bytes = 10;
  uint64 memory_cache_bytes = 11;
  uint64 memory_rss_bytes = 12;
  uint64 network_rx_bytes = 13;
  uint64 network_tx_bytes = 14;
}
```

#### ResourceProfile

```protobuf
message ResourceProfile {
  string container_id = 1;
  string pod_name = 2;
  string namespace = 3;
  string deployment = 4;
  uint32 cpu_request_millicores = 5;
  uint32 cpu_limit_millicores = 6;
  uint64 memory_request_bytes = 7;
  uint64 memory_limit_bytes = 8;
  float confidence = 9;
  string model_version = 10;
  Timestamp generated_at = 11;
  TimeWindow time_window = 12;
}
```

#### Anomaly

```protobuf
message Anomaly {
  string container_id = 1;
  string pod_name = 2;
  string namespace = 3;
  AnomalyType type = 4;
  Severity severity = 5;
  string message = 6;
  Timestamp detected_at = 7;
  oneof details {
    MemoryLeakDetails memory_leak = 8;
    CpuSpikeDetails cpu_spike = 9;
    OomRiskDetails oom_risk = 10;
  }
}
```

---

### Enums

#### TimeWindow
| Value | Description |
|-------|-------------|
| `TIME_WINDOW_UNSPECIFIED` | Not specified |
| `TIME_WINDOW_PEAK` | Peak hours |
| `TIME_WINDOW_OFF_PEAK` | Off-peak hours |
| `TIME_WINDOW_WEEKLY` | Weekly pattern |

#### AnomalyType
| Value | Description |
|-------|-------------|
| `ANOMALY_TYPE_UNSPECIFIED` | Not specified |
| `ANOMALY_TYPE_MEMORY_LEAK` | Memory leak detected |
| `ANOMALY_TYPE_CPU_SPIKE` | CPU spike detected |
| `ANOMALY_TYPE_OOM_RISK` | OOM risk detected |

#### Severity
| Value | Description |
|-------|-------------|
| `SEVERITY_UNSPECIFIED` | Not specified |
| `SEVERITY_WARNING` | Warning level |
| `SEVERITY_CRITICAL` | Critical level |

---

## ResourceRecommendation CRD

The `ResourceRecommendation` Custom Resource Definition represents a resource recommendation.

### API Version

```yaml
apiVersion: predictor.io/v1
kind: ResourceRecommendation
```

### Spec Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `targetRef` | object | Yes | Reference to target workload |
| `targetRef.apiVersion` | string | No | API version (default: apps/v1) |
| `targetRef.kind` | string | Yes | Kind (Deployment, StatefulSet, DaemonSet) |
| `targetRef.name` | string | Yes | Workload name |
| `targetRef.containerName` | string | No | Container name (if multiple) |
| `recommendation` | object | Yes | Recommended resources |
| `recommendation.cpuRequest` | string | No | CPU request (e.g., "100m") |
| `recommendation.cpuLimit` | string | No | CPU limit |
| `recommendation.memoryRequest` | string | No | Memory request (e.g., "128Mi") |
| `recommendation.memoryLimit` | string | No | Memory limit |
| `recommendation.confidence` | number | No | Confidence score (0-1) |
| `recommendation.modelVersion` | string | No | Model version |
| `recommendation.generatedAt` | string | No | Generation timestamp |
| `recommendation.timeWindow` | string | No | Time window (peak, off-peak, weekly, all) |
| `costImpact` | object | No | Cost impact analysis |
| `autoApply` | boolean | No | Auto-apply flag (default: false) |
| `requiresApproval` | boolean | No | Requires approval (default: true) |
| `riskLevel` | string | No | Risk level (low, medium, high) |

### Status Fields

| Field | Type | Description |
|-------|------|-------------|
| `phase` | string | Current phase (Pending, Approved, Applied, RolledBack, Failed, Rejected) |
| `conditions` | array | Status conditions |
| `appliedAt` | string | Application timestamp |
| `appliedBy` | string | User/system that applied |
| `approvedAt` | string | Approval timestamp |
| `approvedBy` | string | User who approved |
| `previousResources` | object | Previous resource values |
| `outcome` | object | Outcome tracking data |
| `lastUpdated` | string | Last update timestamp |
| `message` | string | Status message |
| `generatedPatch` | string | Generated YAML patch |

### Example

```yaml
apiVersion: predictor.io/v1
kind: ResourceRecommendation
metadata:
  name: api-server-rec
  namespace: production
spec:
  targetRef:
    kind: Deployment
    name: api-server
  recommendation:
    cpuRequest: "100m"
    cpuLimit: "500m"
    memoryRequest: "128Mi"
    memoryLimit: "256Mi"
    confidence: 0.87
    modelVersion: "v1.2.0"
    timeWindow: all
  costImpact:
    currentMonthlyCost: "$45.00"
    projectedMonthlyCost: "$28.00"
    monthlySavings: "$17.00"
  riskLevel: low
  requiresApproval: true
status:
  phase: Pending
  conditions:
    - type: Ready
      status: "True"
      lastTransitionTime: "2024-12-21T10:30:00Z"
      reason: RecommendationGenerated
      message: Recommendation ready for review
```

### Short Names

- `rr`
- `resrec`

### Additional Printer Columns

```bash
kubectl get rr -o wide
```

| Column | Description |
|--------|-------------|
| Target | Target workload name |
| CPU-Req | Recommended CPU request |
| Mem-Req | Recommended memory request |
| Confidence | Confidence score |
| Phase | Current phase |
| Age | Resource age |
