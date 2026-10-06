# http1.1 — an HTTP/1.1 server written from scratch in Go

An HTTP/1.1 server built on raw TCP sockets (`net.Listen` / `net.Conn`).
It does **not** use `net/http` to serve requests. The project was written
test-first (TDD), follows the SOLID principles, and is packaged as an 11 MB
distroless Docker image.

`net/http` appears in **tests only**. There it acts as an independent
reference client, which shows that our server interoperates with a
widely used HTTP implementation.

---

## Table of contents

1. [Quick start](#quick-start)
2. [Using the server](#using-the-server)
3. [Configuration](#configuration)
4. [Docker](#docker)
5. [Development workflow](#development-workflow)
6. [Architecture](#architecture)
7. [HTTP/1.1 features implemented](#http11-features-implemented)
8. [Concepts learned](#concepts-learned)
9. [Why this is useful](#why-this-is-useful)
10. [Limitations and next steps](#limitations-and-next-steps)

---

## Quick start

Requirements: Go 1.27+. Docker is optional.

```bash
# run directly
go run ./cmd/server

# or build a binary
go build -o bin/server ./cmd/server      # Windows: -o bin/server.exe
./bin/server
```

The server listens on `:8080`. Stop it with `Ctrl+C`; it finishes in-flight
requests before exiting.

```bash
curl http://localhost:8080/
```

## Using the server

The demo application exposes these routes:

| Method   | Path            | Description                                       |
|----------|-----------------|---------------------------------------------------|
| `GET`    | `/`             | Plain-text index of the routes                    |
| `GET`    | `/health`       | Liveness probe → `{"status":"ok"}`                |
| `GET`    | `/hello/{name}` | Greeting that uses a path parameter               |
| `POST`   | `/echo`         | Echoes the request body and its `Content-Type`    |
| `GET`    | `/notes`        | List notes (JSON)                                 |
| `POST`   | `/notes`        | Create a note: `{"text":"..."}` → `201` + `Location` |
| `GET`    | `/notes/{id}`   | Fetch a note                                      |
| `DELETE` | `/notes/{id}`   | Delete a note → `204`                             |

Every `GET` route also answers `HEAD` automatically.

### Examples

```bash
# path parameters
curl -i http://localhost:8080/hello/gopher

# JSON REST resource
curl -i -X POST -d '{"text":"buy milk"}' http://localhost:8080/notes
curl    http://localhost:8080/notes
curl    http://localhost:8080/notes/1
curl -i -X DELETE http://localhost:8080/notes/1

# chunked request body
curl -H "Transfer-Encoding: chunked" -d "streamed body" http://localhost:8080/echo

# Expect: 100-continue (curl sends it for large bodies; force it here)
curl -v -H "Expect: 100-continue" -d "hello" http://localhost:8080/echo

# HEAD: headers only, Content-Length still advertised
curl -I http://localhost:8080/health

# 405 with Allow header
curl -i -X PUT http://localhost:8080/notes

# keep-alive: curl reuses one TCP connection for both requests
curl -v http://localhost:8080/health http://localhost:8080/hello/again

# raw protocol by hand (WSL/Linux)
printf 'GET /hello/raw HTTP/1.1\r\nHost: x\r\nConnection: close\r\n\r\n' | nc localhost 8080
```

### Using the packages in your own code

```go
rt := router.New()
rt.HandleFunc("GET", "/users/{id}", func(w response.Writer, r *request.Request) {
    response.JSON(w, status.OK, map[string]string{"id": r.Param("id")})
})

h := handler.Chain(rt, middleware.Logger(logger), middleware.Recover(logger))
srv := server.New(h)
log.Fatal(srv.ListenAndServe(":8080"))
```

## Configuration

Settings come from environment variables, following the twelve-factor
convention. Invalid values stop the server at startup with a clear error.

| Variable           | Default    | Meaning                                         |
|--------------------|------------|-------------------------------------------------|
| `ADDR`             | `:8080`    | Listen address                                  |
| `READ_TIMEOUT`     | `10s`      | Max time to read request headers and body       |
| `WRITE_TIMEOUT`    | `10s`      | Max time to write a response                    |
| `IDLE_TIMEOUT`     | `60s`      | Max wait for the next request on a keep-alive connection |
| `SHUTDOWN_TIMEOUT` | `15s`      | Grace period for in-flight requests on shutdown |
| `MAX_BODY_BYTES`   | `10485760` | Request body limit (larger bodies get `413`)    |
| `LOG_LEVEL`        | `info`     | `debug`, `info`, `warn` or `error`              |

Logs are structured JSON written to stdout:

```json
{"time":"...","level":"INFO","msg":"request","method":"GET","path":"/health","status":200,"duration":249375}
```

## Docker

The `Dockerfile` has several stages:

| Stage     | Purpose                                                        |
|-----------|----------------------------------------------------------------|
| `base`    | Downloads modules and copies the source                        |
| `test`    | Runs `go test -race ./...` (the race detector needs cgo, available in the Debian image) |
| `build`   | Builds a static, stripped binary (`CGO_ENABLED=0`, `-trimpath -ldflags="-s -w"`) |
| `runtime` | `distroless/static:nonroot`: no shell, no package manager, non-root user, ~11 MB |

From WSL (`/mnt/c/Users/<you>/Desktop/http1.1`):

```bash
# run the test suite with the race detector inside a container
docker build --target test -t http11:test .

# build the production image
docker build --target runtime -t http11:latest .

# run it
docker run -d --name http11 -p 8080:8080 -e LOG_LEVEL=debug http11:latest
curl http://localhost:8080/health

# graceful stop: docker sends SIGTERM, the server drains and exits with code 0
docker stop http11
docker logs http11
docker rm http11
```

## Development workflow

Every commit was checked with these commands:

```bash
go test ./...                               # unit + integration tests
golangci-lint run ./...                     # lint (config: .golangci.yml)
go test -coverprofile=coverage.out ./...    # coverage
go tool cover -html=coverage.out            # browse coverage
```

Install the linter with
`go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest`.
On Windows, `go test -race` needs a C toolchain, so race tests run in the
Docker `test` stage instead.

### TDD cycle used for every feature

1. **Red**: write a failing test that describes the behaviour, and confirm it
   fails for the expected reason (for example `undefined: Parser`).
2. **Green**: write the simplest code that makes it pass.
3. **Refactor**: clean up while the tests stay green, then lint and commit.

One case from the router shows why this matters. The test
`TestRouter_LessSpecificRouteServesMethodMissingOnMoreSpecific` was written
first and failed. It exposed a real bug: with `GET /users/me` and
`POST /users/{id}` registered, `POST /users/me` returned 405 instead of
reaching the parameter route. The test came first, so the fix came with proof.

### Test numbers

About 1,900 lines of production code and 1,800 lines of tests. Total
statement coverage is **~94%**:

| Package      | Coverage | Package      | Coverage |
|--------------|---------:|--------------|---------:|
| `config`     | 100%     | `headers`    | 98%      |
| `handler`    | 100%     | `router`     | 99%      |
| `middleware` | 100%     | `response`   | 96%      |
| `notes`      | 100%     | `request`    | 94%      |
| `status`     | 100%     | `app`        | 94%      |
| `server`     | 90%      | `cmd/server` | 78%      |

## Architecture

### Project layout

```
http1.1/
├── cmd/server/          entrypoint: config → wiring → signals → graceful shutdown
├── internal/
│   ├── headers/         case-insensitive header multimap + field-line parsing
│   ├── status/          status codes, reason phrases, "may this have a body?"
│   ├── request/         Request type + streaming parser (line, headers, body framing, chunked)
│   ├── response/        Writer interface, Buffered implementation, Text/JSON/Error helpers
│   ├── handler/         Handler interface, Func adapter, Middleware + Chain
│   ├── router/          method + path routing with {params}, 404/405, HEAD→GET
│   ├── middleware/      Recover (panic → 500), Logger (structured access log)
│   ├── server/          TCP accept loop, per-connection loop, keep-alive, timeouts, shutdown
│   ├── config/          environment-variable configuration
│   ├── notes/           demo domain: Store interface + in-memory implementation
│   └── app/             composition root: routes + middleware + handlers
├── Dockerfile           multi-stage: test (race) → build (static) → distroless runtime
└── .golangci.yml        lint rules
```

`internal/` is enforced by the Go compiler: code outside this module cannot
import these packages.

### Dependency graph

The arrows point toward abstractions. There are no import cycles, and the
low-level protocol packages know nothing about routing or the application.

```
            cmd/server
                │
     ┌──────────┼───────────┐
     ▼          ▼           ▼
   config      app        server ─────────┐
                │           │             │
   ┌────────┬───┴────┐      │             │
   ▼        ▼        ▼      ▼             ▼
 notes   router  middleware handler ◄── (depends only on the Handler interface)
            │        │      │
            └────────┴──────┤
                            ▼
                 request   response
                    │         │
                    ▼         ▼
                 headers   status
```

### Life of a request

```
TCP bytes ──► server.conn ──► request.Parser ──► handler chain ──► response.Buffered ──► TCP bytes
              (keep-alive     (request line,      Logger            (status line,
               loop,           headers, Host,      └ Recover          sorted headers,
               deadlines)      body framing)         └ Router         exact Content-Length,
                                                       └ handler      Date, body)
```

1. **Accept**: `Server.Serve` accepts a `net.Conn` and starts one goroutine
   per connection.
2. **Await**: the connection is *idle*. It peeks for the first byte with an
   `IdleTimeout` deadline, and `Shutdown` may close it in this state.
3. **Parse**: the connection becomes *active*. Under a `ReadTimeout`
   deadline, the parser reads the request line and headers, then attaches a
   body reader that streams the body lazily: `Content-Length` uses a
   fixed-length reader, chunked encoding uses a chunked decoder, and
   otherwise the body is empty.
4. **Handle**: the middleware chain and router run the handler. The handler
   writes into an in-memory `Buffered` response.
5. **Drain**: any body bytes the handler did not read are discarded, so the
   next pipelined request starts at the right byte.
6. **Respond**: the server decides keep-alive vs. close, sets
   `Connection`, and writes the response under a `WriteTimeout`.
7. **Loop or close**: on keep-alive, go back to step 2.

### How SOLID shows up in the code

| Principle | Where |
|-----------|-------|
| **S**ingle Responsibility | Each package has one reason to change. `headers` knows nothing about requests; the parser does not write responses; `server` does not route. Inside `request`, framing (`body.go`) and chunk decoding (`chunked.go`) are separate. |
| **O**pen/Closed | `handler.Middleware` adds behaviour (logging, recovery) without modifying handlers or the router. New routes are registered, not coded into the server. |
| **L**iskov Substitution | Any `response.Writer` works for handlers, and any `notes.Store` works for the notes handler. Tests rely on this by substituting implementations freely. |
| **I**nterface Segregation | `response.Writer` has 4 methods and `handler.Handler` has 1. Handlers never see `Finish()`, because only the server needs it. |
| **D**ependency Inversion | `server` depends on the `handler.Handler` abstraction, not on the router. `notesHandler` depends on `notes.Store`, not on `MemoryStore`. The clock (`Options.Now`) and environment (`config.Load(lookup)`) are injected, so tests are deterministic. |

### Key design decisions

- **Buffered responses.** Handlers write into memory, and the server sends
  the exact `Content-Length`. This keeps keep-alive framing trivially
  correct, and if a handler panics, a clean 500 can still be sent because
  nothing has reached the wire yet. The trade-off is that very large
  responses are not streamed (see next steps).
- **Lazy, bounded body readers.** The request body is an `io.Reader` and is
  never read eagerly. Size limits are enforced while reading, so the server
  never allocates a body larger than allowed.
- **Fail at startup, not at runtime.** Invalid route patterns, duplicate
  routes and bad configuration panic or error immediately when the server
  starts.
- **Explicit connection state machine (`idle` ↔ `active`).** This is what
  makes graceful shutdown correct: idle connections are closed at once, and
  active ones finish their current request with `Connection: close`.
- **Deterministic output.** Headers are written in sorted order and the clock
  is injectable, so the tests can assert exact bytes.

## HTTP/1.1 features implemented

| Feature | RFC | Notes |
|---|---|---|
| Request line parsing and validation | 9112 §3 | Method token, origin-form or `*` target, version |
| HTTP/1.0 and HTTP/1.1 | 9112 §2.6 | Other versions → `505` |
| Case-insensitive headers, multi-value | 9110 §5 | Canonicalized names (`content-type` → `Content-Type`) |
| Strict field-line syntax | 9112 §5 | Whitespace before the colon is rejected; control characters are rejected |
| Mandatory `Host` for 1.1, single `Host` | 9112 §3.2 | Missing or duplicate → `400` |
| `Content-Length` bodies | 9112 §6 | Identical duplicates allowed; conflicting → `400` |
| `Transfer-Encoding: chunked` | 9112 §7.1 | Extensions and trailers parsed and discarded |
| Request smuggling defence | 9112 §6.3 | Both `TE` and `CL` → `400` |
| Persistent connections (keep-alive) | 9112 §9.3 | 1.1 default-on; 1.0 opt-in via `Connection: keep-alive` |
| Pipelining | 9112 §9.3.2 | Requests processed in order on one connection |
| `Connection: close` | 9112 §9.6 | Honoured and echoed |
| `Expect: 100-continue` | 9110 §10.1.1 | Interim response sent only when the handler reads the body |
| `HEAD` | 9110 §9.3.2 | Automatic from `GET`, no body, correct `Content-Length` |
| `405` + `Allow` | 9110 §15.5.6 | Computed across all matching routes |
| No body for `1xx`/`204`/`304` | 9110 §6.4.1 | Enforced by the writer |
| `Date` header | 9110 §6.6.1 | IMF-fixdate |
| Limits | — | Line length, header count, body size → `431`/`413` |
| Timeouts | — | Read → `408`; idle and write deadlines |
| Graceful shutdown | — | SIGINT/SIGTERM → drain → exit 0 |

## Concepts learned

**Networking and protocol**
- HTTP/1.1 is plain text over TCP. A request is a request line, header
  lines, an empty line, and an optional body.
- **Message framing** is the hard part. TCP is a byte stream with no message
  boundaries, so `Content-Length` or chunked encoding is the only way to know
  where one request ends and the next begins. Getting this wrong breaks
  keep-alive and enables **request smuggling**.
- Keep-alive and pipelining explain why unread bodies must be drained.
- `100 Continue` lets a client avoid uploading a large body that the server
  would reject anyway.
- Defensive parsing: every input is bounded (line length, header count, body
  size, time), because a server talks to untrusted peers.

**Go**
- `net.Listener`/`net.Conn` and a **goroutine per connection**, the
  idiomatic Go concurrency model.
- `bufio.Reader` (`ReadSlice`, `Peek`) for efficient line-oriented parsing.
- Composing `io.Reader`s (fixed-length, chunked, max-bytes, expect-continue
  wrappers) shows how small interfaces combine.
- `sync.Mutex`, `sync.RWMutex` and `sync.WaitGroup` for connection tracking
  and the in-memory store.
- `context.Context` and `signal.NotifyContext` for cancellation and graceful
  shutdown.
- Deadlines (`SetReadDeadline`) instead of per-read timeouts.
- Error wrapping with `%w`, plus `errors.Is`/`errors.As` and sentinel
  errors. The server maps parse errors to status codes with
  `errors.Is`, without string matching.
- `log/slog` structured logging, `encoding/json`, and `internal/` packages.
- Functional middleware (`func(Handler) Handler`) and function adapters
  (`handler.Func`).

**Testing**
- Table-driven tests and `t.Run` subtests.
- Testing at three levels:
  1. **Unit**: pure functions and types (`headers`, `status`, `router`).
  2. **Component**: the full app stack driven by raw HTTP strings, with no
     socket (`internal/app`).
  3. **Integration**: real TCP on `127.0.0.1:0` with `net/http` as an
     independent reference client (`internal/server`, `cmd/server`).
- Dependency injection for determinism: fake clock, fake environment lookup,
  `io.Discard` loggers.
- Testing concurrency and time-based behaviour (shutdown waiting for
  in-flight requests, idle timeouts, slow clients) without flakiness.
- The race detector (`-race`) run in CI-like conditions via Docker.

**Engineering practice**
- TDD's red/green/refactor rhythm.
- Small, conventional commits, with lint and tests checked after each one.
- Multi-stage Docker builds, static binaries, distroless non-root images,
  and handling SIGTERM for orchestrators.

## Why this is useful

- **Learning.** It shows what `net/http`, nginx or any web framework does
  for you. Once you have written framing, keep-alive and graceful shutdown
  yourself, many production issues become easier to diagnose: stuck
  connections, 408/413/431 errors, smuggling advisories, and slow shutdowns
  in Kubernetes.
- **A reference for clean Go.** It is a compact, fully tested example of
  package design, interfaces and dependency injection that you can reuse in
  other projects.
- **Embedded and constrained targets.** Small, dependency-free HTTP stacks
  are common on gateways and ECUs. This code base is small (~1.9k lines),
  has no third-party dependencies, and makes every limit explicit.
- **A testing playground.** The parser is an ideal target for fuzzing and
  for conformance tools.
- **Container-ready.** An 11 MB non-root image with correct signal
  handling suits Docker, Compose and Kubernetes.

## Limitations and next steps

These are intentionally out of scope. Each would make a good next exercise.

- **Streaming responses.** Add a `Flush`-capable writer that switches to
  chunked responses for large or unbounded output (server-sent events,
  downloads).
- **TLS** (`crypto/tls` wrapping the listener) and **HTTP/2**.
- **Absolute-form request targets** (proxies) and the `CONNECT` method.
- **Request context** with per-request cancellation when the client
  disconnects.
- **Fuzzing**: `go test -fuzz=FuzzParse ./internal/request`.
- **Static files** with `Range`, `ETag` and `If-None-Match` → `304`.
- **Persistent store**: implement `notes.Store` with SQLite or Postgres. No
  handler would need to change (Dependency Inversion in practice).
- **Docker health check** and a `docker-compose.yml`.
