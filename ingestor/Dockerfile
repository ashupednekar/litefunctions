FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /out/ingestor ./cmd/main.go

FROM alpine:3.20

WORKDIR /

COPY --from=builder /out/ingestor /ingestor

EXPOSE 3000

ENTRYPOINT ["/ingestor"]
