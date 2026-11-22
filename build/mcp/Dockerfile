FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

COPY common/ /app/common/
COPY mcp/go.mod mcp/go.sum /app/mcp/
WORKDIR /app/mcp
RUN go mod download

COPY mcp/ /app/mcp/

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -a -installsuffix cgo -o /out/mcp .

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /

COPY --from=builder /out/mcp /mcp

ENV MCP_LISTEN_ADDR=:8090
ENV MCP_PATH=/mcp

ENTRYPOINT ["/mcp"]
