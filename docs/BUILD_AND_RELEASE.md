# Build and Release Guide

This guide explains how to build and release Smart Scheduler with proper semantic versioning to get images like `ghcr.io/chitender/kube-smartscheduler:v1.0.2` instead of SHA256 digests.

## 🚀 Quick Start

### Option 1: Automatic Release (Recommended)

1. **Create and push a semantic version tag:**
```bash
git tag v1.0.2
git push origin v1.0.2
```

2. **GitHub Actions automatically builds and pushes:**
   - `ghcr.io/chitender/kube-smartscheduler:v1.0.2`
   - `ghcr.io/chitender/kube-smartscheduler:latest`
   - `ghcr.io/chitender/kube-smartscheduler:v1`
   - `ghcr.io/chitender/kube-smartscheduler:v1.0`
   - `ghcr.io/chitender/kube-smartscheduler:abc1234`

3. **Pull the versioned image:**
```bash
docker pull ghcr.io/chitender/kube-smartscheduler:v1.0.2
```

### Option 2: Manual Release

1. **Configure your environment:**
```bash
source config.env
```

2. **Authenticate with GitHub Container Registry:**
```bash
echo $GITHUB_TOKEN | docker login ghcr.io -u chitender --password-stdin
```

3. **Build and push:**
```bash
git tag v1.0.2
make docker-build-and-push
```

## 📋 Prerequisites

### GitHub Token Setup

