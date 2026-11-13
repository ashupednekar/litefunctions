# Shared settings for multi-platform publishing. Local docker-build targets stay native.
CONTAINER_TOOL ?= docker
PLATFORMS ?= linux/amd64,linux/arm64
BUILDER ?= litefunctions-builder

# BuildKit defines $BUILDPLATFORM/$TARGETOS/$TARGETARCH automatically, which lets
# multi-platform publishes cross-compile on the native build host. The legacy
# builder (still the fallback for Docker CLIs that do not ship the buildx plugin,
# e.g. Colima) defines none of them, so local builds pass the host platform
# explicitly. Override LOCAL_PLATFORM to build for a different platform.
HOST_ARCH ?= $(shell uname -m)
ifeq ($(HOST_ARCH),arm64)
LOCAL_PLATFORM ?= linux/arm64
else ifeq ($(HOST_ARCH),aarch64)
LOCAL_PLATFORM ?= linux/arm64
else
LOCAL_PLATFORM ?= linux/amd64
endif

LOCAL_OS := $(word 1,$(subst /, ,$(LOCAL_PLATFORM)))
LOCAL_ARCH := $(word 2,$(subst /, ,$(LOCAL_PLATFORM)))
LOCAL_BUILD_ARGS ?= --build-arg BUILDPLATFORM=$(LOCAL_PLATFORM) --build-arg TARGETOS=$(LOCAL_OS) --build-arg TARGETARCH=$(LOCAL_ARCH)

.PHONY: docker-builder
docker-builder:
	@if $(CONTAINER_TOOL) buildx inspect "$(BUILDER)" >/dev/null 2>&1; then \
		$(CONTAINER_TOOL) buildx inspect "$(BUILDER)" --bootstrap; \
	else \
		$(CONTAINER_TOOL) buildx create --name "$(BUILDER)" --driver docker-container --bootstrap; \
	fi
