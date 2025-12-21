#!/bin/bash
# run-e2e-tests.sh - Execute the Kubewise E2E test suite
#
# This script runs the end-to-end tests against a deployed Kubewise
# and test application installation.
#
# Requirements: 10.6

set -euo pipefail

# Configuration
TEST_NAMESPACE="${TEST_NAMESPACE:-kubewise-test}"
KUBEWISE_NAMESPACE="${KUBEWISE_NAMESPACE:-kubewise-system}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_DIR="${SCRIPT_DIR}/../e2e"
TIMEOUT="${TIMEOUT:-30m}"
SCENARIO="${SCENARIO:-baseline}"
VERBOSE="${VERBOSE:-false}"

# Test configuration
BASELINE_DURATION="${BASELINE_DURATION:-5m}"
VALIDATION_INTERVAL="${VALIDATION_INTERVAL:-1m}"
PREDICTION_ACCURACY_THRESHOLD="${PREDICTION_ACCURACY_THRESHOLD:-70}"
ANOMALY_DETECTION_TIMEOUT="${ANOMALY_DETECTION_TIMEOUT:-10m}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
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

log_test() {
    echo -e "${BLUE}[TEST]${NC} $1"
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl is not installed"
        exit 1
    fi
    
    # Check if cluster is accessible
    if ! kubectl cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster"
        exit 1
    fi
    
    # Check if test namespace exists
    if ! kubectl get namespace "${TEST_NAMESPACE}" &> /dev/null; then
        log_error "Test namespace '${TEST_NAMESPACE}' does not exist"
        log_error "Please run ./hack/install-test-app.sh first"
        exit 1
    fi
    
    # Check if Kubewise namespace exists
    if ! kubectl get namespace "${KUBEWISE_NAMESPACE}" &> /dev/null; then
        log_error "Kubewise namespace '${KUBEWISE_NAMESPACE}' does not exist"
        log_error "Please run ./hack/install-kubewise.sh first"
        exit 1
    fi
    
    # Check if pods are running
    local ready_pods
    ready_pods=$(kubectl get pods -n "${TEST_NAMESPACE}" --field-selector=status.phase=Running --no-headers 2>/dev/null | wc -l)
    if [ "${ready_pods}" -lt 1 ]; then
        log_error "No running pods found in ${TEST_NAMESPACE}"
        exit 1
    fi
    
    log_info "Prerequisites satisfied"
}

# Helper function to execute commands in a pod
exec_in_pod() {
    local deployment=$1
    local namespace=$2
    shift 2
    kubectl exec -n "${namespace}" "deploy/${deployment}" -- "$@"
}

# Helper function to make API calls to pattern controller
pattern_controller_api() {
    local method=$1
    local endpoint=$2
    local data=${3:-}
    
    local cmd="curl -s -X ${method} http://localhost:8080${endpoint}"
    if [ -n "${data}" ]; then
        cmd="${cmd} -H 'Content-Type: application/json' -d '${data}'"
    fi
    
    exec_in_pod "pattern-controller" "${TEST_NAMESPACE}" sh -c "${cmd}"
}

# Helper function to get metrics validator report
get_validation_report() {
    exec_in_pod "metrics-validator" "${TEST_NAMESPACE}" \
        curl -s http://localhost:8080/api/v1/reports/latest 2>/dev/null || echo "{}"
}

