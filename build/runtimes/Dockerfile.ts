FROM oven/bun:1.2.2

RUN apt-get update -y \
    && apt-get install -y --no-install-recommends git ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY package.json ./
RUN bun install

COPY . .
RUN mkdir -p pkg/functions \
    && chown -R bun:bun /app

USER bun

ENTRYPOINT ["bun", "run", "main.ts"]
