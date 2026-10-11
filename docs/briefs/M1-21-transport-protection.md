# M1-21: Transport protection

Status: ready | Depends on: M1-18 | Milestone: M1

## Goal

After this ticket the public listener of `bothy serve` accepts plaintext HTTP only from loopback
peers and from peers listed in `server.trusted_proxies`, and closes every other plaintext
connection at accept time, before a byte is read. An instance operator who configures a
certificate gets server-side TLS with TLS 1.2 as the floor and the Go standard cipher selection.
Every response of both listeners carries the headers of SEC-HDR-1 and SEC-HDR-2, including HSTS
with `http.hsts_max_age` whenever the instance is served over HTTPS. One list,
`server.trusted_proxies`, then decides both who may send plaintext and whose forwarded headers
count; this closes the server side of threat T-NW-02.

## Read first

- `docs/spec/19-security.md`: "2. Transport" (SEC-TLS-1 to SEC-TLS-3), "3. HTTP security headers"
  (SEC-HDR-1, SEC-HDR-2), "9. Logs without secrets and without content" (SEC-LOG-5), "14. Tests"
  (SEC-T01, SEC-T03, SEC-T17), "15. Decisions of this chapter" (R-SE-01),
  "16. Configuration values of this chapter" (`http.hsts_max_age`).
- `docs/spec/16-operations.md`: "4. Network" ("4.1 Basic rules", "4.2 Two listeners", "4.3 Edge and identity"),
  "5.1 Configuration", "5.2 Secret provider", "6.1 Environments", "14. Configuration values of this chapter"
  (`server.listen_addr`, `server.internal_listen_addr`, `server.public_url`, `server.trusted_proxies`,
  `secrets.cache_ttl`), "15. Decisions" (R-OP-01, R-OP-02).
- `docs/spec/02-architecture.md`: "9. Request path" ("9.1 Chain in the api node"), "13. Configuration loading",
  "14.1 Error kinds", "14.4 Failure of a dependency" (row secret provider),
  "15. Start and graceful shutdown" ("15.1 Start", "15.2 Health").
- `docs/spec/08-identity.md`: "2.4 Trusted proxy header" (rules 1 and 2), "13. Login protection"
  (client IP rule), "15. Tests" (ID-T08, ID-T25).
- `docs/spec/06-api.md`: "2. Basic rules" (row "Client IP"), "5. Errors".
- `docs/PLAN.md`: section 7, rows M1-18, M1-21, M1-43, M1-64, M1-73;
  "Known contradictions that M1 resolves" (W1 row "trusted proxies", W4); section 19 decisions P-08,
  P-09, P-10, P-11.
- ADR 0011 (`docs/adr/0011-12-factor-configuration-and-secret-provider.md`).
- `docs/security/threat-model.md`: "2. System boundaries" (TB1, TB2), "5.3 Network edge and proxy", register
  rows T-NW-02 and T-ID-03.
- `infra/compose/compose.yaml` (anchor `x-server-env`, services `api` and `api-tailscale`),
  `infra/compose/.env.example`, `docs/operations/install-compose.md` "5.1 Profile direct",
  `docs/operations/tailscale.md`, `docs/operations/configuration.md` sections 2, 4 (generated) and
  9 ("Compose variables").
- Code merged by earlier tickets: `services/server/internal/platform/config` (M1-07),
  `internal/platform/secrets` (M1-09), `internal/platform/lifecycle` and `internal/platform/health`
  (M1-11), `internal/platform/httpserver` (M1-18: middleware chain, security headers, client IP
  from trusted proxies, problem bodies).
- `docs/quality/gates.md`, `docs/quality/push-discipline.md`, `docs/quality/red-team-gate.md`,
  `CONTRIBUTING.md`.

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
7. No en dash and no em dash in any file you write (`uv run --project tools --locked --quiet python tools/hooks/text_rules.py <file>` exits 0).
   White label: no company, person, host or instance names; examples use `example.com`, people
   Alex, Sam, Robin, documentation addresses from RFC 5737 and RFC 3849.
