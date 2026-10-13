# M4-09: Login: DPoP, refresh and step-up

Status: ready | Depends on: M4-02, M4-08 | Milestone: M4

## Goal

After this ticket `packages/auth` is the pure Dart login core of the apps: DPoP proofs (RFC 9449) signed through a signer port with the device key, adoption of the server nonce from `DPoP-Nonce` and from the gateway `hello`, one retry on `use_dpop_nonce`, the access token in memory and the refresh token in a secret store port, refresh as a single flight per refresh family (a second caller waits for the running refresh; the new token is stored before it is used), the step-up flow of 08 6.2 and 14.3 with the step-up key (challenge, compact JWS over `challenge`, `sid` and `iat`, confirmation, repeat of the original request with the same `Idempotency-Key`), and the clients of the login operations the onboarding screens call (OIDC with PKCE and an app redirect, password with TOTP, passkey where offered, enrollment code, device pairing, polling a pending device, logout). Dio interceptors plug all of this into the client factory of M4-02. Tests use a fake signer and check the proof structure against RFC 9449 and that a parallel refresh runs only once. Why: every request of the apps and the gateway `identify` need a valid proof from the device key, and two refreshes of one family would look like theft and revoke the session (08 7.4, 15 8); this logic must exist once, without Flutter, so the app, the console parts and later the share extension's sibling logic agree.

## Read first

