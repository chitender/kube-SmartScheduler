#!/bin/bash

# Helm Chart Testing Script for Smart Scheduler
# This script validates, tests, and optionally deploys the Helm chart

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
CHART_DIR="${PROJECT_ROOT}/helm/smart-scheduler"
RELEASE_NAME="smart-scheduler-test"
NAMESPACE="smart-scheduler-test"
TEST_VALUES_FILE="${PROJECT_ROOT}/helm/smart-scheduler/test-values.yaml"
SKIP_DEPS="false"
DRY_RUN="false"
INSTALL="false"
UPGRADE="false"
UNINSTALL="false"
RUN_TESTS="false"

# Functions
print_header() {
    echo -e "\n${BLUE}========================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}========================================${NC}\n"
}

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

print_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

check_prerequisites() {
    print_header "Checking Prerequisites"
    
    local missing=0
    
    if ! command -v helm &> /dev/null; then
        print_error "helm is not installed"
        missing=1
    else
        local helm_version=$(helm version --short)
        print_success "Helm is installed: ${helm_version}"
    fi
    
    if [ "$INSTALL" = "true" ] || [ "$UPGRADE" = "true" ] || [ "$DRY_RUN" = "true" ]; then
        if ! command -v kubectl &> /dev/null; then
            print_error "kubectl is not installed"
            missing=1
        else
            if kubectl cluster-info &> /dev/null; then
                print_success "kubectl is installed and configured"
            else
                print_warning "kubectl is installed but cluster is not accessible"
                missing=1
            fi
        fi
    fi
    
    if [ $missing -eq 1 ]; then
        print_error "Prerequisites check failed"
        exit 1
    fi
}

update_dependencies() {
    print_header "Updating Helm Dependencies"
    
    if [ "$SKIP_DEPS" = "false" ]; then
        print_info "Updating chart dependencies..."
        cd "${CHART_DIR}"
        helm dependency update
        print_success "Dependencies updated"
        cd "${PROJECT_ROOT}"
    else
        print_info "Skipping dependency update (--skip-deps flag set)"
    fi
}

lint_chart() {
    print_header "Linting Helm Chart"
    
    cd "${CHART_DIR}"
    
    print_info "Running helm lint..."
    if helm lint . --debug; then
        print_success "Chart linting passed"
    else
        print_error "Chart linting failed"
        exit 1
    fi
    
    cd "${PROJECT_ROOT}"
}

validate_chart() {
    print_header "Validating Chart Metadata"
    
    cd "${CHART_DIR}"
    
    if [ ! -f "Chart.yaml" ]; then
        print_error "Chart.yaml not found"
        exit 1
    fi
    
    local chart_version=$(grep "^version:" Chart.yaml | awk '{print $2}')
    local app_version=$(grep "^appVersion:" Chart.yaml | awk '{print $2}' | tr -d '"')
    
    print_success "Chart version: ${chart_version}"
    print_success "App version: ${app_version}"
    
    cd "${PROJECT_ROOT}"
}

template_chart() {
    print_header "Rendering Chart Templates"
    
    local values_flag=""
    if [ -f "${TEST_VALUES_FILE}" ]; then
        values_flag="-f ${TEST_VALUES_FILE}"
        print_info "Using test values file: ${TEST_VALUES_FILE}"
    else
        print_info "Using default values"
    fi
    
    cd "${CHART_DIR}"
    
    print_info "Rendering templates with default values..."
    if helm template "${RELEASE_NAME}" . ${values_flag} > /dev/null 2>&1; then
        print_success "Template rendering with default values succeeded"
    else
        print_error "Template rendering with default values failed"
        exit 1
    fi
    
    # Test with different values
    print_info "Testing template rendering with different configurations..."
    
    # Test with webhook disabled
    if helm template "${RELEASE_NAME}" . --set webhook.enabled=false > /dev/null 2>&1; then
        print_success "Template rendering with webhook disabled succeeded"
    else
        print_error "Template rendering with webhook disabled failed"
        exit 1
    fi
    
    # Test with cert-manager disabled
    if helm template "${RELEASE_NAME}" . --set certificates.certManager.enabled=false > /dev/null 2>&1; then
        print_success "Template rendering with cert-manager disabled succeeded"
    else
        print_error "Template rendering with cert-manager disabled failed"
        exit 1
    fi
    
    # Test with development mode
    if helm template "${RELEASE_NAME}" . --set development.enabled=true > /dev/null 2>&1; then
        print_success "Template rendering with development mode succeeded"
    else
        print_error "Template rendering with development mode failed"
        exit 1
    fi
    
    cd "${PROJECT_ROOT}"
}

