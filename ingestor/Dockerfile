FROM golang:1.24-alpine

WORKDIR /app

COPY go.mod ./go.mod
COPY go.sum ./go.sum
RUN go mod download

COPY . .

RUN go build -o /usr/local/bin/ingestor ./cmd/

EXPOSE 8080

ENTRYPOINT ["ingestor"]
