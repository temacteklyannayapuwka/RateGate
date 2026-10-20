# RateGate

RateGate is an HTTP gateway project written in Go.

## Features

- HTTP server listening on port 8080.
- `GET /health` returns `200 OK` with `{"status":"ok"}`.
- HTTP server timeouts.
- Reverse proxy forwarding requests to a configured upstream, preserving the method, path, query and body.
- `502 Bad Gateway` when the upstream cannot be reached.
- Per-key token bucket rate limiting: 10 requests per second with a burst of 10.
- `429 Too Many Requests` with `Retry-After` when a key exceeds its limit.

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

All requests except `GET /health` and `HEAD /health` require an `X-API-Key` header and pass through the rate limiter before forwarding:

```sh
curl -i -H 'X-API-Key: demo-client' 'http://localhost:8080/api/items?page=1'
```

Keys identify independent request budgets; they are not validated credentials and do not provide authentication. Missing, blank or longer than 256-byte keys return `400 Bad Request`. Leading and trailing whitespace is ignored. Health checks bypass the limiter.

Limits are stored in memory per process and reset on restart. Idle keys are removed lazily after at least one minute. Multiple instances do not share limits.