- `docs/PLAN.md`: section 3; section 10 rows M4-09, M4-10, M4-13, M4-18, M4-25, M4-35, M4-36.
- `docs/spec/08-identity.md`: 2.2 (OIDC with the server as relying party, `client_challenge`, `app_redirect_uri`, `login_code`), 2.4 (proxy header login), 2.5 (local accounts, password with TOTP, passkeys only with a public domain), 3.2 (enrollment code input rules), 5.1 "Registration" (fields `device_id`, `dpop_jwk`, `step_up_jwk`, `attestation`), 5.2 (pending device: tokens, polling `GET /devices/{id}`), 5.3 (pairing), 6 "Device keys and step-up" (6.1, 6.2, 6.3 fresh step-up), 7 "Token model" (7.2, 7.3 DPoP with nonce rules, 7.4 refresh families and reuse), 8 "Session duration", 13 (login protection, responses), 14.1 to 14.3 (flows), 15 (ID-T14, ID-T15, ID-T17, ID-T18 as the server's view of what this client must do right).
- `docs/spec/06-api.md`: 3 "Authentication per principal kind" (3.1 DPoP, 3.2 login flows, 3.3 step-up), 5 (codes `use_dpop_nonce`, `step_up_required` with `reason`, `session_revoked`, `device_pending`), 7 (idempotency on repeat).
- `docs/spec/15-client-architecture.md`: 3.1 (`auth` depends on `api_client` and `domain`; no Flutter), 8 "Network and login" (all bullets), 9 (ES256 for all clients, two keys), 21 (R-CL-04, R-CL-05, R-CL-07).
- `docs/spec/04-sync-protocol.md`: 6.3 items 2, 3, 6 and 7 (`hello` with `dpop_nonce`; proof in `identify`, `resume`, `reauth` with `htm = GET` and the HTTPS form of the gateway URL).
- `docs/api/openapi.yaml`: the operations `beginOidcLogin`, `completeOidcLogin`, `loginWithTrustedHeader`, `beginPasskeyLogin`, `completePasskeyLogin`, `loginWithPassword`, `beginEnrollment`, `completeEnrollment`, `redeemEnrollmentCode`, `beginPairing`, `completePairing`, `refreshToken`, `logout`, `createStepUpChallenge`, `completeStepUpWithDeviceKey`, `getDevice`, `getDiscovery`, and the scheme `dpopAuth`.
- RFC 9449 (DPoP) sections 4.2, 4.3, 7, 8, 11.1, 11.3; RFC 7638 (JWK thumbprint); RFC 7515 (compact JWS) and RFC 7518 3.4 (ES256 signature as `R || S`, 64 bytes); RFC 7636 (PKCE S256).
- ADRs: `0067-jose-profile-with-the-standard-library.md` (the server's JOSE profile this client must produce), `0056` (extensions never refresh the app's family).
- Briefs: `docs/briefs/M4-02-dart-api-client.md` (factory and interceptor slot, `ApiProblem`), `docs/briefs/M4-08-domain.md` (`Uuid7Generator`, `DeviceId`, `AppConfig`); for the server side the M1 briefs `M1-33-identity-devices-dpop.md`, `M1-34-identity-sessions.md`, `M1-35-identity-step-up.md`, `M1-41-identity-oidc.md`, `M1-38-identity-device-approval.md`.
- `docs/DECISIONS.md`: D-W-100, D-W-114, D-W-115; section 10.

This ticket touches no API operation, no table, no action, no event and no configuration key.

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
7. No en dash and no em dash in any file you write; the `text-rules` pre-commit hook rejects them, and the ARB test of M4-06 rejects them in interface texts. Use a hyphen or rephrase.
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory. Interface texts come from `packages/l10n`, never as literals in widgets.
9. Lifetimes and windows only through named keys: the client does not decide `auth.access_token_ttl` or `auth.dpop_nonce_ttl`; it reacts to the server's `exp`, `DPoP-Nonce`, `use_dpop_nonce` and `reauth_required`. Backoff for retries of the login polling uses `client.outbox_retry_base` and `client.outbox_retry_cap` through `AppConfig` (M4-08). No new key.
10. Specification first: if a flow needs a field the API lacks, stop (Boundaries).

Product requirements implemented here:

11. DPoP proof per request: header `typ = dpop+jwt`, `alg = ES256`, `jwk` (public part only); payload `jti` (unique), `htm`, `htu` (scheme and host in lowercase, default port removed, without query and fragment), `iat`, `ath` (base64url SHA-256 of the access token) when a token is sent, `nonce` when the server has issued one; the latest nonce wins (08 7.3, 06 3.1).
12. Gateway proofs with `htm = "GET"`, `htu` = HTTPS form of the gateway URL, `nonce` from `hello` (04 6.3, R-SY-01).
13. Exactly one refresh per family at a time; the new refresh token is stored before the new access token is used; a refresh needs a proof with the family's DPoP key (08 7.4, 15 8, R-CL-07).
14. Two device keys: `dpop` without user confirmation, `step_up` with it; ES256 (15 9, R-CL-04, R-CL-05).
15. Step-up: challenge, OS confirmation, compact JWS with only `alg` in the header over `{challenge, sid, iat}`, confirmation endpoint, then the original request repeated with the same `Idempotency-Key` (08 6.2, 14.3); the terminal's fresh step-up uses the same flow and is consumed by the server (08 6.3).
16. OIDC in the system browser with PKCE on the app leg (`client_challenge`, `client_verifier`), redirect to the app through a port the app implements (loopback `127.0.0.1` on desktop, 15 8); never a web view.
17. A revoked session or device (`401 session_revoked`, gateway `4012`) ends in a signed-out state that tells the app to delete local data (R-SY-14).
18. No token, refresh token, proof, code, password or TOTP in any log, exception message or `toString` (15 16, 19 SEC-LOG rules).

## Scope

Create in `packages/auth/lib/src/`:

- `ports.dart`: `DeviceKeySigner` (`String get alg`, `Map<String, String> get publicJwk`, `String get thumbprint`, `Future<Uint8List> sign(Uint8List signingInput)` returning the JOSE `R || S` form), `DeviceKeys` (`dpop`, `stepUp`), `SecretStore` (`read`, `write`, `delete` by key), `BrowserLauncher`, `RedirectReceiver` (`Future<Uri> waitForRedirect(Uri expectedPrefix, Duration timeout)`), `Clock` (Design note 1).
- `jose.dart` (base64url, compact JWS assembly, JWK thumbprint per RFC 7638), `dpop.dart` (`DpopProofFactory`), `nonce_store.dart`, `tokens.dart` (`TokenState`, `SessionInfo` with `sid`, `deviceId`, `accessExpiresAt`), `refresh.dart` (`RefreshCoordinator`), `step_up.dart` (`StepUpService`, `StepUpPrompt` port), `interceptors.dart` (`DpopInterceptor`, `RefreshInterceptor`, `StepUpInterceptor`), `login_flows.dart` (`LoginService`, Design note 5), `auth_events.dart` (`AuthEvent` stream: `signedIn`, `devicePending`, `signedOut(reason)`, `refreshFailed`), `pkce.dart`.
- `lib/auth.dart` exports; tests under `test/` (Tests) with a software P-256 signer in `test/support/` (dev dependency `pointycastle`, Design note 2).

Change:

- `packages/auth/pubspec.yaml` (dependencies: `api_client`, `domain`, `crypto` from dart.dev; dev: `test`, `pointycastle`), `README.md`.
- `docs/spec/15-client-architecture.md` 8 (one sentence each: the interceptor order, the signed-out event).
- `docs/DECISIONS.md` section 10: rows `D-M4-09-<n>`; root `CHANGELOG.md` under `[Unreleased]`.

## Out of scope

- Key generation and secure storage on the platforms: M4-10 (Apple), M4-11 (Windows), M5-01 (Android); the secret store implementation with `flutter_secure_storage`: M4-12 (database key) and M4-18 (tokens).
- The onboarding screens E1 to E7 and the loopback listener: M4-18 implements `RedirectReceiver` and `BrowserLauncher`.
- Passkey ceremony on native platforms (only for instances with a public domain, 15 8): this ticket implements the API calls with a `PasskeyAuthenticator` port and no platform code; M4-18 decides whether the platform offers it.
- Extension child sessions (`createExtensionSession`): M5-04 and M5-06. Console cookie sessions: M6-01.
- Gateway connection: M4-13 calls `DpopProofFactory.forGateway`.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M4-09-auth-dpop`; one writer per worktree.
- Conventional Commits with the package or component as scope (for example `feat(client): ...`, `feat(sync-engine): ...`, `build(gates): ...`, `docs(spec): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`. Never publish a Dart package (`publish_to: none` in every `pubspec.yaml`).
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` with port 0 and make no network call; fakes replace the server unless a section below names a container. Temporary directories only via `mktemp -d` (Dart `Directory.systemTemp.createTemp`, pytest `tmp_path`), removed at the end. Stop only processes you started, by PID.
- Never read or print secrets. No network calls except package downloads for pinned tools and pub packages and what the gates of `make check` do themselves.
- The Flutter SDK at the version pinned in the root `pubspec.yaml` must be on `PATH` (and Go and Docker where a section names them); without them the gates fail by design (R-QA-02), and the ticket stops with `Verdict: BLOCKED <tool> missing` instead of skipping.
- Never lower a threshold, disable a gate, delete a test, raise `repo.max_file_kb` or extend `quality/licenses/allowed.txt`; never exclude hand-written code from a lint, format, coverage or doc gate.
- The product name appears only at the places of 02 "Code layout" ("Places that carry the product name"); the app shows `branding.product_name` from the instance, never a literal.
- No permission logic in the client (15 1 principle 1): controls follow server responses, snapshots and events, never an own derivation from grants.
- If the core contract (chapters 00, 03, 04, 05) or `docs/api/` lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M4 by D-W-114). Where the merged code of an earlier ticket names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Ports and no I/O.** `auth` performs HTTP only through the `api_client` it is given and does nothing else with the outside world: browser, redirect, keys, secrets and time come through ports, so every flow is testable with fakes (15 1 principle 5). The package has no global state; one `AuthSession` object per server address owns the token state, the nonce store and the coordinator.

