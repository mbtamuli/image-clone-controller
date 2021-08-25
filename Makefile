IMG ?= webhook:latest
BINARY = image-clone-controller
GOOS = $(shell go env GOOS)

##@ General

.DEFAULT_GOAL := help
.PHONY: help
help: ## Show this help screen.
	@echo 'Available targets are:'
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-25s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: start-local-cluster
start-local-cluster: ## Start kind cluster.
	kind create cluster --name image-clone-controller

.PHONY: stop-local-cluster
stop-local-cluster: ## Stop kind cluster.
	kind delete cluster --name image-clone-controller

.PHONY: kind-load-docker
kind-load-docker: ## Load docker-image in kind cluster.
	kind load docker-image $(IMG) $(IMG) --name image-clone-controller

##@ Build

.PHONY: clean
clean: ## Clean build artifacts.
	rm -rf $(BINARY)

.PHONY: fmt
fmt: ## Run go fmt.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet.
	go vet ./...

.PHONY: build
build: ## Build the controller binary.
	GOOS=$(GOOS) go build -o $(BINARY)

.PHONY: docker-build
docker-build: ## Build controller docker image.
	docker build -t $(IMG) .

.PHONY: docker-push
docker-push: ## Push controller docker image.
	docker push $(IMG)

##@ Deploy

.PHONY: deploy
deploy: ## Deploy the controller and related resources to the cluster.
	cd manifests; \
	kubectl create namespace image-clone-controller; \
	kubectl create secret --namespace image-clone-controller generic registry-cred \
		--from-literal=registry="$(REGISTRY)" \
		--from-literal=registry-username="$(REGISTRY_USERNAME)" \
		--from-literal=registry-password="$(REGISTRY_PASSWORD)"; \
	kubectl apply --filename rbac.yaml; \
	kubectl apply --filename deployment.yaml;

.PHONY: undeploy
undeploy: ## Remove controller and related resources from the cluster.
	cd manifests; \
	kubectl delete secret --namespace image-clone-controller registry-cred; \
	kubectl delete --filename rbac.yaml; \
	kubectl delete --filename deployment.yaml; \
	kubectl delete namespace image-clone-controller;

# Misc

ifneq ($(EXCLUDE_NAMESPACES),)
deploy: modify_deployment
endif

# Using tee instead of in-place replacement to keep parity between macOS and Linux
# See https://unix.stackexchange.com/q/13711
modify_deployment:
	sed "s#kube-system,local-path-storage,image-clone-controller#$(EXCLUDE_NAMESPACES)#" deployment.yaml | tee test.yaml
