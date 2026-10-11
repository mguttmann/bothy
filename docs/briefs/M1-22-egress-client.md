# M1-22: Egress client and SSRF lint

Status: ready | Depends on: M1-07 | Etappe: M1

## Goal

After this ticket every outgoing HTTP connection of the server has one place to go through:
`internal/platform/egress`. It refuses targets that are not allowed before any connection, checks
the IP address after DNS resolution at connection time (so DNS rebinding does not help), never
follows a redirect it has not checked, bounds time and response size, and always verifies TLS.
Two Semgrep rules make every other HTTP client, transport or dialer in production code and every
disabled certificate check a gate failure. OIDC discovery and JWKS (M1-41), push, webhooks, object
storage and transcription build on this client later.

## Read first

- `docs/spec/19-sicherheit.md`: "8. SSRF-Verbot und ausgehende Verbindungen" (SEC-NET-1 to
  SEC-NET-6), "2. Transport" (SEC-TLS-4, SEC-TLS-5), "9. Logs ohne Geheimnisse und ohne Inhalte",
  "14. Tests" (SEC-T02, SEC-T09), "15. Entscheidungen dieses Kapitels" (R-SE-04), "16.
  Konfigurationswerte dieses Kapitels" (`egress.allowed_hosts`, `egress.timeout`,
  `egress.max_response_bytes`).
- `docs/spec/02-architektur.md`: "6. Code-Layout", "5. Schichten im Modul" (platform only in
  adapters), "8.5 Was nie in einer Transaktion passiert", "13. Konfigurationsladen", "14.2 Fristen"
  (row "Aufrufe nach außen"), "14.4 Ausfall einer Abhängigkeit".
- `docs/spec/16-betrieb.md`: "4.1 Grundregeln" (rules 3 and 4).
- `docs/spec/08-identitaet.md`: "2.2 OIDC: Server als Relying Party, App als Client des Servers"
  (the first consumer: discovery and JWKS).
- `docs/spec/11-erweiterungen.md`: "7. Ausgehende Webhooks" (row "SSRF-Schutz": only `https`, host
  in `webhook.allowed_hosts`, connection bound to the checked address, no redirects).
- `docs/spec/12-dateien.md`: "8.2 Transkript" (`voice.transcription_url` is instance
  configuration).
- `docs/PLAN.md`: section 7, rows M1-11, M1-22, M1-41; "Bekannte Widersprüche" W9; decisions
  P-10, P-11.
- `docs/security/threat-model.md`: "2. Systemgrenzen" (TB7), register row T-NW-01.
- `.semgrep/` (existing rules and their fixtures, especially `bothy-go-route-registration.yaml`
  for the `nosemgrep` precedent), the `semgrep` target in `Makefile`.
- `docs/operations/configuration.md` sections 2 and 4, the generator merged by M1-08.
- Code merged by earlier tickets: `services/server/internal/platform/config` (M1-07),
  `services/server/architecture/modules.yaml` and `.go-arch-lint.yml` (M1-05).
- `docs/quality/gates.md`, `CONTRIBUTING.md`.

## Fixed requirements that apply

1. No MVP. New from scratch and directly in the final state; stages are only the order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined
   interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface with a docstring/doc
   comment (purpose, parameters, return value, errors, example where useful); comments explain the
   why, not the what. API documentation is generated from code (OpenAPI from the routes, dartdoc,
   Python API docs in MkDocs).
4. Documentation in the repo: README per module/service, ADR for every architecture decision,
   CHANGELOG per release, keep the documentation site current. A change without matching
   documentation is not done.
5. Quality gates (SonarQube, linters, type checking, security scanners, test coverage) block the
   merge.
6. Test first: every behaviour change starts with a failing test (red output goes into the
   report), then code until green, then cleanup.
7. No en dash and no em dash in any file you write. White label: no company, person, host or
   instance names; examples use `example.com` and addresses from RFC 5737 and RFC 3849.
8. Configuration only through named keys listed in `docs/operations/configuration.md`.
9. SEC-NET-1: all outgoing HTTP connections go through the egress client; a lint forbids
   `http.DefaultClient`, `http.Get`, `http.Post` and own dialers outside this package.
10. SEC-NET-2: allowed targets are only those that consumers name plus `egress.allowed_hosts`;
    everything else is refused before the connection.
11. SEC-NET-3: the IP check runs after DNS resolution at connection time; blocked are loopback,
    RFC 1918, link-local including the cloud metadata address, RFC 6598, RFC 4193, multicast and
    unspecified addresses; the only exception are entries of `egress.allowed_hosts` with
    `allow_private`.
