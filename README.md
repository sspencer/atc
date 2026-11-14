# ATC - API Gateway

ATC is an API gateway written in Go. It sits in front of your backend
services and routes incoming API traffic to them, acting as a single entry
point for clients.

> **Status:** early development. The CLI scaffold is in place; gateway
> functionality is still being built. Today the commands are `version`,
> `serve`, a reverse proxy to a single endpoint, and `listen`, a request logger
> for debugging.

## Requirements

- Go 1.27 or newer
- `make` (optional, for the convenience targets below)
- `git` (used to stamp the version into builds)

## Installation

Install the latest version with `go install`:

```sh
go install github.com/sspencer/atc@latest
```

Or build from source:

```sh
git clone https://github.com/sspencer/atc.git
cd atc
make build        # produces bin/atc
```

`make install` builds and installs `atc` into your `GOBIN`.

## Usage

```sh
atc --help        # list available commands
atc version       # print the atc version
atc serve -p 8080 -e http://localhost:9000  # proxy port 8080 to localhost:9000
atc listen -p 9000  # log and describe every HTTP request received on port 9000
```

### `serve`

`atc serve` runs an HTTP reverse proxy. It listens on `-p`/`--port` (default
8080, or `ATC_PORT`) and forwards every request to the endpoint given with
`-e`/`--endpoint` (or `ATC_ENDPOINT`), which is required and must be an
`http://` or `https://` URL.

The request path and query string are appended to the endpoint, so with
`-e https://api.example.com/v1` a request for `/users?id=7` is forwarded to
`https://api.example.com/v1/users?id=7`. The proxy sets `X-Forwarded-For`,
`X-Forwarded-Host`, and `X-Forwarded-Proto`. Each request is logged with its
status and duration. If the endpoint can't be reached, the client gets
`502 Bad Gateway` and the error is logged. Stop it with Ctrl-C.

```
$ atc serve -p 8080 -e http://localhost:9000
time=2026-11-13T22:21:41.200-08:00 level=INFO msg=proxying endpoint=http://localhost:9000
time=2026-11-13T22:21:41.200-08:00 level=INFO msg=listening addr=:8080
time=2026-11-13T22:21:41.226-08:00 level=INFO msg=proxied method=POST path="/items?x=2" status=200 duration=536.016µs
```

Running `atc listen` on the endpoint's port is an easy way to see what the
proxy forwards.

### `listen`

`atc listen` is a debugging aid for checking that atc forwards traffic. It
accepts any HTTP request, logs the method, path (with query string), and body,
and replies `200 OK` with a JSON description of the request (below). Bodies
over 64 KiB are truncated in the log. The port defaults to 8080 and can also
be set with `ATC_PORT`. Stop it with Ctrl-C.

```
$ atc listen -p 9000
time=2026-11-13T17:35:41.047-08:00 level=INFO msg=listening addr=:9000
time=2026-11-13T17:35:41.110-08:00 level=INFO msg=request method=POST path="/api/items?x=2" body="{\"a\":1}"
```

Requests whose body was truncated are logged with `truncated=true`.

Each reply is a JSON description of the request. It
includes the method, URL, path, parsed query, protocol, host, remote address,
TLS flag, headers, declared `content_length`, actual `body_size`, the body, and
the time it was received. Bodies that aren't valid UTF-8 are returned in
`body_base64` instead of `body`. Echoed bodies are capped at 64 KiB
(`body_truncated: true`), but `body_size` always reports the full size.

```
$ atc listen -p 9000 &
$ curl -s -d '{"a":1}' 'localhost:9000/items?x=2'
{
  "method": "POST",
  "url": "/items?x=2",
  "path": "/items",
  "query": { "x": ["2"] },
  "proto": "HTTP/1.1",
  "host": "localhost:9000",
  "remote_addr": "[::1]:53518",
  "tls": false,
  "headers": { "Content-Length": ["7"], "User-Agent": ["curl/8.23.0"], ... },
  "content_length": 7,
  "body_size": 7,
  "body": "{\"a\":1}",
  "received_at": "2026-11-13T22:27:39.861876329-08:00"
}
```

Put it behind `atc serve` to see exactly what the proxy forwards, including
the `X-Forwarded-*` headers it adds.

### Global flags

| Flag           | Env var          | Default | Description                         |
|----------------|------------------|---------|-------------------------------------|
| `--log-format` | `ATC_LOG_FORMAT` | `text`  | Log output format: `text` or `json` |

Logs are structured, written with Go's `log/slog`. For example,
`atc --log-format json listen` emits one JSON object per line.

### Configuration

Any command-line flag can also be set with an environment variable. Use the
`ATC_` prefix, upper-case the flag name, and replace dashes with underscores.
For example, `--log-format` can be set with:

```sh
export ATC_LOG_FORMAT=json
```

Flags given on the command line take precedence over environment variables.

## Versioning

The version is derived from git rather than hardcoded. In order of preference,
`atc version` reports:

1. The value stamped in at build time (`make build` / `make install` use
   `git describe --tags --always --dirty`).
2. The module version recorded by Go, e.g. when installed with
   `go install github.com/sspencer/atc@v1.2.3`.
3. `dev`, if neither is available.

## Development

Run `make help` to see all targets. The common ones:

| Target       | Description                                  |
|--------------|----------------------------------------------|
| `make build` | Build the `atc` binary into `bin/`           |
| `make run`   | Build and run; pass arguments with `ARGS=""` |
| `make test`  | Run tests with the race detector             |
| `make vet`   | Run `go vet`                                 |
| `make fmt`   | Format source with `gofmt`                   |
| `make check` | Run vet and tests, and verify formatting     |
| `make tidy`  | Tidy `go.mod` and `go.sum`                   |
| `make clean` | Remove build artifacts                       |

Example:

```sh
make run ARGS="version"
```

### Project layout

```
.
├── main.go          # entry point
├── commands/        # cobra commands (root, version, serve, listen, ...)
└── Makefile         # build, test, and lint targets
```