dry_run_install() {
    print_header "Dry-Run Installation Test"
    
    local values_flag=""
    if [ -f "${TEST_VALUES_FILE}" ]; then
        values_flag="-f ${TEST_VALUES_FILE}"
    fi
    
    print_info "Performing dry-run installation..."
    
    if kubectl get namespace "${NAMESPACE}" &> /dev/null; then
        print_info "Namespace ${NAMESPACE} already exists"
    else
        print_info "Creating namespace ${NAMESPACE}..."
        kubectl create namespace "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -
    fi
    
    cd "${CHART_DIR}"
    
    if helm install "${RELEASE_NAME}" . \
        --namespace "${NAMESPACE}" \
        --dry-run \
        --debug \
        ${values_flag} > /tmp/helm-dry-run-output.yaml 2>&1; then
        print_success "Dry-run installation succeeded"
        print_info "Dry-run output saved to /tmp/helm-dry-run-output.yaml"
    else
        print_error "Dry-run installation failed"
        cat /tmp/helm-dry-run-output.yaml || true
        exit 1
    fi
    
    cd "${PROJECT_ROOT}"
}

install_chart() {
    print_header "Installing Helm Chart"
    
    local values_flag=""
    if [ -f "${TEST_VALUES_FILE}" ]; then
        values_flag="-f ${TEST_VALUES_FILE}"
        print_info "Using test values file: ${TEST_VALUES_FILE}"
    fi
    
    # Create namespace if it doesn't exist
    if ! kubectl get namespace "${NAMESPACE}" &> /dev/null; then
        print_info "Creating namespace ${NAMESPACE}..."
        kubectl create namespace "${NAMESPACE}"
    else
        print_info "Namespace ${NAMESPACE} already exists"
    fi
    
    cd "${CHART_DIR}"
    
    print_info "Installing chart..."
    if helm install "${RELEASE_NAME}" . \
        --namespace "${NAMESPACE}" \
        --wait \
        --timeout 10m \
        ${values_flag}; then
        print_success "Chart installed successfully"
    else
        print_error "Chart installation failed"
        exit 1
    fi
    
    cd "${PROJECT_ROOT}"
    
    # Wait for pods to be ready
    print_info "Waiting for pods to be ready..."
    if kubectl wait --for=condition=ready pod \
        -l app.kubernetes.io/name=smart-scheduler \
        -n "${NAMESPACE}" \
        --timeout=5m; then
        print_success "Pods are ready"
    else
        print_warning "Some pods may not be ready yet"
    fi
    
    # Show status
    print_info "Deployment status:"
    kubectl get all -n "${NAMESPACE}"
}

upgrade_chart() {
    print_header "Upgrading Helm Chart"
    
    local values_flag=""
    if [ -f "${TEST_VALUES_FILE}" ]; then
        values_flag="-f ${TEST_VALUES_FILE}"
    fi
    
    cd "${CHART_DIR}"
    
    print_info "Upgrading chart..."
    if helm upgrade "${RELEASE_NAME}" . \
        --namespace "${NAMESPACE}" \
        --wait \
        --timeout 10m \
        ${values_flag}; then
        print_success "Chart upgraded successfully"
    else
        print_error "Chart upgrade failed"
        exit 1
    fi
    
    cd "${PROJECT_ROOT}"
}

uninstall_chart() {
    print_header "Uninstalling Helm Chart"
    
    if helm list -n "${NAMESPACE}" | grep -q "${RELEASE_NAME}"; then
        print_info "Uninstalling chart..."
        if helm uninstall "${RELEASE_NAME}" --namespace "${NAMESPACE}"; then
            print_success "Chart uninstalled successfully"
        else
            print_error "Chart uninstallation failed"
            exit 1
        fi
        
        # Optionally delete namespace
        read -p "Delete namespace ${NAMESPACE}? (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            print_info "Deleting namespace ${NAMESPACE}..."
            kubectl delete namespace "${NAMESPACE}" --timeout=2m || true
            print_success "Namespace deleted"
        fi
    else
        print_warning "Release ${RELEASE_NAME} not found in namespace ${NAMESPACE}"
    fi
}

run_helm_tests() {
    print_header "Running Helm Tests"
    
    if ! helm list -n "${NAMESPACE}" | grep -q "${RELEASE_NAME}"; then
        print_error "Release ${RELEASE_NAME} not found. Please install the chart first."
        exit 1
    fi
    
    print_info "Running helm test..."
    if helm test "${RELEASE_NAME}" --namespace "${NAMESPACE}" --timeout 5m; then
        print_success "All helm tests passed"
    else
        print_error "Some helm tests failed"
        exit 1
    fi
}