12. SEC-NET-4: no automatic redirects for webhooks; for OIDC only to allowed targets.
13. SEC-NET-5: time and size limits per response (`egress.timeout`, `egress.max_response_bytes`).
14. SEC-TLS-4: outgoing TLS always verifies certificate and host name; `InsecureSkipVerify` is
    forbidden by lint.

## Scope

All Go paths are below `services/server/`.

1. **Specification (already decided):** 19 "8. SSRF-Verbot und ausgehende Verbindungen"
   (SEC-NET-1 with the loopback client and the OTLP exemption, SEC-NET-2 with `http` only for
   `allow_private` entries and no environment proxies, SEC-NET-3 with the full blocked list,
   SEC-NET-5 with the metric), R-SE-10, the meaning of `egress.allowed_hosts` in "16.
   Konfigurationswerte dieses Kapitels"; 02 "6. Code-Layout" (`egress/`, W9);
   `docs/operations/configuration.md` "2. Namensregel und Formate" (row Egress-Ziel); the metric
   `bothy.egress.blocked` in `docs/observability/signals.md`. Change them only if the build proves
   a statement wrong, in the same change.
2. **Configuration schema** (`internal/platform/config`, typed config in `internal/wire`):
   `egress.allowed_hosts` (list of entries, default empty, each entry parsed with
   `egress.ParseRule` and rejected at load time if invalid or duplicated), `egress.timeout`
   (duration, default `15s`, `> 0`), `egress.max_response_bytes` (integer, default `1048576`,
   `> 0`). Env names `BOTHY_EGRESS__ALLOWED_HOSTS`, `BOTHY_EGRESS__TIMEOUT`,
   `BOTHY_EGRESS__MAX_RESPONSE_BYTES`.
3. **Package `internal/platform/egress`** (files `doc.go`, `rule.go`, `addr.go`, `client.go`,
   `loopback.go`, `errors.go` and tests), standard library only. Public interface:

    ```go
    // Rule is one entry of egress.allowed_hosts.
    type Rule struct {
        Host         string // lower-case DNS name or IP literal; no wildcard, scheme or path
        Port         uint16 // 0 means only the default port of the scheme
        AllowPrivate bool
    }
    func ParseRule(s string) (Rule, error) // "host[:port][;allow_private]", IPv6 literal in brackets

    type Config struct {
        AllowedHosts     []Rule        // egress.allowed_hosts
        Timeout          time.Duration // egress.timeout
        MaxResponseBytes int64         // egress.max_response_bytes
    }

    // HostPolicy is the allow list of one consumer (for example the hosts of configured identity
    // providers). It is asked before any DNS lookup. It can never allow private addresses.
    type HostPolicy func(ctx context.Context, scheme, host string, port uint16) bool

    type RedirectMode int
    const (
        RedirectNever   RedirectMode = iota // the 3xx response goes back to the caller unchanged
        RedirectChecked                     // follow only targets that pass every check again
    )

    type Options struct {
        Purpose          string        // short label for logs, for example "oidc"
        Hosts            HostPolicy    // nil: only Config.AllowedHosts
        Redirects        RedirectMode
        Timeout          time.Duration // 0: Config.Timeout; a chapter value may override it
        MaxResponseBytes int64         // 0: Config.MaxResponseBytes
    }

    type Resolver interface {
        LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
    }

    type Deps struct {
        Resolver Resolver       // nil: net.DefaultResolver
        RootCAs  *x509.CertPool // nil: system roots; never a way to skip verification
        Logger   *slog.Logger
    }

    func New(cfg Config, opts Options, deps Deps) (*Client, error)
    func (c *Client) Do(req *http.Request) (*http.Response, error)
    // HTTPClient returns an *http.Client whose transport enforces every check, for libraries that
    // take a client (OIDC, S3). Callers must not replace its Transport or CheckRedirect.
    func (c *Client) HTTPClient() *http.Client
    // NewLoopback returns a client that reaches only loopback addresses over plain http, without
    // redirects; it is the client for `bothy healthcheck`.
    func NewLoopback(timeout time.Duration) *http.Client

    var ErrBlocked = errors.New("egress: blocked")
    var ErrResponseTooLarge = errors.New("egress: response too large")
    type BlockedError struct { Reason Reason; Host string } // errors.Is(err, ErrBlocked)
    type Reason string // "scheme_not_allowed", "host_not_allowed", "address_blocked",
                       // "redirect_not_allowed", "url_not_allowed"
    ```

