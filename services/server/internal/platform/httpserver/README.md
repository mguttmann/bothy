# Package `platform/httpserver`

Status: listener part built (M1-11); M1-18 adds the middleware chain, the problem format and
the per-route deadlines, M1-21 the plaintext peer check in `Listen`. Normative are
[02-architecture.md](../../../../../docs/spec/02-architecture.md) section "Request path" and
[16-operations.md](../../../../../docs/spec/16-operations.md) section "Two listeners"; the chapters
win on any difference.

## Interface

- `Listen(ctx, addr)`: the one place where the server opens a listener.
- `Config`: `http.read_header_timeout`, `http.idle_timeout`, `http.max_header_bytes`, bound by
  `internal/wire`.
- `NewServer(cfg, handler, log)`: an `http.Server` with these limits and
  `ErrorLog = otel.ServerErrorLog(log)`, so lines of net/http never carry a client address
  (19-security.md SEC-LOG-5). No `ReadTimeout` or `WriteTimeout`: per-route deadlines come
  with M1-18.
- `Internal(registry)`: the mux of the internal listener with `/healthz` and `/readyz`; later
  tickets may add stack-internal webhook routes before the listener starts.
- `Public(mux)`: the handler of the public listener; a request no route matches gets `404`
  with an empty body until M1-18 replaces the fallback.

## Code

| File | Content |
|---|---|
| `httpserver.go` | `Config`, `Listen`, `NewServer`, `Internal`, `Public` |

## Dependencies

`platform/health` and `platform/otel`.