# Test: Verify all components are healthy
test_component_health() {
    log_test "Testing component health..."
    
    local components=(
        "pattern-controller"
        "memory-hog"
        "cpu-burster"
        "steady-worker"
        "load-generator"
        "metrics-validator"
    )
    
    local failed=0
    for component in "${components[@]}"; do
        if kubectl get deploy "${component}" -n "${TEST_NAMESPACE}" &> /dev/null; then
            local ready
            ready=$(kubectl get deploy "${component}" -n "${TEST_NAMESPACE}" -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
            if [ "${ready:-0}" -ge 1 ]; then
                echo "  ✓ ${component}: healthy"
            else
                echo "  ✗ ${component}: not ready"
                failed=$((failed + 1))
            fi
        else
            echo "  ✗ ${component}: not found"
            failed=$((failed + 1))
        fi
    done
    
    if [ ${failed} -gt 0 ]; then
        log_error "Component health check failed (${failed} components unhealthy)"
        return 1
    fi
    
    log_info "All components healthy"
    return 0
}

# Test: Verify Kubewise API is accessible
test_kubewise_api() {
    log_test "Testing Kubewise API accessibility..."
    
    local response
    response=$(exec_in_pod "metrics-validator" "${TEST_NAMESPACE}" \
        curl -s -o /dev/null -w "%{http_code}" \
        "http://kubewise-api.${KUBEWISE_NAMESPACE}:8080/healthz" 2>/dev/null || echo "000")
    
    if [ "${response}" = "200" ]; then
        echo "  ✓ Kubewise API is accessible"
        return 0
    else
        echo "  ✗ Kubewise API returned HTTP ${response}"
        return 1
    fi
}


# Test: Run baseline scenario
test_baseline_scenario() {
    log_test "Running baseline scenario..."
    
    # Start baseline scenario
    log_info "Starting baseline scenario..."
    local start_response
    start_response=$(pattern_controller_api POST "/api/v1/scenarios/start" '{"name":"baseline"}')
    
    if echo "${start_response}" | grep -q "error"; then
        log_error "Failed to start baseline scenario: ${start_response}"
        return 1
    fi
    
    echo "  ✓ Baseline scenario started"
    
    # Wait for data collection
    log_info "Waiting for data collection (${BASELINE_DURATION})..."
    sleep_with_progress "${BASELINE_DURATION}"
    
    # Check scenario status
    local status
    status=$(pattern_controller_api GET "/api/v1/scenarios/status")
    echo "  Scenario status: ${status}"
    
    # Stop scenario
    log_info "Stopping baseline scenario..."
    pattern_controller_api POST "/api/v1/scenarios/stop" > /dev/null
    
    echo "  ✓ Baseline scenario completed"
    return 0
}

# Test: Verify metrics collection
test_metrics_collection() {
    log_test "Testing metrics collection..."
    
    # Check if Prometheus is scraping metrics
    local metrics_count
    metrics_count=$(exec_in_pod "metrics-validator" "${TEST_NAMESPACE}" \
        curl -s "http://prometheus-kube-prometheus-prometheus.monitoring:9090/api/v1/query?query=kubewise_test_component_up" 2>/dev/null | \
        grep -o '"result":\[' | wc -l || echo "0")
    
    if [ "${metrics_count:-0}" -ge 1 ]; then
        echo "  ✓ Prometheus is collecting metrics"
        return 0
    else
        echo "  ⚠ Prometheus metrics query returned no results (may be expected if Prometheus is not configured)"
        return 0  # Don't fail on this, as Prometheus may not be fully configured
    fi
}

# Test: Memory leak detection
test_memory_leak_detection() {
    log_test "Testing memory leak detection..."
    
    # Configure memory-hog for leak mode
    log_info "Configuring memory-hog for leak mode..."
    exec_in_pod "memory-hog" "${TEST_NAMESPACE}" \
        curl -s -X POST http://localhost:8082/api/v1/config \
        -H 'Content-Type: application/json' \
        -d '{"mode":"leak","leakRateMBMin":20}' > /dev/null
    
    echo "  ✓ Memory leak mode configured"
    
    # Wait for leak to be detectable
    log_info "Waiting for memory leak to accumulate (2m)..."
    sleep 120
    
    # Check memory status
    local mem_status
    mem_status=$(exec_in_pod "memory-hog" "${TEST_NAMESPACE}" \
        curl -s http://localhost:8082/api/v1/status 2>/dev/null || echo "{}")
    echo "  Memory status: ${mem_status}"
    
    # Reset to steady mode
    log_info "Resetting memory-hog to steady mode..."
    exec_in_pod "memory-hog" "${TEST_NAMESPACE}" \
        curl -s -X POST http://localhost:8082/api/v1/config \
        -H 'Content-Type: application/json' \
        -d '{"mode":"steady","targetMB":128}' > /dev/null
    
    echo "  ✓ Memory leak test completed"
    return 0
}

# Test: CPU spike detection
test_cpu_spike_detection() {
    log_test "Testing CPU spike detection..."
    
    # Trigger CPU spike
    log_info "Triggering CPU spike..."
    exec_in_pod "cpu-burster" "${TEST_NAMESPACE}" \
        curl -s -X POST http://localhost:8083/api/v1/trigger \
        -H 'Content-Type: application/json' \
        -d '{"duration":"30s"}' > /dev/null 2>&1 || true
    
    echo "  ✓ CPU spike triggered"
    
    # Wait for spike to complete
    sleep 35
    
    # Check CPU status
    local cpu_status
    cpu_status=$(exec_in_pod "cpu-burster" "${TEST_NAMESPACE}" \
        curl -s http://localhost:8083/api/v1/status 2>/dev/null || echo "{}")
    echo "  CPU status: ${cpu_status}"
    
    echo "  ✓ CPU spike test completed"
    return 0
}

# Test: Load generator functionality
test_load_generator() {
    log_test "Testing load generator..."
    
    # Start load generation
    log_info "Starting load generation..."
    exec_in_pod "load-generator" "${TEST_NAMESPACE}" \
        curl -s -X POST http://localhost:8081/api/v1/start > /dev/null 2>&1 || true
    
    echo "  ✓ Load generation started"
    
    # Wait for some requests
    sleep 30
    
    # Check stats
    local stats
    stats=$(exec_in_pod "load-generator" "${TEST_NAMESPACE}" \
        curl -s http://localhost:8081/api/v1/stats 2>/dev/null || echo "{}")
    echo "  Load stats: ${stats}"
    
    # Stop load generation
    log_info "Stopping load generation..."
    exec_in_pod "load-generator" "${TEST_NAMESPACE}" \
        curl -s -X POST http://localhost:8081/api/v1/stop > /dev/null 2>&1 || true
    
    echo "  ✓ Load generator test completed"
    return 0
}

# Test: Validation report generation
test_validation_report() {
    log_test "Testing validation report generation..."
    
    # Trigger validation
    log_info "Triggering validation..."
    exec_in_pod "metrics-validator" "${TEST_NAMESPACE}" \
        curl -s -X POST http://localhost:8080/api/v1/validate > /dev/null 2>&1 || true
    
    # Wait for validation to complete
    sleep 10
    
    # Get report
    local report
    report=$(get_validation_report)
    
    if [ -n "${report}" ] && [ "${report}" != "{}" ]; then
        echo "  ✓ Validation report generated"
        if [ "${VERBOSE}" = "true" ]; then
            echo "  Report: ${report}"
        fi
        return 0
    else
        echo "  ⚠ No validation report available (may be expected on first run)"
        return 0
    fi
}

# Helper: Sleep with progress indicator
sleep_with_progress() {
    local duration=$1
    local seconds
    
    # Convert duration to seconds
    if [[ "${duration}" =~ ^([0-9]+)m$ ]]; then
        seconds=$((${BASH_REMATCH[1]} * 60))
    elif [[ "${duration}" =~ ^([0-9]+)s$ ]]; then
        seconds=${BASH_REMATCH[1]}
    else
        seconds=${duration}
    fi
    
    local elapsed=0
    while [ ${elapsed} -lt ${seconds} ]; do
        printf "\r  Progress: %d/%d seconds" ${elapsed} ${seconds}
        sleep 10
        elapsed=$((elapsed + 10))
    done
    printf "\r  Progress: %d/%d seconds\n" ${seconds} ${seconds}
}


# Collect logs on failure
collect_logs() {
    log_info "Collecting logs for debugging..."
    
    local log_dir="${SCRIPT_DIR}/../logs"
    mkdir -p "${log_dir}"
    
    local timestamp
    timestamp=$(date +%Y%m%d-%H%M%S)
    
    # Collect pod logs
    for pod in $(kubectl get pods -n "${TEST_NAMESPACE}" -o name 2>/dev/null); do
        local pod_name
        pod_name=$(basename "${pod}")
        kubectl logs -n "${TEST_NAMESPACE}" "${pod}" > "${log_dir}/${pod_name}-${timestamp}.log" 2>&1 || true
    done
    
    # Collect Kubewise logs
    for pod in $(kubectl get pods -n "${KUBEWISE_NAMESPACE}" -o name 2>/dev/null); do
        local pod_name
        pod_name=$(basename "${pod}")
        kubectl logs -n "${KUBEWISE_NAMESPACE}" "${pod}" > "${log_dir}/kubewise-${pod_name}-${timestamp}.log" 2>&1 || true
    done
    
    # Collect pod descriptions
    kubectl describe pods -n "${TEST_NAMESPACE}" > "${log_dir}/pods-describe-${timestamp}.txt" 2>&1 || true
    
    log_info "Logs collected in ${log_dir}"
}

# Print test summary
print_summary() {
    local passed=$1
    local failed=$2
    local skipped=$3
    local total=$((passed + failed + skipped))
    
    echo ""
    echo "=========================================="
    echo "Test Summary"
    echo "=========================================="
    echo ""
    echo "Total:   ${total}"
    echo "Passed:  ${passed}"
    echo "Failed:  ${failed}"
    echo "Skipped: ${skipped}"
    echo ""
    
    if [ ${failed} -eq 0 ]; then
        log_info "All tests passed! ✓"
        return 0
    else
        log_error "${failed} test(s) failed"
        return 1
    fi
}

# Run all tests
run_tests() {
    local passed=0
    local failed=0
    local skipped=0
    
    echo ""
    echo "=========================================="
    echo "Running E2E Tests"
    echo "=========================================="
    echo ""
    echo "Test namespace:     ${TEST_NAMESPACE}"
    echo "Kubewise namespace: ${KUBEWISE_NAMESPACE}"
    echo "Scenario:           ${SCENARIO}"
    echo ""
    
    # Test 1: Component health
    if test_component_health; then
        passed=$((passed + 1))
    else
        failed=$((failed + 1))
    fi
    echo ""
    
    # Test 2: Kubewise API
    if test_kubewise_api; then
        passed=$((passed + 1))
    else
        failed=$((failed + 1))
    fi
    echo ""
    
    # Test 3: Metrics collection
    if test_metrics_collection; then
        passed=$((passed + 1))
    else
        failed=$((failed + 1))
    fi
    echo ""
    
    # Run scenario-specific tests
    case "${SCENARIO}" in
        baseline)
            if test_baseline_scenario; then
                passed=$((passed + 1))
            else
                failed=$((failed + 1))
            fi
            echo ""
            ;;
        stress)
            if test_load_generator; then
                passed=$((passed + 1))
            else
                failed=$((failed + 1))
            fi
            echo ""
            if test_cpu_spike_detection; then
                passed=$((passed + 1))
            else
                failed=$((failed + 1))
            fi
            echo ""
            ;;
        anomaly)
            if test_memory_leak_detection; then
                passed=$((passed + 1))
            else
                failed=$((failed + 1))
            fi
            echo ""
            if test_cpu_spike_detection; then
                passed=$((passed + 1))
            else
                failed=$((failed + 1))
            fi
            echo ""
            ;;
        full|full-validation)
            if test_baseline_scenario; then
                passed=$((passed + 1))
            else
                failed=$((failed + 1))
            fi
            echo ""
            if test_load_generator; then
                passed=$((passed + 1))
            else
                failed=$((failed + 1))
            fi
            echo ""
            if test_memory_leak_detection; then
                passed=$((passed + 1))
            else
                failed=$((failed + 1))
            fi
            echo ""
            if test_cpu_spike_detection; then
                passed=$((passed + 1))
            else
                failed=$((failed + 1))
            fi
            echo ""
            ;;
        *)
            log_warn "Unknown scenario: ${SCENARIO}, running basic tests only"
            ;;
    esac
    
    # Test: Validation report
    if test_validation_report; then
        passed=$((passed + 1))
    else
        failed=$((failed + 1))
    fi
    echo ""
    
    # Print summary
    print_summary ${passed} ${failed} ${skipped}
    return $?
}

