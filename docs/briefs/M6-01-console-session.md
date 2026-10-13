# M6-01: Console session following the BFF pattern

Status: ready | Depends on: M1-39, M1-40, M1-41, M1-43, M1-45, M1-49, M1-64 | Milestone: M6

## Goal

After this ticket a person can log in to the web console without any token ever reaching the browser (08 7.5, R-ID-06, RFC 10017): the login operations of the people (OIDC, passkey, password with TOTP, proxy header) gain a console form that registers a device with `platform = 'web'`, starts a session with `client_component = 'console'` and answers with a cookie `__Host-console_session` (`Secure`, `HttpOnly`, `SameSite=Strict`, `Path=/`, no `Domain`) whose value is 256 bits of randomness, stored only as an HMAC in `auth_sessions.cookie_hash`. Every later console request authenticates by that cookie through the same `kernel/actor.Authenticator` as the apps; every state-changing console request must carry the fixed CSRF header that this ticket names in `openapi.yaml` and an `Origin` equal to the console origin, also on WebSocket upgrades (SEC-CORS-3, SEC-CORS-4). Directly after login the console session can read only the person's own profile until a step-up succeeds; besides the passkey (M1-40) the console gets the two step-up methods 08 6.2 promises to it: password with TOTP and a new OIDC login with `prompt=login` and `max_age=0`, specified first in `openapi.yaml`. The cookie value is replaced at login and after every step-up (session fixation), the session ends after `auth.console_session_max_age` or `auth.console_session_idle_timeout` without use. "View as person" becomes a state of the console session with a start and an end (07 9.2, R-DM-49, R-AZ-32): `viewAsUser` from the console stores the target and the start on the session, every later console request carries `actor.ViewAs` until the end operation or the expiry after `authz.view_as_max_duration` (the first request after it gets `401 step_up_required`, 07 9.2), with audit at start and end; ending the view, logging out, the step-up and reading one's own session are always decided without `view_as` (R-AZ-32). ID-T19 is green. Why: owners read all content except DMs through the console, so its session must neither expose tokens to scripts nor depend on a password alone (08 7.5); M6-02 to M6-18 build the console on this session.

## Read first

