# M1-41: OIDC login with PKCE

Status: ready | Depends on: M1-37 | Milestone: M1

## Goal

After this ticket the server is a confidential OpenID Connect relying party of any number of
configured OIDC providers and, at the same time, the authorization server of its own apps:
Authorization Code with PKCE (`S256`) on both legs, a one-time `login_code` for the app,
DPoP-bound product tokens, linking of a further OIDC identity, linking through an enrollment code
and provisioning as `pending`. Microsoft Entra ID works as the reference provider with a documented
example configuration. This is the login path of company instances (baseline B9, ADR 0019);
provider tokens never reach a device and are never stored.

## Read first

- `docs/spec/08-identity.md`: 2.1 "Port", 2.2 "OIDC: server as relying party, app as client of
  the server", 2.3 "Example: Microsoft Entra ID", 3.1 "Mapping an external identity", 3.2
  "Enrollment code", 3.3 "Linking and unlinking another identity", 3.5 "Delay when disabled at
  the provider", 5.1 "Registration", 5.2 "Approval", 7.3 "DPoP", 13 "Login protection", 14.1
  "Login with OIDC (native app)", 15 "Tests" (ID-T01 to ID-T05, ID-T27), 16 (R-ID-01, R-ID-02),
  18 "Configuration values of this chapter".
- `docs/spec/03-data-model.md`: 1 "Principles" (points 1, 9, 12, 13); 4.2 tables `principals`,
  `users`, `identity_providers`, `user_identities`, `enrollment_codes`, `auth_login_flows`,
  `devices`, `device_keys`, `auth_sessions`.
- `docs/spec/05-actions.md` and `docs/spec/actions.yaml`: `auth.login.begin`,
  `auth.login.complete`, `auth.token.refresh`, `identity.link`; 2.4 and 2.5; 3 "Rate-limit classes"
  (`rl.public_auth`, `rl.admin`).
- `docs/spec/06-api.md`: 3 "Authentication per principal kind", 3.1 "DPoP", 3.2 "Login flows",
  5 "Errors", 7 "Idempotency" (point 3: login flows carry no `Idempotency-Key`).
- `docs/spec/07-permissions.md`: 4.1 "Decisions and reasons", 4.2 "Flow".
- `docs/spec/19-security.md`: 8 "SSRF ban and outgoing connections" (SEC-NET-1 to
  SEC-NET-5), 9 "Logs without secrets and without content", 10 "Secrets".
- `docs/spec/02-architecture.md`: 4.1 "Rules", 4.4 "Allowed dependencies", 5 "Layers within a
  module", 9 "Request path", 12.3 "Job catalogue" (`identity.auth_cleanup`), 13
  "Configuration loading".
- `docs/spec/16-operations.md`: 4.3 "Edge and identity".
- ADRs 0011, 0013, 0015, 0019, 0020.
- `docs/api/openapi.yaml`: operations `beginOidcLogin`, `handleOidcCallback`,
  `completeOidcLogin`, `beginIdentityLink`, `completeIdentityLink`; schemas `OidcBeginRequest`,
  `OidcBeginResponse`, `OidcCompleteRequest`, `IdentityLinkBegin`, `IdentityLinkComplete`,
  `DeviceRegistration`, `TokenResponse`, `Identity`.
- `docs/security/threat-model.md`: T-ID-01, T-ID-06, T-NW-01.
- The merged READMEs and package docs of `services/server/internal/identity`,
  `internal/authz`, `internal/platform/egress`, `internal/platform/secrets`, `internal/kernel/*`,
  and the allow-list `services/server/api/operations.yaml`.
- `docs/PLAN.md` section 3 "Way of working per ticket" and section 7 rows M1-33 to M1-42;
  `docs/quality/gates.md`; `CONTRIBUTING.md`.

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
6. Test first: every behavior change has a test that was red before the change (the red run is
   quoted in the report), then green, then refactor.
7. No en dash or em dash characters in any file you write; use a hyphen or rephrase.
8. White label: no company, person, host or tenant names; domains only `example.com` and
   `example.org`, people only Alex, Sam, Robin; example credentials contain the word `EXAMPLE`.
9. Limits, time windows and thresholds only through named configuration keys listed in
   `docs/operations/configuration.md`; a new key goes into the chapter table and the configuration
   schema in the same change.
