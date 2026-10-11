# M1-43: Trusted proxy header login

Status: ready | Depends on: M1-21, M1-37 | Etappe: M1

## Goal

After this ticket a person behind a reverse proxy that sets an identity header (for example
`tailscale serve`) logs in with that header instead of a login flow: the driver `trusted_header`
evaluates the header only while the public listener is bound to loopback, only from trusted peers,
only for the login itself, and, with `auth.trusted_header_bind_requests`, on every later request of
the session. Device keys and DPoP-bound tokens stay the same as for every other driver. A
misconfiguration fails closed: the driver stays off and readiness reports it (W4).

## Read first

- `docs/spec/08-identitaet.md`: 2.1 "Port", 2.4 "Vertrauenswürdiger Proxy-Header", 3.1
  "Zuordnung einer externen Identität", 3.2 "Einladungs-Code", 5.1, 5.2, 7.3 "DPoP", 13 "Schutz
  der Anmeldung", 15 (ID-T08, ID-T09, ID-T10, ID-T25), 16 (R-ID-03), 18
  (`auth.trusted_header_bind_requests`).
- `docs/spec/16-betrieb.md`: 4.2 "Zwei Listener", 4.3 "Rand und Identität", 14
  (`server.listen_addr`, `server.trusted_proxies`), R-OP-02.
- `docs/spec/19-sicherheit.md`: 2 "Transport" (SEC-TLS-1), 9 (SEC-LOG-2: identity headers never
  logged), 14 SEC-T01.
- `docs/spec/02-architektur.md`: 9 "Request-Pfad" (step 4), 13 "Konfigurationsladen" point 4,
  15.2 "Health".
- `docs/spec/03-datenmodell.md`: 4.2 `identity_providers`, `user_identities`, `auth_sessions`
  (`auth_method`, `revoke_reason`), `enrollment_codes`, `devices`.
- `docs/spec/05-aktionen.md` and `docs/spec/actions.yaml`: `auth.login.complete`,
  `identity_provider.manage`, 2.5.
- `docs/spec/06-api.md`: 3.2 "Anmeldeabläufe" point 2, 5 "Fehler" point 5 (public endpoints
  answer `404` when switched off).
- `docs/spec/07-rechte.md`: 4.1 (`session_revoked`), SR7.
- `docs/PLAN.md` section 7 "Bekannte Widersprüche, die M1 auflöst" W4, rows M1-11, M1-21, M1-34,
  M1-36, M1-37, M1-43, M1-64; section 3.
- `docs/operations/tailscale.md` sections 2 to 4.
- ADRs 0019, 0020.
- `docs/api/openapi.yaml`: operation `loginWithTrustedHeader`; schemas
  `TrustedHeaderLoginRequest`, `DeviceRegistration`, `TokenResponse`; operations
  `createIdentityProvider`, `updateIdentityProvider`.
- `docs/security/threat-model.md`: T-ID-02, T-NW-02.
- Merged READMEs and code: `internal/identity` (authenticator of M1-34, provider registry of M1-36,
  enrollment of M1-37, the provisioning handle helper of M1-41 if merged), `internal/platform/httpserver`
  (client IP and trusted proxies of M1-18 and M1-21), `internal/platform/health` (M1-11),
  `services/server/api/operations.yaml`.
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
6. Test first: every behaviour change has a test that was red before the change (the red run is
   quoted in the report), then green, then refactor.
7. No en dash or em dash characters in any file you write; use a hyphen or rephrase.
8. White label: logins in tests are `alex@example.com`, `sam@example.com`; no real tailnet,
   host or IP except `127.0.0.1`, `::1` and the documentation ranges of RFC 5737 and RFC 3849.
9. Limits and switches only through named configuration keys (`server.trusted_proxies`,
   `auth.trusted_header_bind_requests`).
10. The header is evaluated only while the public listener is bound to loopback (08 2.4 rule 1 as
    resolved by W4: driver stays off, `/readyz` reports `trusted_header_requires_loopback`,
    `identity_provider.manage` refuses to enable it).
11. The header is evaluated only when the TCP peer is in `server.trusted_proxies` (08 2.4 rule 2,
    SEC-TLS-1); a missing login header is anonymous and the login answers `401` (rule 3, ID-T09).
12. The header replaces only the login, not device keys and tokens (rule 4).
13. With `auth.trusted_header_bind_requests = true` every later request of the session carries the
    same login; a mismatch gives `401`, revokes the session and is audited (rule 5, ID-T10).
14. RFC 2047 encoded values are decoded before the comparison; the login is compared, not the
    display name (rule 6).