2. **Proofs.** `DpopProofFactory(DeviceKeySigner key, NonceStore nonces, Clock clock, Uuid7Generator ids)`; `Future<String> forRequest({required String method, required Uri url, String? accessToken})` and `Future<String> forGateway({required Uri gatewayUrl, required String accessToken, required String nonce})`. `htu` normalization: lowercase scheme and host, drop the default port (443 for `https`, 80 for `http`), drop query and fragment, keep the path as sent; `wss` becomes `https` for the gateway form. `jti`: a fresh UUIDv7 string. `iat`: whole seconds from the clock. Header members in the order `typ`, `alg`, `jwk`; JSON without whitespace. ES256 signatures are `R || S` with 32 bytes each (RFC 7518 3.4); the signer port returns that form, and a test with a software signer verifies the proof with the public JWK. Ruling for docs/DECISIONS.md: the client builds JOSE by hand on `crypto` and base64url, matching the server's profile of ADR 0067, because a JOSE library would add algorithms and header forms the server rejects.

3. **Nonce.** `NonceStore` keeps the newest nonce per server; set from every response header `DPoP-Nonce` (also on success) and from `hello.dpop_nonce`. `DpopInterceptor`: adds `Authorization: DPoP <token>` and `DPoP: <proof>` to every request that needs a person's token (all except the unauthenticated login begin calls, which still carry a proof where the API requires one: `beginPairing`, `completePairing`, `completeOidcLogin`); on `401` with problem `use_dpop_nonce` it takes the new nonce and repeats the request once with a fresh proof; a second `use_dpop_nonce` is returned to the caller.

