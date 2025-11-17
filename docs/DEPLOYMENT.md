# Smart Scheduler Deployment Guide

This guide provides comprehensive instructions for deploying the Smart Scheduler using the Helm chart, including cert-manager as a subchart dependency.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Helm Installation](#helm-installation)
- [ArgoCD Installation](#argocd-installation)
- [Configuration](#configuration)
- [Verification](#verification)
- [Troubleshooting](#troubleshooting)

## Prerequisites

Before deploying Smart Scheduler, ensure you have:

1. **Kubernetes Cluster** (v1.24+)
2. **kubectl** configured to access your cluster
3. **Helm 3.x** installed (for direct Helm installation)
4. **ArgoCD** installed (for GitOps deployment)
5. **Image Pull Secrets** configured (if using private registries)

## Quick Start

### Using Helm CLI

```bash
# Add the repository (if using a Helm repository)
helm repo add smart-scheduler https://your-helm-repo.com
helm repo update

# Install with default values
helm install smart-scheduler smart-scheduler/smart-scheduler \
  --namespace monitoring \
  --create-namespace

# Or install with custom values file
helm install smart-scheduler smart-scheduler/smart-scheduler \
  --namespace monitoring \
  --create-namespace \
  -f values.yaml
```

### Using ArgoCD

See the [ArgoCD Installation](#argocd-installation) section for detailed instructions.

## Helm Installation

### Step 1: Prepare Values File

Create a `values.yaml` file with your configuration:

```yaml
# Image configuration
image:
  registry: ghcr.io
  repository: chitender/kube-smartscheduler
  pullPolicy: IfNotPresent
  tag: "v1.0.8"

imagePullSecrets:
  - name: registry-secret

# cert-manager configuration
cert-manager:
  enabled: true
  global:
    imagePullSecrets:
      - name: registry-secret
    commonLabels:
      productname: infra
      microservicename: certmanager
      project: your-project
      env: production
      shared: 'true'
  nodeSelector:
    nodepool: infra
  tolerations:
    - key: kubernetes.azure.com/scalesetpriority
      operator: Equal
      value: spot
      effect: NoSchedule
    - key: nodepool
      operator: Equal
      value: infra
      effect: NoSchedule
  installCRDs: true
  image:
    registry: quay.io
    repository: jetstack/cert-manager-controller
    tag: v1.18.2
    pullPolicy: IfNotPresent

# Smart Scheduler configuration
webhook:
  enabled: true
  excludeNamespaces:
    - kube-system
    - kube-public
    - cert-manager
    - smart-scheduler-system
    - monitoring

certificates:
  certManager:
    enabled: true
    issuer:
      selfSigned: true

# Resource configuration
resources:
  limits:
    cpu: 500m
    memory: 128Mi
  requests:
    cpu: 10m
    memory: 64Mi

# Node placement
nodeSelector:
  nodepool: infra

tolerations:
  - key: kubernetes.azure.com/scalesetpriority
    operator: Equal
    value: spot
    effect: NoSchedule
  - key: nodepool
    operator: Equal
    value: infra
    effect: NoSchedule

# Labels
labels:
  productname: infra
  microservicename: smartscheduler
  project: your-project
  env: production
  shared: 'true'
```

### Step 2: Update Helm Dependencies

If installing from a local chart, update dependencies:

```bash
cd helm/smart-scheduler
helm dependency update
```

This will download the cert-manager chart based on the dependency defined in `Chart.yaml`.

**Note:** After updating dependencies, a `Chart.lock` file will be created. Commit this file to your repository to ensure consistent deployments.

### Step 3: Install the Chart

```bash
# Install with namespace creation
helm install smart-scheduler ./helm/smart-scheduler \
  --namespace monitoring \
  --create-namespace \
  -f values.yaml

# Or upgrade if already installed
helm upgrade smart-scheduler ./helm/smart-scheduler \
  --namespace monitoring \
  -f values.yaml
```

### Step 4: Verify Installation

```bash
# Check pods
kubectl get pods -n monitoring

# Check cert-manager pods
kubectl get pods -n cert-manager

# Check webhook configuration
kubectl get mutatingwebhookconfiguration

# Check certificates
kubectl get certificates -n monitoring
kubectl get issuers -n monitoring
```

## ArgoCD Installation

### Step 1: Prepare ArgoCD Application Manifest

Create an ArgoCD Application manifest based on your GitOps repository structure:

```yaml
{{- if eq .Values.isenabled_smartscheduler true }}
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: {{ .Values.project }}-{{ .Values.env }}-smartscheduler
  namespace: argocd
  finalizers:
    - resources-finalizer.argocd.argoproj.io
spec:
  project: {{ .Values.project }}-admin-{{ .Values.env }}
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=true
      - ApplyOutOfSyncOnly=true
  revisionHistoryLimit: 2
  destination:
    server: {{ .Values.kubernetesServer }}
    namespace: monitoring
  source:
    path: helm/smart-scheduler
    repoURL: https://github.com/chitender/kube-SmartScheduler
    targetRevision: main
    helm:
      values: |
        # cert-manager configuration
        cert-manager:
          enabled: true
          global:
            imagePullSecrets:
              - name: registry-secret
            nodeSelector:
              nodepool: infra
            commonLabels:
              productname: infra
              microservicename: certmanager
              project: {{ .Values.project }}
              env: {{ .Values.env }}
              shared: 'true'
          tolerations:
            - key: kubernetes.azure.com/scalesetpriority
              operator: Equal
              value: spot
              effect: NoSchedule
            - key: nodepool
              operator: Equal
              value: infra
              effect: NoSchedule
          installCRDs: true
          image:
            registry: quay.io
            repository: jetstack/cert-manager-controller
            tag: v1.18.2
            pullPolicy: IfNotPresent

        # Smart Scheduler configuration
        image:
          registry: ghcr.io/chitender
          repository: kube-smartscheduler
          pullPolicy: IfNotPresent
          tag: "v1.0.8"

        imagePullSecrets:
          - name: registry-secret

        webhook:
          enabled: true
          excludeNamespaces:
            - kube-system
            - kube-public
            - cert-manager
            - smart-scheduler-system
            - monitoring

        certificates:
          certManager:
            enabled: true
            issuer:
              selfSigned: true

        resources:
          limits:
            cpu: 500m
            memory: 128Mi
          requests:
            cpu: 10m
            memory: 64Mi

        nodeSelector:
          nodepool: infra

        tolerations:
          - key: kubernetes.azure.com/scalesetpriority
            operator: Equal
            value: spot
            effect: NoSchedule
          - key: nodepool
            operator: Equal
            value: infra
            effect: NoSchedule

        labels:
          productname: infra
          microservicename: smartscheduler
          project: {{ .Values.project }}
          env: {{ .Values.env }}
          shared: 'true'
{{- end }}
```

### Step 2: Apply ArgoCD Application

```bash
# Apply the ArgoCD Application manifest
kubectl apply -f argocd-application.yaml

# Or if using Helm to template the ArgoCD Application
helm template . | kubectl apply -f -
```

### Step 3: Monitor ArgoCD Sync

```bash
# Check application status
argocd app get <project>-<env>-smartscheduler

# Watch sync status
argocd app watch <project>-<env>-smartscheduler

# Or via kubectl
kubectl get application -n argocd
```

## Configuration

### Cert-Manager Configuration

The cert-manager subchart can be configured through the `cert-manager` section in `values.yaml`:

```yaml
cert-manager:
  # Enable/disable cert-manager installation
  enabled: true
  
  # Global settings
  global:
    imagePullSecrets:
      - name: registry-secret
    commonLabels:
      productname: infra
      env: production
  
  # Node placement
  nodeSelector:
    nodepool: infra
  
  tolerations:
    - key: kubernetes.azure.com/scalesetpriority
      operator: Equal
      value: spot
      effect: NoSchedule
  
  # Install CRDs
  installCRDs: true
  
  # Image configuration
  image:
    registry: quay.io
    repository: jetstack/cert-manager-controller
    tag: v1.18.2
    pullPolicy: IfNotPresent
```

**Note:** If cert-manager is already installed in your cluster, set `cert-manager.enabled: false` to avoid conflicts.

### Smart Scheduler Configuration

Key configuration sections:

#### Image Configuration

```yaml
image:
  registry: ghcr.io
  repository: chitender/kube-smartscheduler
  tag: "v1.0.8"
  pullPolicy: IfNotPresent

imagePullSecrets:
  - name: registry-secret
```

#### Webhook Configuration

```yaml
webhook:
  enabled: true
  port: 9443
  failurePolicy: Fail
  excludeNamespaces:
    - kube-system
    - kube-public
    - cert-manager
```

#### Certificate Management

```yaml
certificates:
  certManager:
    enabled: true
    issuer:
      selfSigned: true
      # Or use existing issuer:
      # existing: letsencrypt-prod
```

#### Resource Limits

```yaml
resources:
  limits:
    cpu: 500m
    memory: 128Mi
  requests:
    cpu: 10m
    memory: 64Mi
```

#### Node Placement

```yaml
nodeSelector:
  nodepool: infra

tolerations:
  - key: kubernetes.azure.com/scalesetpriority
    operator: Equal
    value: spot
    effect: NoSchedule
```

## Verification

### 1. Check Pod Status

```bash
# Smart Scheduler pods
kubectl get pods -n monitoring -l app.kubernetes.io/name=smart-scheduler

# cert-manager pods
kubectl get pods -n cert-manager
```

### 2. Check Webhook Configuration

```bash
# Verify mutating webhook is created
kubectl get mutatingwebhookconfiguration | grep smart-scheduler

# Check webhook details
kubectl get mutatingwebhookconfiguration smart-scheduler-mutating-webhook-configuration -o yaml
```

### 3. Check Certificates

```bash
# Check certificate resources
kubectl get certificates -n monitoring
kubectl get issuers -n monitoring

# Check certificate secret
kubectl get secret smart-scheduler-webhook-server-cert -n monitoring

# Verify certificate details
kubectl get certificate smart-scheduler-serving-cert -n monitoring -o yaml
```

### 4. Test Pod Interception

Create a test deployment with Smart Scheduler annotation:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
  namespace: default
  annotations:
    smart-scheduler.io/schedule-strategy: "base=1,weight=1,nodeSelector=zone:us-west-1a"
spec:
  replicas: 1
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
        image: nginx:1.21
```

```bash
# Apply test deployment
kubectl apply -f test-deployment.yaml

# Check if pod was intercepted
kubectl get pod -l app=test-app -o yaml | grep nodeSelector

# Check Smart Scheduler logs
kubectl logs -n monitoring -l app.kubernetes.io/name=smart-scheduler --tail=50
```

### 5. Check Metrics

```bash
# Port-forward to metrics endpoint
kubectl port-forward -n monitoring svc/smart-scheduler-metrics 8080:8080

# Query metrics
curl http://localhost:8080/metrics | grep smart_scheduler
```

## Troubleshooting

### Cert-Manager Not Installing

**Issue:** cert-manager pods are not starting

**Solutions:**
1. Check if CRDs are installed:
   ```bash
   kubectl get crds | grep cert-manager
   ```

2. If CRDs are missing and `installCRDs: false`, install manually:
   ```bash
   kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.18.2/cert-manager.crds.yaml
   ```

3. Check cert-manager pod logs:
   ```bash
   kubectl logs -n cert-manager -l app.kubernetes.io/name=cert-manager
   ```

### Webhook Certificate Issues

**Issue:** Webhook certificates not being generated

**Solutions:**
1. Check certificate status:
   ```bash
   kubectl describe certificate -n monitoring smart-scheduler-serving-cert
   ```

2. Check issuer status:
   ```bash
   kubectl describe issuer -n monitoring smart-scheduler-selfsigned-issuer
   ```

3. Verify cert-manager is running:
   ```bash
   kubectl get pods -n cert-manager
   ```

### Webhook Not Intercepting Pods

**Issue:** Pods are not being intercepted by the webhook

**Solutions:**
1. Verify webhook configuration:
   ```bash
   kubectl get mutatingwebhookconfiguration smart-scheduler-mutating-webhook-configuration -o yaml
   ```

2. Check webhook service:
   ```bash
   kubectl get svc -n monitoring | grep webhook
   ```

3. Check Smart Scheduler logs:
   ```bash
   kubectl logs -n monitoring -l app.kubernetes.io/name=smart-scheduler
   ```

4. Verify namespace is not excluded:
   ```bash
   kubectl get mutatingwebhookconfiguration smart-scheduler-mutating-webhook-configuration -o jsonpath='{.webhooks[0].namespaceSelector}'
   ```

### Image Pull Errors

**Issue:** Pods cannot pull images

**Solutions:**
1. Verify image pull secrets:
   ```bash
   kubectl get secret registry-secret -n monitoring
   ```

2. Check image pull secret is referenced:
   ```bash
   kubectl get deployment smart-scheduler -n monitoring -o yaml | grep imagePullSecrets
   ```

3. For cert-manager, ensure image pull secrets are configured:
   ```yaml
   cert-manager:
     global:
       imagePullSecrets:
         - name: registry-secret
   ```

### ArgoCD Sync Issues

**Issue:** ArgoCD application is out of sync

**Solutions:**
1. Check application status:
   ```bash
   argocd app get <app-name>
   ```

2. Force refresh:
   ```bash
   argocd app get <app-name> --refresh
   ```

3. Check Helm dependency issues:
   ```bash
   # In your chart directory
   helm dependency update
   helm dependency build
   ```

4. Verify chart structure in Git repository matches local structure

## Additional Resources

- [Smart Scheduler README](../README.md)
- [Versioning Guide](VERSIONING.md)
- [Build and Release Guide](BUILD_AND_RELEASE.md)
- [Cert-Manager Documentation](https://cert-manager.io/docs/)

## Support

For issues and questions:
- GitHub Issues: https://github.com/kube-smartscheduler/smart-scheduler/issues
- Documentation: https://github.com/kube-smartscheduler/smart-scheduler/docs

