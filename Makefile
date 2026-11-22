.PHONY: build
build:
	$(MAKE) -C operator docker-build
	$(MAKE) -C operator docker-push
	$(MAKE) -C operator helm-sync
	$(MAKE) -C ingestor docker-build
	$(MAKE) -C ingestor docker-push
	$(MAKE) -C portal docker-build
	$(MAKE) -C portal docker-push
	$(MAKE) -C mcp docker-build
	$(MAKE) -C mcp docker-push
	$(MAKE) runtime-python
	$(MAKE) runtime-ts
	$(MAKE) runtime-lua
	$(MAKE) runtime-assets
	$(MAKE) gitea-setup

.PHONY: local-images
local-images: local-image-operator local-image-ingestor local-image-portal local-image-mcp runtime-local

.PHONY: local-image-operator
local-image-operator:
	$(MAKE) -C operator local-build-push

.PHONY: local-image-ingestor
local-image-ingestor:
	$(MAKE) -C ingestor local-build-push

.PHONY: local-image-portal
local-image-portal:
	$(MAKE) -C portal local-build-push

.PHONY: local-image-mcp
local-image-mcp:
	$(MAKE) -C mcp local-build-push

.PHONY: helm-sync
helm-sync:
	$(MAKE) -C operator helm-sync

# The local registry is the chart's Zot NodePort. Bootstrap it with default
# images before building, otherwise the first push has nowhere to go.
.PHONY: bootstrap-local-registry
bootstrap-local-registry: helm-sync
	@if curl -fsS http://localhost:$(REGISTRY_PORT)/v2/ >/dev/null 2>&1; then \
		echo "local registry already running"; \
		exit 0; \
	fi; \
	echo "starting local registry via chart defaults"; \
	helm upgrade --install $(RELEASE) ./chart --namespace $(NAMESPACE) --create-namespace \
		--server-side=true --force-conflicts \
		--set gatewayApi.tls.certManager.enabled=false; \
	kubectl -n $(NAMESPACE) rollout status deployment/$(RELEASE)-zot --timeout=180s || true; \
	for i in $$(seq 1 60); do \
		if curl -fsS http://localhost:$(REGISTRY_PORT)/v2/ >/dev/null 2>&1; then \
			echo "local registry ready"; \
			exit 0; \
		fi; \
		sleep 2; \
	done; \
	echo "local registry did not become ready on localhost:$(REGISTRY_PORT)" >&2; \
	exit 1

.PHONY: local-push
local-push: bootstrap-local-registry
	$(MAKE) -j4 local-images
	helm upgrade --install $(RELEASE) ./chart --namespace $(NAMESPACE) --create-namespace \
		--server-side=true --force-conflicts \
		--set gatewayApi.tls.certManager.enabled=false \
		--set-string images.operator=$(LOCAL_REGISTRY)/litefunctions-operator:latest \
		--set-string images.ingestor=$(LOCAL_REGISTRY)/litefunctions-ingestor:latest \
		--set-string images.portal=$(LOCAL_REGISTRY)/litefunctions-portal:latest \
		--set-string images.mcp=$(LOCAL_REGISTRY)/litefunctions-mcp:latest \
		--set-string operator.runtimeRegistry=$(LOCAL_REGISTRY)
	kubectl -n $(NAMESPACE) rollout restart deployment/litefunctions-operator
	kubectl -n $(NAMESPACE) rollout restart deployment/litefunctions-ingestor
	kubectl -n $(NAMESPACE) rollout restart deployment/litefunctions-portal
	kubectl -n $(NAMESPACE) rollout restart deployment/litefunctions-mcp
	kubectl -n $(NAMESPACE) rollout status deployment/litefunctions-operator
	kubectl -n $(NAMESPACE) rollout status deployment/litefunctions-ingestor
	kubectl -n $(NAMESPACE) rollout status deployment/litefunctions-portal
	kubectl -n $(NAMESPACE) rollout status deployment/litefunctions-mcp

.PHONY: runtime-local
runtime-local: runtime-python-local runtime-ts-local runtime-lua-local ## Build and push shared dynamic runtimes to the local registry.

.PHONY: runtime-python-local
runtime-python-local:
	$(MAKE) -C runtimes/python local-push

