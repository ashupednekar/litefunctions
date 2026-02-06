FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY common/ /app/common/
COPY ingestor/go.mod ingestor/go.sum /app/ingestor/
WORKDIR /app/ingestor
RUN go mod download

COPY ingestor/ /app/ingestor/

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /out/ingestor ./cmd/main.go

FROM alpine:3.20

WORKDIR /

COPY --from=builder /out/ingestor /ingestor

EXPOSE 3000

ENTRYPOINT ["/ingestor"]