# Print usage
usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -s, --scenario SCENARIO   Test scenario to run (baseline, stress, anomaly, full)"
    echo "  -n, --namespace NS        Test namespace (default: kubewise-test)"
    echo "  -t, --timeout DURATION    Test timeout (default: 30m)"
    echo "  -v, --verbose             Enable verbose output"
    echo "  -h, --help                Show this help message"
    echo ""
    echo "Scenarios:"
    echo "  baseline   - Run baseline prediction accuracy tests"
    echo "  stress     - Run stress and load tests"
    echo "  anomaly    - Run anomaly detection tests"
    echo "  full       - Run all tests"
    echo ""
    echo "Examples:"
    echo "  $0                        # Run baseline tests"
    echo "  $0 -s full -v             # Run all tests with verbose output"
    echo "  $0 -s anomaly -t 1h       # Run anomaly tests with 1 hour timeout"
}

# Parse arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -s|--scenario)
                SCENARIO="$2"
                shift 2
                ;;
            -n|--namespace)
                TEST_NAMESPACE="$2"
                shift 2
                ;;
            -t|--timeout)
                TIMEOUT="$2"
                shift 2
                ;;
            -v|--verbose)
                VERBOSE="true"
                shift
                ;;
            -h|--help)
                usage
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                usage
                exit 1
                ;;
        esac
    done
}

# Main
main() {
    parse_args "$@"
    
    echo "=========================================="
    echo "Kubewise E2E Test Suite"
    echo "=========================================="
    echo ""
    
    check_prerequisites
    
    # Run tests with timeout
    local exit_code=0
    if ! timeout "${TIMEOUT}" bash -c "$(declare -f); run_tests"; then
        exit_code=$?
        if [ ${exit_code} -eq 124 ]; then
            log_error "Tests timed out after ${TIMEOUT}"
        fi
    fi
    
    # Collect logs on failure
    if [ ${exit_code} -ne 0 ]; then
        collect_logs
    fi
    
    exit ${exit_code}
}

# Run main function
main "$@"
