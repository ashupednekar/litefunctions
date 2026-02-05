.PHONY: build
build:
	$(MAKE) -C operator docker-build
	$(MAKE) -C operator docker-push
	$(MAKE) -C operator helm-sync
	$(MAKE) -C ingestor docker-build
	$(MAKE) -C ingestor docker-push
	$(MAKE) -C portal docker-build
	$(MAKE) -C portal docker-push