1. Go to [GitHub Settings > Developer settings > Personal access tokens](https://github.com/settings/tokens)
2. Create a new token with `write:packages` permission
3. Save the token securely

### Local Development Setup

```bash
# 1. Clone the repository
git clone https://github.com/chitender/kube-SmartScheduler.git
cd kube-SmartScheduler

# 2. Load configuration
source config.env

# 3. Set your GitHub token (if building locally)
export GITHUB_TOKEN=ghp_your_token_here
```

## 🏗️ Building Images

### Check Current Configuration

```bash
make version
```

Output will show:
```
Version: v1.0.2
Image Tags:
  Main: ghcr.io/chitender/kube-smartscheduler:v1.0.2
  Latest: ghcr.io/chitender/kube-smartscheduler:latest
  Major: ghcr.io/chitender/kube-smartscheduler:v1
  Minor: ghcr.io/chitender/kube-smartscheduler:v1.0
  Commit: ghcr.io/chitender/kube-smartscheduler:abc1234
```

### Build Locally

```bash
# Build all tags
make docker-build

# Build specific version
make docker-build VERSION=v1.0.2

# Build and push
make docker-build-and-push
```

### Build Multi-Architecture

```bash
# Setup buildx (one time)
make docker-buildx-setup

# Build for AMD64 and ARM64
make docker-buildx-push
```

## 🚢 Release Process

### Interactive Release Script

```bash
./scripts/release.sh
```

This script will:
1. Show current version
2. Suggest next version (patch/minor/major)
3. Create git tag
4. Run pre-release checks
5. Build and push multi-architecture images
6. Push tag to remote

### Manual Release Steps

```bash
# 1. Create semantic version tag
git tag v1.0.2

# 2. Check version info
make version

# 3. Run pre-release checks
make pre-release

# 4. Build and push
make release

# 5. Push tag to GitHub
git push origin v1.0.2
```

## 🤖 GitHub Actions Workflow

The repository includes a GitHub Actions workflow (`.github/workflows/release.yml`) that:

### Triggers
- **Automatic**: When you push a tag starting with `v` (e.g., `v1.0.2`)
- **Manual**: Through GitHub Actions UI with custom version

### What it does
1. Builds multi-architecture images (AMD64 + ARM64)
2. Pushes to `ghcr.io/chitender/kube-smartscheduler` with multiple tags
3. Generates release summary with pull commands
4. Uses GitHub Container Registry authentication automatically

### Manual Trigger

1. Go to [GitHub Actions](https://github.com/chitender/kube-SmartScheduler/actions)
2. Select "Release" workflow
3. Click "Run workflow"
4. Enter version (e.g., `v1.0.2`)

## 📦 Using Released Images

### Pull Specific Version

```bash
# Exact version
docker pull ghcr.io/chitender/kube-smartscheduler:v1.0.2

# Latest release
docker pull ghcr.io/chitender/kube-smartscheduler:latest

# Major version (gets latest v1.x.x)
docker pull ghcr.io/chitender/kube-smartscheduler:v1

# Minor version (gets latest v1.0.x)
docker pull ghcr.io/chitender/kube-smartscheduler:v1.0
```

### Update Kubernetes Deployment

```bash
# Update to specific version
kubectl set image deployment/smart-scheduler-controller-manager \
  manager=ghcr.io/chitender/kube-smartscheduler:v1.0.2 \
  -n smart-scheduler-system

# Using Helm
helm upgrade smart-scheduler ./helm/smart-scheduler \
  --set image.registry=ghcr.io \
  --set image.repository=chitender/kube-smartscheduler \
  --set image.tag=v1.0.2
```

### Docker Compose

```yaml
services:
  smart-scheduler:
    image: ghcr.io/chitender/kube-smartscheduler:v1.0.2
    # ... other configuration
```

## 🔧 Configuration Options

### Environment Variables

```bash
# Registry configuration
export REGISTRY=ghcr.io
export REPOSITORY=chitender/kube-smartscheduler

# Version override
export VERSION=v1.0.2

# GitHub authentication
export GITHUB_TOKEN=ghp_your_token_here
```

### Custom Registry

```bash
# Use different registry
make docker-build-and-push \
  REGISTRY=your-registry.com \
  REPOSITORY=your-org/smart-scheduler \
  VERSION=v1.0.2
```

## 🐛 Troubleshooting

### Issue: Getting SHA256 instead of tags

**Problem**: Images show as `ghcr.io/chitender/kube-smartscheduler@sha256:...`

**Solution**: This happens when:
1. You're pulling by digest instead of tag
2. Images weren't pushed with proper tags

**Fix**:
```bash
# Check available tags
docker pull ghcr.io/chitender/kube-smartscheduler:v1.0.2

# List all tags (if you have access)
curl -H "Authorization: Bearer $GITHUB_TOKEN" \
  https://api.github.com/users/chitender/packages/container/kube-smartscheduler/versions
```

### Issue: Authentication Failed

**Problem**: `denied: permission_denied`

**Solution**:
```bash
# Login to GitHub Container Registry
echo $GITHUB_TOKEN | docker login ghcr.io -u chitender --password-stdin

# Verify token has packages:write permission
```

### Issue: Multi-arch build fails

**Problem**: Platform errors during build

**Solution**:
```bash
# Install buildx
docker buildx install

# Setup builder
make docker-buildx-setup

# Try again
make docker-buildx-push
```

### Issue: Version detection problems

**Problem**: Wrong version detected

**Solution**:
```bash
# Check git describe output
git describe --tags --always --dirty

# Create proper semantic version tag
git tag v1.0.2
git push origin v1.0.2

# Override version manually
VERSION=v1.0.2 make docker-build
```

## 📈 Best Practices

1. **Always use semantic versioning**: `vMAJOR.MINOR.PATCH`
2. **Test before releasing**: Use `make pre-release`
3. **Use GitHub Actions**: Automatic builds are more reliable
4. **Tag commits properly**: Don't rely on commit hashes
5. **Document breaking changes**: For major version bumps

## 🎯 Examples

### Development Workflow

```bash
# 1. Make changes
git add .
git commit -m "Add new feature"

# 2. Test locally
make docker-build VERSION=v1.1.0-dev
make pre-release VERSION=v1.1.0-dev

# 3. Create release
git tag v1.1.0
git push origin v1.1.0

# 4. GitHub Actions builds and pushes automatically

# 5. Verify
docker pull ghcr.io/chitender/kube-smartscheduler:v1.1.0
```

### Hotfix Release

```bash
# 1. Create hotfix branch
git checkout -b hotfix/v1.0.3

# 2. Make fix
git commit -m "Fix critical bug"

# 3. Tag and release
git tag v1.0.3
git push origin v1.0.3

# 4. Images available immediately
docker pull ghcr.io/chitender/kube-smartscheduler:v1.0.3
```

### Pre-release Testing

```bash
# 1. Create pre-release tag
git tag v2.0.0-beta.1
git push origin v2.0.0-beta.1

# 2. Test pre-release
docker pull ghcr.io/chitender/kube-smartscheduler:v2.0.0-beta.1

# 3. Final release
git tag v2.0.0
git push origin v2.0.0
``` 