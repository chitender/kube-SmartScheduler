# Production Deployment Guide

This guide provides step-by-step instructions for deploying Smart Scheduler in production environments with appropriate guardrails and safety measures.

## Pre-Deployment Checklist

### 1. Understand the Risks

Read [ARCHITECTURE_RISKS.md](./ARCHITECTURE_RISKS.md) to understand:
- Native scheduler bypass implications
- Rebalancing risks and conflicts
- State management considerations
- Production readiness requirements

### 2. Validate Your Use Case

Smart Scheduler is suitable for:
- ✅ Explicit node placement requirements
- ✅ Cost optimization with specific node types
- ✅ Multi-zone distribution with guarantees
- ✅ On-demand vs spot instance distribution

Not suitable for:
- ❌ Workloads requiring nuanced scheduler scoring
- ❌ Stateful workloads (unless rebalancing disabled)
- ❌ Critical workloads without opt-out mechanisms
- ❌ Environments without proper monitoring

### 3. Prepare Your Cluster

```bash
# Verify Kubernetes version (v1.24+)
kubectl version --short

# Check available nodes and labels
kubectl get nodes --show-labels

# Verify cert-manager is installed
kubectl get pods -n cert-manager
```

## Deployment Configuration

### Step 1: Create Production Values File

```yaml
# production-values.yaml
image:
  registry: ghcr.io
  repository: chitender/kube-smartscheduler
  tag: "1.0.7"  # Use specific version, not latest

# Disable cert-manager if already installed
cert-manager:
  enabled: false

certificates:
  certManager:
    enabled: true
    issuer:
      selfSigned: true

# Webhook configuration
webhook:
  enabled: true
  failurePolicy: Ignore  # Start with Ignore, move to Fail after validation
  excludeNamespaces:
    - kube-system
    - kube-public
    - cert-manager
    - critical-production
    - monitoring

# Resource limits
resources:
  limits:
    cpu: 500m
    memory: 256Mi
  requests:
    cpu: 50m
    memory: 128Mi

# Replica count (start with 1 for state management)
replicaCount: 1

# Node placement
nodeSelector:
  node-role.kubernetes.io/control-plane: ""

tolerations:
  - key: node-role.kubernetes.io/control-plane
    operator: Exists
    effect: NoSchedule
```

### Step 2: Deploy with Guardrails

```bash
# Create namespace
kubectl create namespace smart-scheduler-system

# Install with production values
helm install smart-scheduler ./helm/smart-scheduler \
  --namespace smart-scheduler-system \
  --values production-values.yaml \
  --wait --timeout 10m

# Verify deployment
kubectl get pods -n smart-scheduler-system
kubectl get mutatingwebhookconfiguration | grep smart-scheduler
```

### Step 3: Configure Monitoring

```bash
# Check if metrics are exposed
kubectl port-forward -n smart-scheduler-system \
  deployment/smart-scheduler 8080:8080

# In another terminal, check metrics
curl http://localhost:8080/metrics | grep smart_scheduler
```

### Step 4: Set Up Alerts

Create Prometheus alerts for:
- Webhook latency > 100ms
- Error rate > 1%
- Rebalance failures
- ConfigMap update conflicts

## Gradual Rollout Strategy

### Phase 1: Opt-in Testing (Week 1-2)

1. **Enable opt-in mode** (requires label):
   ```yaml
   # Only deployments with this label get Smart Scheduler
   metadata:
     labels:
       smart-scheduler.io/enabled: "true"
   ```

2. **Select 2-3 non-critical workloads**
3. **Monitor for 1 week**
4. **Validate placement and rebalancing behavior**

### Phase 2: Expanded Testing (Week 3-4)

1. **Add 10-20 more workloads**
2. **Test different placement strategies**
3. **Monitor ConfigMap state management**
4. **Validate under load**

### Phase 3: Production Rollout (Week 5+)

1. **Remove opt-in requirement** (if desired)
2. **Enable for all suitable workloads**
3. **Keep critical workloads opted out**
4. **Monitor continuously**

## Guardrails Configuration

### 1. Namespace Allowlist/Denylist

```yaml
# In values.yaml
webhook:
  excludeNamespaces:
    - kube-system
    - critical-production
    - payment-processing
```

### 2. Opt-in Labels

Modify webhook to only process deployments with specific labels:

```yaml
# Deployment must have this label
metadata:
  labels:
    smart-scheduler.io/enabled: "true"
```

### 3. Critical Workload Protection

```yaml
# For critical workloads, add opt-out
apiVersion: apps/v1
kind: Deployment
metadata:
  name: critical-app
  annotations:
    smart-scheduler.io/rebalance-disabled: "true"
    smart-scheduler.io/placement-disabled: "true"
```

### 4. Rebalance Rate Limiting

