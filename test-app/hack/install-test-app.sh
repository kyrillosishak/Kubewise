#!/bin/bash
# install-test-app.sh - Deploy the Kubewise test application
#
# This script deploys all test application components to the cluster
# using the Helm chart.
#
# Requirements: 1.4

set -euo pipefail

# Configuration
TEST_NAMESPACE="${TEST_NAMESPACE:-kubewise-test}"
TEST_RELEASE="${TEST_RELEASE:-kubewise-test}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHART_DIR="${SCRIPT_DIR}/../charts/kubewise-test"
TIMEOUT="${TIMEOUT:-5m}"
BUILD_IMAGES="${BUILD_IMAGES:-false}"

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
    
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl is not installed"
        exit 1
    fi
    
    if ! command -v helm &> /dev/null; then
        log_error "helm is not installed"
        exit 1
    fi
    
    # Check if cluster is accessible
    if ! kubectl cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster"
        exit 1
    fi
    
    # Check if chart exists
    if [ ! -d "${CHART_DIR}" ]; then
        log_error "Helm chart not found at ${CHART_DIR}"
        exit 1
    fi
    
    log_info "Prerequisites satisfied"
}

# Build Docker images
build_images() {
    if [ "${BUILD_IMAGES}" != "true" ]; then
        log_info "Skipping image build (set BUILD_IMAGES=true to build)"
        return 0
    fi
    
    log_info "Building Docker images..."
    
    local test_app_dir="${SCRIPT_DIR}/.."
    
    # Build all component images
    make -C "${test_app_dir}" docker-build
    
    log_info "Docker images built successfully"
}

# Load images into kind cluster
load_images() {
    log_info "Checking if running in kind cluster..."
    
    local cluster_name
    cluster_name=$(kubectl config current-context | sed 's/kind-//')
    
    if kind get clusters 2>/dev/null | grep -q "^${cluster_name}$"; then
        log_info "Kind cluster detected: ${cluster_name}"
        
        local components=(
            "pattern-controller"
            "load-generator"
            "memory-hog"
            "cpu-burster"
            "steady-worker"
            "metrics-validator"
        )
        
        for component in "${components[@]}"; do
            local image="kubewise-test/${component}:latest"
            if docker images --format '{{.Repository}}:{{.Tag}}' | grep -q "^${image}$"; then
                log_info "Loading ${component} image into kind..."
                kind load docker-image "${image}" --name "${cluster_name}"
            else
                log_warn "Image ${image} not found locally, skipping"
            fi
        done
    else
        log_info "Not running in kind cluster, skipping image loading"
    fi
}

# Install test application using Helm
install_test_app() {
    log_info "Installing test application..."
    
    # Create values override for testing
    local values_file
    values_file=$(mktemp)
    cat > "${values_file}" << 'EOF'
# Test configuration overrides
namespace: kubewise-test
createNamespace: true

# Enable all components
patternController:
  enabled: true
  config:
    timeAcceleration: "1.0"

memoryHog:
  enabled: true
  config:
    mode: "steady"
    targetMB: "128"

cpuBurster:
  enabled: true
  config:
    mode: "steady"
    targetPercent: "20"

steadyWorker:
  enabled: true
  config:
    cpuWorkMs: "10"
    memoryAllocKB: "32"

loadGenerator:
  enabled: true
  config:
    mode: "constant"
    rps: "5"
    autoStart: "false"

metricsValidator:
  enabled: true
  config:
    kubewiseURL: "http://kubewise-api.kubewise-system:8080"
    prometheusURL: "http://prometheus-kube-prometheus-prometheus.monitoring:9090"
    validationInterval: "1m"

serviceMonitor:
  enabled: true
EOF
    
    helm upgrade --install "${TEST_RELEASE}" "${CHART_DIR}" \
        --namespace "${TEST_NAMESPACE}" \
        --create-namespace \
        --values "${values_file}" \
        --wait \
        --timeout "${TIMEOUT}"
    
    rm -f "${values_file}"
    
    log_info "Test application installed successfully"
}


