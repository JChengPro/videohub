ARG GO_VERSION=1.24.5

FROM golang:${GO_VERSION} AS build
WORKDIR /src/backend

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./

ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /out/api ./cmd
RUN go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker
RUN go build -trimpath -ldflags="-s -w" -o /out/admin ./cmd/admin

FROM alpine:3.21 AS base
RUN apk add --no-cache ca-certificates tzdata && adduser -D -H -s /sbin/nologin app
WORKDIR /app
COPY --from=build /src/backend/configs ./configs
RUN mkdir -p ./.run/uploads && chown -R app:app /app
USER app

FROM base AS api
COPY --from=build /out/api /app/api
COPY --from=build /out/admin /app/admin
EXPOSE 8080
ENTRYPOINT ["/app/api"]

FROM base AS worker

USER root

RUN apk add --no-cache ffmpeg
USER app

COPY --from=build /out/worker /app/worker
ENTRYPOINT ["/app/worker"]
