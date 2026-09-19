FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -o /goredis ./cmd/goredis

FROM gcr.io/distroless/static-debian12
COPY --from=build /goredis /goredis
EXPOSE 6379
VOLUME ["/data"]
WORKDIR /data
ENTRYPOINT ["/goredis"]
CMD ["--port", "6379", "--aof", "/data/db.aof"]
