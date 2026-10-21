# Package `platform/egress`

Status: built (M1-22). The normative sources are
[19-security.md](../../../../../docs/spec/19-security.md) section 8 "SSRF ban and outgoing
connections" (SEC-NET-1 to SEC-NET-5), SEC-TLS-4, R-SE-04 and R-SE-10, and the
[configuration reference](../../../../../docs/operations/configuration.md) (section 2, row
"Egress target"); if this file differs, the chapter applies.

## Purpose

The only way the server opens outgoing HTTP connections. A target that neither an entry in
`egress.allowed_hosts` nor the consumer allows is rejected before any connection and without a
DNS query; the IP address is checked after resolution during connection setup, and only the
checked address is connected. Redirects, time, response size and TLS are limited or always
checked.

## Code

| File | Content |
|---|---|
| `rule.go` | `Rule`, `ParseRule`: entries `host[:port][;allow_private]` from `egress.allowed_hosts` |
| `addr.go` | address check: always blocked ranges and private ranges (only with `allow_private`) |
| `client.go` | `Config`, `Options`, `HostPolicy`, `Deps`, `New`, `Client.Do`, `Client.HTTPClient`: check before the connection, dialer with address check, redirects, limits, log and counter |
| `loopback.go` | `NewLoopback`: client for `bothy healthcheck`, only loopback and `http` |
| `errors.go` | `ErrBlocked`, `ErrResponseTooLarge`, `BlockedError`, `Reason` |
| `doc.go` | package description with all rules and a consumer example |

## Interface

- `New(cfg, opts, deps)` builds one client per consumer: `Config` from `egress.*` (built by
  `internal/wire` with `EgressConfig`), `Options` with purpose, host policy of the consumer,
  redirect mode and own limits of a domain chapter, `Deps` with resolver, root certificates,
  logger and counter.
- `Options.HTTP2` lets a TLS connection negotiate HTTP/2 by ALPN (`ForceAttemptHTTP2`), for
  services that accept only HTTP/2 such as APNs (brief M3-20); plain `http` stays HTTP/1.1 and
  every check applies unchanged.
- `Client.Do(req)` sends; `Client.HTTPClient()` returns an `*http.Client` for libraries that has
  the same checks in its transport.
- `errors.Is(err, ErrBlocked)` recognizes every rejection; `*BlockedError` names reason and host.
  A body above the limit returns `ErrResponseTooLarge` when read.
- `NewLoopback(timeout)` for the health check.

## Rules

1. Only targets from `egress.allowed_hosts` and from the host policy of the consumer
   (`Options.Hosts`) are allowed; everything else ends with `host_not_allowed` without a DNS
   query. An entry without a port allows only the default port of the scheme.
2. `https` always, `http` only with a matching entry with `allow_private`. URLs with a user part
   are rejected.
3. After resolution, only addresses that pass the check are chosen; mapped IPv6 addresses are
   unpacked first. The ranges from SEC-NET-3 are always blocked or blocked except with
   `allow_private`. `net.Dialer.Control` checks the socket address a second time.
4. `allow_private` exists only in `egress.allowed_hosts` (operator); the host policy of a consumer
   never opens private addresses or `http`.
5. Redirects: `RedirectNever` returns the 3xx response, `RedirectChecked` follows only checked
   targets, at most `maxRedirects` (the redirect limit of net/http,
   [defaultCheckRedirect](https://github.com/golang/go/blob/62cd170026b7f9054b7b58ae27136c36fcd8b15c/src/net/http/client.go#L839-L844), retrieved 2026-10-12). A redirect with an unreadable `Location` ends in both modes with an error
   that does not quote the header. A response `101` (protocol switch) is always an error, because
   no deadline would limit reading the switched connection.
6. No environment proxies; TLS from version 1.2, certificate and host name always verified.
7. Timeout `egress.timeout` (or the consumer's value) for the whole request across all redirects,
   also for resolution and connection setup, which net/http detaches from the cancellation of the
   request ("Detach from the request context's cancellation signal", `Transport.getConn` in
   [net/http transport.go of Go 1.27.2](https://github.com/golang/go/blob/go1.27.2/src/net/http/transport.go),
   retrieved 2026-10-12); the addresses of a name share the remaining time. If a library puts the transport of
   `HTTPClient` into its own client, the limit applies per redirect step. Size limit
   `egress.max_response_bytes` (or the consumer's value) for the body, counted after decoding by
   the transport; `egress.max_response_bytes` also limits the header lines.
8. Every rejection: one log entry at `Warn` with `bothy.egress.purpose`, `bothy.reason` and
   `server.address`, never path, query, headers or content; the counter `bothy.egress.blocked`
   through `Deps.Metrics`. Error texts of `Client.Do` name only scheme and host.

## Dependencies

Standard library only (register: `platform.egress` without a vendor group). `platform/otel`
implements the counter through the interface `Metrics`; until then a client without `Metrics`
counts nothing. Users are adapters (`identity/adapters/oidc` from M1-41, push, webhooks, object
storage, transcription) and `cmd/bothy` for the health check.

## Lint

The Semgrep rule `bothy-go-egress-only` reports, in production code outside this package,
`http.DefaultClient`, `http.DefaultTransport`, `http.Get` and related functions (also as a function
value), own `http.Client`, `http.Transport`, `net.Dialer` and `tls.Dialer` (also as a zero value,
struct field, embedded value, type definition, alias or in slices, arrays and maps), reverse
proxies, `net.Dial`, `tls.Dial` and related ones, and `x.Transport = nil`; test code is exempt.
What the rule does not see is listed in docs/DECISIONS.md M1-22/9. Every necessary construction in
this package carries a `nosemgrep` comment with a reason. `bothy-go-tls-verify` reports
`InsecureSkipVerify` with any value other than `false`, also in tests; for this, `.semgrepignore`
replaces the built-in ignore list of Semgrep, which skipped test files (`*_test.go` among the
"Common test paths" of the default template, which a `.semgrepignore` replaces:
[Semgrep, Ignore files, folders, and code](https://docs.semgrep.dev/ignoring-files-folders-code),
retrieved 2026-10-12; M1-22/13).

## Chapters

[19-security.md](../../../../../docs/spec/19-security.md),
[02-architecture.md](../../../../../docs/spec/02-architecture.md) (code layout, deadlines),
[signals.md](../../../../../docs/observability/signals.md),
[configuration.md](../../../../../docs/operations/configuration.md)
