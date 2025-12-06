# Architecture Risks and Production Considerations

This document outlines known architectural risks, limitations, and production considerations for Smart Scheduler. Understanding these is critical before deploying in production environments.

## ⚠️ Critical Architectural Considerations

### 1. Native Scheduler Bypass

**Risk:** Smart Scheduler uses `nodeSelector` mutations, which are hard filters that bypass Kubernetes native scheduler scoring.

**Impact:**
- Overrides soft preferences (affinity weights, topology spread constraints)
- Ignores resource-based scoring and bin-packing optimizations
- May conflict with other scheduling policies

**Mitigation:**
- Use Smart Scheduler for explicit placement requirements only
- Combine with native scheduler features where soft preferences are needed
- Consider using `preferredDuringScheduling` affinity rules for non-critical workloads
- Document scheduling intent clearly in deployment annotations

**When to Use:**
- ✅ When you need guaranteed placement (e.g., specific node types, zones)
- ✅ When cost optimization requires explicit node selection
- ❌ When you need nuanced scoring and soft preferences

### 2. Rebalancing Risks

**Risk:** Rebalancing operations can delete pods or update deployments, potentially conflicting with other controllers.

**Potential Conflicts:**
- **HPA (Horizontal Pod Autoscaler)**: Rebalancing may fight with HPA scaling decisions
- **PDB (Pod Disruption Budgets)**: Rebalancing deletions must respect PDB constraints
- **Rollout Strategies**: Can interfere with blue-green or canary deployments
- **Stateful Workloads**: Risky for workloads that appear stateless but have state

**Mitigation Strategies:**

```yaml
# Example: Opt-out annotation for critical workloads
apiVersion: apps/v1
kind: Deployment
metadata:
  annotations:
    smart-scheduler.io/rebalance-disabled: "true"
```

**Best Practices:**
- Disable rebalancing for stateful workloads
- Set appropriate PDBs before enabling rebalancing
- Use rebalancing only for truly stateless workloads
- Monitor rebalancing actions closely in production
- Consider time windows for rebalancing operations

### 3. State Management in ConfigMaps

**Risk:** Concurrent access to ConfigMap-based state tracking can lead to race conditions.

**Concerns:**
- Multiple controller replicas accessing the same ConfigMap
- Retry logic and partial failures
- Leader election must be rock solid
- Thundering herd scenarios during scale events

**Current Implementation:**
- Uses atomic ConfigMap updates with optimistic concurrency
- Leader election for controller operations
- Retry logic with exponential backoff

**Validation Needed:**
- ✅ Test with 500+ deployments simultaneously
- ✅ Verify behavior during node scale events
- ✅ Validate leader election failover scenarios
- ⚠️ Monitor for ConfigMap update conflicts

**Recommendations:**
- Start with single replica in production
- Monitor ConfigMap update conflicts
- Consider moving to CRD-based state for high-scale scenarios

### 4. Production Readiness Checklist

Before deploying in production, validate:

#### Scale Testing
- [ ] Test with 500+ deployments with placement strategies
- [ ] Verify behavior during cluster scale events
- [ ] Test concurrent annotation updates
- [ ] Validate webhook performance under load

#### Guardrails
- [ ] Configure namespace allowlist/denylist
- [ ] Set opt-in labels for selective application
- [ ] Configure max deletion rate for rebalancing
- [ ] Mark critical workloads as "do not touch"

#### Failure Policy
- [ ] Decide on `failurePolicy: Fail` vs `Ignore`
- [ ] Set up alerts for webhook failures
- [ ] Monitor admission latency
- [ ] Track error budgets

#### Observability
- [ ] Set up metrics dashboards
- [ ] Configure alerts for:
  - High admission latency
  - Rebalance failures
  - ConfigMap conflicts
  - Webhook errors
- [ ] Enable audit logging

## 🛡️ Guardrails and Safety Features

### Namespace Filtering

```yaml
# In values.yaml
webhook:
  excludeNamespaces:
    - kube-system
    - kube-public
    - cert-manager
    - critical-production
```

### Opt-in Labels

Only apply Smart Scheduler to deployments with specific labels:

