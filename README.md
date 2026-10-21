# RateGate

[English](README.md) | [Русский](README.ru.md)

RateGate is an HTTP gateway project written in Go.

## Features

- HTTP server listening on port 8080.
- `GET /health` returns `200 OK` with `{"status":"ok"}`.
- HTTP server timeouts.
- Graceful shutdown on Ctrl+C or SIGTERM, allowing active requests to finish.
- Reverse proxy forwarding requests to a configured upstream, preserving the method, path, query and body.
- `502 Bad Gateway` when the upstream cannot be reached.
- Upstream request deadlines with `504 Gateway Timeout` if the deadline expires before response headers arrive.
- Configurable per-key token bucket rate limiting, defaulting to 10 requests per second with a burst of 10.
- `429 Too Many Requests` with `Retry-After` when a key exceeds its limit.

## Run

Requires Go 1.22 or later.

```sh
UPSTREAM_URL=http://localhost:9000 go run ./cmd/rategate
```

Set the rate and burst capacity at startup:

```sh
UPSTREAM_URL=http://localhost:9000 RATE_LIMIT=5 RATE_BURST=20 go run ./cmd/rategate
```

`RATE_LIMIT` is the number of tokens replenished per second. `RATE_BURST` is the maximum token count, also available immediately for a new key. Each request consumes one token. Both settings default to `10` when unset or empty and accept integers from `1` to `1000000`. Invalid values stop the server at startup.

`UPSTREAM_TIMEOUT` sets the total upstream request deadline, defaulting to `5s` when unset or empty. It accepts positive Go durations up to `1m`, for example `500ms` or `10s`. Invalid values stop the server at startup.

```sh
UPSTREAM_URL=http://localhost:9000 UPSTREAM_TIMEOUT=3s go run ./cmd/rategate
```

The deadline includes sending the request and reading the response body. If it expires after response headers have been sent to the client, the response is interrupted; its status cannot be changed to `504`. Client cancellation also cancels the upstream request.

On Ctrl+C or SIGTERM, RateGate closes its listeners and idle connections, then waits for active requests to finish. The shutdown grace period is `UPSTREAM_TIMEOUT` plus 5 seconds. If it expires, remaining connections are closed and the process exits with an error.

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

Limits are stored in memory per process and reset on restart. Idle keys are removed lazily after at least one minute, once their budgets would be fully replenished. Multiple instances do not share limits.

## Validation

```sh
go test -race ./...
go vet ./...
go build ./cmd/rategate
```

GitHub Actions checks formatting, tests, race detection and compilation on Go 1.22 and 1.26. It also builds the Docker image and checks container startup, proxying, the non-root user and graceful shutdown.

## Docker

Build the image from the repository root:

```sh
docker build -t rategate .
```

Run with an upstream service on the Docker host:

```sh
docker run --rm --name rategate \
  -p 127.0.0.1:8080:8080 \
  --add-host=host.docker.internal:host-gateway \
  --stop-timeout=70 \
  -e UPSTREAM_URL=http://host.docker.internal:9000 \
  -e RATE_LIMIT=10 \
  -e RATE_BURST=10 \
  -e UPSTREAM_TIMEOUT=5s \
  rategate
```

`localhost` inside the container refers to the container itself. On native Linux, the host service must listen on an address reachable from Docker, rather than only `127.0.0.1`. For an upstream container, attach both containers to the same Docker network and use its container name in `UPSTREAM_URL`.

The final image contains a static binary and CA certificates for HTTPS upstreams, and runs as a non-root user. Go is needed only during image building. The 70-second stop timeout allows the maximum configured shutdown grace period of 65 seconds.

Stop gracefully from another terminal:

```sh
docker stop rategate
```
