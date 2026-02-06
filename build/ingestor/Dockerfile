FROM golang AS builder

WORKDIR /app

COPY common/ /app/common/
COPY ingestor/go.mod ingestor/go.sum /app/ingestor/
WORKDIR /app/ingestor
RUN go mod download

COPY ingestor/ /app/ingestor/

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main cmd/main.go

FROM scratch

WORKDIR /

COPY --from=builder /app/ingestor/main /ingestor

ENV NATS_URL=nats://litefunctions-nats:4222

CMD ["/ingestor"]
