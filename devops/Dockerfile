FROM golang AS builder

WORKDIR /app

RUN go install github.com/a-h/templ/cmd/templ@latest
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN templ generate && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
        -tags netgo \
        -ldflags="-s -w" \
        -o app

FROM scratch

WORKDIR /app
COPY --from=builder /app/app server
COPY --from=builder /app/static .
COPY --from=builder /app/migrations .
RUN ls 

ENV LISTEN_PORT=3000

ENTRYPOINT ["/app/server", "listen"]