.PHONY: runtime-ts-local
runtime-ts-local:
	$(MAKE) -C runtimes/ts local-push

.PHONY: runtime-lua-local
runtime-lua-local:
	$(MAKE) -C runtimes/lua local-push

# Static Go/Rust runtimes are built per function by the project CI workflow;
# there is no shared image to push. Use the portal build or the Actions run.

.PHONY: runtime-python
runtime-python:
	$(CONTAINER_TOOL) build -t ashupednekar535/litefunctions-runtime-py -f build/runtimes/Dockerfile.python runtimes/python && $(CONTAINER_TOOL) push ashupednekar535/litefunctions-runtime-py

.PHONY: runtime-ts
runtime-ts:
	$(CONTAINER_TOOL) build -t ashupednekar535/litefunctions-runtime-ts -f build/runtimes/Dockerfile.ts runtimes/ts && $(CONTAINER_TOOL) push ashupednekar535/litefunctions-runtime-ts

.PHONY: runtime-lua
runtime-lua:
	$(CONTAINER_TOOL) build -t ashupednekar535/litefunctions-runtime-lua -f build/runtimes/Dockerfile.lua runtimes/lua && $(CONTAINER_TOOL) push ashupednekar535/litefunctions-runtime-lua

.PHONY: runtime-assets
runtime-assets:
	$(CONTAINER_TOOL) build -t $(RUNTIME_ASSETS_IMAGE):latest -f build/runtime-assets/Dockerfile . && $(CONTAINER_TOOL) push $(RUNTIME_ASSETS_IMAGE):latest

.PHONY: gitea-setup
gitea-setup:
	$(CONTAINER_TOOL) build -t ashupednekar535/litefunctions-gitea-setup -f build/gitea-setup/Dockerfile build/gitea-setup && $(CONTAINER_TOOL) push ashupednekar535/litefunctions-gitea-setup

include build/images.mk
LOCAL_REGISTRY ?= localhost:30050
REGISTRY_PORT ?= 30050
RELEASE ?= litefunctions
NAMESPACE ?= litefunctions
RUNTIME_ASSETS_IMAGE ?= ashupednekar535/litefunctions-runtime-assets

.PHONY: build-multiarch
build-multiarch:
	$(MAKE) -C operator docker-buildx
	$(MAKE) -C operator helm-sync
	$(MAKE) -C ingestor docker-buildx
	$(MAKE) -C portal docker-buildx
	$(MAKE) -C mcp docker-buildx
	$(MAKE) runtime-python-buildx runtime-ts-buildx runtime-lua-buildx runtime-assets-buildx gitea-setup-buildx

.PHONY: gitea-setup-buildx
gitea-setup-buildx: docker-builder
	$(CONTAINER_TOOL) buildx build --builder "$(BUILDER)" --platform "$(PLATFORMS)" --push -t ashupednekar535/litefunctions-gitea-setup -f build/gitea-setup/Dockerfile build/gitea-setup

.PHONY: runtime-assets-buildx
runtime-assets-buildx: docker-builder
	$(CONTAINER_TOOL) buildx build --builder "$(BUILDER)" --platform "$(PLATFORMS)" --push -t $(RUNTIME_ASSETS_IMAGE):latest -f build/runtime-assets/Dockerfile .

.PHONY: runtime-python-buildx
runtime-python-buildx: docker-builder
	$(CONTAINER_TOOL) buildx build --builder "$(BUILDER)" --platform "$(PLATFORMS)" --push -t ashupednekar535/litefunctions-runtime-py -f build/runtimes/Dockerfile.python runtimes/python

.PHONY: runtime-ts-buildx
runtime-ts-buildx: docker-builder
	$(CONTAINER_TOOL) buildx build --builder "$(BUILDER)" --platform "$(PLATFORMS)" --push -t ashupednekar535/litefunctions-runtime-ts -f build/runtimes/Dockerfile.ts runtimes/ts

.PHONY: runtime-lua-buildx
runtime-lua-buildx: docker-builder
	$(CONTAINER_TOOL) buildx build --builder "$(BUILDER)" --platform "$(PLATFORMS)" --push -t ashupednekar535/litefunctions-runtime-lua -f build/runtimes/Dockerfile.lua runtimes/lua
