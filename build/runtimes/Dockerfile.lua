FROM alpine:3.20

ARG LUA_VERSION=5.4

RUN apk add --no-cache \
    lua${LUA_VERSION} \
    lua${LUA_VERSION}-dev \
    luarocks${LUA_VERSION} \
    git \
    ca-certificates \
    build-base \
    openssl-dev \
    && luarocks-${LUA_VERSION} --lua-version=${LUA_VERSION} install luasocket \
    && luarocks-${LUA_VERSION} --lua-version=${LUA_VERSION} install lua-cjson \
    && ln -sf /usr/bin/lua${LUA_VERSION} /usr/bin/lua \
    && ln -sf /usr/bin/luarocks-${LUA_VERSION} /usr/bin/luarocks \
    && apk del --no-cache build-base openssl-dev git lua${LUA_VERSION}-dev


WORKDIR /app

COPY . .
RUN mkdir -p /app/pkg/functions

CMD ["lua", "/app/main.lua"]
