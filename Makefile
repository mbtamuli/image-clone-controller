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

.PHONY: clean
clean: ## Clean build artifacts.
	rm -rf $(BINARY)

.PHONY: fmt
fmt: ## Run go fmt.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet.
	go vet ./...

.PHONY: start-kind-cluster
start-kind-cluster: ## Start kind cluster.
	sed "s#CURRENT_DIR#$(PWD)#" kind-config.yaml | kind create cluster --name image-clone-controller --config -

.PHONY: stop-kind-cluster
stop-kind-cluster: ## Stop kind cluster.
	kind delete cluster --name image-clone-controller

.PHONY: local-deploy
local-deploy: ## Deploy to cluster for development, mounting the current directory using directory mount for kind.
	@kubectl create namespace image-clone-controller; \
	kubectl create secret --namespace image-clone-controller generic registry-cred \
		--from-literal=registry="$(REGISTRY)" \
		--from-literal=registry-username="$(REGISTRY_USERNAME)" \
		--from-literal=registry-password="$(REGISTRY_PASSWORD)"; \
	kubectl apply --filename rbac.yaml; \
	kubectl apply --filename deployment-kind.yaml;

##@ Build

.PHONY: build
build: ## Build the binary.
	GOOS=$(GOOS) go build -o $(BINARY)

docker-build: ## Build docker image.
	docker build -t ${IMG} .

docker-push: ## Push docker image.
	docker push ${IMG}

kind-load-docker: ## Load docker-image in kind cluster.
	kind load docker-image ${IMG} ${IMG} --name image-clone-controller

##@ Deploy

.PHONY: deploy
deploy: ## Deploy the controller and related resources to the cluster.
	@kubectl create namespace image-clone-controller; \
	kubectl create secret --namespace image-clone-controller generic registry-cred \
		--from-literal=registry="$(REGISTRY)" \
		--from-literal=registry-username="$(REGISTRY_USERNAME)" \
		--from-literal=registry-password="$(REGISTRY_PASSWORD)"; \
	kubectl apply --filename rbac.yaml; \
	kubectl apply --filename deployment.yaml;


.PHONY: undeploy
undeploy: ## Remove controller and related resources from the cluster.
	kubectl delete secret --namespace image-clone-controller registry-cred; \
	kubectl delete --filename rbac.yaml; \
	kubectl delete --filename deployment.yaml; \
	kubectl delete namespace image-clone-controller
