# Shared settings for multi-platform publishing. Local docker-build targets stay native.
CONTAINER_TOOL ?= docker
PLATFORMS ?= linux/amd64,linux/arm64
BUILDER ?= litefunctions-builder

.PHONY: docker-builder
docker-builder:
	@if $(CONTAINER_TOOL) buildx inspect "$(BUILDER)" >/dev/null 2>&1; then \
		$(CONTAINER_TOOL) buildx inspect "$(BUILDER)" --bootstrap; \
	else \
		$(CONTAINER_TOOL) buildx create --name "$(BUILDER)" --driver docker-container --bootstrap; \
	fi
