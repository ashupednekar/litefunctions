# Build the manager binary
# BuildKit sets BUILDPLATFORM automatically; the default keeps the legacy
# builder (used when the buildx plugin is unavailable) working.
ARG BUILDPLATFORM=linux/amd64
FROM --platform=$BUILDPLATFORM golang:1.26 AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
# Copy the Go Modules manifests
COPY common/ /workspace/common/
COPY operator/go.mod operator/go.sum /workspace/operator/
WORKDIR /workspace/operator
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY operator/cmd/ cmd/
COPY operator/api/ api/
COPY operator/internal/ internal/

# Cross-compile for the requested image platform on the native build host.
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -a -o operator ./cmd/

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/operator/operator .
USER 65532:65532

ENTRYPOINT ["/operator", "manager"]
