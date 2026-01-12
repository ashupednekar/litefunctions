FROM golang AS builder

ARG GIT_TOKEN
ARG GIT_USER=lwsrepos
ARG PROJECT
ARG NAME 

WORKDIR /app
COPY . .
RUN ls

#RUN curl -H "Authorization: Bearer $GIT_TOKEN" https://raw.githubusercontent.com/$GIT_USER/$PROJECT/main/functions/rs/$NAME.rs -o pkg/function.go
RUN go build cmd/main.go

RUN ls

FROM scratch 
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=builder /app/main /func

CMD ["/func"]