```yaml
# In deployment annotation
metadata:
  annotations:
    smart-scheduler.io/schedule-strategy: "..."
    smart-scheduler.io/rebalance-max-pods: "2"  # Max pods deleted per rebalance
    smart-scheduler.io/rebalance-window: "02:00-04:00"  # Only rebalance during this window
```

## Monitoring and Observability

### Required Metrics

1. **Admission Latency**
   ```promql
   histogram_quantile(0.95, 
     rate(smart_scheduler_webhook_duration_seconds_bucket[5m])
   )
   ```

2. **Placement Decisions**
   ```promql
   rate(smart_scheduler_placement_decisions_total[5m])
   ```

3. **Rebalance Actions**
   ```promql
   rate(smart_scheduler_rebalance_actions_total[5m])
   ```

4. **Error Rate**
   ```promql
   rate(smart_scheduler_errors_total[5m])
   ```

### Recommended Dashboards

Create Grafana dashboards for:
- Webhook performance (latency, throughput)
- Placement decisions by strategy
- Rebalance operations and success rate
- Error rates and types
- ConfigMap state management

### Alerting Rules

```yaml
groups:
  - name: smart_scheduler
    rules:
      - alert: HighWebhookLatency
        expr: histogram_quantile(0.95, rate(smart_scheduler_webhook_duration_seconds_bucket[5m])) > 0.1
        annotations:
          summary: "Webhook latency exceeds 100ms"
      
      - alert: HighErrorRate
        expr: rate(smart_scheduler_errors_total[5m]) > 0.01
        annotations:
          summary: "Error rate exceeds 1%"
      
      - alert: RebalanceFailures
        expr: rate(smart_scheduler_rebalance_actions_total{result="failed"}[5m]) > 0
        annotations:
          summary: "Rebalancing operations failing"
```

## Handling Conflicts

### HPA Conflicts

If using HPA, disable rebalancing or coordinate carefully:

```yaml
metadata:
  annotations:
    smart-scheduler.io/rebalance-disabled: "true"
```

### PDB Conflicts

Ensure PDBs are configured before enabling rebalancing:

```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: app-pdb
spec:
  minAvailable: 2
  selector:
    matchLabels:
      app: my-app
```

### Rollout Strategy Conflicts

For blue-green or canary deployments, disable rebalancing:

```yaml
metadata:
  annotations:
    smart-scheduler.io/rebalance-disabled: "true"
```

## Troubleshooting

### Webhook Not Working

```bash
# Check webhook configuration
kubectl get mutatingwebhookconfiguration

# Check certificate status
kubectl get certificate -n smart-scheduler-system

# Check webhook logs
kubectl logs -n smart-scheduler-system -l app.kubernetes.io/name=smart-scheduler
```

### Placement Not Applied

```bash
# Verify annotation format
kubectl get deployment <name> -o jsonpath='{.metadata.annotations.smart-scheduler\.io/schedule-strategy}'

# Check if namespace is excluded
kubectl get mutatingwebhookconfiguration -o yaml | grep -A 10 excludeNamespaces

# Check operator logs
kubectl logs -n smart-scheduler-system -l app.kubernetes.io/name=smart-scheduler | grep placement
```

### Rebalancing Issues

```bash
# Check rebalance controller logs
kubectl logs -n smart-scheduler-system -l app.kubernetes.io/name=smart-scheduler | grep rebalance

# Verify PDBs are configured
kubectl get pdb

# Check rebalance metrics
kubectl port-forward -n smart-scheduler-system deployment/smart-scheduler 8080:8080
curl http://localhost:8080/metrics | grep rebalance
```

## Rollback Plan

If issues occur:

```bash
# 1. Disable webhook (set failurePolicy: Ignore if not already)
# 2. Scale down controller
kubectl scale deployment smart-scheduler -n smart-scheduler-system --replicas=0

# 3. Remove problematic annotations from deployments
kubectl annotate deployment <name> smart-scheduler.io/schedule-strategy-

# 4. Uninstall if necessary
helm uninstall smart-scheduler -n smart-scheduler-system
```

## Best Practices

1. **Start Small**: Begin with opt-in mode and few workloads
2. **Monitor Closely**: Set up comprehensive monitoring before full rollout
3. **Use Guardrails**: Configure namespace exclusions and opt-in labels
4. **Protect Critical Workloads**: Always opt-out critical workloads
5. **Test Rebalancing**: Validate rebalancing behavior before enabling
6. **Document Decisions**: Keep track of which workloads use Smart Scheduler
7. **Regular Reviews**: Review placement strategies and rebalancing behavior regularly

## Support

For issues or questions:
- Open an issue on GitHub
- Check [ARCHITECTURE_RISKS.md](./ARCHITECTURE_RISKS.md) for known limitations
- Review logs and metrics for troubleshooting

