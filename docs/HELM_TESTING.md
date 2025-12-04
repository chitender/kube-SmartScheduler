# Helm Chart Testing Guide

This guide provides comprehensive instructions for testing the Smart Scheduler Helm chart.

## Table of Contents

- [Quick Start](#quick-start)
- [Test Script](#test-script)
- [Manual Testing](#manual-testing)
- [Makefile Targets](#makefile-targets)
- [Test Scenarios](#test-scenarios)
- [Troubleshooting](#troubleshooting)

## Quick Start

The easiest way to test the Helm chart is using the provided test script:

```bash
# Basic validation (lint, template rendering)
./scripts/test-helm.sh

# Dry-run installation test
./scripts/test-helm.sh --dry-run

# Full test with installation
./scripts/test-helm.sh --install --test --verify
```

## Test Script

The test script (`scripts/test-helm.sh`) provides comprehensive testing capabilities:

### Usage

```bash
./scripts/test-helm.sh [OPTIONS]
```

### Options

- `-h, --help` - Show help message
- `-n, --namespace NAME` - Namespace to use (default: smart-scheduler-test)
- `-r, --release NAME` - Release name (default: smart-scheduler-test)
- `-f, --values FILE` - Path to values file (default: helm/smart-scheduler/test-values.yaml)
- `--skip-deps` - Skip updating Helm dependencies
- `--dry-run` - Perform dry-run installation test
- `--install` - Install the chart (requires cluster access)
- `--upgrade` - Upgrade the chart (requires cluster access)
- `--uninstall` - Uninstall the chart (requires cluster access)
- `--test` - Run helm tests (requires installed chart)
- `--verify` - Verify deployed resources (requires installed chart)

### Examples

#### Basic Validation

Validate and lint the chart without requiring cluster access:

```bash
./scripts/test-helm.sh
```

#### Dry-Run Test

Test installation without actually deploying:

```bash
./scripts/test-helm.sh --dry-run
```

#### Full Installation Test

Install, test, and verify the chart:

```bash
./scripts/test-helm.sh --install --test --verify
```

#### Custom Values

Test with custom values file:

```bash
./scripts/test-helm.sh --install -f custom-values.yaml
```

#### Cleanup

Uninstall test deployment:

```bash
./scripts/test-helm.sh --uninstall
```

## Manual Testing

You can also test the Helm chart manually using Helm CLI commands.

### Prerequisites

```bash
# Verify Helm is installed
helm version

# Verify kubectl access (for cluster tests)
kubectl cluster-info
```

### Step 1: Update Dependencies

```bash
cd helm/smart-scheduler
helm dependency update
cd ../..
```

### Step 2: Lint the Chart

```bash
cd helm/smart-scheduler
helm lint . --debug
cd ../..
```

### Step 3: Validate Templates

Render templates to check for syntax errors:

```bash
cd helm/smart-scheduler
helm template smart-scheduler . --debug

# With test values
helm template smart-scheduler . -f test-values.yaml --debug
cd ../..
```

### Step 4: Dry-Run Installation

Test installation without deploying:

```bash
helm install smart-scheduler-test ./helm/smart-scheduler \
  --namespace smart-scheduler-test \
  --create-namespace \
  --dry-run \
  --debug
```

### Step 5: Install the Chart

```bash
# Create namespace
kubectl create namespace smart-scheduler-test

# Install with default values
helm install smart-scheduler-test ./helm/smart-scheduler \
  --namespace smart-scheduler-test \
  --wait \
  --timeout 10m

# Or with test values
helm install smart-scheduler-test ./helm/smart-scheduler \
  --namespace smart-scheduler-test \
  --values ./helm/smart-scheduler/test-values.yaml \
  --wait \
  --timeout 10m
```

### Step 6: Verify Installation

```bash
# Check deployment status
kubectl get deployment -n smart-scheduler-test

# Check pods
kubectl get pods -n smart-scheduler-test

# Check services
kubectl get services -n smart-scheduler-test

# Check webhook
kubectl get mutatingwebhookconfiguration | grep smart-scheduler

# Check CRDs
kubectl get crd | grep smart-scheduler

# View logs
kubectl logs -n smart-scheduler-test -l app.kubernetes.io/name=smart-scheduler
```

### Step 7: Run Helm Tests

The chart includes test pods that verify functionality:

```bash
helm test smart-scheduler-test --namespace smart-scheduler-test --timeout 5m
```

### Step 8: Upgrade Test

```bash
helm upgrade smart-scheduler-test ./helm/smart-scheduler \
  --namespace smart-scheduler-test \
  --wait \
  --timeout 10m
```

### Step 9: Uninstall

```bash
helm uninstall smart-scheduler-test --namespace smart-scheduler-test

# Optionally delete namespace
kubectl delete namespace smart-scheduler-test
```

## Makefile Targets

The Makefile includes convenient targets for Helm testing:

```bash
# Lint the chart
make helm-lint

# Render templates
make helm-template

# Render with test values
make helm-template-test

# Update dependencies
make helm-dependency-update

# Dry-run installation
make helm-dry-run

# Run all validation tests
make helm-test

# Install and run full tests
make helm-install-test

# Uninstall test deployment
make helm-uninstall-test

# Package the chart
make helm-package
```

## Test Scenarios

### Scenario 1: Basic Validation

Test chart structure and template rendering:

```bash
make helm-test
```

### Scenario 2: With Webhook Disabled

Test chart without webhook:

```bash
helm template smart-scheduler-test ./helm/smart-scheduler \
  --set webhook.enabled=false \
  --debug
```

### Scenario 3: With cert-manager Disabled

Test chart with manual certificates:

```bash
helm template smart-scheduler-test ./helm/smart-scheduler \
  --set certificates.certManager.enabled=false \
  --debug
```

### Scenario 4: Development Mode

Test with development configuration:

```bash
helm template smart-scheduler-test ./helm/smart-scheduler \
  --set development.enabled=true \
  --set logging.level=debug \
  --debug
```

### Scenario 5: Multi-Namespace

Test multi-namespace configuration:

```bash
helm template smart-scheduler-test ./helm/smart-scheduler \
  --set multiNamespace.enabled=true \
  --set multiNamespace.watchNamespaces={default,test-ns} \
  --debug
```

### Scenario 6: Custom Resources

Test with custom resource limits:

```bash
helm template smart-scheduler-test ./helm/smart-scheduler \
  --set resources.requests.cpu=100m \
  --set resources.requests.memory=128Mi \
  --set resources.limits.cpu=1000m \
  --set resources.limits.memory=512Mi \
  --debug
```

## Helm Test Pods

The chart includes test pods that verify:

1. **Connection Test** - Verifies webhook service connectivity
2. **Health Test** - Verifies health endpoints are working
3. **Metrics Test** - Verifies metrics endpoint is accessible
4. **Deployment Test** - Verifies deployment is ready and pods are running
5. **RBAC Test** - Verifies RBAC permissions are configured correctly

Test pods are automatically created when you run `helm test` and cleaned up after completion.

## Troubleshooting

### Chart Linting Errors

If linting fails, check:

```bash
# View detailed lint output
cd helm/smart-scheduler
helm lint . --debug

# Check for common issues:
# - Missing required values
# - Syntax errors in templates
# - Invalid YAML
```

### Template Rendering Errors

If template rendering fails:

```bash
# Render with debug output
helm template smart-scheduler-test ./helm/smart-scheduler --debug

# Check specific template
helm template smart-scheduler-test ./helm/smart-scheduler --show-only templates/deployment.yaml
```

### Installation Failures

If installation fails:

```bash
# Check deployment status
kubectl describe deployment -n smart-scheduler-test

# Check pod events
kubectl get events -n smart-scheduler-test --sort-by='.lastTimestamp'

# Check pod logs
kubectl logs -n smart-scheduler-test -l app.kubernetes.io/name=smart-scheduler

# Check certificate status (if using cert-manager)
kubectl get certificate -n smart-scheduler-test
kubectl describe certificate -n smart-scheduler-test
```

### Webhook Issues

If webhook is not working:

```bash
# Check webhook configuration
kubectl get mutatingwebhookconfiguration | grep smart-scheduler
kubectl describe mutatingwebhookconfiguration <webhook-name>

# Check webhook service
kubectl get service -n smart-scheduler-test | grep webhook
kubectl describe service -n smart-scheduler-test <webhook-service>

# Check certificates
kubectl get secret -n smart-scheduler-test | grep webhook
kubectl describe secret -n smart-scheduler-test <webhook-secret>
```

### Test Pod Failures

If helm tests fail:

```bash
# View test pod logs
kubectl logs -n smart-scheduler-test <test-pod-name>

# Check test pod status
kubectl get pods -n smart-scheduler-test | grep test

# Describe test pod
kubectl describe pod -n smart-scheduler-test <test-pod-name>
```

### Certificate Issues

If certificates are not working:

```bash
# Check cert-manager issuer
kubectl get issuer -n smart-scheduler-test
kubectl describe issuer -n smart-scheduler-test

# Check certificate status
kubectl get certificate -n smart-scheduler-test
kubectl describe certificate -n smart-scheduler-test

# Check certificate secret
kubectl get secret -n smart-scheduler-test | grep cert
kubectl describe secret -n smart-scheduler-test <cert-secret>
```

## Best Practices

1. **Always lint before deploying** - Catch syntax errors early
2. **Use dry-run first** - Validate installation before deploying
3. **Test with different configurations** - Verify chart flexibility
4. **Check logs regularly** - Monitor for errors during testing
5. **Clean up test resources** - Remove test deployments after testing
6. **Use test values file** - Keep test configuration separate from defaults
7. **Run helm tests** - Verify functionality after installation

## CI/CD Integration

The test script can be integrated into CI/CD pipelines:

```yaml
# Example GitHub Actions workflow
- name: Test Helm Chart
  run: |
    ./scripts/test-helm.sh --dry-run
    ./scripts/test-helm.sh --install --test --verify
    ./scripts/test-helm.sh --uninstall
```

For non-interactive environments, you may need to modify the script to skip prompts.

## Additional Resources

- [Helm Documentation](https://helm.sh/docs/)
- [Helm Chart Testing Guide](https://helm.sh/docs/chart_tests/)
- [Kubernetes Documentation](https://kubernetes.io/docs/)

