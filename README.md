# ATC - API Gateway

ATC is an API gateway written in Go. It sits in front of your backend
services and routes incoming API traffic to them, acting as a single entry
point for clients.

> **Status:** early development. The CLI scaffold is in place; gateway
> functionality is still being built. Today the commands are `version` and
> `listen`, a request logger for debugging.

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
atc listen -p 9000  # log every HTTP request received on port 9000
```

### `listen`

`atc listen` is a debugging aid for checking that atc forwards traffic. It
accepts any HTTP request, logs the method, path (with query string), and body,
and replies `200 OK`. Bodies over 64 KiB are truncated in the log. The port
defaults to 8080 and can also be set with `ATC_PORT`. Stop it with Ctrl-C.

```
$ atc listen -p 9000
time=2026-11-13T17:35:41.047-08:00 level=INFO msg=listening addr=:9000
time=2026-11-13T17:35:41.110-08:00 level=INFO msg=request method=POST path="/api/items?x=2" body="{\"a\":1}"
```

Requests whose body was truncated are logged with `truncated=true`.

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
├── commands/        # cobra commands (root, version, ...)
└── Makefile         # build, test, and lint targets
```