10. The server is a confidential client at the provider and the authorization server of its apps;
    PKCE `S256` on both legs (08 2.2, R-ID-01).
11. A person is matched only by `(provider_id, issuer, subject)` or by a valid enrollment code,
    never by e-mail or display name (08 3.1, R-ID-02, ID-T04).
12. ID token checks: signature over the provider JWKS, algorithms only `RS256`, `PS256`, `ES256`,
    `EdDSA`, `iss`, `aud`, `azp`, `exp`, `iat` with `auth.clock_leeway`, `nonce` (08 2.2 step 5,
    ID-T03); mix-up protection with the `iss` response parameter (RFC 9207) or the ID token `iss`
    (08 2.2 step 4, ID-T02); Entra tenant allow-list (08 2.3, ID-T05).
13. `allow_provisioning` creates a person with status `pending`, without grants, device `pending`
    (08 3.1 step 3).
14. Discovery, JWKS and token requests only through the egress client (SEC-NET-1, SEC-NET-2,
    SEC-NET-4); JWKS cached for `auth.oidc_jwks_cache_ttl`, reloaded once for an unknown `kid`
    (08 2.2 step 5).
15. A new login at the provider at the latest after `auth.idp_recheck_interval` (08 3.5, T-ID-06).
16. Client secret and client key only as `secret_ref` resolved by the secret provider (03 point 9,
    19 section 10). Provider tokens (`access_token`, `refresh_token`, `id_token`), `code`,
    `state`, `nonce`, `login_code` and `client_verifier` never appear in a log, trace, audit
    detail, event or error body (SEC-LOG-1 to SEC-LOG-4, SEC-SEC-3).

## Scope

Layering (02 "6. Code layout", PLAN row M1-41): a package outside the layers would fail the
architecture gate (02 7.2, "package without a component"), so the work lands in the layers of the
module:

- `services/server/internal/identity/domain/oidcflow/`: pure code without I/O and without
  third-party imports: flow state machine (`started` -> `code_issued` -> `completed`, any state ->
  `failed`), validation of `app_redirect_uri` and `client_challenge`, PKCE helpers
  (`S256(verifier)`, constant-time compare), derivation of the server PKCE verifier, ID token claim
  validation over already parsed claims, handle derivation for provisioning, callback error codes.
- `services/server/internal/identity/app/`: use cases `BeginOIDCLogin`, `HandleOIDCCallback`,
  `CompleteOIDCLogin`, `BeginIdentityLink`, `CompleteIdentityLink`; outbound port
  `OIDCProviderClient` with `Metadata(ctx, providerID) (ProviderMetadata, error)`,
  `ExchangeCode(ctx, providerID, code, codeVerifier, redirectURI) (IDTokenClaims, error)` (the
  adapter verifies the JWS and returns parsed claims; the use case validates the claims with the
  domain functions); repository methods for `auth_login_flows`.
- `services/server/internal/identity/adapters/oidc/`: the provider client over
  `internal/platform/egress`: discovery, JWKS cache, token request with client authentication,
  JWS verification and the client assertion through `internal/kernel/jose` (ADR 0067; add
  `RS256` and `PS256` verification and `PS256`, `RS256`, `ES256` signing there, with the RFC test
  vectors the ADR names). Add no JOSE library and no OIDC or OAuth client library: every check
  must be visible here and run through the egress client.
- `services/server/internal/identity/adapters/oidc/oidctest/`: the fake identity provider (Design
  notes, "Fake IdP"); it is the test double named in `docs/PLAN.md` section 5.
- `services/server/internal/identity/adapters/postgres/`: sqlc queries for `auth_login_flows`
  (`queries/*.sql`, regenerated `sqlcgen/`).
- `services/server/internal/identity/adapters/httpapi/`: handlers for `beginOidcLogin`,
  `handleOidcCallback`, `completeOidcLogin`, `beginIdentityLink`, `completeIdentityLink`.
- `services/server/migrations/`: one new goose migration with the next free sequential number in
  the format of the existing files, creating `auth_login_flows` exactly as in 03 (after the
  revision named in the preconditions) with its indexes; `make migrations-lint` green.
- Config validator for `identity_provider.oidc.v1` registered with the provider registry of M1-36
  (Design notes, "Provider configuration").
