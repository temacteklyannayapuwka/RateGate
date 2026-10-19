# RateGate

RateGate is an HTTP gateway project written in Go.

## Features

- HTTP server listening on port 8080.
- `GET /health` returns `200 OK` with `{"status":"ok"}`.
- HTTP server timeouts.
- Reverse proxy forwarding requests to a configured upstream, preserving the method, path, query and body.
- `502 Bad Gateway` when the upstream cannot be reached.

## Run

Requires Go 1.22 or later.

```sh
UPSTREAM_URL=http://localhost:9000 go run ./cmd/rategate
```

Check the server:

```sh
curl -i http://localhost:8080/health
```

`UPSTREAM_URL` is required and must be an absolute HTTP or HTTPS URL without credentials or a fragment. The upstream service must be running separately. An upstream path prefix is prepended to request paths.

All requests except `GET /health` and `HEAD /health` are forwarded to the upstream:

```sh
curl -i 'http://localhost:8080/api/items?page=1'
```
