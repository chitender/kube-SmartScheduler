# Smart Scheduler: Intelligent Pod Placement for Kubernetes Cost Optimization

## The Problem: Kubernetes Scheduling Doesn't Understand Your Business Goals

Kubernetes' default scheduler is brilliant at bin-packing pods efficiently, but it doesn't understand your business priorities. When you're running a mix of on-demand and spot instances, or need to distribute workloads across specific zones or node types, the default scheduler treats all nodes equally.

### Real-World Scenarios Where Default Scheduling Falls Short

**Scenario 1: Cost Optimization with Spot Instances**
You want to run 80% of your workload on spot instances (cheaper) and 20% on on-demand (reliable). The default scheduler has no concept of this ratio—it just places pods wherever resources are available.

**Scenario 2: Multi-Zone Distribution**
You need guaranteed distribution across availability zones for high availability, but also want to optimize costs. The scheduler's topology spread constraints help, but they don't give you fine-grained control over the exact distribution.

**Scenario 3: Node Type Preferences**
You have GPU nodes, high-memory nodes, and standard nodes. You want to ensure critical workloads get GPU nodes first, but also want to utilize all node types efficiently. The default scheduler's scoring doesn't align with your business logic.

**Scenario 4: Base Capacity Guarantees**
You need at least 3 pods on reliable nodes before distributing the rest. The default scheduler doesn't understand "base capacity" concepts—it's all about resource availability.

## Enter Smart Scheduler: Business Logic Meets Kubernetes Scheduling

Smart Scheduler is a Kubernetes operator that extends pod placement with **weighted distribution strategies** and **base capacity guarantees**. It works by mutating pods at admission time to add `nodeSelector` constraints based on your business rules.

### How It Works: The Architecture

```
┌─────────────────┐
│   Deployment    │
│  (with strategy │
│   annotation)   │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ MutatingWebhook │ ◄─── Intercepts pod creation
│   (Smart        │
│   Scheduler)    │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Pod Mutation   │ ◄─── Adds nodeSelector based on:
│                 │      - Base count guarantees
│                 │      - Weighted distribution
│                 │      - Current placement state
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Kubernetes     │
│  Scheduler      │ ◄─── Schedules to matching nodes
└─────────────────┘
```

**Key Components:**

1. **Mutating Admission Webhook**: Intercepts pod creation requests and applies placement strategies
2. **State Manager**: Tracks current pod distribution using ConfigMaps (atomic updates)
3. **Rebalance Controller**: Monitors placement drift and corrects deviations
4. **Strategy Parser**: Parses annotation-based placement rules

### The Magic: Annotation-Based Placement

Instead of complex CRDs or configuration files, you define placement strategies directly in your Deployment annotations:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web-app
  annotations:
    smart-scheduler.io/schedule-strategy: "base=2,weight=1,nodeSelector=node-type:ondemand;weight=3,nodeSelector=node-type:spot"
spec:
  replicas: 10
  # ... rest of deployment
```

**What This Does:**
- `base=2`: Guarantees first 2 pods go to on-demand nodes
- `weight=1`: On-demand nodes get weight 1
- `weight=3`: Spot nodes get weight 3
- Remaining 8 pods distributed 1:3 ratio = 2 on-demand + 6 spot
- **Final distribution**: 4 on-demand, 6 spot

## Hello World: Your First Smart Scheduler Deployment

Let's walk through a complete example from installation to deployment.

### Step 1: Install Smart Scheduler

```bash
# Install cert-manager (required for webhook certificates)
helm repo add jetstack https://charts.jetstack.io
helm install cert-manager jetstack/cert-manager \
  --namespace cert-manager --create-namespace \
  --set installCRDs=true --version 1.18.2

# Install Smart Scheduler
git clone https://github.com/kube-smartscheduler/smart-scheduler.git
cd smart-scheduler

helm install smart-scheduler ./helm/smart-scheduler \
  --namespace smart-scheduler-system --create-namespace \
  --set cert-manager.enabled=false \
  --set certificates.certManager.enabled=true