15. Matching only by `(provider_id, subject)` or an enrollment code; provisioning only with
    `allow_provisioning` (08 3.1).
16. Identity headers never appear in logs, traces, audit details or error bodies (SEC-LOG-2).

## Scope

- `services/server/internal/identity/domain/trustedheader/`: pure functions: header value
  decoding and validation, loopback decision for a bound address, the config type
  `identity_provider.trusted_header.v1` and its validation.
- `services/server/internal/identity/app/`: use case `LoginWithTrustedHeader`; driver state
  (active or off with reason); the per-request binding check called by the authenticator.
- `services/server/internal/identity/adapters/httpapi/`: handler `loginWithTrustedHeader`; the
  handler passes the raw TCP peer address (never a forwarded address) and the configured header
  values to the use case.
- The `kernel/actor.Authenticator` implementation of M1-34 in `internal/identity`: binding check
  for sessions with `auth_method = 'trusted_header'` (Design notes).
- `identity_provider.manage` path of M1-36: config validator for `trusted_header` and the refusal
  to enable the driver without loopback.
- `internal/platform/health` registration (through `internal/wire`) of the readiness check
  `trusted_header_requires_loopback`.
- `internal/wire`: computes the loopback flag from the bound public listener address and passes it
  into the `identity` config.
- If `internal/platform/config` contains a start-time cross-check "header driver only with a
  loopback listener" (02 13 point 4), remove it in favour of the readiness check and adapt its
  test; no other change in `platform/config`.
- The specification is in place (planning, spec first; W4 resolved there): 08 2.4 (rules 1 to 7
  with W4, binding, RFC 2047), 2.6 (`identity_provider.trusted_header.v1`), 3.1 (enrollment code
  at the header login), ID-T08 to ID-T10; 16 4.3; 19 SEC-T01; 02 13 point 4; 03
  `auth_sessions.revoke_reason` with `trusted_header_mismatch`; `openapi.yaml`
  (`TrustedHeaderLoginRequest.enrollment_code`). Change them only where the implementation proves
  them wrong (if the descriptions of `404` and `409` on the provider operations lack the loopback
  rule, add it, spec first). `services/server/api/operations.yaml`: `loginWithTrustedHeader` moves
  to the built list; regenerated server code.
- `docs/operations/tailscale.md` section 4 with a complete provider config example;
  `services/server/internal/identity/README.md`; CHANGELOG entry under "Unreleased" of
  `services/server/CHANGELOG.md`; `docs/DECISIONS.md` entry for W4 and for the rulings below;
  `docs/security/threat-model.md` T-ID-02 test column with the Go test names.

## Out of scope

- The identity query over the local API of the network agent (Tailscale `WhoIs`): a later driver
  (08 2.4).
- TLS and plaintext acceptance of listeners: M1-21 (reused here).
- Binding check on the gateway upgrade: the authenticator is shared, but the gateway endpoint is
  M1-64; note it in the report as a test M1-64 must carry.