- `docs/briefs/M1-64-gateway-connection.md` is listed below; the gateway upgrade and its `Origin` check come from M1-64.
- `docs/spec/08-identity.md`: 2.4 (proxy header login, loopback only), 2.5 (local accounts, password with TOTP), 6.1 (row "Web console": no device key), 6.2 (the paragraph "Web console": WebAuthn with `userVerification = required`, without a passkey a new OIDC login with `prompt=login` and `max_age=0` and an `auth_time` after the start of the challenge, local accounts password with TOTP; "The operations for the last two methods come with the console session (PLAN M6-01), spec first"), 6.3 item 3 (console step-up for the terminal: passkey only; M6-18 enforces it), 7.1 (row "Console session"), 7.5 (all of it), 8 (row "Console"), 11 (disabling a person ends console sessions too), 13 (login protection), 14.3, 15 (ID-T19, ID-T27), 16 (R-ID-06), 18 (`auth.console_session_max_age`, `auth.console_session_idle_timeout`, `auth.webauthn_origins`).
- `docs/spec/07-permissions.md`: 3 (principal context, row `view_as`), 4.1 (reason `view_as_read_only`), 4.2 (steps 4 and 7), 9.2 ("View as person"; the last bullet asks for exactly what this ticket adds), SR5, SR6.
- `docs/spec/03-data-model.md` (core contract, read only): `auth_sessions` (`client_component`, `cookie_hash`, `step_up_method` with `password_totp` and `oidc_reauth`, the view-as columns of R-DM-49 and their checks), `devices` (`platform = 'web'`), `auth_challenges`, `auth_login_flows`, `local_credentials`; R-DM-49.
- `docs/spec/05-actions.md` (core contract, read only): 2.4 (step-up), rows `auth.step_up`, `auth.logout`, `self.read`, `access.view_as`, `access.view_as.end` (R-AZ-32), `session.revoke`; R-AZ-32.
- `docs/spec/06-api.md`: 3 "Authentication" (row "Person in the web console"), 4 (problem codes `step_up_required`, `unauthenticated`, `forbidden`, `csrf_failed` if you add it, see Design note 5).
- `docs/spec/19-security.md`: 3 (SEC-HDR-1 to SEC-HDR-3), 4 (SEC-CORS-1 to SEC-CORS-4), 14 (SEC-T03, SEC-T04).
- `docs/spec/02-architecture.md`: 3.2 "Request path" and the chain in the api node (where authentication runs), 9.1 (`rl.public_auth`).
- `docs/api/openapi.yaml`: operations `beginOidcLogin`, `handleOidcCallback`, `completeOidcLogin`, `beginPasskeyLogin`, `completePasskeyLogin`, `loginWithPassword`, `loginWithTrustedHeader`, `logout`, `createStepUpChallenge`, `completeStepUpWithPasskey`, `getMe`, `viewAsUser`; schemas `Session`, `StepUpChallenge`, `ViewAsSnapshot`; components `securitySchemes`.
- `docs/DECISIONS.md`: D-ID-*, D-W-100, D-W-120 (the extension of D-W-100 to M5 and M6), D-W-104 (the pattern of core contract amendments for briefs); section 10 (format of `D-M6-01-<n>` rows).
- ADRs: 0020 (device binding), 0028 (web only for the console), 0067 (JOSE profile).
- Briefs (merged names win, D-W-100): `M1-34-identity-sessions.md` (`ClientComponent`, `StartSession`, `Auth`, `SessionEnded`, the authenticator), `M1-35-identity-step-up.md` (`StepUp`, `CreateChallenge`), `M1-39-identity-local-accounts.md` (password and TOTP verification, lockout), `M1-40-identity-passkeys.md` (`CompleteStepUpWithPasskey`, `virtualauthn`), `M1-41-identity-oidc.md` (`BeginOIDCLogin`, `CompleteOIDCLogin`, the fake IdP `oidctest`), `M1-43-identity-trusted-header.md`, `M1-45-identity-revocation.md` (logout, `user.disable`, closing connections), `M1-49-authz-explain.md` (`viewAsUser`, `actor.ViewAs`), `M1-18-http-chain.md` (chain stages, security headers, CORS, problem writer, `httpservertest`), `M1-64-gateway-connection.md` (upgrade and `Origin` check of SEC-CORS-3).
- Code on the base branch (all under `services/server/`): `internal/identity` (port `auth.go`, `stepup.go`, `app/`, `adapters/httpapi`, `adapters/postgres`, `adapters/oidc/oidctest`, `adapters/webauthn/virtualauthn`), `internal/kernel/actor`, `internal/authz` (view-as), `internal/platform/httpserver`, `internal/gateway` (upgrade), `api/operations.yaml`, `internal/identity/README.md`.

## Fixed requirements that apply

Owner's fixed build rules (apply verbatim):

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

Rules for every brief:

