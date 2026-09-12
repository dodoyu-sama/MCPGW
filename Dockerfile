FROM node:20 AS web
WORKDIR /web
COPY web/package.json web/package-lock.json* ./
RUN npm install
COPY web/ ./
RUN npm run build

FROM golang:1.22 AS build
WORKDIR /src
RUN apt-get update && apt-get install -y gcc
COPY go.mod ./
RUN go mod download
COPY . .
COPY --from=web /web/dist ./web/dist
RUN CGO_ENABLED=1 go build -o /out/mcpgw ./cmd/mcpgw

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/mcpgw /usr/local/bin/mcpgw
COPY config.docker.yaml /etc/mcpgw/config.docker.yaml
COPY config.docker-postgres.yaml /etc/mcpgw/config.docker-postgres.yaml
EXPOSE 8080
ENTRYPOINT ["mcpgw", "--config", "/etc/mcpgw/config.docker.yaml"]
