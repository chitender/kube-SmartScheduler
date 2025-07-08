# SmartScheduler

A production-ready Kubernetes operator for intelligent pod placement with weighted scheduling strategies, automatic rebalancing, and centralized policy management.

## 🚀 Features

### Core Functionality
- **Intelligent Pod Placement**: Weighted distribution across node types (on-demand/spot, zones, etc.)
- **Base Count Guarantees**: Ensure minimum pods on preferred nodes before distribution
- **Atomic State Management**: ConfigMap-based state tracking with conflict resolution
- **Automatic Rebalancing**: Drift detection and corrective actions when placement deviates
- **Enhanced Error Handling**: Graceful fallback to default scheduling on failures

### Advanced Features
- **Pod Affinity/Anti-Affinity**: Beyond simple nodeSelector, support for complex placement rules
- **CRD-Based Policies**: Centralized `PodPlacementPolicy` CRD for enterprise management
- **Priority-Based Policies**: Multiple policies with priority handling and conflict resolution
- **Real-time Monitoring**: Comprehensive metrics and health endpoints
- **Production Ready**: Helm charts, RBAC, security contexts, and multi-namespace support

## 📦 Installation

### Prerequisites
- Kubernetes 1.19+ cluster
- cert-manager (for automatic TLS certificate management)
- Helm 3.0+ (for Helm installation)

### Option 1: Helm Installation (Recommended)

```bash
# Add the SmartScheduler Helm repository
helm repo add smart-scheduler https://smart-scheduler.github.io/helm-charts
helm repo update

# Install with default values
helm install smart-scheduler smart-scheduler/smart-scheduler \
  --namespace smart-scheduler-system \
  --create-namespace

# Install with custom values
helm install smart-scheduler smart-scheduler/smart-scheduler \
  --namespace smart-scheduler-system \
  --create-namespace \
  --values custom-values.yaml
```

### Option 2: Direct Kubernetes Manifests

```bash
# Clone the repository
git clone https://github.com/kube-smartscheduler/smart-scheduler.git
cd smart-scheduler

# Apply the manifests
kubectl apply -f deploy/
```

### Option 3: Local Development Build

```bash
# Build and deploy locally
make build
make docker-build IMG=smart-scheduler:latest
make deploy IMG=smart-scheduler:latest
```

## 🎯 Quick Start

### 1. Annotation-Based Usage (Simple)

Add annotations to your Deployment to enable smart scheduling:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web-app
  annotations:
    smart-scheduler.io/schedule-strategy: "base=2,weight=1,nodeSelector=node-type:ondemand;weight=3,nodeSelector=node-type:spot"
spec:
  replicas: 10
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
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
```

**Result**: 
- First 2 pods → on-demand nodes (base guarantee)
- Remaining 8 pods → distributed 1:3 ratio = 2 on-demand + 6 spot
- **Final distribution**: 4 on-demand, 6 spot

### 2. CRD-Based Usage (Enterprise)

Create centralized placement policies using the `PodPlacementPolicy` CRD:

```yaml
apiVersion: smartscheduler.io/v1
kind: PodPlacementPolicy
metadata:
  name: web-app-policy
  namespace: production
spec:
  enabled: true
  priority: 100
  selector:
    matchLabels:
      tier: web
  strategy:
    base: 2
    rules:
    - name: "on-demand-nodes"
      weight: 1
      nodeSelector:
        node-type: ondemand
      description: "Reliable on-demand instances for base capacity"
    - name: "spot-nodes"
      weight: 3
      nodeSelector:
        node-type: spot
      affinity:
      - type: "anti-affinity"
        labelSelector:
          app: web-app
        topologyKey: "kubernetes.io/hostname"
        requiredDuringScheduling: false
      description: "Cost-effective spot instances for scale-out"
    rebalancePolicy:
      enabled: true
      driftThreshold: 15.0
      checkInterval: 5m
      maxPodsPerRebalance: 2
```

Apply policies that automatically manage multiple deployments:

```bash
kubectl apply -f policy.yaml

# Check policy status
kubectl get podplacementpolicy -n production
kubectl describe podplacementpolicy web-app-policy -n production
```

## 🔧 Configuration

### Helm Values Configuration

```yaml
# values.yaml
replicaCount: 1