- Job `identity.auth_cleanup` (created by M1-35, 02 12.3): also deletes `auth_login_flows` rows
  with `expires_at < now()`.
- The refresh use case of M1-34: the re-login rule of `auth.idp_recheck_interval` (Design notes).
- `internal/platform/egress` is used as M1-22 built it: one client per provider with
  `Options.Hosts` (the host policy of "Egress targets" below) and, in tests, `Deps.RootCAs`; no
  change to the egress package (19 SEC-NET-2).
- `internal/wire`: wiring of the use cases, the provider client and the new config key.
- Configuration: register `auth.oidc_client_assertion_ttl` (08 section 18) in the configuration
  schema of `platform/config`; `auth.oidc_jwks_cache_ttl` covers discovery document and keys.
  Regenerate the reference if the schema changes (`make config-reference`, `make config-doc-check`).
- The specification is in place (planning, spec first): 08 2.2 (account effects only at complete,
  enrollment code at begin, `iss`, callback redirect parameters and error codes, link mode), 2.6
  (`identity_provider.oidc.v1`), 3.1, 14.1 and 18; 03 `auth_login_flows` with `external_identity`
  and `enrollment_code_id` (R-DM-30); `openapi.yaml` (`OidcBeginRequest.enrollment_code`,
  `handleOidcCallback` with `iss` and the redirect description). Change them only where the
  implementation proves them wrong. `services/server/api/operations.yaml`: the five operations move
  from `later` to the built list; regenerated server code (`make codegen-check` without diff).
- Docs: `docs/operations/oidc-entra.md` (new, English like the other operations pages): app
  registration as web application, the single redirect URI `https://<host>/api/v1/auth/oidc/callback`,
  certificate credential for `private_key_jwt`, optional `groups` claim (pointer to M1-42),
  tenant allow-list, a complete `identity_provider.oidc.v1` example with `EXAMPLE` values and
  `example.com` hosts; add it to the `nav` of `mkdocs.yml` next to "Tailscale".
- `services/server/internal/identity/README.md` (OIDC section, ports, job), CHANGELOG entry under
  "Unreleased" of `services/server/CHANGELOG.md`, `docs/DECISIONS.md` entries for every ruling of
  this brief, `docs/security/threat-model.md` T-ID-01 test column with the Go test names.

## Out of scope

- Groups from claims, the `groups_claim` provider setting and the membership sync: M1-42.
- Trusted proxy header login: M1-43. Local accounts, passkeys: M1-39, M1-40.
- Provider CRUD, `user_identities` reads and `unlinkIdentity`: M1-36. Enrollment code issuance
  and the setup lock: M1-37. Device approval and pairing: M1-38. Sessions, token pair, refresh
  families: M1-34 (only the re-login rule is added here).
- Step-up by OIDC re-authentication (`prompt=login`, `max_age=0`, `step_up_method = oidc_reauth`)
  for the web console: M6-01.
- Automatic takeover of provider-side account locks (SCIM, Shared Signals): proposed ADR, not M1.
- Microsoft Graph calls for group overage: never (08 2.3).

## Boundaries

- A new migration needs its line in `migrations.sha256`: run `make migrations-lock` after creating
  the file and before `make check`; never edit a merged migration (M1-01, ADR 0007).
- Work in a dedicated git worktree on the branch `feat/M1-41-identity-oidc`, created from the base
  branch the launcher names.
- Conventional Commits with the module as scope (for example `feat(identity): ...`) and a DCO
  sign-off on every commit (`git commit -s`).
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the Scope list, except the docs and changelog entries named in this
  brief.
- Follow the host rules supplied by the launcher.
- Tests bind only `127.0.0.1` and port 0 (P-10). PostgreSQL only through the disposable container
  helper `internal/platform/pg/pgtest`; without a Docker-compatible runtime the test fails, it
  never skips (P-09).
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at
  the end.
- Never read or print secrets; test keys and secrets are generated at runtime.
- No network calls except package downloads for pinned tools; never contact a real identity
  provider.
- A gap in the core contract (00, 03, 04, 05, `docs/spec/actions.yaml`) ends the ticket with
  `Verdict: BLOCKED` (P-11); never build a deviation.

## Design notes

**Preconditions (check before writing code; any failure: `Verdict: BLOCKED <which>`).**