verify_resources() {
    print_header "Verifying Deployed Resources"
    
    local failed=0
    
    # Check deployment
    if kubectl get deployment -n "${NAMESPACE}" -l app.kubernetes.io/name=smart-scheduler &> /dev/null; then
        print_success "Deployment exists"
    else
        print_error "Deployment not found"
        failed=1
    fi
    
    # Check service account
    if kubectl get serviceaccount -n "${NAMESPACE}" -l app.kubernetes.io/name=smart-scheduler &> /dev/null; then
        print_success "Service account exists"
    else
        print_warning "Service account not found (might be using default)"
    fi
    
    # Check services
    if kubectl get service -n "${NAMESPACE}" -l app.kubernetes.io/name=smart-scheduler &> /dev/null; then
        print_success "Services exist"
    else
        print_warning "Services not found"
    fi
    
    # Check webhook if enabled
    local webhook_enabled=$(helm get values "${RELEASE_NAME}" -n "${NAMESPACE}" -o json 2>/dev/null | jq -r '.webhook.enabled // true' || echo "true")
    if [ "$webhook_enabled" = "true" ]; then
        if kubectl get mutatingwebhookconfiguration -l app.kubernetes.io/name=smart-scheduler &> /dev/null; then
            print_success "MutatingWebhookConfiguration exists"
        else
            print_warning "MutatingWebhookConfiguration not found"
        fi
    fi
    
    # Check CRDs
    if kubectl get crd podplacementpolicies.smartscheduler.io &> /dev/null; then
        print_success "CRD exists"
    else
        print_warning "CRD not found (might be disabled or already installed)"
    fi
    
    if [ $failed -eq 1 ]; then
        print_error "Resource verification failed"
        exit 1
    fi
}

show_usage() {
    cat << EOF
Usage: $0 [OPTIONS]

Options:
    -h, --help              Show this help message
    -n, --namespace NAME    Namespace to use (default: smart-scheduler-test)
    -r, --release NAME      Release name (default: smart-scheduler-test)
    -f, --values FILE       Path to values file (default: helm/smart-scheduler/test-values.yaml)
    --skip-deps             Skip updating Helm dependencies
    --dry-run               Perform dry-run installation test
    --install               Install the chart (requires cluster access)
    --upgrade               Upgrade the chart (requires cluster access)
    --uninstall             Uninstall the chart (requires cluster access)
    --test                  Run helm tests (requires installed chart)
    --verify                Verify deployed resources (requires installed chart)
    
Examples:
    # Validate and lint the chart
    $0
    
    # Dry-run installation test
    $0 --dry-run
    
    # Install and test the chart
    $0 --install --test --verify
    
    # Install with custom values
    $0 --install -f custom-values.yaml
    
    # Uninstall the chart
    $0 --uninstall

EOF
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_usage
            exit 0
            ;;
        -n|--namespace)
            NAMESPACE="$2"
            shift 2
            ;;
        -r|--release)
            RELEASE_NAME="$2"
            shift 2
            ;;
        -f|--values)
            TEST_VALUES_FILE="$2"
            shift 2
            ;;
        --skip-deps)
            SKIP_DEPS="true"
            shift
            ;;
        --dry-run)
            DRY_RUN="true"
            shift
            ;;
        --install)
            INSTALL="true"
            shift
            ;;
        --upgrade)
            UPGRADE="true"
            shift
            ;;
        --uninstall)
            UNINSTALL="true"
            shift
            ;;
        --test)
            RUN_TESTS="true"
            shift
            ;;
        --verify)
            VERIFY="true"
            shift
            ;;
        *)
            print_error "Unknown option: $1"
            show_usage
            exit 1
            ;;
    esac
done

# Main execution
main() {
    print_header "Smart Scheduler Helm Chart Testing"
    
    check_prerequisites
    
    # Always run these validation steps
    validate_chart
    update_dependencies
    lint_chart
    template_chart
    
    # Optional steps based on flags
    if [ "$DRY_RUN" = "true" ]; then
        dry_run_install
    fi
    
    if [ "$INSTALL" = "true" ]; then
        install_chart
        if [ "${VERIFY:-false}" = "true" ]; then
            verify_resources
        fi
        if [ "$RUN_TESTS" = "true" ]; then
            run_helm_tests
        fi
    fi
    
    if [ "$UPGRADE" = "true" ]; then
        upgrade_chart
    fi
    
    if [ "$UNINSTALL" = "true" ]; then
        uninstall_chart
    fi
    
    if [ "${VERIFY:-false}" = "true" ] && [ "$INSTALL" != "true" ]; then
        verify_resources
    fi
    
    if [ "$RUN_TESTS" = "true" ] && [ "$INSTALL" != "true" ]; then
        run_helm_tests
    fi
    
    print_header "Testing Complete"
    print_success "All tests passed!"
}

main