4. **Semgrep rules** with fixtures, run by `make semgrep` (rule `X.yaml`, fixture `X.go`):
    - `.semgrep/bothy-go-egress-only.yaml` and `.semgrep/bothy-go-egress-only.go`: flags
      `http.DefaultClient`, `http.DefaultTransport`, `http.Get(...)`, `http.Head(...)`,
      `http.Post(...)`, `http.PostForm(...)`, `http.Client{...}`, `&http.Client{...}`,
      `new(http.Client)`, `http.Transport{...}`, `&http.Transport{...}`, `net.Dial(...)`,
      `net.DialTimeout(...)`, `net.Dialer{...}`, `&net.Dialer{...}`, `tls.Dial(...)`,
      `tls.DialWithDialer(...)`. `paths: exclude: ["*_test.go"]`. Inside
      `internal/platform/egress` each necessary construction carries
      `// nosemgrep: bothy-go-egress-only -- the egress package is the one place that builds
      transports (19 SEC-NET-1)`. Fixture `ok` cases: `egress.New(...)`, `client.Do(req)`,
      `http.NewRequestWithContext(...)`.
    - `.semgrep/bothy-go-tls-verify.yaml` and `.semgrep/bothy-go-tls-verify.go`: flags
      `tls.Config{..., InsecureSkipVerify: $V, ...}` for any `$V` other than the literal `false`, and
      `$C.InsecureSkipVerify = $V` likewise. No path exclusion, also not for tests.
5. **Existing code.** Run the new rules over the base branch. Every finding in production code is
   changed in this ticket to use `egress.New` or `egress.NewLoopback` (for example a health probe
   of `bothy healthcheck` if M1-11 is already merged). No new `nosemgrep` outside the egress
   package.
6. **Docs and records**: `doc.go` of the package (rules, examples for a consumer);
   `services/server/README.md` platform table gets the row `egress`; if
   `services/server/architecture/modules.yaml` lists platform packages or their vendor
   permissions, add `egress` there (W9); `docs/security/threat-model.md` "2. Systemgrenzen" row TB7
   names the egress client among the controls; `services/server/CHANGELOG.md` under `Unreleased`
   (`Security`).

## Out of scope

- Consumers: OIDC discovery and JWKS with their host policy (M1-41), push (M3), webhooks (M2),
  object storage (M3), transcription (M3). Do not build clients for them here.
- The OTLP exporters of `internal/platform/otel`: exempt from the egress client by SEC-NET-1
  (operator-configured collector endpoint); nothing to build here.
- Extra CA bundles for private PKI beyond `Deps.RootCAs`: no configuration key exists.
- Incoming TLS and HSTS: M1-21.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-22-egress-client`, created from the
  base branch the launcher names. One writer per worktree.
- Conventional Commits with the module or component as scope (for example
  `feat(egress): check addresses at connection time`), each commit signed off for the DCO
  (`git commit -s`); squash into meaningful commits before the merge.
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope above, except `docs/DECISIONS.md`,
  `services/server/CHANGELOG.md` and generated references named above.
- The core contract (00, 03, 04, 05) is read-only; a gap there ends the ticket with
  `Verdict: BLOCKED` (P-11).
- Follow the host rules supplied by the launcher. Tests bind only `127.0.0.1` and port 0 and
  never reach the internet; DNS answers in tests come from a fake resolver.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d` (in Go
  tests `t.TempDir()`), removed at the end.
- Never read or print secrets; never open `.env` files.
- No network calls except package downloads for pinned tools.
- Do not lower a threshold, disable a gate or delete a test.

## Design notes

1. **Entry format of `egress.allowed_hosts`.** `host[:port][;allow_private]`; host is a DNS name
   (normalised to lower case, no trailing dot) or an IP literal (IPv6 in brackets when a port
   follows); no wildcard, scheme, path or user info; port `1` to `65535`; the only flag is
   `allow_private`. In the YAML file the list holds these strings; in the environment the entries
   are comma separated. Duplicate hosts with the same port are a load error.
2. **Before any connection** (in a custom `http.RoundTripper` so that `HTTPClient()` is covered
   too): the URL must have no user info (`url_not_allowed`); scheme `https`, or `http` only when
   a `Rule` with `AllowPrivate` matches host and port (`scheme_not_allowed`); host and port must
   match a `Rule` (port `0` matches only the default port of the scheme) or be allowed by
   `Options.Hosts` (`host_not_allowed`). The resolver is not called for a refused host.