```yaml
# Deployment must have this label
metadata:
  labels:
    smart-scheduler.io/enabled: "true"
```

### Rebalance Rate Limiting

```yaml
# In deployment annotation
metadata:
  annotations:
    smart-scheduler.io/rebalance-max-pods: "2"
    smart-scheduler.io/rebalance-window: "02:00-04:00"
```

### Critical Workload Protection

```yaml
# Opt-out for critical workloads
metadata:
  annotations:
    smart-scheduler.io/rebalance-disabled: "true"
    smart-scheduler.io/placement-disabled: "true"
```

## 📊 Observability Requirements

### Required Metrics

1. **Admission Latency**
   - Histogram: `smart_scheduler_webhook_duration_seconds`
   - P50, P95, P99 percentiles
   - Alert if P95 > 100ms

2. **Placement Decisions**
   - Counter: `smart_scheduler_placement_decisions_total`
   - By strategy, by namespace, by result

3. **Rebalance Actions**
   - Counter: `smart_scheduler_rebalance_actions_total`
   - By reason, by namespace
   - Gauge: `smart_scheduler_rebalance_pods_deleted_total`

4. **Error Tracking**
   - Counter: `smart_scheduler_errors_total`
   - By error type, by component
   - Error budget: 99.9% success rate

### Recommended Alerts

```yaml
# Example Prometheus alerts
- alert: SmartSchedulerHighLatency
  expr: histogram_quantile(0.95, smart_scheduler_webhook_duration_seconds) > 0.1
  annotations:
    summary: "Webhook latency exceeds 100ms"

- alert: SmartSchedulerHighErrorRate
  expr: rate(smart_scheduler_errors_total[5m]) > 0.01
  annotations:
    summary: "Error rate exceeds 1%"

- alert: SmartSchedulerRebalanceFailures
  expr: rate(smart_scheduler_rebalance_actions_total{result="failed"}[5m]) > 0
  annotations:
    summary: "Rebalancing operations failing"
```

## 🔄 Failure Policy Recommendations

### Development/Testing
```yaml
webhook:
  failurePolicy: Ignore
  # Allows pods to be created even if webhook fails
  # Good for testing, but monitor failures
```

### Production
```yaml
webhook:
  failurePolicy: Fail
  # Strict enforcement - pods fail if webhook fails
  # Requires robust webhook infrastructure
  # Must have proper monitoring and alerting
```

**Hybrid Approach (Recommended):**
- Use `Ignore` initially with comprehensive monitoring
- Switch to `Fail` once confidence is established
- Always have alerting for webhook failures

## 🚀 Future Enhancements

### Two-Tier Model
- **Tier 1**: Webhook-based (current) - lightweight, simple
- **Tier 2**: Scheduler Framework Plugin - deep integration, preserves native scoring

### Policy Conflict Resolution
- Document selector overlap scenarios
- Define tie-break rules clearly
- Add observability for "which policy won"

### Rebalance Safety Enhancements
- Rate limiting per namespace
- PDB-aware deletion logic
- Rollout window constraints
- "Simulate-only" mode for testing

### E2E Test Matrix
- Kind-based integration tests
- Multi-AZ scenarios
- Spot interruption handling
- HPA interaction tests
- Cluster Autoscaler/Karpenter integration

## 📚 Additional Resources

- [Kubernetes Scheduler Framework](https://kubernetes.io/docs/concepts/scheduling-eviction/scheduling-framework/)
- [Pod Disruption Budgets](https://kubernetes.io/docs/concepts/workloads/pods/disruptions/)
- [Horizontal Pod Autoscaling](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/)

## ⚠️ Known Limitations

1. **No Scheduler Plugin**: Currently webhook-only, doesn't integrate with native scheduler scoring
2. **ConfigMap State**: May not scale to 1000+ deployments without optimization
3. **Rebalancing**: Can conflict with other controllers if not carefully configured
4. **Hard Filters Only**: Uses nodeSelector (hard filter), not soft preferences

## 🤝 Contributing Improvements

If you encounter issues or have suggestions for addressing these risks, please:
1. Open an issue with detailed description
2. Propose solutions with code examples
3. Contribute test cases for edge scenarios
4. Share production experiences and lessons learned

