# Smart Scheduler Debugging Guide

This guide explains how to configure the Smart Scheduler for debugging and testing purposes.

## Namespace-Specific Watching

For debugging and testing, you can configure the Smart Scheduler to watch only specific namespaces instead of the entire cluster.

### Benefits of Namespace Scoping

- **Reduced Noise**: Only see logs related to your test namespace
- **Faster Startup**: Smaller cache footprint for development
- **Isolation**: Avoid affecting production workloads
- **Focused Testing**: Test specific scenarios without cluster-wide impact

## Configuration Options

### 1. Command Line Flag (Binary)

When running the binary directly:

```bash
# Watch single namespace
./bin/manager --watch-namespaces=default

# Watch multiple namespaces  
./bin/manager --watch-namespaces=default,test-ns,demo

# Watch all namespaces (default behavior)
./bin/manager
```

### 2. Helm Configuration

Update your `values.yaml`:

```yaml
# Multi-namespace support
multiNamespace:
  enabled: true
  # List of namespaces to watch (empty means all namespaces)
  watchNamespaces:
    - default
    - test-namespace
    - demo-namespace

# Development settings
development:
  enabled: true
  debug: true
  debugApiRequests: true  # Enable detailed API request logging

# Logging configuration
logging:
  level: debug
  development: true
  encoder: console
```

Then upgrade your Helm release:

```bash
helm upgrade smart-scheduler ./helm/smart-scheduler \
  --namespace smart-scheduler-system \
  --values your-debug-values.yaml
```

### 3. Kubernetes Deployment (Manual)

Edit the deployment directly:

```bash
kubectl edit deployment -n smart-scheduler-system smart-scheduler-controller-manager
```

Add to the args section:
```yaml
spec:
  template:
    spec:
      containers:
      - name: manager
        args:
        - --leader-elect
        - --watch-namespaces=default,test-ns  # Add this line
        - --debug-api-requests               # Optional: API debugging
        - --zap-log-level=debug              # Optional: Debug logging
        - --zap-devel                        # Optional: Development mode
```

## Example Debug Configuration

Here's a complete debug configuration for watching only the `default` namespace:

### debug-values.yaml
```yaml
# Multi-namespace support
multiNamespace:
  enabled: true
  watchNamespaces:
    - default

# Development settings
development:
  enabled: true
  debug: true
  debugApiRequests: true

# Logging configuration
logging:
  level: debug
  development: true
  encoder: console

# Image configuration (use latest for testing)
image:
  tag: "latest"
  pullPolicy: Always

# Resource limits for debugging
resources:
  limits:
    cpu: 1000m
    memory: 256Mi
  requests:
    cpu: 100m
    memory: 128Mi
```

### Deploy Debug Configuration

```bash
# Install with debug configuration
helm install smart-scheduler-debug ./helm/smart-scheduler \
  --namespace smart-scheduler-debug \
  --create-namespace \
  --values debug-values.yaml

# Or upgrade existing installation
helm upgrade smart-scheduler ./helm/smart-scheduler \
  --namespace smart-scheduler-system \
  --values debug-values.yaml
```

## Verification

### 1. Check Logs

```bash
# Check if namespace scoping is enabled
kubectl logs -n smart-scheduler-system deployment/smart-scheduler-controller-manager | grep "Watching"

# Expected output:
# "Watching specific namespaces" namespaces=["default"]
```

### 2. Test Deployment

Create a test deployment in your watched namespace:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: debug-test
  namespace: default
  annotations:
    smart-scheduler.io/schedule-strategy: "base=1,weight=1,nodeSelector=node-type:worker"
spec:
  replicas: 3
  selector:
    matchLabels:
      app: debug-test
  template:
    metadata:
      labels:
        app: debug-test
    spec:
      containers:
      - name: test
        image: nginx:alpine
        resources:
          requests:
            cpu: 10m
            memory: 16Mi
```

### 3. Monitor Activity

```bash
# Follow logs in real-time
kubectl logs -n smart-scheduler-system deployment/smart-scheduler-controller-manager -f

# You should see:
# - Webhook requests for pod creation
# - Controller reconciliation for the deployment
# - Placement strategy application
# - State management updates
```

## Debug Log Examples

With debug configuration enabled, you'll see detailed logs like:

```
2025-07-10T12:51:07Z INFO Starting Smart Scheduler Manager watchNamespaces=default
2025-07-10T12:51:07Z INFO Watching specific namespaces namespaces=["default"]  
2025-07-10T12:51:07Z INFO Configured single namespace cache namespace=default

=== WEBHOOK REQUEST START ===
Found parent deployment deploymentName=debug-test
Successfully applied smart scheduling nodeSelector=map[node-type:worker]

=== REBALANCE RECONCILE START ===
Processing rebalance check for deployment strategy="base=1,weight=1,nodeSelector=node-type:worker"
No rebalancing required, scheduling next check
```

## Troubleshooting

### No Events in Watched Namespace

1. Verify namespace scoping is applied:
   ```bash
   kubectl logs -n smart-scheduler-system deployment/smart-scheduler-controller-manager | grep -A5 -B5 "namespace"
   ```

2. Check RBAC permissions for the target namespace

3. Ensure the deployment has the correct annotations

### Still Seeing Other Namespaces

1. Check if multiple Smart Scheduler instances are running
2. Verify the correct Helm values are applied
3. Restart the deployment after configuration changes

### Performance Issues

1. Reduce log level from debug to info for production-like testing
2. Disable API request debugging for performance testing
3. Use specific namespaces instead of cluster-wide watching

## Best Practices

1. **Use Separate Namespace**: Create a dedicated namespace for testing (e.g., `smart-scheduler-debug`)
2. **Minimal Resources**: Use small pod resources for faster scheduling
3. **Clean Up**: Remove test deployments when done debugging
4. **Log Management**: Use log aggregation tools for better analysis
5. **Version Control**: Keep debug configurations in version control

## Reverting to Cluster-Wide Watching

To revert to watching all namespaces:

```yaml
# values.yaml
multiNamespace:
  enabled: false
  watchNamespaces: []
```

Or remove the `--watch-namespaces` flag from the deployment args. 