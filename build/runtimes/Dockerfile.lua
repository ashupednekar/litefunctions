FROM alpine:3.20

RUN apk add --no-cache \
    lua5.4 \
    lua5.4-dev \
    luarocks \
    git \
    ca-certificates \
    build-base \
    openssl-dev \
    && luarocks-5.4 --lua-version=5.4 install luasocket \
    && luarocks-5.4 --lua-version=5.4 install lua-cjson

WORKDIR /app

COPY . .
RUN mkdir -p /app/pkg/functions

CMD ["lua", "/app/main.lua"]
