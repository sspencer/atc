# ATC - API Gateway

ATC is an API gateway written in Go. It sits in front of your backend
services and routes incoming API traffic to them, acting as a single entry
point for clients.

> **Status:** early development. The CLI scaffold is in place; gateway
> functionality is still being built. Today the only command is `version`.

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
```

### Configuration

Any command-line flag can also be set with an environment variable. Use the
`ATC_` prefix, upper-case the flag name, and replace dashes with underscores.
For example, a `--listen-addr` flag can be set with:

```sh
export ATC_LISTEN_ADDR=:8080
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
