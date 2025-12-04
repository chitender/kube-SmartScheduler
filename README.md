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

Before deploying Smart Scheduler, ensure you have:

- **Kubernetes cluster** (v1.24+)
- **kubectl** configured to access your cluster
- **Helm 3.0+** installed
- **cert-manager** (required for webhook TLS certificates)

### Quick Start - Complete Installation Steps

Follow these steps to deploy Smart Scheduler in your Kubernetes cluster:

#### Step 1: Clone the Repository

```bash
git clone https://github.com/kube-smartscheduler/smart-scheduler.git
cd smart-scheduler
```

#### Step 2: Install cert-manager

Smart Scheduler requires cert-manager for webhook certificate management:

```bash
# Add cert-manager Helm repository
helm repo add jetstack https://charts.jetstack.io
helm repo update

# Install cert-manager
helm install cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --create-namespace \
  --set installCRDs=true \
  --version 1.18.2 \
  --wait --timeout 5m

# Verify cert-manager is running
kubectl get pods -n cert-manager
```

**Expected Output:**
```
NAME                                       READY   STATUS    RESTARTS   AGE
cert-manager-xxx                           1/1     Running   0          XXs
cert-manager-cainjector-xxx                1/1     Running   0          XXs
cert-manager-webhook-xxx                   1/1     Running   0          XXs
```

#### Step 3: Install Smart Scheduler

```bash
# Create namespace
kubectl create namespace smart-scheduler-system

# Update Helm chart dependencies
cd helm/smart-scheduler
helm dependency update
cd ../..

# Install Smart Scheduler (with cert-manager disabled since we installed it separately)
helm install smart-scheduler ./helm/smart-scheduler \
  --namespace smart-scheduler-system \
  --set cert-manager.enabled=false \
  --set certificates.certManager.enabled=true \
  --wait --timeout 10m
```

**Alternative:** If you want Helm to install cert-manager as a subchart:

```bash
# Install with cert-manager subchart enabled
helm install smart-scheduler ./helm/smart-scheduler \
  --namespace smart-scheduler-system \
  --wait --timeout 10m
```

#### Step 4: Verify Installation

```bash
# Check deployment status
kubectl get pods -n smart-scheduler-system

# Check webhook configuration
kubectl get mutatingwebhookconfiguration | grep smart-scheduler

# Check certificates
kubectl get certificates,issuers -n smart-scheduler-system

# Verify certificate is ready
kubectl get certificate -n smart-scheduler-system -o jsonpath='{.items[0].status.conditions[?(@.type=="Ready")].status}'
# Should output: True

# View operator logs
kubectl logs -n smart-scheduler-system -l app.kubernetes.io/name=smart-scheduler --tail=50
```

#### Step 5: Test Pod Placement

Create a test deployment to verify smart scheduling is working:

```yaml
# test-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
  namespace: default
  annotations:
    smart-scheduler.io/schedule-strategy: "base=2,weight=1,nodeSelector=node-type:ondemand;weight=3,nodeSelector=node-type:spot"
spec:
  replicas: 10
  selector:
    matchLabels:
      app: test-app
  template:
    metadata:
      labels:
        app: test-app
    spec:
      containers:
      - name: nginx
        image: nginx:alpine
        resources:
          requests:
            cpu: 10m
            memory: 16Mi
```

```bash
# Apply test deployment
kubectl apply -f test-deployment.yaml

# Check pod distribution
kubectl get pods -l app=test-app -o wide

# Verify nodeSelector is applied by webhook
kubectl get pod -l app=test-app -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.nodeSelector}{"\n"}{end}'
```

### Installation Options

#### Option 1: From Local Chart (Recommended)

```bash
# Clone and install from local chart
git clone https://github.com/kube-smartscheduler/smart-scheduler.git
cd smart-scheduler

# Update dependencies and install
cd helm/smart-scheduler
helm dependency update
cd ../..

helm install smart-scheduler ./helm/smart-scheduler \
  --namespace smart-scheduler-system \
  --create-namespace \
  --set cert-manager.enabled=false
```

#### Option 2: From Helm Repository (When Published)