8. Configuration only through named keys listed in `docs/operations/configuration.md`; a new key
   gets its row in the chapter table first (Ruling L4), and the reference is regenerated.
9. SEC-TLS-1 (19 "2. Transport"): plaintext HTTP only from loopback or from peers in
   `server.trusted_proxies`; every other plaintext connection is refused while the connection is
   set up.
10. SEC-TLS-2: where the server terminates TLS itself, at least TLS 1.2, TLS 1.3 preferred, Go
    standard cipher suites without an own list.
11. SEC-TLS-3 and SEC-HDR-1: HSTS with `http.hsts_max_age` as soon as the instance is reachable
    over HTTPS, also behind a TLS proxy; `X-Content-Type-Options: nosniff`,
    `Referrer-Policy: no-referrer`, no version in `Server` on every response.
12. SEC-HDR-2: API responses carry `Cache-Control: no-store`, `Cross-Origin-Resource-Policy:
    same-origin` and `Content-Type: application/json` or `application/problem+json`.
13. SEC-LOG-5: IP addresses never appear in general logs.
14. R-OP-01: two listeners, public and internal; health stays on the internal listener (L2).
15. P-10: tests bind only `127.0.0.1` and port 0.

## Scope

All Go paths are below `services/server/`.

1. **Specification (already decided):** 16 "4.1 Basic rules" rule 1, "4.3 Edge and identity",
   "5.2 Secret provider" and "14" (`server.tls_cert_ref`, `server.tls_key_ref`); 19 SEC-TLS-1 to
   SEC-TLS-3, R-SE-08 and R-SE-09; `docs/operations/configuration.md` sections 4 and 9
   (`BOTHY_TRUSTED_PROXIES`); `install-compose.md` "5.1"; `infra/compose/.env.example` and the
   passthrough `BOTHY_SERVER__TRUSTED_PROXIES` in service `api` of `infra/compose/compose.yaml`.
   Change them only if the build proves a statement wrong, in the same change.
2. **Configuration schema** (`internal/platform/config`, typed config in `internal/wire`): add
   `server.tls_cert_ref` and `server.tls_key_ref` (type `secret_ref`, default empty) and, if not
   yet present, `http.hsts_max_age` (type duration, default `31536000s`, whole seconds, `>= 0`).
   Cross checks at load time (02 "13. Configuration loading" step 4): both TLS keys set or both
   empty; with TLS set, `server.public_url` must use `https`. Error messages name key and source,
   never a value.
3. **Transport** in `internal/platform/httpserver` (new files `transport.go`, `tlscert.go`, tests):
    - `type TransportConfig struct { ListenAddr string; PublicURL *url.URL; TrustedProxies
      []netip.Prefix; TLSCertRef, TLSKeyRef <secret reference type of internal/platform/secrets> }`
      where the zero value of both refs means plaintext.
    - `func ListenPublic(ctx context.Context, cfg TransportConfig, provider <secret provider
      interface of internal/platform/secrets>, log *slog.Logger) (net.Listener, error)`: opens
      `cfg.ListenAddr`; returns a TLS listener in TLS mode, otherwise the plaintext listener wrapped
      by `PeerFilter`.
    - `func PeerFilter(inner net.Listener, trusted []netip.Prefix, name string, log *slog.Logger)
      net.Listener`: `Accept` returns only connections whose peer is loopback or inside `trusted`;
      it closes all others itself and keeps accepting.
    - A certificate source used as `tls.Config.GetCertificate` (unexported is fine) that loads and
      validates the key pair at start and follows rotation through the secret provider.
4. **Headers**: HSTS per design note 6 in the security-header middleware of M1-18. If M1-18
   already emits `Strict-Transport-Security`, make that single implementation follow design note 6;
   never keep two. Fix every response kind of design note 7 that lacks SEC-HDR-1, SEC-HDR-2 or a
   problem body.
5. **Lifecycle** (`internal/platform/lifecycle`): open the public listener through `ListenPublic`;
   load and validate TLS material at start step 2 ("Resolve secrets", 02 "15.1 Start") so that
   bad material aborts before the database wait. The internal listener keeps its plain
   `net.Listen`.
