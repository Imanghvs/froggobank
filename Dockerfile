# syntax=docker/dockerfile:1

FROM golang:1.27.1-alpine3.23 AS api-build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY docs/ ./docs/
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags="-s -w" -o /out/api ./cmd/api

FROM golang:1.27.1-alpine3.23 AS migrations-build
WORKDIR /src
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOBIN=/out go install github.com/pressly/goose/v3/cmd/goose@v3.26.0

FROM alpine:3.23 AS migrations
RUN apk add --no-cache ca-certificates
WORKDIR /app
USER 65532:65532
COPY --from=migrations-build /out/goose /usr/local/bin/goose
COPY migrations/ /migrations/
ENV GOOSE_DRIVER=postgres
ENTRYPOINT ["goose", "-dir", "/migrations"]
CMD ["up"]

FROM alpine:3.23 AS api
RUN apk add --no-cache ca-certificates
WORKDIR /app
USER 65532:65532
COPY --from=api-build /out/api /usr/local/bin/api
ENV HTTP_PORT=8080
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=5s --start-period=10s --retries=5 \
    CMD wget -q -O /dev/null "http://127.0.0.1:${HTTP_PORT}/ready" || exit 1
ENTRYPOINT ["api"]
