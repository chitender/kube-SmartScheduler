# SmartScheduler - Kubernetes Intelligent Pod Placement Operator

SmartScheduler is a Kubernetes operator that provides intelligent pod placement using custom annotations to implement sophisticated scheduling strategies including base counts and weighted distributions across different node types.

## Features

- **Custom Annotation-Based Scheduling**: Define placement strategies using simple annotations on Deployments
- **Base Count Placement**: Ensure a minimum number of pods are placed on preferred nodes
- **Weighted Distribution**: Distribute remaining pods across node types using configurable weights
- **Dynamic Tracking**: Real-time tracking of pod distribution across node selectors
- **Fail-Safe Operation**: Graceful fallback to default Kubernetes scheduling if strategies fail
- **Production Ready**: Built with controller-runtime, includes health checks, metrics, and proper RBAC

## Architecture

SmartScheduler consists of:

1. **Mutating Admission Webhook**: Intercepts pod creation and applies placement logic
2. **Controller**: Watches Deployments and manages webhook configurations
3. **Placement Engine**: Parses strategies and calculates optimal pod placement
4. **Certificate Management**: Automatic TLS certificate handling via cert-manager

## Quick Start

### Prerequisites

- Kubernetes cluster (v1.20+)
- kubectl configured
- cert-manager installed (for automatic certificate management)

### Installation

1. **Install cert-manager** (if not already installed):
```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml
```

2. **Deploy SmartScheduler**:
```bash
kubectl apply -f deploy/
```

3. **Verify installation**:
```bash
kubectl get pods -n smart-scheduler-system
kubectl get mutatingwebhookconfiguration smart-scheduler-mutating-webhook
```

### Usage

Add the scheduling strategy annotation to your Deployment:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web-app
  annotations:
    smart-scheduler.io/schedule-strategy: "base=1,weight=1,nodeSelector=node-type:ondemand;weight=2,nodeSelector=node-type:spot"
spec:
  replicas: 6
  selector:
    matchLabels:
      app: web-app
  template:
    metadata:
      labels:
        app: web-app
    spec:
      containers:
      - name: web
        image: nginx:1.21
```

This strategy will:
- Place the first pod on `node-type: ondemand` nodes (base=1)
- Distribute remaining 5 pods with a 1:2 ratio between ondemand and spot nodes
- Result: 1 base + 2 ondemand + 3 spot = 6 total pods

## Annotation Format

The `smart-scheduler.io/schedule-strategy` annotation uses this format:

```
base=<count>,weight=<weight>,nodeSelector=<key>:<value>[,<key>:<value>];weight=<weight>,nodeSelector=<key>:<value>
```

### Parameters

- **base**: Number of pods to place using the first rule (minimum guaranteed)
- **weight**: Relative weight for this node type in distribution
- **nodeSelector**: Key-value pairs for node selection (use `:` for key-value, `,` for multiple pairs)

### Examples

**Example 1: Basic on-demand/spot distribution**
```
smart-scheduler.io/schedule-strategy: "base=2,weight=1,nodeSelector=node-type:ondemand;weight=3,nodeSelector=node-type:spot"
```
- 2 pods guaranteed on on-demand nodes
- Remaining pods distributed 1:3 ratio (on-demand:spot)

**Example 2: Multi-zone with instance types**
```
smart-scheduler.io/schedule-strategy: "base=1,weight=2,nodeSelector=zone:us-west-1a,node-type:ondemand;weight=1,nodeSelector=zone:us-west-1b,node-type:spot"
```
- 1 pod guaranteed in us-west-1a on on-demand
- Remaining pods distributed 2:1 ratio between the zones

**Example 3: GPU workload distribution**
```
smart-scheduler.io/schedule-strategy: "base=1,weight=1,nodeSelector=accelerator:nvidia-tesla-k80;weight=2,nodeSelector=accelerator:nvidia-tesla-v100"
```
- 1 pod guaranteed on K80 nodes
- Remaining pods prefer V100 nodes with 1:2 ratio

## How It Works

1. **Deployment Creation**: User creates a Deployment with scheduling strategy annotation
2. **Controller Watch**: SmartScheduler controller detects the annotation
3. **Pod Interception**: When pods are created, the mutating webhook intercepts them
4. **Strategy Parsing**: The webhook parses the annotation and determines placement
5. **Pod Counting**: Current pod distribution is calculated by querying existing pods
6. **Placement Decision**: Based on counts and weights, the optimal node selector is chosen
7. **Pod Mutation**: The pod's nodeSelector is modified before creation

## Development

### Building

```bash
# Build binary
make build

# Build Docker image
make docker-build

# Run tests
make test
```

### Testing Locally

```bash
# Generate certificates for local testing
make generate-certs

# Run locally (requires kubeconfig)
make run
```

### Custom Certificate Management

If not using cert-manager, generate certificates manually:

```bash
make generate-certs
kubectl create secret tls smart-scheduler-webhook-certs \
  --cert=config/certs/server.crt \
  --key=config/certs/server.key \
  -n smart-scheduler-system
```

## Monitoring

SmartScheduler exposes metrics on `:8080/metrics`:

- `smart_scheduler_pods_placed_total`: Total pods placed by strategy
- `smart_scheduler_placement_errors_total`: Total placement errors
- `controller_runtime_*`: Standard controller-runtime metrics

Health checks available:
- `:8081/healthz`: Liveness probe
- `:8081/readyz`: Readiness probe

## Troubleshooting

### Common Issues

**1. Webhook not intercepting pods**
```bash
# Check webhook configuration
kubectl get mutatingwebhookconfiguration smart-scheduler-mutating-webhook -o yaml

# Check webhook pod logs
kubectl logs -n smart-scheduler-system deployment/smart-scheduler-controller-manager
```

**2. Certificate issues**
```bash
# Check certificate status
kubectl get certificate -n smart-scheduler-system
kubectl describe certificate smart-scheduler-serving-cert -n smart-scheduler-system

# Check cert-manager logs
kubectl logs -n cert-manager deployment/cert-manager
```

**3. Pods not being placed correctly**
```bash
# Check pod annotations and nodeSelector
kubectl get pod <pod-name> -o yaml | grep -A 10 -B 5 nodeSelector

# Check webhook logs for placement decisions
kubectl logs -n smart-scheduler-system deployment/smart-scheduler-controller-manager | grep "placement"
```

### Debug Mode

Enable debug logging:
```bash
kubectl set env deployment/smart-scheduler-controller-manager -n smart-scheduler-system LOG_LEVEL=debug
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make changes and add tests
4. Ensure all tests pass: `make test`
5. Submit a pull request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Roadmap

- [ ] Support for StatefulSets and Jobs
- [ ] Topology-aware scheduling (zones, regions)
- [ ] Cost-aware scheduling with spot pricing integration
- [ ] Web UI for managing placement strategies
- [ ] Integration with cluster-autoscaler
- [ ] Custom Resource Definitions for advanced policies
- [ ] Prometheus alerting rules
- [ ] Helm chart packaging
SmartScheduler: An intelligent Kubernetes operator that dynamically assigns pods to nodes based on custom weights, base counts, and label-based placement strategies for optimal workload distribution.