- Provider CRUD itself: M1-36. Enrollment code issuance: M1-37. Device approval: M1-38.
- The Compose profile `tailscale`: M7 portability.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-43-identity-trusted-header`, created from
  the base branch the launcher names.
- Conventional Commits with the module as scope (`feat(identity): ...`) and a DCO sign-off on every
  commit (`git commit -s`).
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the Scope list, except the docs and changelog entries named in this
  brief.
- Follow the host rules supplied by the launcher.
- Tests bind only `127.0.0.1` and port 0 (P-10); never bind `0.0.0.0` or `::` (non-loopback cases
  are tested with constructed addresses). PostgreSQL only through `internal/platform/pg/pgtest`;
  without a Docker-compatible runtime the test fails, it never skips (P-09).
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at
  the end.
- Never read or print secrets.
- No network calls except package downloads for pinned tools; never install or call a real proxy
  or tailnet.
- A gap in the core contract (00, 03, 04, 05, `docs/spec/actions.yaml`) ends the ticket with
  `Verdict: BLOCKED` (P-11); never build a deviation.

## Design notes

**Preconditions (check before writing code; any failure: `Verdict: BLOCKED <which>`).**

1. `docs/spec/03-datenmodell.md` `auth_sessions.revoke_reason` contains `trusted_header_mismatch`
   (also in `SessionRevokedPayload.reason`); ID-T10 requires a revocation with a stated reason.
2. The authenticator and the session revocation of M1-34 exist (revocation also revokes the family
   and writes `session.revoked`), and M1-37 provides enrollment-code verification and consumption
   plus the shared login completion (device registration, session, token pair).

**Provider configuration `identity_provider.trusted_header.v1`** (strict decoding, unknown fields
`400 validation_failed`; normative in 08 2.6):

| Field | Type | Required | Meaning |
|---|---|---|---|
| `login_header` | HTTP header name (RFC 9110 token) | yes | carries the login, e.g. `Tailscale-User-Login` |
| `name_header` | HTTP header name | no | display name at provisioning, e.g. `Tailscale-User-Name` |

Only one `trusted_header` provider exists (unique index, M1-36).

**Loopback decision.** `internal/wire` reads the actual bound address of the public listener
(`server.listen_addr` after listen) and computes `PublicListenerLoopback = ip.IsLoopback()` (true
for `127.0.0.0/8` and `::1`; false for the unspecified addresses `0.0.0.0` and `::` and every
other address). Reason: the configured string can be a host name; the bound socket is the fact.

**Driver state.** Active exactly when an enabled `trusted_header` provider exists and
`PublicListenerLoopback` is true. Otherwise:

- `loginWithTrustedHeader` answers `404 not_found` (06 5 point 5: public endpoints do not reveal
  configuration).
- The readiness check `trusted_header_requires_loopback` fails (503 on `/readyz` with only this
  check name) while an enabled `trusted_header` provider exists and the listener is not loopback.
- `createIdentityProvider` with `kind = trusted_header` and `enabled` true (the default), and
  `updateIdentityProvider` setting `enabled` true, answer `409 conflict` with
  `reason: trusted_header_requires_loopback` and a `hint` naming `server.listen_addr`; audit deny.

**Peer and header evaluation** (one function used by login and binding):

1. The peer is the IP of the TCP connection (`RemoteAddr`), never a forwarded value. If it is not
   in `server.trusted_proxies`, the identity headers are treated as absent.
2. Exactly one value of `login_header`; several values are treated as absent and logged as a
   warning without the values.
3. Decode RFC 2047 encoded words with `mime.WordDecoder`, charsets `utf-8` and `us-ascii` only;
   any other charset or a decoding error is treated as absent.
4. The decoded value must be valid UTF-8 without control, format or bidi characters and not
   empty; otherwise absent.
5. `name_header` follows the same rules; failure only drops the display name.

**`loginWithTrustedHeader`** (`POST /api/v1/auth/trusted-header/login`, anonymous,
`auth.login.complete`, `rl.public_auth`, header `DPoP` without `ath`, no `Idempotency-Key`):

1. `authorize(anonymous, auth.login.complete, instance)`.
2. Driver off: `404 not_found`.
3. Login absent: `401 unauthenticated`, audit deny (ID-T09).
4. External identity: `ProviderID`, `Issuer` empty, `Subject` = decoded login, `DisplayName` =
   decoded name or the login.
5. One Unit of Work: lookup `user_identities` by `(provider_id, subject)` with `unlinked_at IS
   NULL` and `issuer IS NULL`; else optional `enrollment_code` in the body (new field,
   specification first): verify and consume through M1-37, insert the identity, audit
   `identity.link`, device `active` (08 3.2); else `allow_provisioning`: person and device
   `pending`, `users.email` NULL, handle derived from the part of the login before `@` with the
   rule of M1-41 (reuse its helper if merged, otherwise implement the same rule in
   `domain/trustedheader` and note it for consolidation); else rollback, `401 unauthenticated`
   with the neutral body, audit deny in its own transaction. A wrong enrollment code is a `401`
   with the same body (no hint which part failed).
6. Principal `disabled` or `deleted`: `401`. Otherwise device registration, session
   (`auth_method = 'trusted_header'`, `identity_id`) and token pair through the shared functions;
   `last_login_at = now`; audit allow with details `{provider_id, device_id}` (never the login).
7. `201 TokenResponse`, `Cache-Control: no-store`.

**Per-request binding** (only when `auth.trusted_header_bind_requests = true`): in the
authenticator, after the access token and DPoP checks succeed, for a session with
`auth_method = 'trusted_header'`: evaluate the header as above and compare the decoded login
byte-exact with `user_identities.subject` of `auth_sessions.identity_id` (load it with the session
lookup; an identity that was unlinked counts as mismatch). Missing or different: revoke the
session with the reason from precondition 1 (family revoked, `session.revoked` written), audit
deny with `deny_reason = 'session_revoked'` and the route's action, answer `401 session_revoked`.
The check runs for every request authenticated with such a session, including `refreshToken`,
because the authenticator is shared. With the switch off, no header is read after the login.

**Readiness versus start failure (W4).** 16 4.3 applies: the driver is a runtime setting of the
owner, so a misconfiguration turns the driver off and fails readiness instead of preventing the
start. 08 2.4 rule 1, ID-T08 and SEC-T01 say so; record W4 in `docs/DECISIONS.md` with a link.

## Tests (write them first)

Unit tests (`domain/trustedheader`):

- `TestIsLoopbackListener`: `127.0.0.1`, `127.5.5.5`, `::1` true; `0.0.0.0`, `::`,
  `192.0.2.10`, `2001:db8::1` false (constructed `net.TCPAddr` values).
- `TestDecodeHeaderValue`: plain ASCII; `=?utf-8?q?R=C3=B6bin?=` decodes to `Röbin`; base64
  encoded word; unsupported charset absent; control and bidi characters absent; empty absent.
- `TestTrustedHeaderConfigValidation`: valid names; invalid token characters; unknown field.

Integration tests (PostgreSQL 18, real HTTP chain on `127.0.0.1:0`; the peer is `127.0.0.1`, so
"trusted" uses `server.trusted_proxies = 127.0.0.1/32` and "untrusted" uses `192.0.2.0/24`;
the loopback flag is injected into the identity config for the off cases):

- `TestIDT08_DriverOffWithoutLoopback`: flag false with an enabled provider: `/readyz` 503 naming
  `trusted_header_requires_loopback`, login `404`; `updateIdentityProvider` enabling and
  `createIdentityProvider` both `409` with the reason; with the flag true and the provider enabled
  `/readyz` is ready.
- `TestIDT09_MissingLoginHeader401`: no header (tagged device simulated) gives `401` and an audit
  deny.
- `TestSECT01_HeaderFromUntrustedPeerIgnored`: same request with an untrusted peer gives `401`
  even with a valid header.
- `TestIDT10_BindingMismatchRevokesSession`: login as `alex@example.com`, then a request with
  `sam@example.com` gives `401 session_revoked`; the session has `revoked_at` and the reason, the
  refresh family is revoked, `session.revoked` exists in the event log, the audit entry exists, and
  a following refresh fails.
- `TestBinding_MissingHeaderRevokes` and `TestBinding_OffDoesNotReadHeader`.
- `TestTrustedHeaderLogin_KnownIdentity`, `TestTrustedHeaderLogin_EnrollmentCodeLinks` (device
  `active`, code used), `TestTrustedHeaderLogin_ProvisioningPending`,
  `TestTrustedHeaderLogin_NoMappingNeutral401`, `TestTrustedHeaderLogin_DisabledPerson401`,
  `TestTrustedHeaderLogin_MultipleValuesAbsent`.
- `TestIDT25_ForwardedForNotTrusted` stays green (rate limit counts the real peer; reuse of M1-18).
- `TestTrustedHeaderNeverLogged`: log and audit capture contain no header value.
- Route count (AZ-P1) and response validation pass with the operation in the allow-list.

Commands that must pass:

```sh
cd services/server && go test -race ./internal/identity/... ./internal/platform/... ./internal/wire/...
make codegen-check
make arch
make api-lint
make docs
make check
```

## Acceptance criteria

- [ ] Preconditions verified and named in the report.
- [ ] ID-T08 (after W4), ID-T09, ID-T10 and SEC-T01 for the driver green with the names above;
      T-ID-02 in the threat model names them.
- [ ] Header evaluated only with a loopback public listener and a trusted TCP peer; forwarded
      addresses never count.
- [ ] RFC 2047 values decoded; the login, not the name, is compared.
- [ ] Binding revokes the session with a reason, an audit entry and `session.revoked`.
- [ ] Header values never in logs, audit details or problem bodies.
- [ ] The implementation matches 08, 16, 19 and `openapi.yaml` (changed only where proven wrong);
      `operations.yaml`, `tailscale.md`, DECISIONS (W4) updated; `make check` green.

## Definition of done

- Code and tests as above; every public symbol has a doc comment (purpose, parameters, return
  value, errors, example where useful); comments explain why.
- `internal/identity/README.md` current; CHANGELOG entry under "Unreleased";
  `docs/DECISIONS.md` entries; no ADR unless a decision beyond ADR 0019 was taken (then MADR 4 and
  `make adr-index`).
- `make check` green.
- Independent review (`make review` and a code review over the diff) and the red team gate over
  the diff, verdict recorded in the report; red team item 4 of the M1 section (forged proxy
  headers) applies directly.
- Merge locally into the base branch only if review and red team are green and `make check` is
  green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then the
commits, the files with line counts, the test results with counts (including the red runs), what
is not proven (at least: behaviour behind a real `tailscale serve`, WebSocket upgrade binding),
and follow-ups that could not be done in scope with the reason (at least the gateway binding test
for M1-64). Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
