#!/bin/bash

# Smart Scheduler Release Script
# This script demonstrates how to create semantic version releases

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if we're in a git repository
if ! git rev-parse --git-dir > /dev/null 2>&1; then
    log_error "Not in a git repository"
    exit 1
fi

# Get current version
CURRENT_VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "v0.1.0")
log_info "Current version: $CURRENT_VERSION"

# Parse semantic version
if [[ $CURRENT_VERSION =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)(-[a-zA-Z0-9-]+)?(\+[a-zA-Z0-9-]+)?$ ]]; then
    MAJOR=${BASH_REMATCH[1]}
    MINOR=${BASH_REMATCH[2]}
    PATCH=${BASH_REMATCH[3]}
    PRE_RELEASE=${BASH_REMATCH[4]}
    BUILD_META=${BASH_REMATCH[5]}
else
    # Default for new repository
    MAJOR=0
    MINOR=1
    PATCH=0
    PRE_RELEASE=""
    BUILD_META=""
fi

# Calculate next versions
NEXT_PATCH="v$MAJOR.$MINOR.$((PATCH + 1))"
NEXT_MINOR="v$MAJOR.$((MINOR + 1)).0"
NEXT_MAJOR="v$((MAJOR + 1)).0.0"

echo ""
log_info "Version bump options:"
echo "  1) Patch release: $NEXT_PATCH (bug fixes)"
echo "  2) Minor release: $NEXT_MINOR (new features, backward compatible)"
echo "  3) Major release: $NEXT_MAJOR (breaking changes)"
echo "  4) Custom version"
echo "  5) Build current version (no new tag)"
echo "  6) Exit"

read -p "Select option [1-6]: " choice

case $choice in
    1)
        NEW_VERSION=$NEXT_PATCH
        ;;
    2)
        NEW_VERSION=$NEXT_MINOR
        ;;
    3)
        NEW_VERSION=$NEXT_MAJOR
        ;;
    4)
        read -p "Enter custom version (e.g., v1.2.3, v2.0.0-beta.1): " NEW_VERSION
        if [[ ! $NEW_VERSION =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9-]+)?(\+[a-zA-Z0-9-]+)?$ ]]; then
            log_error "Invalid semantic version format"
            exit 1
        fi
        ;;
    5)
        NEW_VERSION=$CURRENT_VERSION
        log_warning "Building current version without creating new tag"
        ;;
    6)
        log_info "Exiting"
        exit 0
        ;;
    *)
        log_error "Invalid option"
        exit 1
        ;;
esac

log_info "Selected version: $NEW_VERSION"

# Check for uncommitted changes
if [ "$choice" != "5" ] && [ -n "$(git status --porcelain)" ]; then
    log_warning "You have uncommitted changes:"
    git status --short
    read -p "Continue anyway? [y/N]: " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log_info "Aborting release"
        exit 1
    fi
fi

# Create tag if new version
if [ "$choice" != "5" ]; then
    log_info "Creating git tag: $NEW_VERSION"
    git tag "$NEW_VERSION"
    log_success "Tag created: $NEW_VERSION"
fi

# Show current version info
log_info "Running version check..."
make version VERSION="$NEW_VERSION"

echo ""
read -p "Proceed with build and release? [y/N]: " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    if [ "$choice" != "5" ]; then
        log_warning "Removing created tag"
        git tag -d "$NEW_VERSION"
    fi
    log_info "Release cancelled"
    exit 1
fi

# Run pre-release checks
log_info "Running pre-release checks..."
if ! make pre-release VERSION="$NEW_VERSION"; then
    log_error "Pre-release checks failed"
    if [ "$choice" != "5" ]; then
        log_warning "Removing created tag"
        git tag -d "$NEW_VERSION"
    fi
    exit 1
fi

# Build and push images
log_info "Building and pushing multi-architecture images..."
if ! make release VERSION="$NEW_VERSION"; then
    log_error "Release build failed"
    if [ "$choice" != "5" ]; then
        log_warning "Removing created tag"
        git tag -d "$NEW_VERSION"
    fi
    exit 1
fi

# Push tag to remote
if [ "$choice" != "5" ]; then
    log_info "Pushing tag to remote..."
    git push origin "$NEW_VERSION"
    log_success "Tag pushed to remote: $NEW_VERSION"
fi

log_success "Release completed successfully!"
echo ""
log_info "Release Summary:"
echo "  Version: $NEW_VERSION"
echo "  Images available with tags:"
echo "    - $NEW_VERSION"
echo "    - latest"
echo "    - v$MAJOR"
echo "    - v$MAJOR.$MINOR"
echo "    - $(git rev-parse --short HEAD)"
echo ""
log_info "Next steps:"
echo "  1. Update deployment manifests to use new version"
echo "  2. Update Helm chart version if applicable"
echo "  3. Create release notes"
echo "  4. Announce the release" 