6. Test first: write the failing test, run it and keep the red output for the report, then make it green, then clean up.
7. No en dash and no em dash in any file you write; the `text-rules` pre-commit hook rejects them. Use a hyphen or rephrase.
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory. The cookie name carries no product name (02 6 "Places that carry the product name").
9. Limits, windows and thresholds only through named configuration keys listed in `docs/operations/configuration.md`; code comments and docs name the key, never the number.
10. Specification first: a change to behaviour, API, event, right or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`, `actions.yaml`) is not edited by this ticket; R-DM-49 and R-AZ-32 are already in it. Everything this ticket adds to the repository is English; existing text stays as it is.

Product requirements implemented here:

11. No token of any kind reaches the browser: no access token, refresh token, preview token or login code appears in a console response body, header other than `Set-Cookie`, redirect URL or log (08 7.5, R-ID-06).
12. Cookie `__Host-console_session` with `Secure`, `HttpOnly`, `SameSite=Strict`, `Path=/`, without `Domain` and without `Max-Age` beyond `auth.console_session_max_age`; value 256 bits from the cryptographic random number generator (ID-T27); stored only as an HMAC-SHA-256 with the token pepper in `auth_sessions.cookie_hash`; new value at login and after every successful step-up (08 7.5).
13. A console session is a session of a device with `platform = 'web'`, `client_component = 'console'`, no device keys, no refresh family, no push registration, no child session; it ends at `auth.console_session_max_age` after creation and after `auth.console_session_idle_timeout` without a request (08 8).
14. CSRF: every state-changing console request (every method except `GET`, `HEAD`, `OPTIONS`) carries the fixed header that `openapi.yaml` names (Design note 5) and an `Origin` equal to the origin of `server.public_url`; otherwise `403` before the handler. A WebSocket upgrade with a cookie and a foreign or missing `Origin` is rejected with `403` before the upgrade (SEC-CORS-3, SEC-CORS-4, ID-T19).
15. After login and before the first successful step-up of this session, the console session may only call `getMe` (`self.read`), the step-up operations, `logout` and the discovery; everything else ends with `401 step_up_required` (08 7.5 "before that it only shows the person's own profile").
16. Console step-up methods: passkey (`userVerification = required`), a new OIDC login with `prompt=login` and `max_age=0` whose `auth_time` lies after the start of the challenge, and, for local accounts without a passkey, password with TOTP (08 6.2). `step_up_method` records which one. Step-up from a console session never applies to another session (08 6.2).
17. "View as person" in the console: start only by an owner with step-up and audit (`access.view_as`), state on the console session (R-DM-49), end by `access.view_as.end` (R-AZ-32) or after `authz.view_as_max_duration`, audit at start and end; writing actions during the view end with `forbidden` `view_as_read_only`, DMs of the person stay invisible (07 9.2, SR6).
18. Logout, `session.revoke`, `device.revoke` of the web device and `user.disable` end the console session at once; the next request with the old cookie gets `401` and a response that clears the cookie.

## Scope

All paths under `services/server/` unless they start with `docs/`. Every new Go file starts with `// SPDX-License-Identifier: Apache-2.0`.

Create:

- `internal/identity/domain/consolesession/`: pure rules: cookie value generation and HMAC derivation through a port for the pepper, the absolute and idle expiry decision, the "step-up seen" gate of requirement 15, the view-as expiry decision (Design notes 2, 3, 7).
- `internal/identity/app/console.go`: use cases `StartConsoleSession` (called by the login use cases in their console form), `AuthenticateConsoleCookie`, `RotateConsoleCookie`, `StartViewAs`, `EndViewAs`, `ConsoleStepUpWithPassword`, `BeginConsoleOIDCReauth`, `CompleteConsoleOIDCReauth` (Design notes 1, 4, 6, 7).
- `internal/identity/adapters/httpapi/console.go`: the cookie writer and reader, the handlers of the new operations of Design note 6.
- `internal/platform/httpserver`: a chain stage `ConsoleGuard` (CSRF header and `Origin` for cookie-authenticated requests; Design note 5) with its unit tests in the `httpservertest` harness.
- Migration `migrations/<next free number>_identity_console_session.sql`: the columns `auth_sessions.view_as_user_id` and `auth_sessions.view_as_started_at` with the checks and the foreign key of R-DM-49 (`make migrations-lint`, `make migrations-lock`).
- Tests named under "Tests".

Change:

