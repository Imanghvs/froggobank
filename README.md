# FroggoBank

An open-source banking backend built with Go.

## Requirements

- Go 1.25+

## Run

```bash
go run ./cmd/api
```

## Configuration

FroggoBank uses environment variables for configuration.

| Variable | Default | Description |
| --- | --- | --- |
| `HTTP_PORT` | `8080` | HTTP server port |
| `LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, or `error` |
| `DATABASE_URL` | required | PostgreSQL connection URL |

Example:

```bash
DATABASE_URL='postgres://froggobank:froggobank@localhost:5432/froggobank' \
go run ./cmd/api
```

## Health Checks

### Liveness

```http
GET /health
```

Returns 200 OK when the application process is running.

### Readiness

```http
GET /ready
```