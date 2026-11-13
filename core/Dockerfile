FROM golang:1.23-alpine AS build

WORKDIR /src

RUN apk add --no-cache ca-certificates git tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ENV CGO_ENABLED=0 GOOS=linux

RUN go build -trimpath -ldflags="-s -w" -o /out/core-api ./cmd/api && \
    go build -trimpath -ldflags="-s -w" -o /out/seed      ./cmd/seed

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S buddy && adduser -S -G buddy -h /home/buddy buddy

COPY --from=build /out/core-api /usr/local/bin/core-api
COPY --from=build /out/seed      /usr/local/bin/seed

USER buddy
WORKDIR /home/buddy

EXPOSE 8081

ENV HTTP_ADDR=:8081 \
    LOG_LEVEL=info

HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=5 \
    CMD wget -qO- "http://127.0.0.1:${HTTP_ADDR#:}/healthz" >/dev/null || exit 1

ENTRYPOINT ["/usr/local/bin/core-api"]
