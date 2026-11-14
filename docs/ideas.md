[README](../README.md) › Ideas

# Ideas

Feature enhancements and other ideas for `atc`, collected on 2026-11-13
(after v0.0.1, with the `serve` and `listen` commands in place), using
comparable gateways and proxies as reference points: nginx, OpenResty, Caddy,
Traefik, Envoy, Kong, Tyk, KrakenD and Apache APISIX. These are suggestions,
not a release plan. Ideas are removed from this page once they are
implemented; the [README](../README.md) describes those. Check the code before
relying on any detail.

[`examples/gateway.toml`](examples/gateway.toml) sketches a full configuration
for `atc` in its finished form, using many of these ideas together. It is a
design sketch, not a working config.

## How to read this page

Each idea has an ID to refer to it by (for example `CF-1`), an effort flag and
a short description. Where another tool has the feature, it is named as a
reference point.

| Flag | Effort | Roughly |
|------|--------|---------|
| **S** | Small | A day or two. One package, no new concepts, little doc change |
| **M** | Medium | Up to a week. New command or config section, several packages, new docs and tests |
| **L** | Large | More than a week. New concept or subsystem, design decisions to make first |

Areas:

| Prefix | Area |
|--------|------|
| [`CF`](#configuration) | Configuration |
| [`RO`](#routing) | Routing |
| [`TR`](#transforms-and-scripting) | Transforms and scripting |
| [`UI`](#admin-api-and-ui) | Admin API and UI |
| [`OB`](#observability) | Observability |
| [`RS`](#resilience) | Resilience |
| [`SE`](#security) | Security |
| [`PR`](#protocols) | Protocols |
| [`TE`](#testing-and-development) | Testing and development |
| [`DI`](#distribution) | Distribution |

## Top picks

The ideas with the best value for their effort, in suggested order. Earlier
picks are groundwork for later ones.

| ID | Idea | Effort | Why |
|----|------|--------|-----|
| [`CF-1`](#configuration) | Config file | M | Routes, scripts and auth need more structure than flags and env vars can hold; viper, already a dependency, reads TOML, YAML and JSON |
| [`RO-1`](#routing) | Routing to multiple upstreams | M | Turns `atc` from a single-endpoint proxy into a gateway; most other ideas attach settings to a route |
| [`TR-6`](#transforms-and-scripting) | Named middleware chains | M | The shape that auth, rate limits, caching and scripts plug into; settling it early keeps those features consistent |
| [`RS-1`](#resilience) | Timeouts | S | Cheap, and without them one slow upstream can tie up the proxy |
| [`TR-1`](#transforms-and-scripting) | Header and path rewrites | S | Covers the common transforms in config, before scripting exists, and runs faster than a script |
| [`OB-1`](#observability) | Request IDs | S | Ties a request's log lines together across `atc` and its upstreams, which makes every later feature easier to debug |
| [`TR-2`](#transforms-and-scripting) | Embedded scripting | L | The flagship feature: arbitrary logic on requests and responses without rebuilding `atc` |
| [`UI-1`](#admin-api-and-ui) | Admin API | M | The backend the admin UI needs, and useful on its own for automation and health checks |
| [`UI-3`](#admin-api-and-ui) | Live traffic view | M | Shows requests as they pass through; `listen` already proves the per-request details are cheap to collect |
| [`OB-3`](#observability) | Prometheus metrics | S | The standard way to watch traffic, latency and errors per route, and the data source for the UI dashboard |

## Configuration

Today `atc` is configured by flags and `ATC_*` environment variables. These
ideas add structured config while keeping flags and env vars as overrides.

| ID | Idea | Effort | Description |
|----|------|--------|-------------|
| `CF-1` | Config file | **M** | `atc serve -c atc.toml` loads routes, upstreams and settings. Viper reads TOML, YAML and JSON, so all three cost little; TOML is the documented default. Precedence: flags, then env vars, then the file, then defaults. Caddy, Traefik and Envoy all use file config. |
| `CF-2` | Config commands | **S** | `atc config validate` checks a file and reports every error with its location without starting the server; `atc config print` shows the effective config after merging flags, env vars and the file. |
| `CF-3` | Hot reload | **M** | Reload on file change (viper wraps fsnotify) or `SIGHUP`, swapping routes in place without dropping in-flight requests. A config that fails validation is rejected and the old one keeps running. Caddy and Traefik reload this way. |
| `CF-4` | SQLite config store | **M** | `--store sqlite:atc.db` keeps config in a database the admin UI can write to, using a pure-Go driver (`modernc.org/sqlite`) so builds stay free of cgo. Kong (Postgres) and APISIX (etcd) keep config in a store. Decide first whether UI edits can also be written back to a TOML file, or whether editing in the UI requires the database. |
| `CF-5` | Config history and rollback | **M** | Keep every saved version in the SQLite store with who changed it and when, show diffs, and roll back to any version. Needs `CF-4`. |
| `CF-6` | Import and export | **S** | `atc config export` and `atc config import` move config between a TOML file and the SQLite store, so a database-backed setup can still be reviewed in git. |
| `CF-7` | Line-based config | **M** | A short format in the style of a Caddyfile or `nginx.conf`, such as `route /api -> http://localhost:9000`. Pleasant for small setups, but needs its own parser and error messages. Worth it only if TOML proves too heavy in practice. |
| `CF-8` | Include files | **S** | `include = ["conf.d/*.toml"]` merges more files, sorted by name, so teams can own their own routes and upstreams. Duplicate names are an error that names both files. |
| `CF-9` | Secret references | **S** | `"${NAME}"` reads an environment variable and `"${file:/path}"` reads a file, so keys and passwords stay out of the config (Docker and Kubernetes mount secrets as files). Secrets are redacted in `atc config print` and the admin UI. |

## Routing

`serve` forwards everything to one endpoint. These ideas choose among
upstreams.

| ID | Idea | Effort | Description |
|----|------|--------|-------------|
| `RO-1` | Routing to multiple upstreams | **M** | Match requests by path prefix, host, method or header and send them to different upstreams, with the first or most specific match winning. Every gateway listed above has this. |
| `RO-2` | Load balancing | **M** | Several targets per upstream with weights and round-robin, least-connections, random, IP-hash or consistent-hash selection (the last keeps a user on one target, for long-lived connections). |
| `RO-3` | Active health checks | **M** | Probe each target on an interval and take failing ones out of rotation until they recover. Pairs with `RO-2`. Traefik, Envoy and nginx Plus have this. |
| `RO-4` | Weighted and canary routing | **S** | Send a percentage of traffic to a new version, or route by header or cookie for testing. Needs `RO-2`. |
| `RO-5` | Traffic mirroring | **M** | Copy a percentage of requests to a second upstream and discard its responses, to test a new service against real traffic, optionally comparing the two responses in the admin UI. Envoy and nginx (`mirror`) have this. |
| `RO-6` | Service discovery | **M** | Find an upstream's targets from DNS SRV records, refreshed on an interval, so instances can come and go without a config change. Works with Consul and Kubernetes headless services; native Consul and Kubernetes APIs could follow. Traefik and Envoy discover services this way. |
| `RO-7` | Sticky sessions | **S** | Keep a client on the target or canary version it first got, using a cookie with a lifetime. Needs `RO-2` or `RO-4`. |
| `RO-8` | Response aggregation | **L** | One route calls several upstreams in parallel and merges their JSON responses under named keys, with a deadline, field filtering and optional parts that may fail without failing the request. Saves mobile clients several round trips. KrakenD is built around this. |
| `RO-9` | Redirects | **S** | A route that answers with a `301`, `302`, `307` or `308` to a target built from placeholders, for moved paths and old API versions. |
| `RO-10` | Static files | **S** | Serve a directory, with an index file, cache headers for fingerprinted assets and a single-page-app fallback to `index.html`, so a web app and its API can share one gateway. Caddy and nginx do this. |

## Transforms and scripting

| ID | Idea | Effort | Description |
|----|------|--------|-------------|
| `TR-1` | Header and path rewrites | **S** | Add, set or remove request and response headers, strip or rewrite path prefixes, and add query parameters, all per route in config. Values can use placeholders such as `{client_ip}`, `{param.id}` or `{claim.sub}`. |
| `TR-2` | Embedded scripting | **L** | `on_request` and `on_response` hooks that can change headers, bodies, status and the chosen upstream, or answer directly (for example a `403`). Candidates, all pure Go: [gopher-lua](https://github.com/yuin/gopher-lua) (Lua 5.1; small and fast, familiar from OpenResty, Kong and Envoy), [goja](https://github.com/dop251/goja) (JavaScript; best known, heavier per VM, used the way Tyk uses JS) and [Starlark](https://github.com/google/starlark-go) (Python-like, sandboxed by design). Lua is the suggested first engine, behind an engine-neutral hook API. Design first: a pool of preloaded VMs (most engines aren't safe to share across goroutines), per-script timeouts and memory limits, no file or network access unless granted, a small standard library (JSON, base64, logging), reload with the config, and whether a route runs one script or a chain like middleware. Body access means buffering, which breaks streaming, so it should be opt-in per route with a size cap. |
| `TR-3` | WASM filters | **L** | Run filters compiled to WebAssembly through [wazero](https://github.com/tetratelabs/wazero), so users can write them in any language that targets WASM, with stronger isolation than an embedded interpreter. Envoy and APISIX support proxy-wasm filters. An alternative or later addition to `TR-2`. |
| `TR-4` | JSON body transforms | **M** | Add, remove or rename fields in JSON request and response bodies from config, without a script. KrakenD does this. Shares the buffering limits of `TR-2`. |
| `TR-5` | Compression | **S** | gzip, brotli or zstd responses for clients that accept them, with a minimum size and a list of content types. |
| `TR-6` | Named middleware chains | **M** | Define steps once (`[middleware.cors]`, `[middleware.user-auth]`) and list them by name on each route, with a default chain every route runs first. Requests pass through in order and responses in reverse. Auth, rate limits, caching and scripts become middleware types rather than separate route settings. Traefik's middlewares and Kong's plugins work this way. |
| `TR-7` | Uniform error responses | **S** | Errors `atc` generates itself (`401`, `429`, `502`, `504`, ...) use RFC 9457 problem details (`application/problem+json`), so clients get one error shape from every service, without leaking upstream error text. |

## Admin API and UI

Live updates in the admin UI use server-sent events (SSE), not WebSockets
(see [Not planned](#not-planned)).

| ID | Idea | Effort | Description |
|----|------|--------|-------------|
| `UI-1` | Admin API | **M** | A JSON API on a separate port (`--admin-port`), bound to localhost by default and protected by a token or basic auth: read, validate, apply and roll back config, and report upstream health and per-route stats. Envoy, Caddy and Traefik each have an admin API. |
| `UI-2` | Admin UI config editor | **L** | A web UI embedded in the binary with `go:embed`, built on `UI-1`: view and edit routes, upstreams, scripts and settings, with validation and a diff before saving. Editing needs a writable store; see `CF-4`. |
| `UI-3` | Live traffic view | **M** | Stream each request over SSE as it passes through: method, path, route, upstream, status, duration and, optionally, headers and bodies. Keep the last N requests in a ring buffer so the view isn't empty when opened, and give each event an `id` so a reconnecting browser resumes with `Last-Event-ID` without gaps. Send a keepalive comment about every 15 seconds, and redact headers such as `Authorization` and `Cookie`. Filter by route, status or path. |
| `UI-4` | Live dashboard | **M** | Request rate, error rate and latency per route, updated over the same SSE connection as `UI-3`, along with named events for upstream health changes and config reloads. Uses the counters from `OB-3`. |
| `UI-5` | Script editor and playground | **M** | Edit a `TR-2` script with syntax highlighting and run it against a sample or captured request to see the result before saving. |
| `UI-6` | Replay a request | **S** | Resend a request from the traffic view, optionally edited, through the proxy. |

## Observability

| ID | Idea | Effort | Description |
|----|------|--------|-------------|
| `OB-1` | Request IDs | **S** | Generate an `X-Request-ID` or pass through the client's, include it in every log line and forward it upstream. |
| `OB-2` | Access logs | **S** | One structured line per request with route, upstream, status, bytes and duration (extending today's `proxied` line), optionally in Common or Combined Log Format for existing log tools. |
| `OB-3` | Prometheus metrics | **S** | `/metrics` on the admin port: request counts, latency histograms and upstream errors, labeled by route and upstream. |
| `OB-4` | OpenTelemetry tracing | **M** | Create a span per request and propagate W3C trace context to upstreams. |
| `OB-5` | Health and readiness endpoints | **S** | `/healthz` and `/readyz` on the admin port for Kubernetes and load balancers; ready only after config has loaded. |

## Resilience

| ID | Idea | Effort | Description |
|----|------|--------|-------------|
| `RS-1` | Timeouts | **S** | Connect, response-header and total timeouts, globally and per route. `serve` has only a read-header timeout today. |
| `RS-2` | Retries | **S** | Retry idempotent requests on connection errors, with backoff and a retry budget so retries can't multiply an outage. |
| `RS-3` | Circuit breaker | **M** | Stop sending traffic to an upstream that keeps failing, then let a few requests through until it recovers. Envoy calls this outlier detection. |
| `RS-4` | Request size limits | **S** | Reject request bodies over a configured size with `413`, globally and per route. |
| `RS-5` | Response caching | **M** | An in-memory cache that honors `Cache-Control`, with per-route rules, `stale-while-revalidate`, and `stale-if-error` to keep serving when an upstream is down. nginx (`proxy_cache`) and Varnish are the reference points. |
| `RS-6` | Concurrency limits and queueing | **S** | Cap in-flight requests to a fragile upstream; extra requests wait in a short queue and get a `503` after a timeout, so a spike can't knock a legacy service over. |

## Security

| ID | Idea | Effort | Description |
|----|------|--------|-------------|
| `SE-1` | TLS | **S** | Serve HTTPS from certificate files, or get certificates automatically from Let's Encrypt with `golang.org/x/crypto/acme/autocert`, as Caddy does. |
| `SE-2` | API keys and basic auth | **S** | Per-route API keys (header or query) and basic auth, passing the caller's identity upstream in a header and removing the key before forwarding. An `any` combinator accepts either a user token or a key. |
| `SE-3` | JWT validation | **M** | Validate bearer tokens against a JWKS URL, check issuer, audience, expiry and required scopes, and forward selected claims as headers. Kong, Tyk and KrakenD all do this. |
| `SE-4` | OAuth2 and OIDC login | **L** | Log browser users in through an identity provider and protect apps behind `atc` with a session cookie, as oauth2-proxy does. |
| `SE-5` | Rate limiting | **M** | Token-bucket limits per client IP, API key or route, answering `429` with `Retry-After` and the IETF `RateLimit` and `RateLimit-Policy` headers. In memory first; shared state (for example Redis) only if several `atc` instances need common limits. |
| `SE-6` | IP allow and deny lists | **S** | CIDR lists per route, aware of `X-Forwarded-For` from trusted proxies. |
| `SE-7` | CORS | **S** | Answer preflight requests and add CORS headers from per-route config. |
| `SE-8` | mTLS to upstreams | **S** | Client certificates and a custom CA for upstream connections. |
| `SE-9` | Consumers and plans | **L** | Named API consumers with keys, the routes and networks each may use, and a plan setting their rate limit and monthly quota. Quotas outlive restarts, so their counters need the SQLite store (`CF-4`) or Redis. Usage per consumer feeds billing and the admin UI. Kong consumers and Tyk policies are the reference points. |

## Protocols

| ID | Idea | Effort | Description |
|----|------|--------|-------------|
| `PR-1` | WebSocket passthrough | **S** | `httputil.ReverseProxy` already handles upgrade requests; add tests and document it. This is about proxying client traffic, not the admin UI. |
| `PR-2` | HTTP/2 to upstreams | **S** | HTTP/2 over TLS and h2c (cleartext) to upstreams that support it. |
| `PR-3` | gRPC proxying | **M** | Route gRPC by service and method, with trailers and streaming intact. Needs `PR-2`. |
| `PR-4` | Streaming passthrough tests | **S** | Server-sent events and chunked responses should already stream, since the status recorder passes flushes through; add tests so it stays that way. |
| `PR-5` | HTTP/3 | **M** | Accept HTTP/3 over QUIC on the TLS listener (with `quic-go`), advertised through `Alt-Svc`. Caddy and Traefik support it. |

## Testing and development

| ID | Idea | Effort | Description |
|----|------|--------|-------------|
| `TE-1` | Fixed responses | **S** | A route that answers with a configured status, headers and body without an upstream, for stubbing an endpoint or a maintenance page. For anything richer, run `mock` as the upstream. |
| `TE-2` | Record and replay | **M** | Save proxied traffic to a file and replay it against an upstream later, for regression tests. |
| `TE-3` | Embeddable handler | **S** | Expose the proxy as an `http.Handler` with a small, stable Go API, so programs and tests can run `atc` routing in-process. |

## Distribution

| ID | Idea | Effort | Description |
|----|------|--------|-------------|
| `DI-1` | Container image | **S** | A small multi-stage image published to a registry, with the config file mounted as a volume. |
| `DI-2` | Release builds | **S** | GoReleaser for binaries, checksums and packages on tagged releases, stamping the version the way the Makefile does. |
| `DI-3` | Document shell completions | **S** | Cobra already provides `atc completion bash\|zsh\|fish\|powershell`; document installing them. |
| `DI-4` | systemd unit | **S** | A sample unit file with sandboxing options and reload on `SIGHUP` (see `CF-3`). |

## Not planned

Decisions already made. Listed so they are not proposed again without a new
reason.

- **WebSockets for the admin UI.** Live updates go one way, server to
  browser, which is what SSE is for. SSE is plain HTTP, needs no extra
  library on either side (`net/http` and the browser's `EventSource`),
  reconnects by itself and passes through proxies and auth like any other
  request. Actions the UI takes go through ordinary `UI-1` API calls.
  Proxying WebSocket traffic for clients (`PR-1`) is a separate matter.
