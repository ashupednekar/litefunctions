IMG ?= ashupednekar535/litefunctions-portal:latest
DOCKERFILE ?= ../build/portal/Dockerfile
DOCKER_CONTEXT ?= ..

.PHONY: docker-build
docker-build:
	$(CONTAINER_TOOL) build -t $(IMG) -f $(DOCKERFILE) $(DOCKER_CONTEXT)

.PHONY: docker-push
docker-push:
	$(CONTAINER_TOOL) push $(IMG)

include ../build/images.mk

.PHONY: docker-buildx
docker-buildx: docker-builder
	$(CONTAINER_TOOL) buildx build --builder "$(BUILDER)" --platform "$(PLATFORMS)" --push -t "$(IMG)" -f $(DOCKERFILE) $(DOCKER_CONTEXT)