6. **Compose contract**: already in place (Scope 1). Add a test that reads
   `infra/compose/compose.yaml` and checks that service `api` passes `BOTHY_SERVER__TRUSTED_PROXIES`
   from `BOTHY_TRUSTED_PROXIES` with the loopback default. Profile `tailscale` stays unchanged (the
   server listens on loopback in the sidecar namespace).
7. **Docs and records**: package doc comment (`doc.go`) of `httpserver` describes the transport
   rules; `services/server/README.md` platform table mentions transport protection;
   `docs/security/threat-model.md` register row T-NW-02 lists SEC-T17 next to SEC-T01 and SEC-T02;
   `services/server/CHANGELOG.md` under `Unreleased` (`Security`). R-SE-08, R-SE-09, the HSTS
   form and the Compose passthrough are already in 16 and 19.

## Out of scope

- The `trusted_header` loopback rule, ID-T08 and the second half of SEC-T01 ("start with
  the proxy header driver and a non-loopback listener"): M1-43 with ruling W4.
- Client IP from `Forwarded` and `X-Forwarded-For` and ID-T25: M1-18. Reuse its parsed
  `server.trusted_proxies`; do not parse the list a second time.
- Rate limits and the `429` response: M1-19. If `internal/platform/ratelimit` is merged when you
  start, include its `429` in the SEC-T03 table; otherwise M1-19 adds that row.
- WebSocket origin check SEC-CORS-3 and close codes: M1-64. Console headers SEC-HDR-3: M6. File
  download headers SEC-HDR-4: M3. Outgoing TLS and the `InsecureSkipVerify` lint: M1-22. TLS to
  PostgreSQL: M1-12.
- Making the installation Compose startable and choosing a fixed Compose subnet: M7. The smoke
  test (M1-73) probes from the network namespace of `api` and needs no proxy entry.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-21-transport-protection`, created from
  the base branch the launcher names. One writer per worktree.
- Conventional Commits with the module or component as scope (for example
  `feat(httpserver): refuse plaintext from untrusted peers`), each commit signed off for the DCO
  (`git commit -s`); squash into meaningful commits before the merge.
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope above, except `docs/DECISIONS.md`,
  `services/server/CHANGELOG.md` and generated references named above.
- The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`,
  `05-actions.md`) is read-only. If this ticket needs something it lacks, stop with
  `Verdict: BLOCKED` (P-11).