3. **At connection time** (custom `DialContext`): resolve the host with `Deps.Resolver` (an IP
   literal is used as is), `Unmap()` every address, drop the zone, and dial only addresses that
   pass the check; dial the checked address itself, never the name again, and recheck the actual
   socket address in `net.Dialer.Control` as a second line. If no address passes:
   `BlockedError{Reason: address_blocked}`. Blocked (always, with or without `allow_private`):
   unspecified and `0.0.0.0/8`, `::`; link-local `169.254.0.0/16` (includes `169.254.169.254`),
   `fe80::/10`; multicast `224.0.0.0/4`, `ff00::/8`; broadcast `255.255.255.255` and reserved
   `240.0.0.0/4`; IETF protocol assignments `192.0.0.0/24`; benchmarking `198.18.0.0/15`;
   deprecated site-local `fec0::/10`; discard-only `100::/64`; IPv4-compatible `::/96`; NAT64
   `64:ff9b::/96` and `64:ff9b:1::/48`; 6to4 `2002::/16`; Teredo `2001::/32`. Blocked unless a
   matching `Rule` has `AllowPrivate`: loopback `127.0.0.0/8`, `::1`; RFC 1918 `10.0.0.0/8`,
   `172.16.0.0/12`, `192.168.0.0/16`; shared address space `100.64.0.0/10` (overlay networks);
   unique local `fc00::/7`. Documentation ranges (RFC 5737, RFC 3849) are not blocked; they are not
   routed on the internet and tests use them as public examples (19 SEC-NET-3, R-SE-10).
4. **`allow_private`** exists only in `egress.allowed_hosts` (operator trust); a consumer policy
   (`Options.Hosts`) never unlocks a private address. Reason: consumer lists can come from data
   such as a discovery document; the operator's file cannot.
5. **TLS**: `MinVersion: tls.VersionTLS12`, verification always on, `RootCAs` from `Deps`, server
   name from the URL host (the transport does this when the dial returns a TCP connection).
6. **Proxies**: `Transport.Proxy` is `nil`; `HTTP_PROXY` and `HTTPS_PROXY` are ignored, because a
   proxy would make the dial check see only the proxy address.
7. **Redirects**: `RedirectNever` returns the 3xx response unchanged (`http.ErrUseLastResponse`);
   `RedirectChecked` runs design notes 2 and 3 for every hop and keeps the built-in redirect limit
   of `net/http`; a refused hop ends with `redirect_not_allowed` or `address_blocked`.
8. **Limits and logs**: overall time `Options.Timeout` or `egress.timeout` (client timeout plus
   request context); response bodies are wrapped so that reading beyond the limit returns
   `ErrResponseTooLarge` (never silent truncation; decompressed bytes count). A blocked attempt logs
   once at `Warn` with purpose, reason and host; never path, query, headers or bodies; and
   increments `bothy.egress.blocked` with `bothy.reason` = the `Reason` value (signals.md).
9. **Callers** map egress errors to their own error kinds (usually `503 unavailable` with the
   internal cause `dependency_unavailable`, 02 "14.1 Fehlerarten"); the egress package does not
   know `kernel/errs` kinds of other modules.
10. **`NewLoopback`** dials only loopback addresses after resolution, plain `http`, no redirects;
    it exists so that `bothy healthcheck` has an allowed client without opening a hole in the
    lint.

## Tests (write them first)

In `services/server/internal/platform/egress`; real servers are `httptest` servers on
`127.0.0.1:0`; DNS answers come from a map-based fake `Resolver`; dial attempts are counted with a
test hook on the unexported dialer.

- `TestSECT09_BlockedRanges`: table with one address per blocked range in IPv4 and IPv6, including
  `169.254.169.254`, `fe80::1`, `fc00::1`, `fd00::1`, `0.0.0.0`, `::`, `224.0.0.1`, `ff02::1`,
  `::ffff:127.0.0.1`, `::ffff:169.254.169.254`, `64:ff9b::a9fe:a9fe`, `2002:a9fe:a9fe::1`,
  `2001::1`, and the first host address of `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16` and
  `100.64.0.0/10`; expected verdict with and without `allow_private`. Private IPv4 host addresses
  are derived from their range in code (`netip.MustParsePrefix("10.0.0.0/8").Addr().Next()`) and
  never written as literals: the generic white-label rule (tools/denylist/generic.rules) flags
  literal private IPv4 host addresses in every tracked file and allows only whole ranges in CIDR
  notation.
- `TestAllowedPublicAddresses`: `192.0.2.10`, `198.51.100.1`, `203.0.113.5`, `2001:db8::10` pass.
- `TestParseRule`: valid forms, IPv6 with port, upper case normalised, and rejections (wildcard,
  scheme, path, port `0` or above `65535`, unknown flag).
