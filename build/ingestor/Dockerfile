FROM golang AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main cmd/main.go

FROM scratch

WORKDIR /

COPY --from=builder /app/main /ingestor

ENV NATS_BROKER_URL=nats://litefunctions-nats:4222

CMD ["/ingestor"]
