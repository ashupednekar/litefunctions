CONTAINER_TOOL ?= docker
LOCAL_REGISTRY ?= localhost:30050
IMAGE ?= litefunctions-runtime-ts
TAG ?= latest
DOCKERFILE ?= ../../build/runtimes/Dockerfile.ts

.PHONY: build
build:
	$(CONTAINER_TOOL) build -t $(IMAGE):$(TAG) -f $(DOCKERFILE) .

.PHONY: local-push
local-push:
	$(CONTAINER_TOOL) build -t $(LOCAL_REGISTRY)/$(IMAGE):$(TAG) -f $(DOCKERFILE) .
	$(CONTAINER_TOOL) push $(LOCAL_REGISTRY)/$(IMAGE):$(TAG)
