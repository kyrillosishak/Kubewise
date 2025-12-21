# Installation Guide

This guide covers the installation of the Smart Container Resource Predictor on your Kubernetes cluster.

## Prerequisites

Before installing, ensure you have:

- **Kubernetes cluster**: Version 1.23 or later
- **Helm**: Version 3.8 or later
- **kubectl**: Configured to access your cluster
- **Cluster permissions**: Ability to create ClusterRoles, DaemonSets, and CRDs
- **Storage**: 
  - TimescaleDB or PostgreSQL for the Recommendation API
  - Optional: S3/MinIO for model storage

### Resource Requirements

| Component | CPU Request | CPU Limit | Memory Request | Memory Limit |
|-----------|-------------|-----------|----------------|--------------|
| Resource Agent (per node) | 10m | 100m | 32Mi | 64Mi |
| Recommendation API | 100m | 500m | 128Mi | 512Mi |
| TimescaleDB | 500m | 2000m | 1Gi | 4Gi |

## Quick Start

### 1. Add the Helm Repository

```bash
helm repo add container-resource-predictor https://example.github.io/container-resource-predictor
helm repo update
```

### 2. Install with Default Values

```bash
helm install predictor container-resource-predictor/container-resource-predictor \
  --namespace predictor-system \
  --create-namespace
```

### 3. Verify Installation

```bash
# Check that all pods are running
kubectl get pods -n predictor-system

# Verify agents are running on all nodes
kubectl get daemonset -n predictor-system

# Check the API is ready
kubectl get deployment -n predictor-system
```

## Installation Methods

### Method 1: Helm Chart (Recommended)

#### From Helm Repository

```bash
helm install predictor container-resource-predictor/container-resource-predictor \
  --namespace predictor-system \
  --create-namespace \
  --values custom-values.yaml
```

#### From Local Chart

```bash
# Clone the repository
git clone https://github.com/example/container-resource-predictor.git
cd container-resource-predictor

# Install from local chart
helm install predictor ./charts/container-resource-predictor \
  --namespace predictor-system \
  --create-namespace
```

### Method 2: kubectl Apply

For environments without Helm:

```bash
# Generate manifests
helm template predictor ./charts/container-resource-predictor \
  --namespace predictor-system \
  > manifests.yaml

# Apply to cluster
kubectl apply -f manifests.yaml
```

## Configuration Options Reference

### Global Settings

| Parameter | Description | Default |
|-----------|-------------|---------|
| `global.imageRegistry` | Global Docker image registry | `""` |
| `global.imagePullSecrets` | Global image pull secrets | `[]` |

### Resource Agent Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `resourceAgent.enabled` | Enable the Resource Agent DaemonSet | `true` |
| `resourceAgent.image.repository` | Agent image repository | `container-resource-predictor/resource-agent` |
| `resourceAgent.image.tag` | Agent image tag | `""` (uses appVersion) |
| `resourceAgent.image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `resourceAgent.resources.requests.cpu` | CPU request | `10m` |
| `resourceAgent.resources.requests.memory` | Memory request | `32Mi` |
| `resourceAgent.resources.limits.cpu` | CPU limit | `100m` |
| `resourceAgent.resources.limits.memory` | Memory limit | `64Mi` |
| `resourceAgent.nodeSelector` | Node selector for agent pods | `{}` |
| `resourceAgent.tolerations` | Tolerations (default: tolerate all) | `[{operator: Exists}]` |
| `resourceAgent.priorityClassName` | Pod priority class | `""` |
| `resourceAgent.config.collectionInterval` | Metrics collection interval (seconds) | `10` |
| `resourceAgent.config.predictionInterval` | Prediction interval (seconds) | `300` |
| `resourceAgent.config.bufferRetentionHours` | Local buffer retention | `24` |
| `resourceAgent.config.logLevel` | Log level (debug, info, warn, error) | `info` |
| `resourceAgent.config.logFormat` | Log format (json, text) | `json` |
| `resourceAgent.mtls.enabled` | Enable mTLS for agent communication | `true` |
| `resourceAgent.mtls.secretName` | Secret containing client certificates | `resource-agent-mtls` |
| `resourceAgent.metrics.enabled` | Enable Prometheus metrics | `true` |
| `resourceAgent.metrics.port` | Metrics port | `9090` |

### Recommendation API Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `recommendationApi.enabled` | Enable the Recommendation API | `true` |
| `recommendationApi.image.repository` | API image repository | `container-resource-predictor/recommendation-api` |
| `recommendationApi.image.tag` | API image tag | `""` (uses appVersion) |
| `recommendationApi.replicaCount` | Number of API replicas | `2` |
| `recommendationApi.resources.requests.cpu` | CPU request | `100m` |
| `recommendationApi.resources.requests.memory` | Memory request | `128Mi` |
| `recommendationApi.resources.limits.cpu` | CPU limit | `500m` |
| `recommendationApi.resources.limits.memory` | Memory limit | `512Mi` |
| `recommendationApi.autoscaling.enabled` | Enable HPA | `true` |
| `recommendationApi.autoscaling.minReplicas` | Minimum replicas | `2` |
| `recommendationApi.autoscaling.maxReplicas` | Maximum replicas | `10` |
| `recommendationApi.autoscaling.targetCPUUtilizationPercentage` | Target CPU utilization | `70` |
| `recommendationApi.podDisruptionBudget.enabled` | Enable PDB | `true` |
| `recommendationApi.podDisruptionBudget.minAvailable` | Minimum available pods | `1` |
| `recommendationApi.service.type` | Service type | `ClusterIP` |
| `recommendationApi.service.restPort` | REST API port | `8080` |
| `recommendationApi.service.grpcPort` | gRPC port | `9000` |
| `recommendationApi.ingress.enabled` | Enable Ingress | `false` |
| `recommendationApi.config.logLevel` | Log level | `info` |
| `recommendationApi.config.modelUpdateSchedule` | Model update cron schedule | `0 2 * * *` |
| `recommendationApi.config.dryRunMode` | Enable dry-run mode by default | `true` |
| `recommendationApi.mtls.enabled` | Enable mTLS | `true` |
| `recommendationApi.mtls.secretName` | Secret containing server certificates | `recommendation-api-mtls` |
| `recommendationApi.metrics.enabled` | Enable Prometheus metrics | `true` |
| `recommendationApi.metrics.port` | Metrics port | `9091` |
| `recommendationApi.metrics.serviceMonitor.enabled` | Enable ServiceMonitor | `false` |

### Database Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `database.external` | Use external database | `false` |
| `database.connectionString` | External database connection string | `""` |
| `database.host` | Database host | `timescaledb` |
| `database.port` | Database port | `5432` |
| `database.name` | Database name | `predictor` |
| `database.user` | Database user | `predictor` |
| `database.existingSecret` | Existing secret for password | `""` |
| `database.secretKey` | Key in secret for password | `password` |

### Model Storage Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `modelStorage.type` | Storage type (s3, minio, local) | `local` |
| `modelStorage.s3.bucket` | S3 bucket name | `predictor-models` |
| `modelStorage.s3.endpoint` | S3 endpoint (for MinIO) | `""` |
| `modelStorage.s3.region` | S3 region | `us-east-1` |
| `modelStorage.local.storageClass` | Storage class for PVC | `""` |
| `modelStorage.local.size` | PVC size | `1Gi` |

### Cost Estimation Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `costEstimation.provider` | Cloud provider (aws, gcp, azure, custom) | `custom` |
| `costEstimation.customPricing.cpuPerCoreHour` | CPU cost per core-hour | `0.05` |
| `costEstimation.customPricing.memoryPerGBHour` | Memory cost per GB-hour | `0.01` |

### RBAC Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `rbac.create` | Create RBAC resources | `true` |
| `rbac.additionalRules` | Additional ClusterRole rules | `[]` |
| `serviceAccount.create` | Create ServiceAccount | `true` |
| `serviceAccount.name` | ServiceAccount name | `""` |
| `serviceAccount.annotations` | ServiceAccount annotations | `{}` |

## Post-Installation Steps

### 1. Configure mTLS Certificates

If using mTLS (recommended), create the certificate secrets:

```bash
# Generate certificates (example using cert-manager)
kubectl apply -f - <<EOF
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: predictor-api-cert
  namespace: predictor-system
