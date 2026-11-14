# ATC Ideas

A running list of features to consider for atc. Sizes are rough:
**S** is a day or two, **M** is about a week, and **L** is several weeks or
needs design work first.

## Top 10

Ordered roughly by what unlocks the most and what later items depend on.

| #  | Idea | Size | Why now |
|----|------|------|---------|
| 1  | [Config file (TOML)](#config-file) | M | Everything below needs more structure than flags and env vars can hold. |
| 2  | [Routing to multiple upstreams](#routing) | M | Turns atc from a single-endpoint proxy into a real gateway. |
| 3  | [Timeouts, retries, and a health endpoint](#resilience) | S | Cheap, and expected of any production proxy. |
| 4  | [Declarative header and path rewrites](#request-and-response-transforms) | S | Covers the common cases before scripting exists, and is fast. |
| 5  | [Request IDs and access logs](#observability) | S | Makes everything else debuggable. |
| 6  | [Embedded scripting](#embedded-scripting) | L | The flagship feature: arbitrary logic on requests and responses. |
| 7  | [Admin API](#admin-api-and-ui) | M | The backend for the UI, and useful on its own for automation. |
| 8  | [Admin UI with live traffic view](#admin-api-and-ui) | L | Edit config and watch requests flow through. |
| 9  | [Authentication (API keys, JWT)](#security) | M | Gateways are usually where auth gets enforced. |
| 10 | [Prometheus metrics](#observability) | S | Standard way to see traffic, latency, and errors per route. |

## Configuration

### Config file

**M.** Load routes, upstreams, and settings from a file with
`atc serve -c atc.toml`.

- Viper is already a dependency and reads TOML, YAML, and JSON, so supporting
  all three costs almost nothing extra. TOML is a good default for
  hand-written config.
- Precedence: flags > env vars > config file > defaults.
- Add `atc config validate` to check a file without starting the server.
- Add `atc config print` to show the effective, merged config.

### Hot reload

**M.** Watch the config file (viper supports fsnotify) or reload on `SIGHUP`,
swapping routes in place without dropping in-flight requests. Reject a bad
config and keep the old one running.

### SQLite config store

**M.** Keep config in a SQLite database instead of, or as well as, a file.
This fits well with the admin UI, which needs somewhere writable to save
edits. A file is easy to review in git, while a database is easy for a UI to
change. One approach:

- The file is the source of truth by default; the admin UI is read-only.
- With `--store sqlite:atc.db`, the database is the source of truth, and the
  UI can edit it. `atc config export` / `import` move between the two.
- Store config history in the database so changes can be audited and rolled
  back.
- Use a pure-Go driver (`modernc.org/sqlite`) to keep builds free of cgo.

### Simple line-based config

**S.** An nginx- or Caddyfile-style format, such as
`route /api -> http://localhost:9000`. Pleasant for small setups, but it means
writing and maintaining a parser. Probably not worth it unless TOML feels too
heavy in practice.

## Routing

### Routing to multiple upstreams

**M.** Match requests by path prefix, host, method, or header and send them to
different upstreams.

```toml
[[route]]
match    = { path = "/api/users" }
upstream = "http://users:8080"

[[route]]
match    = { host = "billing.example.com" }
upstream = "http://billing:8080"
strip_prefix = true
```

### Load balancing

**M.** Several targets per upstream with round-robin, least-connections, or
random selection. Pairs with active health checks that take bad targets out
of rotation.

### Weighted and canary routing

**S** (after load balancing). Send a percentage of traffic to a new version,
or route by header or cookie for testing.

### Traffic mirroring

**M.** Copy requests to a second upstream and discard its responses. Useful
for testing a new service against real traffic.

### Protocol support

- **WebSockets.** **S.** `httputil.ReverseProxy` already handles upgrades;
  this needs testing and documenting.
- **HTTP/2 and h2c to upstreams.** **S.**
- **gRPC proxying.** **M.**
- **Server-sent events / streaming.** **S.** Mostly works already, since
  flushing goes through the status recorder; add tests.

## Request and response transforms

### Declarative header and path rewrites

**S.** Handle common cases in config without scripting: add, set, or remove
headers; rewrite or strip path prefixes; add query parameters. Faster than a
script and easier to read.

### Embedded scripting

**L.** Run user scripts on requests before they are forwarded and on responses
before they are returned, to change headers, bodies, status, and routing, or
to reject requests.

```lua
function on_request(req)
  req.headers["X-Tenant"] = lookup_tenant(req.headers["Authorization"])
  if req.path:match("^/admin") then
    return respond(403, "forbidden")
  end
end

function on_response(req, res)
  res.headers["Server"] = nil
end
```

Embeddable engines, all pure Go:

| Engine | Language | Notes |
|--------|----------|-------|
| [gopher-lua](https://github.com/yuin/gopher-lua) | Lua 5.1 | Small, fast, familiar to nginx/OpenResty and Envoy users. |
| [goja](https://github.com/dop251/goja) | JavaScript (ES5.1+, much of ES6) | Most widely known language; heavier per VM. |
| [Starlark](https://github.com/google/starlark-go) | Python-like | Deterministic and sandboxed by design; no `while` loops. |
| [expr](https://github.com/expr-lang/expr) | Expressions only | Good for route matching and conditions, not full scripts. |
| [wazero](https://github.com/tetratelabs/wazero) (WASM) | Any language that compiles to WASM | Most flexible and best isolated; harder for users to write. |

Things to design up front:

- **VM pooling.** Most engines aren't safe to share across goroutines, so keep
  a pool of preloaded VMs.
- **Limits.** Per-script timeouts, and memory limits where the engine
  supports them, so a bad script can't hang the proxy.
- **Bodies.** Changing a body means buffering it, which breaks streaming and
  costs memory. Make body access opt-in per route with a size cap.
- **Sandboxing.** No file system or network access unless explicitly granted.
- **Built-ins.** A small standard library: JSON, base64, logging, a shared
  key-value store, and possibly an HTTP client.
- **Reload.** Scripts should hot-reload with the config.

Lua is a good first choice: it is cheap per request and well proven in
gateways. The hook API can be kept engine-neutral so JavaScript or WASM could
be added later.

### Body transforms without scripting

**M.** JSON field add/remove/rename, and response templating for simple
mock endpoints.

### Compression

**S.** gzip/brotli/zstd responses to clients that accept them.

## Admin API and UI

### Admin API

**M.** A JSON API on a separate port (such as `--admin-port 9901`) so it isn't
exposed with the proxy:

- Read and update config, validate before applying, and roll back.
- Upstream health and per-route stats.
- `/healthz` and `/readyz` for orchestrators.
- Protected by a token or basic auth; bound to localhost by default.

### Admin UI

**L.** A web UI served by atc itself, embedded in the binary with `go:embed`,
built on the admin API:

- View and edit routes, upstreams, scripts, and settings, with validation and
  a diff before saving. Edits need a writable store; see
  [SQLite config store](#sqlite-config-store).
- A script editor with syntax highlighting and a way to try a script against
  a sample request.
- Dashboard with request rate, error rate, and latency per route, updated
  live over SSE.
- All live updates use server-sent events, not WebSockets; see
  [Live traffic view](#live-traffic-view).

### Live traffic view

**M** (on top of the admin UI). Stream requests as they pass through using
server-sent events (SSE): method, path, route, upstream, status, duration, and
optionally headers and bodies.

- **SSE, not WebSockets.** Traffic flows one way, server to browser, which is
  exactly what SSE is for. It is plain HTTP, needs no extra library on either
  side (`net/http` plus the browser's `EventSource`), reconnects on its own,
  and passes through proxies and auth like any other request. Actions the UI
  takes, like saving config, go through ordinary admin API calls.
- Send an event `id` with each request so a reconnecting browser can pass
  `Last-Event-ID` and pick up from the ring buffer without gaps.
- Send a comment line every ~15 seconds as a keepalive so idle connections
  aren't closed by proxies.
- Push other live updates, such as upstream health changes and config
  reloads, over the same SSE connection as named events.

- Filter by route, status, or path.
- Keep the last N requests in a ring buffer so the view isn't empty when
  opened.
- Off by default for bodies, with redaction for headers like `Authorization`
  and `Cookie`.
- **Replay.** **S** (extra). Resend a captured request, optionally edited.

## Observability

- **Request IDs.** **S.** Generate or pass through `X-Request-ID`, include it in
  every log line and forward it upstream.
- **Access logs.** **S.** One structured line per request with route,
  upstream, status, bytes, and duration. Optionally in Common/Combined Log
  Format.
- **Prometheus metrics.** **S.** `/metrics` on the admin port: request counts,
  latency histograms, and upstream errors, labeled by route.
- **OpenTelemetry tracing.** **M.** Create spans and propagate trace context
  to upstreams.

## Resilience

- **Timeouts.** **S.** Connect, response-header, and total timeouts per route.
- **Retries.** **S.** Retry idempotent requests on connection errors with
  backoff.
- **Circuit breaker.** **M.** Stop sending traffic to an upstream that keeps
  failing, then probe it until it recovers.
- **Request size limits.** **S.** Reject bodies over a configured size.
- **Response caching.** **M.** Honor `Cache-Control` with an in-memory cache.

## Security

- **TLS.** **S.** Serve HTTPS from cert files, or automatically through Let's
  Encrypt using `golang.org/x/crypto/acme/autocert`.
- **Authentication.** **M.** API keys, basic auth, and JWT validation (with
  JWKS) per route, passing the identity upstream in a header.
- **OAuth2 / OIDC login.** **L.** For protecting browser apps behind atc.
- **Rate limiting.** **M.** Per client IP, API key, or route, using a token
  bucket. In memory first; Redis later for multiple atc instances.
- **IP allow and deny lists.** **S.**
- **CORS.** **S.** Handle preflight requests and headers in config.
- **mTLS to upstreams.** **S.**

## Developer experience

- **Mock responses.** **S.** Routes that return a fixed status, headers, and
  body with no upstream, for stubbing APIs during development. `listen`
  already covers part of this.
- **Record and replay.** **M.** Save traffic to a file and replay it against
  an upstream, for regression tests.
- **Docker image.** **S.** A small multi-stage image.
- **Release builds.** **S.** GoReleaser for binaries, checksums, and packages
  on tagged releases.
- **Shell completions.** **S.** Cobra generates them; add an
  `atc completion` command and document it.

## Open questions

- Should config edited in the UI be written back to the TOML file, or should
  the UI require the SQLite store?
- Is one script per route enough, or do scripts need to chain like middleware?
- Run as a single instance only, or plan for several instances sharing config
  and rate-limit state?
