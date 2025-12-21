#!/bin/bash
# setup-kind.sh - Create a kind cluster for Kubewise testing
#
# This script creates a kind cluster with:
# - 1 control-plane node
# - 2 worker nodes with cgroup mounts for resource monitoring
# - metrics-server for resource metrics
# - Prometheus for metrics collection
#
# Requirements: 1.4, 1.6

set -euo pipefail

# Configuration
CLUSTER_NAME="${CLUSTER_NAME:-kubewise-test}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
KIND_CONFIG="${SCRIPT_DIR}/kind-config.yaml"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    local missing=()
    
    if ! command -v kind &> /dev/null; then
        missing+=("kind")
    fi
    
    if ! command -v kubectl &> /dev/null; then
        missing+=("kubectl")
    fi
    
    if ! command -v helm &> /dev/null; then
        missing+=("helm")
    fi
    
    if ! command -v docker &> /dev/null; then
        missing+=("docker")
    fi
    
    if [ ${#missing[@]} -ne 0 ]; then
        log_error "Missing required tools: ${missing[*]}"
        echo "Please install the missing tools and try again."
        echo ""
        echo "Installation guides:"
        echo "  kind:    https://kind.sigs.k8s.io/docs/user/quick-start/#installation"
        echo "  kubectl: https://kubernetes.io/docs/tasks/tools/"
        echo "  helm:    https://helm.sh/docs/intro/install/"
        echo "  docker:  https://docs.docker.com/get-docker/"
        exit 1
    fi
    
    # Check if Docker is running
    if ! docker info &> /dev/null; then
        log_error "Docker is not running. Please start Docker and try again."
        exit 1
    fi
    
    log_info "All prerequisites satisfied"
}

# Delete existing cluster if it exists
delete_existing_cluster() {
    if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
        log_warn "Cluster '${CLUSTER_NAME}' already exists"
        read -p "Do you want to delete it and create a new one? [y/N] " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            log_info "Deleting existing cluster..."
            kind delete cluster --name "${CLUSTER_NAME}"
        else
            log_info "Using existing cluster"
            return 1
        fi
    fi
    return 0
}

# Create kind cluster
create_cluster() {
    log_info "Creating kind cluster '${CLUSTER_NAME}'..."
    
    if [ ! -f "${KIND_CONFIG}" ]; then
        log_error "Kind config file not found: ${KIND_CONFIG}"
        exit 1
    fi
    
    kind create cluster --name "${CLUSTER_NAME}" --config "${KIND_CONFIG}"
    
    log_info "Waiting for nodes to be ready..."
    kubectl wait --for=condition=Ready nodes --all --timeout=120s
    
    log_info "Cluster created successfully"
}

# Install metrics-server
install_metrics_server() {
    log_info "Installing metrics-server..."
    
    # Apply metrics-server manifest
    kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
    
    # Patch for kind (insecure TLS)
    kubectl patch deployment metrics-server -n kube-system --type='json' \
        -p='[
            {"op": "add", "path": "/spec/template/spec/containers/0/args/-", "value": "--kubelet-insecure-tls"},
            {"op": "add", "path": "/spec/template/spec/containers/0/args/-", "value": "--kubelet-preferred-address-types=InternalIP"}
        ]'
    
    log_info "Waiting for metrics-server to be ready..."
    kubectl rollout status deployment/metrics-server -n kube-system --timeout=120s
    
    log_info "metrics-server installed successfully"
}


# Install Prometheus
install_prometheus() {
    log_info "Installing Prometheus..."
    
    # Add Prometheus helm repo
    helm repo add prometheus-community https://prometheus-community.github.io/helm-charts 2>/dev/null || true
    helm repo update
    
    # Create monitoring namespace
    kubectl create namespace monitoring --dry-run=client -o yaml | kubectl apply -f -
    
    # Install kube-prometheus-stack
    helm upgrade --install prometheus prometheus-community/kube-prometheus-stack \
        --namespace monitoring \
        --set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false \
        --set prometheus.prometheusSpec.podMonitorSelectorNilUsesHelmValues=false \
        --set prometheus.prometheusSpec.ruleSelectorNilUsesHelmValues=false \
        --set alertmanager.enabled=false \
        --set grafana.enabled=false \
        --set kubeStateMetrics.enabled=true \
        --set nodeExporter.enabled=true \
        --set prometheus.service.type=ClusterIP \
        --wait \
        --timeout 5m
    
    log_info "Prometheus installed successfully"
}

# Print cluster info
print_cluster_info() {
    echo ""
    log_info "=========================================="
    log_info "Cluster '${CLUSTER_NAME}' is ready!"
    log_info "=========================================="
    echo ""
    echo "Cluster nodes:"
    kubectl get nodes -o wide
    echo ""
    echo "Namespaces:"
    kubectl get namespaces
    echo ""
    echo "To use this cluster:"
    echo "  kubectl cluster-info --context kind-${CLUSTER_NAME}"
    echo ""
    echo "To delete this cluster:"
    echo "  kind delete cluster --name ${CLUSTER_NAME}"
    echo ""
    echo "Next steps:"
    echo "  1. Install Kubewise:    ./hack/install-kubewise.sh"
    echo "  2. Install test app:    ./hack/install-test-app.sh"
    echo "  3. Run E2E tests:       ./hack/run-e2e-tests.sh"
    echo ""
}

# Main
main() {
    echo "=========================================="
    echo "Kubewise Test Cluster Setup"
    echo "=========================================="
    echo ""
    
    check_prerequisites
    
    if delete_existing_cluster; then
        create_cluster
    fi
    
    install_metrics_server
    install_prometheus
    print_cluster_info
}

# Run main function
main "$@"