spec:
  secretName: recommendation-api-mtls
  issuerRef:
    name: cluster-issuer
    kind: ClusterIssuer
  commonName: recommendation-api
  dnsNames:
    - recommendation-api
    - recommendation-api.predictor-system.svc
EOF
```

### 2. Configure Database

For production, use an external TimescaleDB:

```yaml
# values-production.yaml
database:
  external: true
  connectionString: "postgres://user:pass@timescaledb.example.com:5432/predictor?sslmode=require"
```

### 3. Enable Prometheus Monitoring

```yaml
# values-monitoring.yaml
recommendationApi:
  metrics:
    serviceMonitor:
      enabled: true
      labels:
        release: prometheus
```

### 4. Configure Ingress

```yaml
# values-ingress.yaml
recommendationApi:
  ingress:
    enabled: true
    className: nginx
    annotations:
      cert-manager.io/cluster-issuer: letsencrypt
    hosts:
      - host: predictor.example.com
        paths:
          - path: /
            pathType: Prefix
    tls:
      - secretName: predictor-tls
        hosts:
          - predictor.example.com
```

## Upgrading

### Upgrade to a New Version

```bash
helm repo update
helm upgrade predictor container-resource-predictor/container-resource-predictor \
  --namespace predictor-system \
  --values custom-values.yaml
```

### Check Upgrade Status

```bash
helm history predictor -n predictor-system
```

### Rollback if Needed

```bash
helm rollback predictor 1 -n predictor-system
```

## Uninstallation

```bash
# Uninstall the release
helm uninstall predictor -n predictor-system

# Delete the namespace (optional)
kubectl delete namespace predictor-system

# Delete CRDs (optional - this removes all ResourceRecommendation resources)
kubectl delete crd resourcerecommendations.predictor.io
```

## Troubleshooting

### Agents Not Starting

Check agent logs:
```bash
kubectl logs -l app.kubernetes.io/component=agent -n predictor-system
```

Common issues:
- Missing cgroup access: Ensure the agent has read access to `/sys/fs/cgroup`
- Certificate issues: Verify mTLS secrets exist

### API Not Ready

Check API logs:
```bash
kubectl logs -l app.kubernetes.io/component=api -n predictor-system
```

Common issues:
- Database connection: Verify database credentials and connectivity
- Certificate issues: Check mTLS configuration

### No Recommendations Generated

1. Verify agents are collecting metrics:
   ```bash
   kubectl port-forward -n predictor-system ds/resource-agent 9090:9090
   curl http://localhost:9090/metrics | grep collection
   ```

2. Check prediction logs:
   ```bash
   kubectl logs -l app.kubernetes.io/component=agent -n predictor-system | grep prediction
   ```

3. Ensure sufficient data (minimum 10 samples per container)

## Next Steps

- [User Guide](user-guide.md) - Learn how to use recommendations
- [API Reference](api-reference.md) - REST and gRPC API documentation
- [Example Configurations](examples/) - Sample configurations for different scenarios
