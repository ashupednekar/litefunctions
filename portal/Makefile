IMG ?= ashupednekar535/litefunctions-portal:latest
DOCKERFILE ?= ../build/portal/Dockerfile
DOCKER_CONTEXT ?= ..

.PHONY: docker-build
docker-build:
	docker build -t $(IMG) -f $(DOCKERFILE) $(DOCKER_CONTEXT)

.PHONY: docker-push
docker-push:
	docker push $(IMG)
