IMG ?= ashupednekar535/litefunctions-portal:latest
DOCKERFILE ?= ../build/portal/Dockerfile
DOCKER_CONTEXT ?= ..
LOCAL_REGISTRY ?= localhost:30050
LOCAL_IMG ?= $(LOCAL_REGISTRY)/$(notdir $(IMG))

.PHONY: docker-build
docker-build:
	$(CONTAINER_TOOL) build $(LOCAL_BUILD_ARGS) -t $(IMG) -f $(DOCKERFILE) $(DOCKER_CONTEXT)

.PHONY: docker-push
docker-push:
	$(CONTAINER_TOOL) push $(IMG)

.PHONY: local-build-push
local-build-push:
	$(CONTAINER_TOOL) build $(LOCAL_BUILD_ARGS) -t $(LOCAL_IMG) -f $(DOCKERFILE) $(DOCKER_CONTEXT)
	$(CONTAINER_TOOL) push $(LOCAL_IMG)

.PHONY: local-push
local-push: local-build-push ## Build and push to the local registry, then restart the portal deployment.
	kubectl -n litefunctions set image deployment/litefunctions-portal litefunctions-portal=$(LOCAL_IMG)
	kubectl -n litefunctions rollout restart deployment/litefunctions-portal

include ../build/images.mk

.PHONY: docker-buildx
docker-buildx: docker-builder
	$(CONTAINER_TOOL) buildx build --builder "$(BUILDER)" --platform "$(PLATFORMS)" --push -t "$(IMG)" -f $(DOCKERFILE) $(DOCKER_CONTEXT)
