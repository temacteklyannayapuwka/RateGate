# RateGate

RateGate is an HTTP gateway project written in Go.

## Features

- HTTP server listening on port 8080.
- `GET /health` returns `200 OK` with `{"status":"ok"}`.
- HTTP server timeouts.

## Run

Requires Go 1.22 or later.

```sh
go run ./cmd/rategate
```

Check the server:

```sh
curl -i http://localhost:8080/health
```