```bash
# Add Helm repository
helm repo add smart-scheduler https://smart-scheduler.github.io/helm-charts
helm repo update

# Install
helm install smart-scheduler smart-scheduler/smart-scheduler \
  --namespace smart-scheduler-system \
  --create-namespace
```

#### Option 3: Custom Configuration

Create a custom values file:

```yaml
# my-values.yaml
image:
  registry: ghcr.io
  repository: chitender/kube-smartscheduler
  tag: "1.0.7"

cert-manager:
  enabled: false  # Set to false if cert-manager is already installed

certificates:
  certManager:
    enabled: true
    issuer:
      selfSigned: true

webhook:
  enabled: true
  failurePolicy: Ignore  # Use Ignore for testing

resources:
  limits:
    cpu: 500m
    memory: 128Mi
  requests:
    cpu: 10m
    memory: 64Mi
```

Install with custom values:

```bash
helm install smart-scheduler ./helm/smart-scheduler \
  --namespace smart-scheduler-system \
  --create-namespace \
  -f my-values.yaml
```

### Verification Checklist

After installation, verify everything is working:

- [ ] Smart Scheduler pods are running
- [ ] Webhook configuration exists
- [ ] Certificates are ready
- [ ] Test deployment pods have nodeSelector applied
- [ ] Pod distribution matches your strategy

```bash
# Quick verification script
kubectl get pods -n smart-scheduler-system
kubectl get mutatingwebhookconfiguration | grep smart-scheduler
kubectl get certificate -n smart-scheduler-system
```

### Uninstallation

To remove Smart Scheduler:

```bash
# Uninstall Helm release
helm uninstall smart-scheduler --namespace smart-scheduler-system

# Optionally delete namespace
kubectl delete namespace smart-scheduler-system

# If you installed cert-manager separately, uninstall it too
helm uninstall cert-manager --namespace cert-manager
kubectl delete namespace cert-manager
```

## 🎯 Quick Start

### Using Smart Scheduler for Pod Placement

After installation, you can use Smart Scheduler by adding annotations to your Deployments. The webhook will automatically apply placement strategies when pods are created.

### Example 1: Distribute Pods Across Specific Nodes

Place pods on specific nodes by hostname:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web-app
  namespace: default
  annotations:
    smart-scheduler.io/schedule-strategy: "base=3,weight=1,nodeSelector=kubernetes.io/hostname:worker-node-1;weight=5,nodeSelector=kubernetes.io/hostname:worker-node-2"
spec:
  replicas: 8
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
        image: nginx:alpine
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
```

**Result**: 
- 3 pods → worker-node-1 (base guarantee)
- 5 pods → worker-node-2 (remaining pods)

### Example 2: On-Demand vs Spot Distribution

Distribute pods between on-demand and spot instances:

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
        image: nginx:alpine
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
```

**Result**: 
- First 2 pods → on-demand nodes (base guarantee)
- Remaining 8 pods → distributed 1:3 ratio = 2 on-demand + 6 spot
- **Final distribution**: 4 on-demand, 6 spot

### Example 3: Multi-Zone Placement

Distribute pods across multiple availability zones:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web-app
  annotations:
    smart-scheduler.io/schedule-strategy: "base=1,weight=2,nodeSelector=topology.kubernetes.io/zone:us-west-1a;weight=2,nodeSelector=topology.kubernetes.io/zone:us-west-1b;weight=1,nodeSelector=topology.kubernetes.io/zone:us-west-1c"
spec:
  replicas: 12
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
        image: nginx:alpine
```

**Result**: Pods distributed across zones based on weights

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

**Symptoms:** Pods are created but nodeSelector is not applied.

```bash
# Check webhook configuration exists
kubectl get mutatingwebhookconfiguration | grep smart-scheduler

# Check certificate status (must be Ready)
kubectl get certificate -n smart-scheduler-system
kubectl get certificate -n smart-scheduler-system -o jsonpath='{.items[0].status.conditions[?(@.type=="Ready")].status}'
# Should output: True

# Check webhook service
kubectl get service -n smart-scheduler-system | grep webhook

# Check webhook logs for errors
kubectl logs -n smart-scheduler-system -l app.kubernetes.io/name=smart-scheduler --tail=100 | grep -i webhook