image:
  repository: smart-scheduler
  tag: "v0.1.0"
  pullPolicy: IfNotPresent

# Feature toggles
features:
  crdPolicies: true
  rebalancing: true
  driftDetection: true
  enhancedMetrics: true

# Webhook configuration
webhook:
  enabled: true
  failurePolicy: Fail
  excludeNamespaces:
    - kube-system
    - cert-manager

# Monitoring
monitoring:
  serviceMonitor:
    enabled: true
    scrapeInterval: 30s
  prometheusRule:
    enabled: true

# Resources
resources:
  limits:
    cpu: 500m
    memory: 256Mi
  requests:
    cpu: 50m
    memory: 128Mi

# Security
securityContext:
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  runAsNonRoot: true
```

### Environment Variables

The operator supports several environment variables for configuration:

| Variable | Description | Default |
|----------|-------------|---------|
| `ENABLE_CRD_POLICIES` | Enable PodPlacementPolicy CRD support | `true` |
| `ENABLE_REBALANCING` | Enable automatic rebalancing | `true` |
| `ENABLE_DRIFT_DETECTION` | Enable placement drift detection | `true` |
| `ENABLE_ENHANCED_METRICS` | Enable detailed metrics collection | `true` |
| `WEBHOOK_CERT_DIR` | Directory for webhook certificates | `/tmp/k8s-webhook-server/serving-certs` |

## 📊 Monitoring and Observability

### Health Endpoints

- **Health Check**: `GET /healthz` (port 8081)
- **Readiness**: `GET /readyz` (port 8081)
- **Metrics**: `GET /metrics` (port 8080)

### Key Metrics

SmartScheduler exposes comprehensive Prometheus metrics:

```promql
# Pod placement success rate
smart_scheduler_pod_placements_total{status="success"}

# Placement drift percentage
smart_scheduler_placement_drift_percentage

# Rebalancing actions
smart_scheduler_rebalance_actions_total

# Webhook response time
smart_scheduler_webhook_duration_seconds

# Policy application success
smart_scheduler_policy_applications_total{policy="web-app-policy"}
```

### Grafana Dashboard

Import our pre-built Grafana dashboard for comprehensive monitoring:

```bash
# Download dashboard JSON
curl -O https://raw.githubusercontent.com/kube-smartscheduler/smart-scheduler/main/monitoring/grafana-dashboard.json

# Import in Grafana UI or via API
```

## 🛠️ Advanced Usage

### Multi-Zone Placement

```yaml
annotations:
  smart-scheduler.io/schedule-strategy: "base=1,weight=2,nodeSelector=zone:us-west-1a;weight=2,nodeSelector=zone:us-west-1b;weight=1,nodeSelector=zone:us-west-1c"
```

### GPU Workloads with Affinity

```yaml
apiVersion: smartscheduler.io/v1
kind: PodPlacementPolicy
metadata:
  name: gpu-workload-policy
spec:
  selector:
    matchLabels:
      workload: gpu-intensive
  strategy:
    base: 0
    rules:
    - name: "gpu-nodes-preferred"
      weight: 3
      nodeSelector:
        accelerator: nvidia-tesla-v100
      affinity:
      - type: "affinity"
        labelSelector:
          workload: gpu-intensive
        topologyKey: "kubernetes.io/hostname"
        requiredDuringScheduling: false
    - name: "gpu-nodes-fallback"
      weight: 1
      nodeSelector:
        accelerator: nvidia-tesla-k80
```

### Time-Based Rebalancing

```yaml
rebalancePolicy:
  enabled: true
  driftThreshold: 20.0
  rebalanceWindow:
    startTime: "02:00"
    endTime: "04:00"
    days: ["Mon", "Wed", "Fri"]
    timezone: "UTC"
```

## 🐛 Troubleshooting

### Common Issues

#### 1. Webhook Not Working

```bash
# Check webhook configuration
kubectl get mutatingwebhookconfiguration

# Check certificate status
kubectl get certificate -n smart-scheduler-system