4. **Refresh.** `RefreshCoordinator.ensureFresh()` and `RefreshCoordinator.refreshNow()`: while a refresh runs, every caller awaits the same future (single flight). Order: call `refreshToken` with a proof of the DPoP key; on success write the new refresh token to the `SecretStore` and wait for the write to complete, then publish the new access token; on `401 session_revoked` or a reuse rejection emit `signedOut(reason)` and clear the store; on a network error keep the old state and report `refreshFailed` (the caller retries later with backoff). `RefreshInterceptor` refreshes when the access token's `exp` has passed by the injected clock, and on `401 token_expired` or `401 unauthenticated` refreshes once and repeats the request once; the gateway's `reauth_required` (04 6.3 item 6) calls `refreshNow` through M4-13. Ruling for docs/DECISIONS.md: the persisted write happens before the new access token is published, so a crash between rotation and storage falls into the server's `auth.refresh_reuse_grace` (08 7.4) instead of a lost token.

5. **Login flows.** `LoginService` with one method per flow, each returning `LoginOutcome` (`signedIn(TokenState)`, `pending(PendingDevice with deviceId and the verification code)`, `needsSecondFactor`, `failed(ApiProblem)`):
    - `registerDeviceFields()`: the `device_id` (UUIDv7, kept in the `SecretStore` for this server), `dpop_jwk` and `step_up_jwk` from the ports, `platform`, `name`, `app_version` from the caller (08 5.1).
    - OIDC: `beginOidc(providerId)` creates `client_verifier` (32 random bytes, base64url) and `client_challenge` (S256), calls `beginOidcLogin` with the redirect URI of the `RedirectReceiver`, opens `authorization_url` through `BrowserLauncher`, waits for the redirect with `login_code`, calls `completeOidcLogin` with `client_verifier`, the proof and the device fields (08 14.1).
    - Password with TOTP (`loginWithPassword`, second step when the server asks for the code), proxy header (`loginWithTrustedHeader`), passkey (`beginPasskeyLogin`, `completePasskeyLogin` through the `PasskeyAuthenticator` port), enrollment code (`redeemEnrollmentCode` or `beginEnrollment`/`completeEnrollment` as the API defines), pairing (`beginPairing` returns the code and expiry; `pollPairing` calls `completePairing` with backoff until approved, expired or rejected).
    - `pollPendingDevice(deviceId)`: `getDevice` with backoff until `active` (emit `signedIn`) or `revoked` (emit `signedOut(reason)`) (08 5.2).
    - `logout()`: call `logout`, then clear the secret store and emit `signedOut(loggedOut)` even when the call fails offline.
    Codes are normalized as 08 3.2 says (case, spaces and hyphens do not count) before sending: remove spaces and hyphens, write a leading `en_` in lower case and the rest in upper case, drop nothing else; the UI shows them grouped (M4-18).

6. **Step-up.** `StepUpService.confirm({required String purposeText})`: `createStepUpChallenge`; `StepUpPrompt.ask(purposeText)` (the UI shows the text in its language); on consent the step-up key signs the compact JWS (header `{"alg":"ES256"}` only; payload `{"challenge":...,"sid":...,"iat":...}`); `completeStepUpWithDeviceKey`. The OS confirmation happens inside the step-up signer of M4-10 (the key requires user presence); `StepUpPrompt` only explains. `StepUpInterceptor`: on `401 step_up_required` (any `reason`, including `terminal_step_up_not_fresh`), run `confirm` once and repeat the original request with the same headers, including the same `Idempotency-Key`; a cancelled prompt returns a `StepUpCancelled` error to the caller; a second `step_up_required` after a successful confirmation is returned, never looped.

