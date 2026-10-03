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

Example:

```bash
HTTP_PORT=9090 go run ./cmd/api
```