1. `docs/spec/03-data-model.md` table `auth_login_flows` has `external_identity jsonb` (schema
   `external_identity.v1`) and `enrollment_code_id` (FK `enrollment_codes(id)` ON DELETE SET NULL)
   with their checks (R-DM-30). Reason: see "Ruling: account effects only at complete".
2. M1-36 provides the provider registry with per-kind config validation and the `(issuer,
   subject)` lookup; M1-37 provides enrollment-code verification (attempt counting) and
   consumption; M1-33, M1-34 and M1-37 provide device registration from `DeviceRegistration`
   (DPoP proof without `ath`, keys `dpop` and `step_up`), session creation and the token pair.
   Find them through `internal/identity/README.md`; reuse them, never duplicate them.
3. `platform/egress` (M1-22) exists and enforces SEC-NET-3 at connect time.

**Ruling: account effects only at complete.** 08 2.2 step 6 maps the account at the callback. The
`authorization_url` is not bound to the browser that finishes the provider login (there is no
browser session), so anyone who obtains it can finish the provider login with their own provider
account. If the callback linked an identity (enrollment code or link mode), a leaked URL would bind
a foreign provider account to the victim. Therefore the callback only verifies the ID token and
stores the verified external identity in the flow; mapping, linking, provisioning, device, session
and tokens happen at `complete`, which requires the `client_verifier` that only the app instance
that began the flow holds. 08 2.2, 3.1, the diagram 14.1 and R-DM-30 state this.

**Provider configuration `identity_provider.oidc.v1`** (decoded strictly, unknown fields rejected
with `400 validation_failed`; normative table in 08 2.6):

| Field | Type | Required | Meaning |
|---|---|---|---|
| `issuer` | string, `https` URL without query and fragment | yes | exact issuer; for Entra multi-tenant it may contain the literal `{tenantid}` |
| `client_id` | string | yes | client ID at the provider |
| `client_auth` | `private_key_jwt` or `client_secret_basic` | yes | client authentication at the token endpoint; `private_key_jwt` preferred (08 2.2 step 5) |
| `client_secret_ref` | secret reference | exactly with `client_secret_basic` | resolved by `platform/secrets` |
| `client_key_ref` | secret reference to a PKCS#8 PEM private key | exactly with `private_key_jwt` | resolved by `platform/secrets` |
| `client_key_alg` | `PS256`, `RS256` or `ES256` | no, default `PS256` | JWS algorithm of the client assertion |
| `client_key_id` | string | no | JWS header `kid` |
| `client_cert_x5t_s256` | base64url string | no | JWS header `x5t#S256` (Entra certificate credentials need it) |
| `allowed_tenant_ids` | list of strings | yes if `issuer` contains `{tenantid}` | allowed values of the `tid` claim |
| `app_redirect_uris` | list of strings | no | exact private-scheme or claimed `https` redirect URIs of the apps |
| `groups_claim` | string | no | name of the groups claim; accepted and stored here, used by M1-42 (08 3.4) |