```

### Step 2: Label Your Nodes

```bash
# Label nodes for placement
kubectl label node worker-1 node-type=ondemand
kubectl label node worker-2 node-type=spot
kubectl label node worker-3 node-type=spot
```

### Step 3: Deploy with Placement Strategy

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hello-smart-scheduler
  namespace: default
  annotations:
    # Place 3 pods on on-demand, rest on spot (1:2 ratio)
    smart-scheduler.io/schedule-strategy: "base=3,weight=1,nodeSelector=node-type:ondemand;weight=2,nodeSelector=node-type:spot"
spec:
  replicas: 15
  selector:
    matchLabels:
      app: hello-smart-scheduler
  template:
    metadata:
      labels:
        app: hello-smart-scheduler
    spec:
      containers:
      - name: nginx
        image: nginx:alpine
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
```

### Step 4: Verify Placement

```bash
# Check pod distribution
kubectl get pods -l app=hello-smart-scheduler -o wide

# Count pods per node type
kubectl get pods -l app=hello-smart-scheduler \
  --field-selector spec.nodeName=worker-1 --no-headers | wc -l

# Verify nodeSelector was applied
kubectl get pod <pod-name> -o jsonpath='{.spec.nodeSelector}'
# Output: {"node-type":"ondemand"} or {"node-type":"spot"}
```

**Expected Result:**
- 3 pods on `worker-1` (on-demand) - base guarantee
- 12 pods distributed 1:2 ratio = 4 on-demand + 8 spot
- **Final**: 7 on-demand, 8 spot

## Advanced Use Cases

### Multi-Zone Distribution with Cost Optimization

```yaml
annotations:
  smart-scheduler.io/schedule-strategy: "base=1,weight=2,nodeSelector=topology.kubernetes.io/zone:us-west-1a;weight=2,nodeSelector=topology.kubernetes.io/zone:us-west-1b;weight=1,nodeSelector=topology.kubernetes.io/zone:us-west-1c"
```

### GPU Workload Distribution

```yaml
annotations:
  smart-scheduler.io/schedule-strategy: "base=2,weight=3,nodeSelector=accelerator:nvidia-tesla-v100;weight=1,nodeSelector=accelerator:nvidia-tesla-k80"
```

### Specific Node Hostname Placement

```yaml
annotations:
  smart-scheduler.io/schedule-strategy: "base=3,weight=1,nodeSelector=kubernetes.io/hostname:production-node-1;weight=5,nodeSelector=kubernetes.io/hostname:production-node-2"
```

## The Pros: Why Smart Scheduler?

### ✅ Business Logic in Scheduling

- **Weighted Distribution**: Control exact ratios (e.g., 80% spot, 20% on-demand)
- **Base Guarantees**: Ensure minimum capacity on preferred nodes
- **Cost Optimization**: Maximize spot instance usage while maintaining reliability

### ✅ Simple Annotation-Based Configuration

- No complex CRDs or external configuration
- Declarative and GitOps-friendly
- Easy to understand and maintain

### ✅ Automatic Rebalancing

- Monitors placement drift
- Automatically corrects deviations
- Configurable drift thresholds

### ✅ Production Features

- Webhook-based (low latency)
- State management with conflict resolution
- Comprehensive metrics and observability
- RBAC and security best practices

## The Risks: What You Should Know

Smart Scheduler is powerful, but like any tool, it has trade-offs. Here's what you need to understand before deploying in production:

### ⚠️ Native Scheduler Bypass

**The Issue:** Smart Scheduler uses `nodeSelector` (hard filters), which bypasses Kubernetes' native scheduler scoring. This means:
- Soft preferences (affinity weights, topology spread) are ignored
- Resource-based bin-packing optimizations don't apply
- You lose nuanced scoring that the default scheduler provides

**When It Matters:** If you rely heavily on soft preferences or want the scheduler's resource optimization, Smart Scheduler may not be the right fit.

**Mitigation:** Use Smart Scheduler for explicit placement requirements, and combine with native scheduler features where soft preferences are needed.

### ⚠️ Rebalancing Can Conflict with Other Controllers

