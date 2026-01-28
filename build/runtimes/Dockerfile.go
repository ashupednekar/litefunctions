FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ca-certificates

ARG GIT_TOKEN
ARG GIT_USER=lwsrepos
ARG PROJECT
ARG NAME

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags="-w -s" -o /app/main ./cmd/main.go

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/main /func

CMD ["/func"]
