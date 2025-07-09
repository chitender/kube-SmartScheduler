# GitHub Actions Configuration

This directory contains GitHub Actions workflows for building, testing, and deploying the SmartScheduler Kubernetes operator with multi-architecture support.

## Workflows

### 1. `ci.yml` - Continuous Integration
Runs on every push and pull request to validate code quality:

- **Go Testing**: Unit tests with race detection and coverage reporting
- **Linting**: Code quality checks using golangci-lint
- **Security Scanning**: Security vulnerability scanning with Gosec
- **Helm Validation**: Helm chart linting and template validation
- **Build Testing**: Multi-architecture binary builds (AMD64/ARM64)
- **Dockerfile Linting**: Docker best practices validation with Hadolint

### 2. `docker-build.yml` - Multi-Architecture Container Builds
Builds and publishes container images for multiple architectures:

- **Platforms**: `linux/amd64`, `linux/arm64`
- **Registries**: Docker Hub and GitHub Container Registry (GHCR)
- **Security**: Vulnerability scanning with Trivy
- **Testing**: Platform-specific image validation
- **Automation**: Automatic Helm chart updates on version tags

## Prerequisites

### Enable GitHub Features (Optional)
1. **Code Scanning** (for SARIF security reports):
   - Go to `Settings` → `Code security and analysis`
   - Enable "Code scanning" to receive security reports
   - If disabled, SARIF uploads will be skipped automatically

### Required Secrets

Configure these secrets in your GitHub repository settings (`Settings` → `Secrets and variables` → `Actions`):

### Docker Hub (Optional but Recommended)
```bash
DOCKERHUB_USERNAME=your-dockerhub-username
DOCKERHUB_TOKEN=your-dockerhub-access-token
```

**Setup Instructions:**
1. Go to [Docker Hub Settings](https://hub.docker.com/settings/security)
2. Create a new access token with read/write permissions
3. Add the token as `DOCKERHUB_TOKEN` secret in GitHub

### GitHub Container Registry (Automatic)
The workflow uses the built-in `GITHUB_TOKEN` which is automatically provided by GitHub Actions. No additional setup required.

### Workflow Permissions
The workflows are configured with minimal required permissions:

| Job | Permissions | Purpose |
|-----|-------------|---------|
| `test` | `contents: read` | Checkout code and run tests |
| `lint` | `contents: read` | Checkout code and run linting |
| `security` | `contents: read`, `security-events: write` | Security scanning and SARIF upload |
| `build-and-push` | `contents: read`, `packages: write`, `security-events: write`, `actions: read` | Build/push images, security scanning |
| `test-images` | `contents: read`, `packages: read` | Pull and test published images |
| `update-helm-chart` | `contents: write` | Update Helm chart versions and commit changes |

## Image Tagging Strategy

The workflow automatically creates tags based on:

| Trigger | Tags Created | Example |
|---------|--------------|---------|
| Push to `main` | `main`, `edge` | `ghcr.io/user/repo:main` |
| Push to `develop` | `develop` | `ghcr.io/user/repo:develop` |
| Pull Request | `pr-123` | `ghcr.io/user/repo:pr-123` |
| Version Tag | `v1.2.3`, `v1.2`, `v1`, `latest` | `ghcr.io/user/repo:v1.2.3` |

## Image Locations

After successful builds, images are available at:

### GitHub Container Registry (GHCR)
```bash
# Latest from main branch (note: repository name is automatically converted to lowercase)
docker pull ghcr.io/chitender/kube-smartscheduler:main

# Specific version
docker pull ghcr.io/chitender/kube-smartscheduler:v1.0.0

# For ARM64 specifically
docker pull --platform linux/arm64 ghcr.io/chitender/kube-smartscheduler:main
```

### Docker Hub (if configured)
```bash
# Latest from main branch
docker pull YOUR_DOCKERHUB_USERNAME/smart-scheduler:main

# Specific version
docker pull YOUR_DOCKERHUB_USERNAME/smart-scheduler:v1.0.0
```

## Multi-Architecture Support

### Platforms Supported
- **linux/amd64**: Intel/AMD 64-bit processors
- **linux/arm64**: ARM 64-bit processors (Apple Silicon, AWS Graviton, etc.)

### Verification
You can verify multi-architecture support:

```bash
# Inspect the manifest
docker buildx imagetools inspect ghcr.io/YOUR_USERNAME/kube-smartscheduler:main

# Pull for specific platform
docker pull --platform linux/arm64 ghcr.io/YOUR_USERNAME/kube-smartscheduler:main
```

## Development Workflow

### Local Testing
Before pushing, test your changes locally:

```bash
# Run tests
make test

# Build for multiple architectures
make docker-buildx

# Lint code
make lint

# Validate Helm chart
helm lint helm/smart-scheduler/
```

### Creating Releases

1. **Create and push a version tag:**
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```

2. **The workflow will automatically:**
   - Build multi-architecture images
   - Publish to both registries
   - Update Helm chart versions
   - Run security scans
   - Test images on both platforms

### Pull Request Workflow

1. **Create PR**: Opens pull request against `main`
2. **CI Runs**: All validation checks run
3. **Images Built**: Images built but not pushed (for security)
4. **Review**: Code review process
5. **Merge**: After approval, images are built and pushed

## Troubleshooting

### Build Failures

**ARM64 build fails:**
- Check if dependencies support ARM64
- Verify CGO_ENABLED=0 for static builds
- Review build logs for platform-specific errors

**Authentication failures:**
- Verify secrets are configured correctly
- Check token permissions and expiration
- For Docker Hub, ensure token has write permissions

**Security scan failures:**
- Review Trivy/Gosec reports in GitHub Security tab
- Address critical vulnerabilities before merging
- Update base images if needed

**GitHub Actions permission errors:**
- "Resource not accessible by integration": Check job permissions in workflow files
- "Action not found": Use direct tool installation instead of deprecated actions
- SARIF upload failures: Ensure `security-events: write` permission is set

**Gosec scanning issues:**
- The workflow now uses direct `gosec` installation for reliability
- SARIF uploads are skipped for pull requests to avoid permission conflicts
- Text output is displayed for PRs instead of SARIF upload

**Repository name case issues:**
- Docker registry names must be lowercase, but GitHub repository names can contain uppercase
- The workflow automatically converts repository names to lowercase for Docker operations
- Example: `chitender/kube-SmartScheduler` → `chitender/kube-smartscheduler`
- This prevents "invalid reference format: repository name must be lowercase" errors

### Performance Optimization

**Slow builds:**
- GitHub Actions cache is automatically configured
- Consider using smaller base images
- Review Docker layer optimization

**Large images:**
- Use multi-stage builds (already implemented)
- Consider distroless base images (already using)
- Remove unnecessary files in Dockerfile

## Security Considerations

1. **Secrets Management**: Never expose secrets in logs or code
2. **Image Scanning**: All images are scanned for vulnerabilities
3. **SARIF Reports**: Security reports uploaded to GitHub Security tab
4. **Minimal Images**: Using distroless base images for minimal attack surface
5. **Non-root User**: Container runs as non-root user (65532)

## Monitoring and Alerts

Configure GitHub repository settings to get notified about:
- Failed workflow runs
- Security vulnerabilities
- Dependabot alerts
- Code scanning alerts

Go to `Settings` → `Notifications` → `Actions` to configure email/Slack notifications. 