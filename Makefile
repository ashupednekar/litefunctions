.PHONY: build
build:
	$(MAKE) -C operator docker-build
	$(MAKE) -C operator docker-push
	$(MAKE) -C operator helm-sync
	$(MAKE) -C ingestor docker-build
	$(MAKE) -C ingestor docker-push
	$(MAKE) -C portal docker-build
	$(MAKE) -C portal docker-push
	$(MAKE) runtime-python
	$(MAKE) runtime-ts
	$(MAKE) runtime-lua

.PHONY: runtime-python
runtime-python:
	$(CONTAINER_TOOL) build -t ashupednekar535/litefunctions-runtime-py -f build/runtimes/Dockerfile.python runtimes/python && $(CONTAINER_TOOL) push ashupednekar535/litefunctions-runtime-py

.PHONY: runtime-ts
runtime-ts:
	$(CONTAINER_TOOL) build -t ashupednekar535/litefunctions-runtime-ts -f build/runtimes/Dockerfile.ts runtimes/ts && $(CONTAINER_TOOL) push ashupednekar535/litefunctions-runtime-ts

.PHONY: runtime-lua
runtime-lua:
	$(CONTAINER_TOOL) build -t ashupednekar535/litefunctions-runtime-lua -f build/runtimes/Dockerfile.lua runtimes/lua && $(CONTAINER_TOOL) push ashupednekar535/litefunctions-runtime-lua

include build/images.mk

.PHONY: build-multiarch
build-multiarch:
	$(MAKE) -C operator docker-buildx
	$(MAKE) -C operator helm-sync
	$(MAKE) -C ingestor docker-buildx
	$(MAKE) -C portal docker-buildx
	$(MAKE) runtime-python-buildx runtime-ts-buildx runtime-lua-buildx

.PHONY: runtime-python-buildx
runtime-python-buildx: docker-builder
	$(CONTAINER_TOOL) buildx build --builder "$(BUILDER)" --platform "$(PLATFORMS)" --push -t ashupednekar535/litefunctions-runtime-py -f build/runtimes/Dockerfile.python runtimes/python

.PHONY: runtime-ts-buildx
runtime-ts-buildx: docker-builder
	$(CONTAINER_TOOL) buildx build --builder "$(BUILDER)" --platform "$(PLATFORMS)" --push -t ashupednekar535/litefunctions-runtime-ts -f build/runtimes/Dockerfile.ts runtimes/ts

.PHONY: runtime-lua-buildx
runtime-lua-buildx: docker-builder
	$(CONTAINER_TOOL) buildx build --builder "$(BUILDER)" --platform "$(PLATFORMS)" --push -t ashupednekar535/litefunctions-runtime-lua -f build/runtimes/Dockerfile.lua runtimes/lua
