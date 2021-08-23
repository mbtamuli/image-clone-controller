BINARY = image-clone-controller
GOOS = $(shell go env GOOS)

##@ General

.DEFAULT_GOAL := help
.PHONY: help
help: ## Show this help screen.
	@echo 'Available targets are:'
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-25s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: build
build: fmt vet ## Build the binary.
	GOOS=$(GOOS) go build -o $(BINARY)

.PHONY: clean
clean: ## Clean build artifacts.
	rm -rf $(BINARY)

.PHONY: start-local-cluster
start-local-cluster: ## Start local cluster.
	kind create cluster --name image-clone-controller

.PHONY: stop-local-cluster
stop-local-cluster: ## Stop local cluster.
	kind delete cluster --name image-clone-controller

# Miscellaneous, used by other targets

fmt:
	go fmt ./...

vet:
	go vet ./...
