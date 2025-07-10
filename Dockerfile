# Build stage
FROM --platform=$BUILDPLATFORM golang:1.21-alpine AS builder

# Build arguments for multi-architecture support
ARG TARGETOS
ARG TARGETARCH
ARG BUILDPLATFORM
ARG TARGETPLATFORM

# Version build arguments
ARG VERSION=unknown
ARG COMMIT_HASH=unknown  
ARG BUILD_DATE=unknown

# Set default values for architecture if not provided (for local builds)
ENV TARGETOS=${TARGETOS:-linux}
ENV TARGETARCH=${TARGETARCH:-amd64}

WORKDIR /workspace

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the manager binary with proper architecture targeting and version info
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -a \
    -ldflags "-w -s -X 'github.com/kube-smartscheduler/smart-scheduler/pkg/version.Version=${VERSION}' -X 'github.com/kube-smartscheduler/smart-scheduler/pkg/version.CommitHash=${COMMIT_HASH}' -X 'github.com/kube-smartscheduler/smart-scheduler/pkg/version.BuildDate=${BUILD_DATE}'" \
    -o manager cmd/main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM --platform=$TARGETPLATFORM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"] 