- `docs/api/openapi.yaml` first (Design note 6): the console form of the login operations, the security scheme `consoleSession` (`apiKey`, `in: cookie`, name `__Host-console_session`), the CSRF header parameter component, the operations `stepUpWithPassword`, `beginStepUpOidcReauth`, `completeStepUpOidcReauth`, `endViewAs`, `getConsoleSession`, the schema `ConsoleSession`, the response header `Set-Cookie` on the console forms and on `logout`; `make api-lint`, `make api-checks`, `make codegen-check`.
- `internal/identity`: login use cases (OIDC, passkey, password with TOTP, proxy header) accept the console form; the authenticator (`kernel/actor.Authenticator`) accepts the cookie as a second credential kind (Design note 4); `Auth.RevokeSession` and the revocation paths of M1-45 also clear console sessions; `StartSession` gains the component `console` (M1-34 left it for this ticket); the job `identity.auth_cleanup` ends idle and expired console sessions.
- `internal/kernel/actor`: the `Credentials` input gains the cookie value (redacting type), and `Actor` exposes `ClientComponent` so `authorize` and later tickets can tell a console session (M6-18 needs it for the passkey rule).
- `internal/authz`: `viewAsUser` in a console session starts the stored view (Design note 7); the principal context reads `view_as` from the authenticated console session on every request; the exemption of R-AZ-32 for `access.view_as.end`, `auth.logout`, `auth.step_up` and `self.read` on one's own session (07 4.2 gets one sentence for it).
- `internal/gateway`: the upgrade check of SEC-CORS-3 also covers a request that carries the console cookie without an `Origin` (Design note 5).
- `api/operations.yaml`: enable the new operations; regenerate.
- Docs (all new text in English, existing text stays as it is): `docs/spec/08-identity.md` 6.2 and 7.5 (the operations, the cookie name, the CSRF header, the pre-step-up gate, the idle rule), `docs/spec/06-api.md` 3 (the row "Person in the web console" names header and cookie), `docs/spec/07-permissions.md` 9.2 (replace the "in M1" bullet by the stored view with start, end and expiry), `docs/spec/19-security.md` SEC-CORS-4 (header name); `internal/identity/README.md`, `internal/authz/README.md`; `services/server/CHANGELOG.md`; `docs/DECISIONS.md` section 10 rows `D-M6-01-1` to `D-M6-01-8`.

This ticket adds no table, no action (`access.view_as.end` and the columns are in the core contract already), no event type, no configuration key (all keys exist in 08 18 and 07) and no top-level Makefile target.

## Out of scope