# Test webhook connectivity
kubectl get endpoints -n smart-scheduler-system | grep webhook
```

**Common Fixes:**
- Ensure cert-manager is installed and running
- Wait for certificates to be ready (can take 1-2 minutes)
- Check webhook service is accessible from cluster DNS

#### 2. Placement Not Applied

**Symptoms:** Pods are scheduled but not following the placement strategy.

```bash
# Check if deployment has the annotation
kubectl get deployment <name> -o yaml | grep "smart-scheduler.io/schedule-strategy"

# Verify annotation format is correct
kubectl get deployment <name> -o jsonpath='{.metadata.annotations.smart-scheduler\.io/schedule-strategy}'

# Check if pods have nodeSelector applied
kubectl get pod -l app=<your-app> -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.nodeSelector}{"\n"}{end}'

# Check operator logs for errors
kubectl logs -n smart-scheduler-system -l app.kubernetes.io/name=smart-scheduler --tail=100

# Verify RBAC permissions
kubectl auth can-i get deployments --as=system:serviceaccount:smart-scheduler-system:smart-scheduler
kubectl auth can-i get pods --as=system:serviceaccount:smart-scheduler-system:smart-scheduler
kubectl auth can-i get configmaps --as=system:serviceaccount:smart-scheduler-system:smart-scheduler
```

**Common Fixes:**
- Ensure annotation format is correct (see [Annotation Format](#annotation-format) below)
- Check that deployment has owner references (not a standalone pod)
- Verify namespace is not excluded in webhook configuration

#### 3. Cert-Manager Issues

**Symptoms:** Certificate resources fail to create or certificates are not ready.

```bash
# Check cert-manager pods
kubectl get pods -n cert-manager

# Check Certificate resources
kubectl get certificate -n smart-scheduler-system

# Check CertificateRequest status
kubectl get certificaterequest -n smart-scheduler-system

# Check Issuer status
kubectl get issuer -n smart-scheduler-system

# View certificate events
kubectl describe certificate -n smart-scheduler-system
```

**Common Fixes:**
- Ensure cert-manager is installed: `kubectl get pods -n cert-manager`
- Wait for cert-manager to be ready before installing Smart Scheduler
- Check cert-manager logs for errors

#### 4. Pod Distribution Not Matching Expectations

**Symptoms:** Pods are placed but not in the expected distribution.

```bash
# Check current pod distribution
kubectl get pods -l app=<your-app> -o wide

# Count pods per node
kubectl get pods -l app=<your-app> --field-selector spec.nodeName=<node-name> --no-headers | wc -l

# Check placement state ConfigMap
kubectl get configmap -n <namespace> -l app.kubernetes.io/component=placement-state

# Check rebalance controller logs
kubectl logs -n smart-scheduler-system -l app.kubernetes.io/name=smart-scheduler | grep RebalanceController
```

**Common Fixes:**
- Review placement strategy format (base count and weights)
- Check if nodes match the nodeSelector labels
- Allow time for rebalancing to occur (if enabled)
- Verify enough pods exist for the distribution

#### 5. Policy Not Matching Deployments

```bash
# Check policy status
kubectl describe podplacementpolicy <policy-name>

# Verify label selectors match
kubectl get deployment <name> --show-labels

# Check policy logs
kubectl logs -n smart-scheduler-system -l app.kubernetes.io/name=smart-scheduler | grep "PodPlacementPolicyController"
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

### Complete Working Examples

See the [examples/](examples/) directory for comprehensive examples:

- **[Test Placement Deployment](examples/test-placement-deployment.yaml)** - Working example that distributes pods across specific nodes
- Basic on-demand/spot distribution examples
- Multi-zone deployment examples

### Test Placement Example

The `test-placement-deployment.yaml` file demonstrates a real working configuration:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-placement
  namespace: smart-scheduler-test
  annotations:
    smart-scheduler.io/schedule-strategy: "base=3,weight=1,nodeSelector=kubernetes.io/hostname:desktop-worker2;weight=20,nodeSelector=kubernetes.io/hostname:desktop-worker"