- Follow the host rules supplied by the launcher. Tests bind only `127.0.0.1` and port 0.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d` (in Go
  tests `t.TempDir()`), removed at the end by their literal path.
- Never read or print secrets; never open `.env` files. Test keys and certificates are generated
  inside the test.
- No network calls except package downloads for pinned tools.
- Do not lower a threshold, disable a gate or delete a test.

## Design notes

1. **Which listener.** Transport protection applies to the public listener
   (`server.listen_addr`). The internal listener (`server.internal_listen_addr`) stays plaintext
   without a peer filter: it serves only `/healthz`, `/readyz` and in-stack webhooks, evaluates no
   identity or forwarded headers, and is never published on the host (16 "4.1 Basic rules" rule 1,
   "4.2 Two listeners"). Filtering it would force instance operators to add the whole Compose
   network to `server.trusted_proxies`, which would also make forwarded headers from any container
   count on the public listener (R-SE-09).
2. **Plaintext mode** (both TLS refs empty). On every accepted connection take
   `conn.RemoteAddr()`, parse it as `netip.AddrPort`, apply `Addr().Unmap()`. Allowed if the
   address is loopback (`127.0.0.0/8`, `::1`) or contained in a prefix of
   `server.trusted_proxies`. Otherwise close the connection at once, without reading or writing,
   and continue accepting; never return an error from `Accept` for a refused peer (that would
   make `http.Server` back off). A remote address that is not TCP is refused.
3. **Logging a refusal.** The first refusal per listener and process logs at `Warn` with the
   listener name and the hint "plaintext connection from an untrusted peer refused; add the
   address of the reverse proxy as seen by the server to server.trusted_proxies"; later refusals
   log at `Debug`. No log line contains the peer address (SEC-LOG-5).
4. **TLS mode** (both refs set). `tls.Config{MinVersion: tls.VersionTLS12, GetCertificate: ...}`;
   no `CipherSuites`, no `CurvePreferences`, no `MaxVersion`, default `NextProtos` of `net/http`,
   no client certificates. No peer filter in TLS mode. No HTTP-to-HTTPS redirect listener: native
   apps use `server.public_url`, and a redirect listener would accept plaintext from anyone.
5. **Certificate material.** The certificate ref holds a PEM chain (leaf first), the key ref a PEM
   private key in a form `tls.X509KeyPair` accepts. At start the server aborts with a clear error
   (key names, never values) if either ref cannot be read, the pair does not match, the leaf is
   not yet valid or expired, or the leaf does not cover the host of `server.public_url`
   (`x509.Certificate.VerifyHostname`). Rotation: on each handshake ask the provider for both
   values (the provider caches them for `secrets.cache_ttl`, M1-09); when the returned versions
   change, parse the new pair; if parsing fails keep serving the previous certificate and log one
   `Error` per failed version (02 "14.4": rotation failure keeps the old value).
6. **HSTS.** On every response of both listeners, when the scheme of `server.public_url` is
   `https` (which TLS mode enforces): `Strict-Transport-Security: max-age=<http.hsts_max_age in
   whole seconds>`. No `includeSubDomains` (other subdomains belong to the instance operator) and
   no `preload` (not reversible). With an `http` public URL (loopback development, 16
   "6.1 Environments") the header is absent. `max-age=0` is valid and tells browsers to forget the
   rule.
7. **Response kinds that SEC-T03 covers in M1.** Set the SEC-HDR-1 headers in the outermost
   middleware before calling the next handler, so that responses written by inner components
   inherit them. Never set a `Server` header. Kinds: `2xx` JSON of an operation; problem bodies
   for `400` malformed JSON, `401`, `404` unknown path, `405` wrong method, `413` body larger than
   `http.max_json_body_bytes`, `500` from a recovered panic, `500 internal_error` from the proof
   check (cause `authz_not_checked`), `503` while
   shutting down; `/healthz` and `/readyz` on the internal listener in both states and an unknown
   path there. Where an inner component writes a body that is not `application/problem+json`
   (for example the default `ServeMux` 404 or 405 text, or `http.TimeoutHandler` HTML), replace it
   with the problem body of 06 "5. Errors".
8. **One source for trusted proxies.** `TransportConfig.TrustedProxies` is the same parsed slice
   that M1-18 uses for the client IP. Bare addresses in the configuration count as `/32` or
   `/128`, following the type M1-07 defines.
9. **Not supported, on purpose:** PROXY protocol, h2c, Unix socket listeners for the public API.

## Tests (write them first)

All tests live next to the code in `services/server/internal/platform/httpserver` unless named
otherwise. Use a fake `net.Listener` that hands out `net.Pipe` connections with a chosen
`RemoteAddr` to simulate non-loopback peers without binding anything but `127.0.0.1`.

- `TestSECT01_PlaintextFromUntrustedPeerRefused`: peer `192.0.2.10:40000`, trusted list without
  it; a real `http.Server` serves the wrapped fake listener; the client side writes a valid request
  and reads EOF; the handler counter stays 0 and no byte reaches the client.
- `TestSECT01_PlaintextFromTrustedProxyAccepted`: peer `198.51.100.7:40000`, trusted
  `198.51.100.0/24`; `200`.
- `TestSECT01_LoopbackAccepted`: a real listener on `127.0.0.1:0` answers; fake peers
  `[::1]:40000` and `[::ffff:127.0.0.1]:40000` are accepted.
- `TestSECT01_MappedUntrustedRefused`: `[::ffff:192.0.2.10]:40000` is refused.
- `TestSECT01_RefusalKeepsAccepting`: a refused connection followed by a trusted one; the second
  is served.
- `TestSECT01_RefusalLogHasNoAddress`: captured `slog` output has the hint once at `Warn`, later
  refusals at `Debug`, and nowhere the string `192.0.2.10`.
- `TestSECT01_InternalListenerNotFiltered`: a fake untrusted peer gets `/healthz` from the internal
  listener.
- `TestSECT17_TLSBelow12Refused`: TLS listener on `127.0.0.1:0` with a CA and a leaf for
  `127.0.0.1` generated in the test and written with mode `0600` into `t.TempDir()`, referenced as
  `file:` secrets; a client with `RootCAs` set to the test CA and `MaxVersion: tls.VersionTLS11`
  fails the handshake. If the Go toolchain refuses to offer a version below 1.2, send a raw TLS 1.1
  ClientHello and assert a fatal alert or a closed connection. Never use `InsecureSkipVerify`
  (M1-22 lints it repo wide).
- `TestSECT17_TLS12And13`: a client limited to TLS 1.2 negotiates 1.2; a default client negotiates
  1.3; the server config has no `CipherSuites`.
- `TestTLS_StartFailsOnBadMaterial`: only cert set, only key set, mismatched pair, expired leaf,
  leaf without the public URL host, TLS with an `http` public URL; each fails with an error that
  names the key and contains no PEM content.
- `TestTLS_CertificateRotation`: replace the certificate file; after the provider cache expires
  (inject the provider clock or a short `secrets.cache_ttl` in the test config) new handshakes see
  the new serial; a broken replacement keeps the old certificate and logs one error.
- `TestSECT03_HeadersPerResponseKind`: table driven over the kinds of design note 7, once with
  `server.public_url = https://chat.example.com` and once with `http://127.0.0.1:8080`; expected
  header sets come from one table in the test file so later tickets add rows (files in M3, console
  in M6). If M1-18 already has a SEC-T03 test, extend it instead of adding a second one.
