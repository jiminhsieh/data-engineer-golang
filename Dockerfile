FROM golang:1.24.7-alpine3.22 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/weather-etl ./cmd/weather-etl

FROM alpine:3.22.1
RUN apk add --no-cache ca-certificates \
    && addgroup -S -g 10001 etl \
    && adduser -S -D -H -u 10001 -G etl etl \
    && mkdir -p /app/data/raw /app/data/processed /app/logs \
    && chown -R etl:etl /app
COPY --from=build /out/weather-etl /app/weather-etl
WORKDIR /app
USER 10001:10001
EXPOSE 8080
ENTRYPOINT ["/app/weather-etl"]