7. **Secrets in errors.** `TokenState.toString()`, every exception and every log hook redact tokens, proofs, codes, verifiers, passwords and TOTP values (they print the kind and the length only). A test serializes each public type's `toString` and every thrown error of the flows with canary values and asserts that no canary appears.

8. **Rulings for docs/DECISIONS.md** (`D-M4-09-<n>`): hand-built JOSE (note 2); nonce retry once (note 3); persist before publish (note 4); the `LoginService` outcome model and polling backoff keys (note 5); step-up repeat with the same `Idempotency-Key` and no loop (note 6).

## Tests (write them first)

`packages/auth/test/` (fake `HttpClientAdapter` for the API, fake signer from `test/support/software_p256_signer.dart`, fake secret store, fake clock):

- `dpop_proof_test.dart` (RFC 9449 structure): header has exactly `typ`, `alg`, `jwk`; `jwk` has `kty`, `crv`, `x`, `y` and no `d`; payload has `jti`, `htm`, `htu`, `iat`, and `ath` only with a token; `ath` equals base64url(SHA-256(token)) for a known token; `htu` of `HTTPS://Chat.Example.com:443/api/v1/threads?x=1#y` is `https://chat.example.com/api/v1/threads`; the signature is 64 bytes and verifies with the JWK; two proofs have different `jti`; gateway form uses `GET` and `https` for a `wss` URL and the given nonce.
- `nonce_test.dart`: the newest `DPoP-Nonce` is used in the next proof; `401 use_dpop_nonce` repeats once with the new nonce; a second one is returned.
- `refresh_single_flight_test.dart`: ten parallel requests that all see an expired token cause exactly one `refreshToken` call (the fake counts), and all ten complete with the new token; the new refresh token is written before the first repeated request is sent (the fake store records the order); a refresh answered with `401 session_revoked` emits `signedOut` and clears the store; a network error keeps the old token.
- `step_up_test.dart`: `401 step_up_required` triggers challenge, signature and confirmation, then the original `POST` is repeated with the same `Idempotency-Key`; the JWS header is exactly `{"alg":"ES256"}` and the payload carries `challenge`, `sid`, `iat`; a cancelled prompt returns `StepUpCancelled`; a second `step_up_required` is returned without a loop.
- `login_flows_test.dart`: OIDC with PKCE (`client_challenge` is S256 of the verifier; `completeOidcLogin` carries the verifier and the device fields); a pending device polls until `active` and emits `signedIn`, or until `revoked` and emits `signedOut`; pairing polls until approved; password with a second TOTP step; code normalization (`en_ 7k2m 9qxa`, `EN_7K2M-9QXA` and `en_7K2M9QXA` all send `en_7K2M9QXA`: spaces and hyphens removed, the `en_` prefix in lower case, the groups in upper case; nothing else is dropped); logout offline still clears and emits.
- `no_secrets_test.dart` (Design note 7).

Commands that must pass, from the repository root:

- `cd packages/auth && flutter test --coverage`
- `make arch`, `make lint`, `make fmt-check`, `make coverage`, `make licenses`, `make osv`
- `make check`

## Acceptance criteria

- [ ] Proofs match the structure of RFC 9449 and verify with the public key; `htu`, `ath`, `nonce` and the gateway form are correct.
- [ ] A parallel refresh is executed only once; the new refresh token is stored before use; revocation emits the signed-out event.
- [ ] Step-up signs the compact JWS with only `alg` in the header and repeats the original request with the same `Idempotency-Key`, without loops.
- [ ] The login flows (OIDC with PKCE, password with TOTP, proxy header, passkey through a port, enrollment, pairing, pending device, logout) work against the fake API.
- [ ] No secret appears in any `toString`, exception or log hook.
- [ ] `auth` imports no Flutter (`make arch`); rulings recorded.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why and cite the RFC section where a rule comes from.
- `packages/auth/README.md` (English): ports to implement per platform, interceptor order, flows, events.
- 15 8 updated; rulings of Design note 8 in `docs/DECISIONS.md` section 10.
- Root `CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. Which login flows were tested against which fake responses; any API field the flows needed that the spec names differently from this brief.
5. What is not proven (for example a platform test that ran only in CI, or a check done by hand).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100, D-W-114), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
