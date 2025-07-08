# Image URL to use all building/pushing image targets
IMG ?= smart-scheduler:latest

# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.28.3

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test ./... -coverprofile cover.out

.PHONY: lint
lint: ## Run golangci-lint
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run

##@ Build

.PHONY: build
build: fmt vet ## Build manager binary.
	go build -o bin/manager cmd/main.go

.PHONY: run
run: fmt vet ## Run a controller from your host.
	go run ./cmd/main.go

.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	docker build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	docker push ${IMG}

.PHONY: docker-buildx-setup
docker-buildx-setup: ## Set up Docker buildx for multi-architecture builds.
	docker buildx create --name multiarch --driver docker-container --use || true
	docker buildx inspect --bootstrap

.PHONY: docker-buildx
docker-buildx: docker-buildx-setup ## Build multi-architecture docker images (amd64, arm64).
	docker buildx build --platform linux/amd64,linux/arm64 -t ${IMG} .

.PHONY: docker-buildx-push
docker-buildx-push: docker-buildx-setup ## Build and push multi-architecture docker images.
	docker buildx build --platform linux/amd64,linux/arm64 -t ${IMG} --push .

.PHONY: docker-build-amd64
docker-build-amd64: ## Build docker image for AMD64 architecture.
	docker buildx build --platform linux/amd64 -t ${IMG}-amd64 --load .

.PHONY: docker-build-arm64  
docker-build-arm64: ## Build docker image for ARM64 architecture.
	docker buildx build --platform linux/arm64 -t ${IMG}-arm64 --load .

.PHONY: docker-test-multiarch
docker-test-multiarch: docker-buildx ## Test multi-architecture images locally.
	@echo "Testing AMD64 image..."
	docker run --rm --platform linux/amd64 ${IMG} --help || echo "AMD64 test completed"
	@echo "Testing ARM64 image..."
	docker run --rm --platform linux/arm64 ${IMG} --help || echo "ARM64 test completed"

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	kubectl apply -f deploy/

.PHONY: uninstall
uninstall: ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
	kubectl delete -f deploy/ --ignore-not-found=$(ignore-not-found)

.PHONY: deploy
deploy: ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	kubectl apply -f deploy/

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config.
	kubectl delete -f deploy/ --ignore-not-found=$(ignore-not-found)

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
ENVTEST ?= $(LOCALBIN)/setup-envtest

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	test -s $(LOCALBIN)/setup-envtest || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

##@ Certificates

.PHONY: generate-certs
generate-certs: ## Generate self-signed certificates for webhook
	mkdir -p config/certs
	openssl genrsa -out config/certs/ca.key 2048
	openssl req -new -x509 -key config/certs/ca.key -out config/certs/ca.crt -days 365 -subj "/CN=smart-scheduler-ca"
	openssl genrsa -out config/certs/server.key 2048
	openssl req -new -key config/certs/server.key -out config/certs/server.csr -subj "/CN=smart-scheduler-webhook-service.smart-scheduler-system.svc"
	openssl x509 -req -in config/certs/server.csr -CA config/certs/ca.crt -CAkey config/certs/ca.key -CAcreateserial -out config/certs/server.crt -days 365 