**The Issue:** Rebalancing operations delete pods, which can conflict with:
- **HPA (Horizontal Pod Autoscaler)**: Fighting over replica counts
- **PDB (Pod Disruption Budgets)**: Rebalancing must respect PDBs
- **Rollout Strategies**: Can interfere with blue-green or canary deployments
- **Stateful Workloads**: Risky for workloads that appear stateless but have state

**Mitigation:**
```yaml
# Disable rebalancing for critical workloads
metadata:
  annotations:
    smart-scheduler.io/rebalance-disabled: "true"
```

Always configure PDBs before enabling rebalancing, and test thoroughly.

### ⚠️ State Management at Scale

**The Issue:** Smart Scheduler uses ConfigMap-based state tracking. At scale (500+ deployments), you may encounter:
- ConfigMap update conflicts
- Race conditions during concurrent updates
- Thundering herd scenarios

**Mitigation:**
- Start with single replica in production
- Monitor ConfigMap update conflicts
- Consider moving to CRD-based state for high-scale scenarios

### ⚠️ Production Readiness Checklist

Before deploying in production:

- [ ] Test with 500+ deployments simultaneously
- [ ] Verify behavior during cluster scale events
- [ ] Configure namespace allowlist/denylist
- [ ] Set up comprehensive monitoring and alerting
- [ ] Decide on `failurePolicy: Fail` vs `Ignore`
- [ ] Protect critical workloads with opt-out annotations
- [ ] Test rebalancing behavior thoroughly

**Recommended Approach:**
1. Start with opt-in mode (label-based)
2. Test with non-critical workloads first
3. Monitor closely for 1-2 weeks
4. Gradually expand after validation
5. Always protect critical workloads

## Real-World Impact: Cost Savings Example

Let's say you're running 100 pods on AWS:

- **On-demand instances**: $0.10/hour per node
- **Spot instances**: $0.03/hour per node (70% savings)

**Without Smart Scheduler:**
- Default scheduler places pods randomly
- You might get 50% on-demand, 50% spot
- Cost: (50 × $0.10) + (50 × $0.03) = $6.50/hour

**With Smart Scheduler (80% spot, 20% on-demand):**
- 20 pods on-demand, 80 pods on spot
- Cost: (20 × $0.10) + (80 × $0.03) = $4.40/hour
- **Savings: $2.10/hour = $1,839/month**

For larger deployments, the savings scale proportionally.

## Getting Started

Ready to try Smart Scheduler? Here's what you need:

1. **Kubernetes cluster** (v1.24+)
2. **cert-manager** (for webhook certificates)
3. **Helm 3.0+** (for installation)

**Quick Start:**
```bash
# Clone and install
git clone https://github.com/kube-smartscheduler/smart-scheduler.git
cd smart-scheduler

# Follow installation steps from README
# Add placement strategy to your deployment
# Watch the magic happen!
```

**Resources:**
- GitHub: [kube-smartscheduler/smart-scheduler](https://github.com/kube-smartscheduler/smart-scheduler)
- Documentation: [Architecture Risks](./docs/ARCHITECTURE_RISKS.md)
- Production Guide: [Production Deployment Guide](./docs/PRODUCTION_GUIDE.md)

## Conclusion

Smart Scheduler fills a critical gap in Kubernetes scheduling: **bringing business logic to pod placement**. Whether you're optimizing costs with spot instances, ensuring multi-zone distribution, or managing node type preferences, Smart Scheduler gives you the control you need.

However, it's not a silver bullet. Understand the trade-offs:
- You gain explicit control but lose native scheduler scoring
- Rebalancing is powerful but requires careful configuration
- State management works but needs monitoring at scale

If your use case aligns with explicit placement requirements and you're willing to configure guardrails, Smart Scheduler can be a game-changer for cost optimization and workload distribution.

**Have questions or want to contribute?** Check out the [GitHub repository](https://github.com/kube-smartscheduler/smart-scheduler) and open an issue or pull request!

---

*Have you used Smart Scheduler or similar tools? Share your experiences in the comments below!*