- The console app, its login screens and serving it from the server (M6-02); the CSP and the headless browser test (M6-03); the console screens (M6-04 onward).
- The passkey-only rule for `terminal.open` and `terminal.attach` in the console and the stream authentication by cookie (M6-18).
- Device approval for the web device: the console has none; it requires step-up instead (08 7.5).
- Enrollment codes and device pairing in the console: the console uses only E2 and E7 (01 5.6).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M6-01-console-session`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(identity): ...`, `feat(httpserver): ...`, `docs(api): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL runs only as a throwaway container on a random port through `pgtest` (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets. Never log, print or persist a cookie value, a token, a code, a password or a TOTP value; logs of this ticket carry no person or session ID. No network calls except package downloads for pinned tools and Go modules; the OIDC provider is the fake IdP of M1-41 and passkeys come from `virtualauthn` of M1-40.
- Time only through `kernel/clock`; tests move a fake clock, they never sleep for an expiry.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M5 and M6 by D-W-120). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Console form of the login operations** (08 7.5, 14.1; Ruling for docs/DECISIONS.md). The four login paths a console offers (01 5.6 E2) keep their operations and gain a console form selected by the request member `client: console` (enum `app`, `console`; default `app`, so the apps are unchanged): `completeOidcLogin`, `completePasskeyLogin`, `loginWithPassword` (including its TOTP step) and `loginWithTrustedHeader`. In the console form the request carries no `device` block, no DPoP proof and no client-generated device ID; the server creates a device `{platform: web, state: active, name: "Web console"}` (translated name in the console, not on the server: the server stores the fixed English name) for this login, starts the session with `client_component = 'console'` through `StartSession` (the component M1-34 left for this ticket), issues no access and no refresh token, and answers `200` with the schema `ConsoleSession` (Design note 6) and `Set-Cookie`. For OIDC the `app_redirect_uri` of `beginOidcLogin` must be the console callback path on the origin of `server.public_url` (`/console/login/callback`); any other redirect for the console form is `400 validation_failed`. The console holds the `client_verifier` only in memory of the tab and completes with it (PKCE keeps working as in M1-41). Enrollment codes and pairing reject the console form (`400 validation_failed`). Reason: one login implementation per method; the console differs only in what the session gets and how it is carried. Each console login writes the same audit entry as the app login with the component `console`.

2. **Cookie** (08 7.5; ID-T27; Ruling for docs/DECISIONS.md). Name `__Host-console_session`. Value: 32 bytes from `crypto/rand`, base64url without padding. Stored: HMAC-SHA-256 with the token pepper of `auth.token_pepper_ref` (the same derivation as the other token hashes, 03 principle 9). Attributes exactly `Secure; HttpOnly; SameSite=Strict; Path=/`, no `Domain`, `Max-Age` = seconds until `expires_at`. Rotation: after a successful step-up and at login the server writes a new hash in the same transaction as `step_up_at` and returns the new cookie; the old value stops working at commit. Clearing: `logout` and every `401` caused by an unknown, expired, revoked or idle cookie answer with `Set-Cookie: __Host-console_session=; Max-Age=0; Secure; HttpOnly; SameSite=Strict; Path=/`. The cookie is never accepted from a request whose scheme is not HTTPS in production; tests run the chain behind the test TLS of M1-21 or with the transport protection's test switch, never by weakening the attributes.

3. **Expiry and idle** (08 8; Ruling for docs/DECISIONS.md). Absolute: `expires_at = created_at + auth.console_session_max_age` (already in 03). Idle: a request is accepted only if `now - last_seen_at < auth.console_session_idle_timeout`; an accepted request updates `last_seen_at` at most once per minute of the clock (to keep writes bounded; the window is a fixed fraction you document, not a new key, because it only bounds writes and changes no limit). An idle or expired session is revoked with `revoke_reason = 'expired'` by the request that finds it and by the job `identity.auth_cleanup`; both write the same audit entry once (the transition is guarded by `revoked_at IS NULL`).

4. **Authentication in the chain** (02 request path; M1-18). The authenticator gets a second credential kind: when a request has no `Authorization` header and has the cookie, it looks up the session by `cookie_hash` (unique partial index of 03), requires `client_component = 'console'`, not revoked, not expired, not idle, the device `active` and the person `active`, and yields an actor with `ClientComponent = console`, no DPoP binding and `ViewAs` from the stored view (Design note 7). A request with both an `Authorization` header and the cookie is `400 validation_failed` (one credential per request). A console actor never passes as an agent, app or token principal. Rate limits: the console counts like a person in a session (classes of the action); failed cookie lookups count in `rl.public_auth` like every failed credential (02 9.1).

5. **CSRF and `Origin`** (08 7.5; SEC-CORS-3, SEC-CORS-4; Ruling for docs/DECISIONS.md). Header name `X-Console-Request`, fixed value `1`; `openapi.yaml` gets a header parameter component `ConsoleRequest` with this name and `enum: ['1']`, referenced on every state-changing operation that the console may call, and the security scheme `consoleSession` lists it in its description. The chain stage `ConsoleGuard` runs after authentication and before the handler, only for cookie-authenticated requests: methods other than `GET`, `HEAD`, `OPTIONS` need the header with exactly that value and an `Origin` equal to the origin of `server.public_url` (scheme, host, port after normalization); a missing or wrong one is `403 forbidden` with `reason: csrf_check_failed` and no handler call, logged with the reason only. Requests with an `Authorization` header (apps, agents, tools) are not affected. WebSocket: the gateway upgrade (and every later stream upgrade, M6-18) rejects a request that carries the cookie and has a missing or foreign `Origin` with `403` before the upgrade; a foreign `Origin` without a cookie stays rejected as M1-64 built it. No CORS header is added anywhere (SEC-CORS-1). Reason for a custom header: a cross-site form or `fetch` without CORS cannot set it, and it needs no server state; `SameSite=Strict` is the second line.

6. **New operations and schemas in `openapi.yaml`** (spec first; all under `/api/v1`, security `consoleSession` unless noted):
    - `getConsoleSession` `GET /console/session`, action `self.read`: `ConsoleSession {user_id, display_name, expires_at, idle_timeout_seconds, step_up_at?, step_up_method?, step_up_valid_until?, step_up_methods_available: [passkey|oidc_reauth|password_totp], view_as?: {user_id, display_name, started_at, ends_at}}`. The console uses it after a reload to know its state (M6-02).
    - `stepUpWithPassword` `POST /auth/step-up/password`, action `auth.step_up`, body `{challenge_id, password, totp_code}`: only for console sessions of a person with a local account and `auth.local_password_enabled`; verifies through the M1-39 use case (lockout counting as for the login), sets `step_up_method = 'password_totp'`. From a non-console session it is `403 forbidden` (native apps always have a step-up key, 08 6.2).
    - `beginStepUpOidcReauth` `POST /auth/step-up/oidc`, action `auth.step_up`, body `{challenge_id, provider_id, app_redirect_uri, client_challenge}`: starts an OIDC flow with `prompt=login` and `max_age=0` bound to the challenge; returns `authorization_url`. `completeStepUpOidcReauth` `POST /auth/step-up/oidc/complete`, body `{login_code, client_verifier}`: requires the identity of the person (same `(issuer, subject)` as one of their linked identities) and `auth_time` after the creation of the challenge; sets `step_up_method = 'oidc_reauth'`. Both reuse the flow table and the fake IdP of M1-41.
    - `endViewAs` `DELETE /console/session/view-as`, action `access.view_as.end` (R-AZ-32): ends the stored view, `204`; without a view `204` as well (idempotent).
    - The console forms of the four login operations (Design note 1) answer `200 ConsoleSession` with `Set-Cookie`; `logout` answers with the clearing cookie when called with the cookie.
    - Every response that sets the cookie carries `Cache-Control: no-store`.

7. **"View as person" on the console session** (07 9.2; R-DM-49, R-AZ-32; Ruling for docs/DECISIONS.md). `viewAsUser` called from a console session additionally stores `view_as_user_id` and `view_as_started_at = now` on the session in the same transaction as its audit entry; from other sessions it keeps the M1 snapshot behaviour. While stored and `now < view_as_started_at + authz.view_as_max_duration`, the authenticator sets `actor.ViewAs{UserID, StartedAt}` on every request of that session, so `authorize` applies the intersection, `view_as_read_only` and SR6 exactly as M1-49 built them. At or after the end, the next request of the session clears the columns and writes the end audit entry (`access.view_as.end`, reason `expired`) in its own short transaction and answers `401 step_up_required`, as 07 9.2 and M1-49 require ("after expiry every decision ends with `step_up_required`"), so no request that the person sent from a page showing the other person's view runs with the owner's rights; `endViewAs` writes the entry with reason `ended`. While a view is stored, the actions `access.view_as.end`, `auth.logout`, `auth.step_up` and `self.read` on the person's own session are decided without `view_as` (R-AZ-32): without this exemption 07 4.2 steps 4 and 7 would deny them, because they write and the target person has no right on the owner's session, and the owner could neither leave the view nor log out. Implement the exemption in `authz` as a rule over the action and the resource (own `auth_session`), not in the handlers, with a unit test per action. A new `viewAsUser` while a view is stored replaces it and writes an end entry for the old one first. Starting a view needs a fresh step-up (catalogue row); `endViewAs` needs none. Audit details carry only IDs (`target_user_id`, `started_at`, `ended_reason`), never names.

8. **Pre-step-up gate** (08 7.5; requirement 15). A console session with `step_up_at IS NULL` passes only the operations whose action is `self.read`, `auth.step_up`, `auth.logout` or a public discovery; every other operation is `401 step_up_required` (reason `console_step_up_required`) before `authorize` runs. After the first step-up the normal rules apply, including the `auth.step_up_max_age` of each action that needs step-up. Implement the gate as a rule in the domain package and call it from the authenticator result, not as a list in a handler.

9. **Revocation** (08 10, 11; M1-45). `logout` with the cookie revokes the console session with `logout`; `revokeSession` of another session by the person or an owner, `revokeDevice` of the web device and `disableUser` end it with their reasons, publishing `SessionEnded` as for every session. A console session has no gateway connection in M6 (the console holds none, ADR 0028), so closing with `4012` affects nothing; the next HTTP request fails with `401` and clears the cookie.

10. **Rulings for `docs/DECISIONS.md`** (section 10, one row each with date, decision, reason and place): `D-M6-01-1` console form of the login operations and the web device per login (note 1); `D-M6-01-2` cookie name, attributes, rotation and clearing (note 2); `D-M6-01-3` idle rule and the write bound of `last_seen_at` (note 3); `D-M6-01-4` cookie as a second credential kind, one credential per request (note 4); `D-M6-01-5` CSRF header `X-Console-Request: 1` with `Origin`, and the upgrade check (note 5); `D-M6-01-6` the step-up operations for password with TOTP and OIDC re-login (note 6); `D-M6-01-7` the stored view with expiry and end audit (note 7); `D-M6-01-8` the pre-step-up gate (note 8).

## Tests (write them first)

Test world: 07 10 and 11.1 (the Owner Alex with a local account, a passkey from `virtualauthn` and a linked OIDC identity at the fake IdP; Sam with group Leser; Robin without grants); the server runs through `internal/wire` in role `api` with PostgreSQL via `pgtest`; HTTP calls go through the full chain on `127.0.0.1:0` with a cookie jar of the Go test client; the console origin is the origin of the test `server.public_url`.

Unit, `internal/identity/domain/consolesession`:

- `TestCookieValueIs256BitsAndHashed`: 32 random bytes, base64url without padding; two values differ; the stored hash is the HMAC with the pepper port, never the value.
- `TestExpiry`: absolute expiry, idle expiry at exactly the timeout and one tick before; the `last_seen_at` write bound.
- `TestPreStepUpGate`: `self.read`, `auth.step_up`, `auth.logout` pass, `grant.create` and `audit.read` do not; after `step_up_at` is set everything passes the gate.
- `TestViewAsExpiry`: active before `authz.view_as_max_duration`, ended at it.

Unit, `internal/platform/httpserver` (`httpservertest`):

- `TestConsoleGuard`: cookie request `POST` without the header, with a wrong value, with a foreign `Origin`, without `Origin`: `403` with `reason: csrf_check_failed` and no handler call; with both: handler called; `GET` without the header: passes; a request with `Authorization` is not checked.

Integration (`pgtest`, full chain):

- `TestConsoleLogin_Password_SetsCookieOnly` (ID-T19 part 1): the console form of `loginWithPassword` with TOTP answers `200 ConsoleSession`, `Set-Cookie` with exactly the attributes of Design note 2, no token in body or headers (assert by scanning the raw response for the token prefixes and for any JWS); a row in `auth_sessions` with `client_component = 'console'`, `cookie_hash` set, a device `web` `active`, no `refresh_token_families` row.
- `TestConsoleLogin_OIDC_And_Passkey`: the same through the fake IdP (redirect to `/console/login/callback`; another redirect is `400`) and through `virtualauthn`; the trusted header form only on a loopback listener (as M1-43 tests).
- `TestConsoleBeforeStepUp_OnlyOwnProfile` (ID-T19 last case): after login `getMe` and `getConsoleSession` are `200`; `listUsers`, `listAuditEvents` and `viewAsUser` are `401 step_up_required`.
- `TestConsoleStepUp_RotatesCookie`: step-up with the passkey (`userVerification = required`), then the old cookie gives `401` and clears itself, the new cookie works; the same with `stepUpWithPassword` and with the OIDC re-login (`auth_time` before the challenge is rejected, after it is accepted); `step_up_method` is `passkey`, `password_totp`, `oidc_reauth` respectively.
- `TestConsoleCSRF` (ID-T19): a state-changing operation (for example `endViewAs` or `logout`) with the cookie and without `X-Console-Request`, with a foreign `Origin`: `403`; with both: success.
- `TestConsoleWebSocketForeignOrigin` (ID-T19): an upgrade of the gateway path with the cookie and `Origin: https://evil.example.com` or without `Origin`: `403` before the upgrade.
- `TestConsoleIdleAndAbsoluteExpiry`: moving the fake clock past `auth.console_session_idle_timeout` gives `401` with the clearing cookie and `revoke_reason = 'expired'`; the same at `auth.console_session_max_age`; the cleanup job revokes an idle session without a request, with one audit entry.
- `TestConsoleRevocation`: `logout`, `revokeSession` by the Owner from an app session, `revokeDevice` of the web device and `disableUser` each make the next console request `401`.
- `TestConsoleViewAs` (07 9.2): Alex after step-up starts `viewAsUser` for Sam; the next requests see Support only, `createMessage` gives `403 forbidden` with reason `view_as_read_only`, a DM of Sam stays `404`; `endViewAs` succeeds during the view and ends it with an audit entry `ended`; `logout` and a step-up also succeed during a view; a second run lets the clock pass `authz.view_as_max_duration` and the next request gets `401 step_up_required` and ends the view with an audit entry `expired`; `getConsoleSession` shows the view while active.
- `TestConsoleCookieNotAcceptedWithAuthorization`: both credentials in one request: `400`.
- `TestConsoleFormRejectedForPairingAndEnrollment`: `400 validation_failed`.
- Canary (SEC-T10 scope of this ticket): a canary password, TOTP code, cookie value and `client_verifier` appear in no log line, audit detail or error body of the tests above (use the log capture of `httpservertest`).

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/identity/... ./internal/authz/... ./internal/platform/httpserver/... ./internal/gateway/... ./internal/kernel/... ./internal/wire/...`
- `make codegen-check`, `make api-lint`, `make api-checks`, `make migrations-lint`, `make arch`, `make register-check`, `make catalog-check`
- `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] ID-T19 is green: cookie attributes set, missing CSRF header and foreign `Origin` rejected, WebSocket with a foreign `Origin` rejected, before step-up only the own profile.
- [ ] The cookie value is stored only as an HMAC and is new after login and after every step-up; no token reaches the browser.
- [ ] Idle and absolute expiry, logout and every revocation end the console session.
- [ ] Console step-up works with passkey, password with TOTP and a new OIDC login, each recorded in `step_up_method`; the operations are in `openapi.yaml` first.
- [ ] "View as person" is a stored state of the console session with start, end and expiry, audited at start and end, read-only, without DMs.
- [ ] 08 6.2 and 7.5, 06 3, 07 9.2, 19 SEC-CORS-4, `openapi.yaml`, the READMEs of `identity` and `authz`, the CHANGELOG and the rows `D-M6-01-1` to `D-M6-01-8` are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- `internal/identity/README.md` (console session, cookie, CSRF header, step-up methods, idle rule) and `internal/authz/README.md` (stored view) updated in English; existing text stays as it is.
- Rulings of Design note 10 recorded in `docs/DECISIONS.md`; the spec sections named in Scope updated in the same change.
- `services/server/CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven (for example browser behaviour of `SameSite=Strict`, which M6-03 checks in a headless browser).
5. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
