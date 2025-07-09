# Semantic Versioning Guide

Smart Scheduler supports comprehensive semantic versioning for Docker images and releases. This guide explains how to use the versioning features.

## Overview

The project uses semantic versioning (SemVer) format: `vMAJOR.MINOR.PATCH[-prerelease][+buildmeta]`

### Version Sources
1. **Git Tags**: Primary source for version information
2. **Manual Override**: Using `VERSION` environment variable
3. **Default**: `v0.1.0` for new repositories

## Quick Start

### 1. Create Your First Release

```bash
# Create a semantic version tag
git tag v1.0.0
git push origin v1.0.0

# Build and push images with all tags
make release
```

### 2. Using the Release Script (Recommended)

```bash
# Interactive release process
./scripts/release.sh
```

This script will:
- Show current version
- Suggest next version (patch/minor/major)
- Create git tag
- Run pre-release checks
- Build and push multi-architecture images
- Push tag to remote

## Manual Versioning

### Check Current Version

```bash
# Show version information
make version

# Example output:
# Version: v1.2.3
# Commit Hash: abc1234
# Build Date: 2024-01-15T10:30:00Z
# Major: 1
# Minor: 2  
# Patch: 3
#
# Image Tags:
#   Main: docker.io/smart-scheduler:v1.2.3
#   Latest: docker.io/smart-scheduler:latest
#   Major: docker.io/smart-scheduler:v1
#   Minor: docker.io/smart-scheduler:v1.2
#   Commit: docker.io/smart-scheduler:abc1234
```

### Build with Specific Version

```bash
# Override version
make docker-build VERSION=v1.2.3

# Use custom registry
make docker-build REGISTRY=myregistry.com REPOSITORY=my-smart-scheduler VERSION=v1.2.3
```

## Image Tags Generated

For version `v1.2.3`, the following tags are created:

| Tag | Purpose | Example |
|-----|---------|---------|
| `v1.2.3` | Exact version | `smart-scheduler:v1.2.3` |
| `latest` | Latest release | `smart-scheduler:latest` |
| `v1` | Major version | `smart-scheduler:v1` |
| `v1.2` | Minor version | `smart-scheduler:v1.2` |
| `abc1234` | Commit hash | `smart-scheduler:abc1234` |

## Makefile Targets

### Building

```bash
# Build with all tags
make docker-build

# Build single tag only
make docker-build-single

# Multi-architecture build
make docker-buildx

# Multi-architecture build and push
make docker-buildx-push
```

### Pushing

```bash
# Push all tags
make docker-push

# Push only version tag
make docker-push-version

# Build and push together
make docker-build-and-push
```

### Release Management

```bash
# Complete release process
make release

# Local release (no push)
make release-local

# Pre-release checks
make pre-release

# Create git tag interactively
make tag
```

## Version Information in Binary

The built binary includes version information accessible via:

```bash
# Show version and exit
./manager --version

# Version is also logged on startup
./manager
# Output: Starting Smart Scheduler Manager version=v1.2.3 commit=abc1234 ...
```

## Configuration Options

### Environment Variables

```bash
# Override version
export VERSION=v1.2.3

# Custom registry
export REGISTRY=myregistry.com

# Custom repository name
export REPOSITORY=my-smart-scheduler

# Build with custom values
make docker-build
```

### Build Arguments

The Dockerfile accepts these build arguments:

- `VERSION`: Version string
- `COMMIT_HASH`: Git commit hash
- `BUILD_DATE`: Build timestamp

## Examples

### Development Workflow

```bash
# 1. Make changes and commit
git add .
git commit -m "Add new feature"

# 2. Create version tag
git tag v1.1.0

# 3. Build and test locally
make docker-build VERSION=v1.1.0

# 4. Run tests
make pre-release VERSION=v1.1.0

# 5. Release
make release VERSION=v1.1.0

# 6. Push tag
git push origin v1.1.0
```

### CI/CD Integration

```yaml
# GitHub Actions example
- name: Build and push
  run: |
    export VERSION=${GITHUB_REF#refs/tags/}
    make docker-buildx-push
  if: startsWith(github.ref, 'refs/tags/v')
```

### Custom Registry Deployment

```bash
# Deploy to private registry
make docker-build-and-push \
  REGISTRY=myregistry.com \
  REPOSITORY=internal/smart-scheduler \
  VERSION=v1.0.0
```

## Version Strategy

### When to Bump Versions

- **Patch** (`v1.0.1`): Bug fixes, security patches
- **Minor** (`v1.1.0`): New features, backward compatible
- **Major** (`v2.0.0`): Breaking changes, API changes

### Pre-release Versions

```bash
# Beta releases
git tag v2.0.0-beta.1
make release VERSION=v2.0.0-beta.1

# Release candidates
git tag v2.0.0-rc.1
make release VERSION=v2.0.0-rc.1
```

## Troubleshooting

### Common Issues

1. **No git tags found**: Creates default `v0.1.0`
2. **Dirty working directory**: Appends `-dirty` to version
3. **Invalid semver format**: Validation fails in release script

### Debug Version Detection

```bash
# Check git describe output
git describe --tags --always --dirty

# Test version parsing
make version

# Verify image tags
docker images | grep smart-scheduler
```

## Best Practices

1. **Always tag releases**: Don't rely on commit hashes
2. **Follow semantic versioning**: Be consistent with version bumps
3. **Test before release**: Use `make pre-release`
4. **Use release script**: Reduces manual errors
5. **Document breaking changes**: For major version bumps
6. **Maintain changelog**: Track version changes

## Integration with Helm

Update Helm chart version to match:

```yaml
# helm/smart-scheduler/Chart.yaml
version: 1.2.3
appVersion: v1.2.3
```

```bash
# Deploy specific version
helm upgrade smart-scheduler ./helm/smart-scheduler \
  --set image.tag=v1.2.3
``` 