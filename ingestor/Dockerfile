FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

COPY common/ /app/common/
COPY ingestor/go.mod ingestor/go.sum /app/ingestor/
WORKDIR /app/ingestor
RUN go mod download

COPY ingestor/ /app/ingestor/

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -a -installsuffix cgo -o /out/ingestor ./cmd/main.go

FROM alpine:3.20

WORKDIR /

COPY --from=builder /out/ingestor /ingestor

EXPOSE 3000

ENTRYPOINT ["/ingestor"]
