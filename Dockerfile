FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/veyra ./cmd/server

FROM debian:bookworm-slim

LABEL org.opencontainers.image.source="https://github.com/mayvqt/Veyra"

WORKDIR /app
RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates gosu \
  && rm -rf /var/lib/apt/lists/* \
  && groupadd -r veyra \
  && useradd -r -g veyra -d /app -s /usr/sbin/nologin veyra \
  && mkdir -p /config \
  && chown -R veyra:veyra /app /config
COPY --from=build /bin/veyra /usr/local/bin/veyra
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
COPY web ./web
COPY internal/http/templates ./internal/http/templates
RUN chmod +x /usr/local/bin/docker-entrypoint.sh
VOLUME ["/config"]
ENV APP_BIND_ADDR=0.0.0.0:3767
ENV DATABASE_PATH=/config/veyra.db
ENV PUID=99
ENV PGID=100
EXPOSE 3767
ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["veyra"]