# Verify installation
verify_installation() {
    log_info "Verifying test application installation..."
    
    echo ""
    echo "Test application pods:"
    kubectl get pods -n "${TEST_NAMESPACE}" -o wide
    
    echo ""
    echo "Test application services:"
    kubectl get services -n "${TEST_NAMESPACE}"
    
    # Wait for all pods to be ready
    log_info "Waiting for all pods to be ready..."
    kubectl wait --for=condition=Ready pods --all -n "${TEST_NAMESPACE}" --timeout=120s || {
        log_warn "Some pods may not be ready yet"
        echo ""
        echo "Pod status:"
        kubectl get pods -n "${TEST_NAMESPACE}"
    }
}

# Print component endpoints
print_endpoints() {
    echo ""
    log_info "=========================================="
    log_info "Test Application Endpoints"
    log_info "=========================================="
    echo ""
    echo "Pattern Controller:"
    echo "  http://pattern-controller-svc.${TEST_NAMESPACE}:8080"
    echo "  Endpoints:"
    echo "    - POST /api/v1/scenarios/start"
    echo "    - POST /api/v1/scenarios/stop"
    echo "    - GET  /api/v1/scenarios/status"
    echo "    - GET  /api/v1/scenarios/list"
    echo ""
    echo "Memory Hog:"
    echo "  http://memory-hog-svc.${TEST_NAMESPACE}:8082"
    echo "  Endpoints:"
    echo "    - POST /api/v1/config"
    echo "    - GET  /api/v1/status"
    echo ""
    echo "CPU Burster:"
    echo "  http://cpu-burster-svc.${TEST_NAMESPACE}:8083"
    echo "  Endpoints:"
    echo "    - POST /api/v1/config"
    echo "    - GET  /api/v1/status"
    echo ""
    echo "Steady Worker:"
    echo "  http://steady-worker-svc.${TEST_NAMESPACE}:8084"
    echo "  Endpoints:"
    echo "    - GET  /work"
    echo "    - POST /api/v1/config"
    echo ""
    echo "Load Generator:"
    echo "  http://load-generator-svc.${TEST_NAMESPACE}:8081"
    echo "  Endpoints:"
    echo "    - POST /api/v1/config"
    echo "    - POST /api/v1/start"
    echo "    - POST /api/v1/stop"
    echo ""
    echo "Metrics Validator:"
    echo "  http://metrics-validator-svc.${TEST_NAMESPACE}:8080"
    echo "  Endpoints:"
    echo "    - GET  /api/v1/reports"
    echo "    - POST /api/v1/validate"
    echo ""
}

# Print usage info
print_info() {
    echo ""
    log_info "=========================================="
    log_info "Test Application installation complete!"
    log_info "=========================================="
    echo ""
    echo "Namespace: ${TEST_NAMESPACE}"
    echo "Release:   ${TEST_RELEASE}"
    echo ""
    echo "To check status:"
    echo "  kubectl get pods -n ${TEST_NAMESPACE}"
    echo ""
    echo "To view logs:"
    echo "  kubectl logs -n ${TEST_NAMESPACE} -l app=pattern-controller"
    echo "  kubectl logs -n ${TEST_NAMESPACE} -l app=memory-hog"
    echo "  kubectl logs -n ${TEST_NAMESPACE} -l app=cpu-burster"
    echo ""
    echo "To start a test scenario:"
    echo "  kubectl exec -n ${TEST_NAMESPACE} deploy/pattern-controller -- \\"
    echo "    curl -X POST http://localhost:8080/api/v1/scenarios/start \\"
    echo "    -H 'Content-Type: application/json' \\"
    echo "    -d '{\"name\": \"baseline\"}'"
    echo ""
    echo "Available scenarios: baseline, stress, anomaly, full-validation"
    echo ""
    echo "Next steps:"
    echo "  Run E2E tests: ./hack/run-e2e-tests.sh"
    echo ""
}

# Main
main() {
    echo "=========================================="
    echo "Kubewise Test Application Installation"
    echo "=========================================="
    echo ""
    
    check_prerequisites
    build_images
    load_images
    install_test_app
    verify_installation
    print_endpoints
    print_info
}

# Run main function
main "$@"