- `TestHostNotAllowedRefusedBeforeDial`: resolver and dialer counters stay 0.
- `TestSECT09_LiteralPrivateAddressRefused`: rule `127.0.0.1:<port>` without `allow_private`;
  `address_blocked`; the test server counts 0 requests.
- `TestSECT09_DNSNameWithPrivateAnswerRefused`: consumer policy allows `api.example.com`, the fake
  resolver answers `127.0.0.1`; refused at dial time; server count 0.
- `TestSECT09_RedirectToPrivateRefused`: an allowed server (`allow_private` rule) answers `302` to
  `http://internal.example.com:<port>/`, which resolves to `127.0.0.1` and is allowed by the
  consumer policy only; with `RedirectNever` the caller gets the `302`, with `RedirectChecked` the
  error is `address_blocked`; the target server counts 0 in both cases.
- `TestSECT09_MetadataAddressRefusedEvenWithAllowPrivate`: rule `metadata.example.com;allow_private`
  resolving to `169.254.169.254`; refused, dial counter 0.
- `TestAllowPrivateRuleReachesLoopbackService`, `TestHTTPSchemeOnlyWithAllowPrivate`,
  `TestUserInfoRefused`.
- `TestTLSVerificationAlwaysOn`: `httptest.NewTLSServer` behind an `allow_private` rule fails
  without `RootCAs` and succeeds with `RootCAs` from the server certificate.
- `TestResponseLimit`: exactly the limit passes, one byte more returns `ErrResponseTooLarge`; also
  with a gzip encoded body.
- `TestTimeout`, `TestProxyEnvironmentIgnored` (`t.Setenv` of both proxy variables to a counting
  server; it counts 0), `TestBlockedAttemptLog` (purpose, reason, host; no path or query) and
  `TestBlockedAttemptCounted` (`bothy.egress.blocked` with the reason, manual metric reader).
- `TestLoopbackClientRefusesNonLoopback`: the fake resolver answers the first host address of
  `10.0.0.0/8` (derived as above); refused.
- Configuration tests in `internal/platform/config` for the three keys and their env names.
- Semgrep rule tests through the fixtures (`ruleid:` and `ok:` annotations). SEC-T02 also covers
  raw SQL; the existing rule `bothy-go-sql-string-building` stays green.

Commands that must pass: `make test`, `make semgrep`, `make lint`, `make docs`, `make check`.

## Acceptance criteria

- [ ] SEC-T09: one address from each blocked range in IPv4 and IPv6, the metadata address, a DNS
      name with a private answer and a redirect to a private address are all refused, and the
      target is never contacted.
- [ ] Hosts that neither a `Rule` nor the consumer policy allows are refused before DNS.
- [ ] SEC-T02: the lint finds `http.DefaultClient`, `http.Get` and `InsecureSkipVerify` in the
      fixtures; `make semgrep` is green on the repository with no new `nosemgrep` outside
      `internal/platform/egress`.
- [ ] Time and size limits come from `egress.timeout` and `egress.max_response_bytes` or a
      consumer override; oversize bodies fail loudly.
- [ ] Environment proxies are ignored; TLS is always verified.
- [ ] 02 "6. Code-Layout", 19 "8." and "16.", `configuration.md` sections 2 and 4 agree with the
      code; W9 is recorded in `docs/DECISIONS.md`.
- [ ] `make check` is green.

## Definition of done

- Code, tests and documentation from the scope are complete: doc comment on every public symbol
  (purpose, parameters, return value, errors, a consumer example in `doc.go`), README and spec
  updates, `docs/DECISIONS.md` entries, CHANGELOG entry under `Unreleased` in
  `services/server/CHANGELOG.md`. Write an ADR only if you take an architecture decision beyond
  the rulings listed here.
- `make check` is green.
- Independent review: `make review` plus a code review over the diff; then the red team of
  `docs/quality/red-team-gate.md` over the diff with verdict `JA` or `NEIN`, recorded in the
  report. On `NEIN`: fix with a test per finding and have the delta reviewed again.
- Merge locally into the base branch only if review and red team are green and `make check` is
  green on the merged state; then `git worktree remove` the worktree.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then the
commits, the files with line counts, test results with counts (including the red output of the
first test run), what is not proven, and follow-ups that could not be done in scope with the
reason (at least: OTLP exporter routing for M1-10). Then write `{{DONE_PATH}}` with
`exit=0 sha=<sha>` or `exit=1 <reason>`.
