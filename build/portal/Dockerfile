FROM golang AS builder

WORKDIR /app/portal

RUN go install github.com/a-h/templ/cmd/templ@latest
COPY portal/go.mod portal/go.sum ./
COPY ingestor /app/ingestor
RUN go mod download

COPY portal ./
RUN ls migrations

RUN templ generate && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
        -tags netgo \
        -ldflags="-s -w" \
        -o app

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

WORKDIR /app
COPY --from=builder /app/portal/app server

ENV LISTEN_PORT=3000

ENTRYPOINT ["/app/server", "listen"]
