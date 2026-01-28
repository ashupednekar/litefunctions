FROM golang AS builder

ARG GIT_TOKEN
ARG GIT_USER=lwsrepos
ARG PROJECT
ARG NAME 

WORKDIR /app
COPY . .
RUN ls

RUN go build cmd/main.go

FROM scratch 
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=builder /app/main /func

CMD ["/func"]

