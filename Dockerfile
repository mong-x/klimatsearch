# syntax=docker/dockerfile:1
FROM golang:1.27 AS build
WORKDIR /src
RUN apt-get update && apt-get install -y --no-install-recommends gcc libc6-dev libsqlite3-dev \
    && rm -rf /var/lib/apt/lists/*
ENV CGO_ENABLED=1
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -tags fts5 -o /out/klimatsearch ./cmd/klimatsearch

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/klimatsearch /usr/local/bin/klimatsearch
COPY testdata /app/testdata
WORKDIR /app
ENV KLIMAT_EMBEDDER=fake
ENV KLIMAT_LISTEN=:8080
EXPOSE 8080
# production: klimatsearch
# e2e / local demo:
CMD ["klimatsearch", "--embedder=fake", "--demo-fixture", "--ingest-on-start"]