Entra facts for the docs page, retrieved 2026-10-10: certificate assertions use `alg` `PS256`,
`typ` `JWT`, header `x5t#S256`, claims `aud` (token endpoint), `iss` and `sub` (client ID),
`jti`, `nbf`, `exp`, `iat`
(https://learn.microsoft.com/en-us/entra/identity-platform/certificate-credentials) [verified].

**Redirect URI rules** (`app_redirect_uri` at begin): allowed are (a) an exact string match with an
entry of `app_redirect_uris`, (b) `http://127.0.0.1:<port>/callback` or `http://[::1]:<port>/callback`
with a port 1 to 65535 (RFC 8252 7.3). Rejected: `localhost`, any other `http` host, userinfo,
query, fragment, any path other than `/callback` for loopback. Violation: `400 validation_failed`
with pointer `/app_redirect_uri`.

**`client_challenge` and `client_verifier`.** `client_challenge` is exactly 43 base64url characters
without padding, else `400`. `client_verifier` follows RFC 7636 (43 to 128 characters of the
unreserved set). Check `base64url(SHA-256(client_verifier)) == client_challenge` in constant time.
Test vector RFC 7636 appendix B: verifier `dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk` gives
`E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM`.

**Server PKCE towards the provider.** The verifier is not stored (03 `auth_login_flows`): it is
`base64url(HMAC-SHA-256(pepper, "oidc-pkce-verifier:" || flow_id))` with the pepper of
`auth.token_pepper_ref` (43 characters); the challenge is its `S256`. Always send
`code_challenge_method=S256`.

**`beginOidcLogin`** (`POST /api/v1/auth/oidc/begin`, anonymous, `auth.login.begin`,
`rl.public_auth`, no `Idempotency-Key`):

1. `authorize(anonymous, auth.login.begin, instance)`.
2. Load the provider: missing, other kind or `enabled = false` all give the same `404 not_found`.
3. Validate `app_redirect_uri` and `client_challenge`.
4. Optional `enrollment_code` (new optional field, specification first): verify with the M1-37
   verifier (counts a failed attempt, rejects used, expired or exhausted codes); purposes
   `first_login`, `identity_link` and `recovery` are accepted. Any failure: `401 unauthenticated`
   without detail. The code is not consumed here.
5. Load provider metadata (cache below). Discovery checks: document `issuer` equals the configured
   `issuer` string exactly; `response_types_supported` contains `code`; if
   `code_challenge_methods_supported` is present it contains `S256`;
   `id_token_signing_alg_values_supported` intersects the allowed algorithms; `token_endpoint` and
   `jwks_uri` are `https`. A failed check gives `503 unavailable` and one warning log with the
   provider ID only.
6. Insert the flow: `id` UUIDv7, `mode = 'login'`, `state` and `nonce` each 32 bytes from
   `crypto/rand` as base64url, stored only as HMAC (`oidc_state_hash`, `oidc_nonce_hash`),
   `client_challenge`, `app_redirect_uri`, the enrollment code reference, `status = 'started'`,
   `expires_at = now + auth.login_flow_ttl`.
7. `authorization_url` = `authorization_endpoint` with `response_type=code`, `client_id`,
   `redirect_uri = <server.public_url>/api/v1/auth/oidc/callback`, `scope=openid profile email`,
   `state`, `nonce`, `code_challenge`, `code_challenge_method=S256`. Respond `200` with
   `login_attempt_id`, `authorization_url`, `expires_at`; header `Cache-Control: no-store`.

**`handleOidcCallback`** (`GET /api/v1/auth/oidc/callback`, anonymous, `auth.login.begin`, audit of
denials):

1. Find the flow by `HMAC(state)`. Unknown, expired, or `status <> 'started'`: `400
   validation_failed` as problem JSON, no redirect (an unknown flow has no trusted redirect
   target), audit deny.
2. From here every failure sets `status = 'failed'`, `error = <code>` and answers `302` to
   `app_redirect_uri` with query parameters `error=<code>` and `login_attempt_id=<id>`, plus an
   audit deny with details `{provider_id, login_attempt_id, error}`. Closed list of codes:
   `access_denied` (provider sent `error`), `login_failed` (any check below failed),
   `flow_expired`. No other detail reaches the app.
3. `iss` response parameter (RFC 9207): if present it must equal the expected issuer; if discovery
   says `authorization_response_iss_parameter_supported: true`, it is required.
4. Token request through the egress client: `grant_type=authorization_code`, `code`,
   `redirect_uri`, `code_verifier`, client authentication:
    - `client_secret_basic`: RFC 6749 2.3.1 (form-urlencoded ID and secret in the Basic header).
    - `private_key_jwt`: `client_assertion_type=urn:ietf:params:oauth:client-assertion-type:jwt-bearer`,
      `client_assertion` = JWS with header `alg` from `client_key_alg`, `typ` `JWT`, optional
      `kid` and `x5t#S256`; claims `iss` and `sub` = `client_id`, `aud` = `token_endpoint`, `jti`
      (UUIDv7), `iat` and `nbf` = now, `exp = now + auth.oidc_client_assertion_ttl`.
    The response must contain `id_token`; provider `access_token` and `refresh_token` are dropped
    unread.
5. ID token: compact JWS only; header `alg` in `RS256`, `PS256`, `ES256`, `EdDSA` and consistent
   with the key type (never `none`, never `HS*`); `kid` found in the JWKS (unknown `kid`: one
   forced reload, single-flight per provider, then failure); `iss` equals the expected issuer
   (with `{tenantid}` replaced by the token's `tid`); if `allowed_tenant_ids` is set, `tid` is in
   the list; `aud` contains `client_id`; with more than one audience `azp` is required; if `azp`
   is present it equals `client_id`; `exp > now - auth.clock_leeway`; `iat <= now +
   auth.clock_leeway` and `iat >= flow.created_at - auth.clock_leeway`; `HMAC(nonce)` equals
   `oidc_nonce_hash`; `sub` not empty.
6. External identity (stored in the flow, schema `external_identity.v1`, documented in 08):
   `issuer`, `subject`, `display_name` (claim `name`, else `preferred_username`, else absent),
   `email_hint` (claim `email`, display only), `auth_time` (claim `auth_time`, else `iat`), `amr`.
7. One conditional update (`WHERE status = 'started'`): external identity, `login_code` = `lc_`
   plus 32 random bytes as base64url, stored only as `login_code_hash`,
   `login_code_expires_at = now + auth.server_code_ttl`, `status = 'code_issued'`. Respond `302`
   to `app_redirect_uri` with `login_code` and `login_attempt_id` appended to the existing query;
   headers `Cache-Control: no-store` and `Referrer-Policy: no-referrer`.

**`completeOidcLogin`** (`POST /api/v1/auth/oidc/complete`, anonymous, `auth.login.complete`,
header `DPoP` without `ath`):

1. Find the flow by `HMAC(login_code)` with `mode = 'login'`, `status = 'code_issued'`, code and
   flow not expired; otherwise `401 unauthenticated` without detail.
2. PKCE check; mismatch sets `status = 'failed'`, answers `401`, audit deny (ID-T02).
3. One Unit of Work:
    1. Identity lookup by `(provider_id, subject)` with `unlinked_at IS NULL` and `issuer` equal.
    2. Else, if the flow carries an enrollment code: consume it with the M1-37 function (still
       valid, `used_at` set, `used_by_device_id` = the registering device), insert
       `user_identities` (`issuer`, `subject`, `email_at_link` from `email_hint`), audit
       `identity.link` with the person as actor. The device becomes `active` (08 3.2).
    3. Else, if `allow_provisioning`: new principal `kind = 'user'`, `status = 'pending'`, `users`
       row with `email` NULL (never copy the provider e-mail into `users.email`: it is unique and
       would let the provider block or steer logins), identity row; handle and display name below.
    4. Else: roll back, `401 unauthenticated` with the neutral problem body, audit deny in its own
       transaction, flow `failed` (ID-T04).
    5. Principal `disabled` or `deleted`: `401 unauthenticated`, nothing written.
    6. Device registration, session (`auth_method = 'oidc'`, `identity_id`) and token pair through
       the shared functions; `user_identities.last_login_at = now`; flow `status = 'completed'` and
       the external identity set to NULL (data minimization); audit allow `auth.login.complete`.
4. `201 TokenResponse`, `Cache-Control: no-store`.

**Provisioned handle and display name.** Handle from the local part of `preferred_username`, else
from `name`, else `user`; lowercase; characters outside `[a-z0-9._-]` become `-`; leading
characters until the first `[a-z0-9]` are dropped; cut to `principal.handle_max_chars`; on a
case-insensitive collision append `-` and four characters from `[a-z0-9]` (crypto random) within the
length limit and retry. Display name: `display_name` with control, format and bidi characters
removed (so that a provider value cannot block the login), else the handle.

**Link mode** (`beginIdentityLink`, `completeIdentityLink`: human session, `identity.link` with
step-up, `rl.admin`): begin as above with `mode = 'link'`, `user_id` = actor, no enrollment code;
the callback is shared; complete requires the same actor (`flow.user_id` differs: `404
not_found`), PKCE, then inserts `user_identities` in one Unit of Work; a subject already linked to
anyone gives `409 conflict` (partial unique index); audit allow `identity.link`; `201 Identity`.
A `401 step_up_required` does not consume the `login_code`.

**Re-login rule (`auth.idp_recheck_interval`).** In the refresh use case: a session with
`auth_method = 'oidc'` and `now - created_at >= auth.idp_recheck_interval` is revoked
(`revoke_reason = 'expired'`, family revoked) and the refresh answers `401 unauthenticated` (audit
deny). `created_at` of an OIDC session is the time of the provider login.

**Metadata cache.** Per provider: discovery document and JWKS cached for
`auth.oidc_jwks_cache_ttl` (08 18: provider metadata and keys);
single-flight loading; responses limited by `egress.max_response_bytes` and `egress.timeout`.

**Egress targets.** Per provider: the issuer host, then the hosts of `token_endpoint` and
`jwks_uri` from the validated discovery document; all `https`; redirects only within these hosts
(SEC-NET-4); IP checks at connect time stay with the egress client.

**Configuration key** (08 section 18, register it in the schema): `auth.oidc_client_assertion_ttl`,
default `2m`, lifetime of a `private_key_jwt` client assertion.

**Fake IdP** (`oidctest`): `httptest.NewTLSServer` on `127.0.0.1:0`; endpoints
`/.well-known/openid-configuration`, `/authorize` (records `code_challenge`, answers `302` to
`redirect_uri` with `code`, `state` and optionally `iss`), `/token` (checks `grant_type`,
`redirect_uri`, PKCE `S256` against the recorded challenge, client authentication: Basic secret or
a `private_key_jwt` assertion against a registered public key, `aud`, `exp`, unique `jti`),
`/jwks`. Knobs: ID token header and claim overrides (`alg=none`, `HS256`, wrong `aud`, extra
audience without `azp`, wrong `nonce`, expired, foreign `iss`, `tid`), key rotation, omitted `iss`
parameter, provider `error`, Entra mode with issuer template `https://127.0.0.1:<port>/{tenantid}/v2.0`,
request counters per endpoint. Tests configure the egress client with `egress.allowed_hosts` entry
`127.0.0.1` and `allow_private: true` and with the test server's root CA (`Deps.RootCAs` of
M1-22).

**Audit and logs.** Callback: deny entries only (`auth.login.begin` column `abl`). Complete: allow
and deny. Details carry IDs and codes only (`provider_id`, `login_attempt_id`, `device_id`,
`error`), never subject, e-mail, name, codes or tokens. Logs use route patterns, never query
strings (SEC-LOG-3).

## Tests (write them first)

Unit tests (`domain/oidcflow`, table-driven):

- `TestRedirectURIRules`: loopback IPv4 and IPv6 with any port and `/callback` accepted;
  `localhost`, other `http` hosts, userinfo, query, fragment, other loopback paths rejected; exact
  private scheme and claimed `https` entries accepted only on exact match.
- `TestClientChallengeFormat`, `TestPKCES256Vector` (RFC 7636 appendix B), `TestServerVerifierDerivation`
  (deterministic per flow ID and pepper, different per flow, 43 characters).
- `TestIDTokenClaimValidation`: every rule of step 5 with a passing and a failing case, including
  the boundaries of `auth.clock_leeway`.
- `TestProvisionedHandle`: collisions, length limit, invalid characters, empty input; display name
  stripping of bidi and control characters.
- `TestFlowStateMachine`: only the allowed transitions.

Integration tests (PostgreSQL 18 via `pgtest`, fake IdP, real HTTP chain on `127.0.0.1:0`, steerable
clock):

- `TestIDT01_PKCEDowngradeRejected`: begin without or with a malformed `client_challenge` gives
  `400`; the fake IdP sees `code_challenge_method=S256` and a matching `code_verifier` on every
  exchange.
- `TestIDT02_StateIssAndVerifier`: unknown `state` gives `400` and an audit deny; foreign `iss`
  parameter and a missing required `iss` give a redirect with `error=login_failed`; complete with a
  wrong `client_verifier` gives `401`, the flow is `failed`, audit deny.
- `TestIDT03_IDTokenRejected`: `alg=none`, `HS256`, wrong `aud`, wrong `nonce`, expired: each
  ends with `error=login_failed` and no `login_code`.
- `TestIDT04_NoMappingByEmail`: unknown `sub` with the e-mail of an existing person: complete
  gives the neutral `401`, no identity row, audit deny.
- `TestIDT05_EntraTenantAllowList`: issuer template with `tid` outside `allowed_tenant_ids`
  rejected, inside accepted.
- `TestOIDCLogin_KnownIdentity`: full flow; `201 TokenResponse`; session `auth_method = 'oidc'`;
  `last_login_at` set; external identity cleared.
- `TestOIDCLogin_EnrollmentCodeLinksAndActivatesDevice`: code consumed with `used_by_device_id`,
  device `active`, `identity.link` audited; a second use of the same code fails.
- `TestOIDCLogin_LeakedAuthorizationURLCannotLink`: a second party finishes the provider login
  with its own account on a flow begun with Alex's enrollment code, obtains the `login_code`, and
  cannot complete without the verifier; no `user_identities` row exists afterwards and the code is
  unused.
- `TestOIDCLogin_ProvisioningPending`: person and device `pending`, no grants, `users.email` NULL.
- `TestOIDCLogin_DisabledPersonRejected`, `TestOIDCLogin_LoginCodeSingleUse`,
  `TestOIDCLogin_LoginCodeExpires`, `TestOIDCLogin_FlowExpires`, `TestOIDCCallback_Replay`.
- `TestIdentityLink_StepUpRequired`, `TestIdentityLink_OtherActor404`,
  `TestIdentityLink_SubjectAlreadyLinked409`, `TestIdentityLink_Success`.
- `TestOIDCClientAuth_PrivateKeyJWT` (assertion header and claims as specified, `jti` unique,
  `exp` from `auth.oidc_client_assertion_ttl`) and `TestOIDCClientAuth_SecretBasic`.
- `TestJWKS_UnknownKidReloadsOnce` and `TestMetadataCacheTTL` (request counters of the fake IdP).
- `TestOIDCEgress_PrivateTargetsRejected`: an issuer or a discovered `jwks_uri` pointing to a
  private or link-local address without an allow entry fails with `login_failed` and no
  connection.
- `TestOIDCSecretsNeverLogged`: log and audit capture contain none of `code`, `state`, `nonce`,
  `login_code`, `client_verifier`, `id_token`, provider tokens.
- `TestIDPRecheckForcesRelogin`: refresh after `auth.idp_recheck_interval` gives `401`, session
  revoked with `expired`.
- `TestLoginFlowCleanup`: the cleanup job deletes only expired flows.
- The route count test of M1-17 (AZ-P1) passes with the five operations in the allow-list, and
  every response validates against the schema (response validation of the test build).

Commands that must pass:

```sh
cd services/server && go test -race ./internal/identity/... ./internal/platform/egress/...
make codegen-check
make arch
make migrations-lint
make api-lint
make config-doc-check
make docs
make check
```

## Acceptance criteria

- [ ] Preconditions verified and named in the report.
- [ ] ID-T01 to ID-T05 green with the test names above; T-ID-01 in the threat model names them.
- [ ] No account effect before `completeOidcLogin` or `completeIdentityLink`; the leaked-URL test
      is green.
- [ ] Mapping only by `(provider_id, issuer, subject)` or enrollment code; provisioning creates
      `pending` without grants.
- [ ] Discovery, JWKS and token calls go only through `platform/egress`; private targets are
      rejected.
- [ ] JWKS and metadata cache honor `auth.oidc_jwks_cache_ttl` and reload once for an unknown
      `kid`.
- [ ] Refresh of an OIDC session older than `auth.idp_recheck_interval` is refused.
- [ ] No secret or token in logs, audit details or problem bodies.
- [ ] The implementation matches 03, 08 and `openapi.yaml` (changed only where proven wrong);
      `operations.yaml`, the configuration reference, `oidc-entra.md` and the DECISIONS entries
      are updated; `make codegen-check` shows no diff.
- [ ] `make check` green.

## Definition of done

- Code and tests as above; every public symbol has a doc comment (purpose, parameters, return
  value, errors, example where useful); comments explain why.
- `internal/identity/README.md` current (OIDC flow, ports, job, tables); module CHANGELOG entry
  under "Unreleased"; `docs/DECISIONS.md` entries; no new ADR unless a decision beyond ADR 0019
  was taken (then a MADR 4 ADR, headings and text in English, and `make adr-index`).
- `make check` green.
- Independent review (`make review` and a code review over the diff) and the red team gate
  (`docs/quality/red-team-gate.md`) over the diff, verdict recorded in the report; on `NEIN` fix,
  add a test per finding and re-check the delta.
- Merge locally into the base branch only if review and red team are green and `make check` is
  green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then the
commits, the files with line counts, the test results with counts (including the red runs), what
is not proven, and follow-ups that could not be done in scope with the reason. Then write
`{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