- `TestHSTS_ValueAndCondition`: `max-age` equals `http.hsts_max_age` in seconds; no
  `includeSubDomains`, no `preload`; absent with an `http` public URL.
- Configuration tests in `internal/platform/config`: the two new keys, their env names
  `BOTHY_SERVER__TLS_CERT_REF` and `BOTHY_SERVER__TLS_KEY_REF`, the cross checks, `http.hsts_max_age`
  rejects fractions of a second and negative values.

Commands that must pass: `make test`, `make lint`, `make docs`, `make check`; and the Compose
contract check `docker compose -f infra/compose/compose.yaml --env-file infra/compose/.env.example
config --quiet` once with `COMPOSE_PROFILES=direct` and once with `COMPOSE_PROFILES=tailscale`.

## Acceptance criteria

- [ ] SEC-T01 (first half): a plaintext connection from a non-loopback peer outside
      `server.trusted_proxies` is closed at accept time; the handler never runs.
- [ ] Loopback peers, IPv4-mapped loopback and trusted peers are served.
- [ ] SEC-T17: a TLS handshake below 1.2 against a listener with own TLS fails; 1.2 and 1.3 succeed;
      1.3 is negotiated when the client offers it; no own cipher list.
- [ ] Bad TLS material aborts the start with a message naming the key, never the value; rotation
      works without restart and a broken rotation keeps the old certificate.
- [ ] SEC-T03: every response kind of design note 7 on both listeners carries SEC-HDR-1 and, for
      JSON and problem bodies, SEC-HDR-2; no `Server` header; HSTS present exactly when the public
      URL is `https`, with `max-age` from `http.hsts_max_age`.
- [ ] No log line contains a peer IP address.
- [ ] 16, 19, `configuration.md` (regenerated section 4 and section 9), `install-compose.md`,
      Compose file and `.env.example` agree; `make docs` is green.
- [ ] `make check` is green.

## Definition of done

- Code, tests and documentation from the scope are complete: doc comment on every public symbol
  (purpose, parameters, return value, errors), package doc for `httpserver`, README and spec
  updates, `docs/DECISIONS.md` entries, CHANGELOG entry under `Unreleased` in
  `services/server/CHANGELOG.md`. No ADR is needed unless you take an architecture decision beyond
  the rulings listed here; then write one (MADR 4, `docs/adr/0000-template-madr-4.md`).
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
reason. Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
