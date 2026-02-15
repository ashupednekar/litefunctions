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
	docker build -t ashupednekar535/litefunctions-runtime-py -f build/runtimes/Dockerfile.python runtimes/python && docker push ashupednekar535/litefunctions-runtime-py

.PHONY: runtime-ts
runtime-ts:
	docker build -t ashupednekar535/litefunctions-runtime-ts -f build/runtimes/Dockerfile.ts runtimes/ts && docker push ashupednekar535/litefunctions-runtime-ts

.PHONY: runtime-lua
runtime-lua:
	docker build -t ashupednekar535/litefunctions-runtime-lua -f build/runtimes/Dockerfile.lua runtimes/lua && docker push ashupednekar535/litefunctions-runtime-lua
