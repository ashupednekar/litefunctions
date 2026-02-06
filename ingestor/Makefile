IMG ?= ashupednekar535/litefunctions-ingestor

LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

PROTOC_GEN_GO_VERSION ?= v1.36.5
PROTOC_GEN_GO_GRPC_VERSION ?= v1.6.0
PROTOC_GEN_GO ?= $(LOCALBIN)/protoc-gen-go
PROTOC_GEN_GO_GRPC ?= $(LOCALBIN)/protoc-gen-go-grpc

.PHONY: protoc
protoc: $(PROTOC_GEN_GO) $(PROTOC_GEN_GO_GRPC)
	@echo "Generating gRPC code..."
	cd ../common/proto && PATH=$(LOCALBIN):$$PATH protoc \
		--go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		function.proto

$(PROTOC_GEN_GO): $(LOCALBIN)
	@echo "Installing protoc-gen-go..."
	GOBIN=$(LOCALBIN) go install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)

$(PROTOC_GEN_GO_GRPC): $(LOCALBIN)
	@echo "Installing protoc-gen-go-grpc..."
	GOBIN=$(LOCALBIN) go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION)

.PHONY: build
build: protoc
	@echo "Building ingestor..."
	go build -o bin/ingestor ./cmd/

.PHONY: docker-build
docker-build: 
	@echo "Building docker image..."
	docker build -t ${IMG} -f ../build/ingestor/Dockerfile ..

.PHONY: docker-push
docker-push: 
	@echo "Pushing docker image"
	docker push ${IMG}