# Check webhook logs
kubectl logs -n smart-scheduler-system deployment/smart-scheduler -f
```

#### 2. Placement Not Applied

```bash
# Check if deployment has annotations
kubectl get deployment <name> -o yaml | grep annotations -A 5

# Check operator logs
kubectl logs -n smart-scheduler-system deployment/smart-scheduler -c manager

# Verify RBAC permissions
kubectl auth can-i update deployments --as=system:serviceaccount:smart-scheduler-system:smart-scheduler
```

#### 3. Policy Not Matching Deployments

```bash
# Check policy status
kubectl describe podplacementpolicy <policy-name>

# Verify label selectors
kubectl get deployment <name> --show-labels

# Check policy logs
kubectl logs -n smart-scheduler-system deployment/smart-scheduler | grep "PodPlacementPolicyController"
```

### Debug Mode

Enable debug logging for detailed troubleshooting:

```yaml
# In Helm values
logging:
  level: debug
  development: true

# Or via environment variable
development:
  debug: true
```

## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

### Development Setup

```bash
# Clone repository
git clone https://github.com/kube-smartscheduler/smart-scheduler.git
cd smart-scheduler

# Install dependencies
go mod download

# Run tests
make test

# Build locally
make build

# Run locally (requires kubeconfig)
./bin/manager
```

### Testing

```bash
# Run unit tests
make test

# Run integration tests with KinD
make test-integration

# Run webhook tests
make test-webhook

# Lint code
make lint
```

## 📋 Examples

### Complete Examples

See the [examples/](examples/) directory for comprehensive examples:

- [Basic on-demand/spot distribution](examples/ondemand-spot.yaml)
- [Multi-zone deployment](examples/multi-zone.yaml) 
- [GPU workload placement](examples/gpu-workload.yaml)
- [Enterprise policy management](examples/enterprise-policies.yaml)
- [Microservices with anti-affinity](examples/microservices.yaml)

### Real-World Scenarios

#### E-commerce Platform
```yaml
# Frontend: Prioritize availability
smart-scheduler.io/schedule-strategy: "base=3,weight=2,nodeSelector=node-type:ondemand;weight=1,nodeSelector=node-type:spot"

# Backend APIs: Cost-optimized with availability
smart-scheduler.io/schedule-strategy: "base=2,weight=1,nodeSelector=node-type:ondemand;weight=3,nodeSelector=node-type:spot"

# Background jobs: Fully cost-optimized
smart-scheduler.io/schedule-strategy: "base=0,weight=1,nodeSelector=node-type:spot"
```

## 🔒 Security

### Security Features

- **Non-root containers**: Runs as user 65532
- **Read-only filesystem**: Immutable container filesystem
- **Minimal capabilities**: Drops all Linux capabilities
- **Network policies**: Optional network isolation
- **Pod Security Standards**: Compatible with restricted PSS

### RBAC Permissions

SmartScheduler requires minimal RBAC permissions:

- **Pods**: Read, delete (for rebalancing)
- **Deployments**: Read, update (for policy application)
- **ConfigMaps**: Full access (for state management)
- **Events**: Create (for audit trail)
- **PodPlacementPolicies**: Full access (CRD management)

## 📚 API Reference

### Annotation Format

```
smart-scheduler.io/schedule-strategy: "base=<int>,weight=<int>,nodeSelector=<key>:<value>[,<key>:<value>];weight=<int>,nodeSelector=<key>:<value>"
```

### PodPlacementPolicy CRD

See the [API documentation](docs/api.md) for complete CRD specification.

## 🚀 Roadmap

- [ ] **Multi-cluster support**: Placement across clusters
- [ ] **Cost optimization**: Integration with cloud pricing APIs
- [ ] **Machine learning**: Predictive placement based on workload patterns
- [ ] **Custom schedulers**: Support for scheduler plugins
- [ ] **UI Dashboard**: Web interface for policy management
- [ ] **Integration tests**: Comprehensive e2e test suite

## 📄 License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## 🌟 Acknowledgments

- Kubernetes SIG-Scheduling for scheduler extensibility
- cert-manager team for certificate management patterns
- controller-runtime for operator framework
- The broader Kubernetes community

---

**SmartScheduler** - Intelligent Kubernetes Pod Placement Made Simple