spec:
  replicas: 23
  selector:
    matchLabels:
      app: test-placement
  template:
    metadata:
      labels:
        app: test-placement
    spec:
      containers:
      - name: nginx
        image: nginx:alpine
        resources:
          requests:
            cpu: 10m
            memory: 16Mi
```

**To use this example:**

1. Create the namespace:
   ```bash
   kubectl create namespace smart-scheduler-test
   ```

2. Apply the deployment:
   ```bash
   kubectl apply -f examples/test-placement-deployment.yaml
   ```

3. Verify pod distribution:
   ```bash
   kubectl get pods -n smart-scheduler-test -l app=test-placement -o wide
   ```

4. Count pods per node:
   ```bash
   kubectl get pods -n smart-scheduler-test -l app=test-placement --field-selector spec.nodeName=<node-name> --no-headers | wc -l
   ```

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

The `smart-scheduler.io/schedule-strategy` annotation follows this format:

```
smart-scheduler.io/schedule-strategy: "base=<int>,weight=<int>,nodeSelector=<key>:<value>[,<key>:<value>];weight=<int>,nodeSelector=<key>:<value>"
```

**Format Breakdown:**
- `base=<int>` - Number of pods guaranteed to use the first rule (optional, defaults to 0)
- `weight=<int>` - Weight for distribution (higher weight = more pods)
- `nodeSelector=<key>:<value>` - Node selector in format `key:value`
- Multiple rules separated by `;`

**Examples:**

```yaml
# Example 1: Simple two-node distribution
smart-scheduler.io/schedule-strategy: "base=3,weight=1,nodeSelector=kubernetes.io/hostname:node1;weight=2,nodeSelector=kubernetes.io/hostname:node2"

# Example 2: On-demand vs Spot (no base)
smart-scheduler.io/schedule-strategy: "base=0,weight=1,nodeSelector=node-type:ondemand;weight=3,nodeSelector=node-type:spot"

# Example 3: Multi-zone placement
smart-scheduler.io/schedule-strategy: "base=1,weight=2,nodeSelector=zone:us-west-1a;weight=2,nodeSelector=zone:us-west-1b;weight=1,nodeSelector=zone:us-west-1c"

# Example 4: Multiple labels in nodeSelector
smart-scheduler.io/schedule-strategy: "base=2,weight=1,nodeSelector=node-type:ondemand,zone:us-west-1a;weight=2,nodeSelector=node-type:spot,zone:us-west-1b"
```

**How It Works:**
1. First `base` pods are placed using the first rule's nodeSelector
2. Remaining pods are distributed proportionally based on weights
3. Webhook automatically applies nodeSelector when pods are created

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

This project is licensed under the Apache License 2.0 with Commercial Use Restrictions.

**Key Points:**
- ✅ **Free for non-commercial use** - Personal, educational, research, and open-source projects
- ✅ **Free for internal production use** - Use within your organization for internal operations, production systems, and cloud cost optimization
- ⚠️ **Commercial redistribution requires permission** - Contact the maintainer if offering Smart Scheduler as a product or service to third parties

**What's Allowed Without Permission:**
- ✅ Personal or educational use
- ✅ **Internal use within any organization** (including commercial enterprises)
- ✅ **Using in production systems** for your organization's own operations
- ✅ **Cloud cost optimization** within your own infrastructure
- ✅ **Internal workload management** in your Kubernetes clusters
- ✅ Contributing code back to the project
- ✅ Research or academic purposes
- ✅ Open-source projects

**What Requires Permission:**
- ⚠️ Offering Smart Scheduler as a SaaS or commercial service to third parties
- ⚠️ Incorporating into a commercial product sold or licensed to others
- ⚠️ Reselling or redistributing as part of a commercial offering
- ⚠️ Providing managed services based on Smart Scheduler to third parties

**Contact for Commercial Licensing:**
- Email: chitenderkumar.16@gmail.com

See the [LICENSE](LICENSE) file for complete terms and conditions.

## 🌟 Acknowledgments

- Kubernetes SIG-Scheduling for scheduler extensibility
- cert-manager team for certificate management patterns
- controller-runtime for operator framework
- The broader Kubernetes community

---

**SmartScheduler** - Intelligent Kubernetes Pod Placement Made Simple
