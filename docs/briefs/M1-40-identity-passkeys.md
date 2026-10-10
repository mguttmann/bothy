# M1-40: Passkeys

Status: ready | Depends on: M1-37 | Etappe: M1

## Goal

After this ticket the server supports WebAuthn passkeys for login of local accounts, for the first
enrollment with an enrollment code, for adding further passkeys and for step-up, for the web
console and for instances with a public domain. The state of every ceremony lives in the table
`auth_challenges` (03, R-DM-31; created by M1-35), so it works on several nodes; user verification
is always required, attestation is not requested, relying party ID and origins come from
configuration, and a falling signature counter is reported in the audit log. A virtual authenticator makes every path testable without a browser.

## Read first

- `docs/spec/08-identitaet.md`: "Lokale Konten" (WebAuthn paragraph), "Einladungs-Code",
  "Bestätigung", "Step-up-Ablauf", "DPoP", "Schutz der Anmeldung", "Web-Konsole: Sitzung nach dem
  Backend-for-Frontend-Muster" (context only), "Konfigurationswerte dieses Kapitels".
- `docs/spec/03-datenmodell.md`: tables `passkey_credentials`, `auth_challenges` (R-DM-31),
  `enrollment_codes`,
  `user_identities`, `identity_providers`, `auth_sessions` (`step_up_at`, `step_up_method`),
  `audit_events`; Grundsatz 1 (UUIDv7).
- `docs/spec/05-aktionen.md`: actions `auth.login.begin`, `auth.login.complete`, `auth.step_up`,
  `passkey.register`, `passkey.revoke`, `identity.read`; "Spalten Owner, Step-up, Audit, Rechte".
- `docs/spec/07-rechte.md`: "Ablauf" step 8, SR5.
- `docs/spec/06-api.md`: "Anmeldeabläufe" point 3, "Step-up", "Endpunkte" (Auth, Me, Users).
- `docs/spec/02-architektur.md`: "Schichten im Modul" (foreign libraries only in adapters),
  "Modulregister als Quelle", "Konfigurationsladen".
- `docs/spec/16-betrieb.md`: "Umzug und Cloud-Portabilität" (RP ID keeps passkeys on the same
  domain).
- `docs/PLAN.md`: P-03, P-11, row M1-40 (updated by M1-37), section 5 "Teststrategie" (virtual
  WebAuthn authenticator).
- ADR 0019, ADR 0020.
- `docs/api/openapi.yaml`: `beginPasskeyLogin`, `completePasskeyLogin`, `beginEnrollment`,
  `completeEnrollment`, `beginPasskeyRegistration`, `completePasskeyRegistration`, `revokePasskey`,
  `createStepUpChallenge`, `completeStepUpWithPasskey`, `getMyCredentials`, `getUserCredentials`;
  schemas `WebAuthnChallenge`, `PasskeyLoginBeginRequest`, `PasskeyLoginCompleteRequest`,
  `EnrollBeginRequest`, `EnrollCompleteRequest`, `PasskeyRegisterComplete`, `Passkey`,
  `StepUpChallenge`, `StepUpPasskeyRequest`, `StepUpResult`.
- W3C Web Authentication Level 3: https://www.w3.org/TR/webauthn-3/; go-webauthn:
  https://github.com/go-webauthn/webauthn (BSD-3-Clause; pin v0.18.2, released 2026-09-19 per the
  planning research; if the Go proxy does not serve that tag, pin the newest v0.18.x and report it).
- Code: `internal/identity` (M1-35 step-up challenges, M1-36 credential overview, M1-37
  `CompleteLogin` and `EnrollmentCodes`), `internal/authz` (`PrincipalSource`, step 8).

## Fixed requirements that apply

1. No MVP. Built from scratch directly in its final state; etappen are only the build order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined
   interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface has a docstring or
   doc comment (purpose, parameters, return value, errors, example where useful); comments explain
   why, not what. API documentation is generated from code (OpenAPI from the routes, dartdoc,
   Python API docs in the docs site).
4. Documentation in the repo: README per module/service, an ADR for every architecture decision, a
   CHANGELOG entry per release, docs site kept current. A change without matching documentation is
   not done.
5. Quality gates (SonarQube, linters, type checks, security scanners, test coverage) block the
   merge.
6. Test first: every behaviour change starts with a failing test (red), then code (green), then
   cleanup. No en dash or em dash in any text. White label: examples use Alex, Sam, Robin and
   `example.com`. Configuration values only through named keys listed in
   `docs/operations/configuration.md`.
7. WebAuthn with go-webauthn (pinned): RP ID is the domain of the instance, allowed origins as a
   list, `userVerification = required`, `residentKey = preferred`, `attestation = none`
   (08 "Lokale Konten").
8. A decrease of `sign_count` is reported in the audit log (03 `passkey_credentials`).
9. `passkey.register` has Step-up `bedingt` (05 "Spalten Owner, Step-up, Audit, Rechte"): step-up
   once an active passkey exists; the first passkey needs only a human session.
10. Passkeys serve login of local accounts and step-up (03 `passkey_credentials`, 08 "Step-up-
    Ablauf"); the enrollment code confirms the first device (08 "Einladungs-Code").
11. Code stays node neutral (PLAN section 20, "Mehrknoten-Betrieb"): no ceremony state in process
    memory; it lives in `auth_challenges` (08 "2.5").

## Scope

The specification is in place (planning, spec first): 08 "2.5" (configuration keys, ceremony
state in `auth_challenges`, single use, algorithms, passkey login only with a local identity, decoy,
sign counter), 08 "18" (`auth.webauthn_rp_id`, `auth.webauthn_origins`), 07 "4.2" step 8 with
`hat_anmeldedaten` for the conditional step-up, and the descriptions of the passkey operations and
of `WebAuthnChallenge` in `openapi.yaml`. Change them only where the implementation proves them
wrong (spec first, `make api-lint api-checks` green), and regenerate `docs/operations/configuration.md`
if a key changes. `docs/DECISIONS.md`: rulings listed in Design notes.

Production code under `services/server/`:

- `migrations/<next sequence>_identity_passkey_credentials.sql`: table `passkey_credentials`
  exactly as in 03.
- `internal/identity/adapters/postgres/queries/passkey_credentials.sql`; regenerated sqlc code.
- `internal/identity/domain/ceremony.go`: ceremony state (`webauthn_session.v1`: RP ID, required
  user verification, allowed credential IDs, user handle), expiry and single-use decision, sign
  counter rule.
- Repository of `auth_challenges` for the purposes `passkey_login`, `passkey_enroll`,
  `passkey_register` (the table and the `step_up` part come from M1-35).
- `internal/identity/app/passkeys.go`: use cases `BeginPasskeyLogin`, `CompletePasskeyLogin`,
  `BeginEnrollment`, `CompleteEnrollment`, `BeginPasskeyRegistration`,
  `CompletePasskeyRegistration`, `RevokePasskey`, `CompleteStepUpWithPasskey`; extend
  `CreateStepUpChallenge` (M1-35) with `passkey_options`. Outbound port `PasskeyVerifier`.
- `internal/identity/adapters/webauthn`: `PasskeyVerifier` with go-webauthn.
- `internal/identity/adapters/webauthn/virtualauthn`: software authenticator for tests (also used
  later by M1-71 and the console tests).
- `internal/identity/adapters/httpapi`: handlers for the eight operations.
- `internal/authz`: conditional step-up for `passkey.register` (Design notes); identity adapter of
  `authz.PrincipalSource` gets `HasActivePasskey`.
- Configuration schema (M1-07): `auth.webauthn_rp_id`, `auth.webauthn_origins` with start
  validation.
- `architecture/modules.yaml`: vendor `webauthn: [github.com/go-webauthn/webauthn,
  github.com/go-webauthn/webauthn/**]` allowed for `identity` adapters; regenerate
  `.go-arch-lint.yml`.
- Credential overview (M1-36): `passkeys` filled with active passkeys.
- `services/server/api/operations.yaml`: enable `beginPasskeyLogin`, `completePasskeyLogin`,
  `beginEnrollment`, `completeEnrollment`, `beginPasskeyRegistration`,
  `completePasskeyRegistration`, `revokePasskey`, `completeStepUpWithPasskey`.
- `internal/identity/README.md`, `services/server/CHANGELOG.md`.

Outbound port (in `internal/identity/app`, names fixed; JSON in and out so the app layer stays
free of the library):

```go
type CeremonyUser struct { ID ids.ID; Name, DisplayName string; Credentials []StoredPasskey }
type PasskeyVerifier interface {
    CreationOptions(user CeremonyUser, challenge []byte) (optionsJSON []byte, err error)
    VerifyCreation(user CeremonyUser, challenge []byte, responseJSON []byte) (NewPasskey, error)
    RequestOptions(allow []StoredPasskey, challenge []byte) (optionsJSON []byte, err error)
    // VerifyAssertion resolves the credential through lookup (discoverable or allow-listed).
    VerifyAssertion(challenge []byte, responseJSON []byte, lookup func(credentialID, userHandle []byte) (CeremonyUser, error)) (AssertionResult, error)
}
```

## Out of scope

- Console cookie session and the console step-up right after login (M6-01, P-03).
- Associated Domains files and Digital Asset Links for native apps (M5).
- Device approval of the pending devices that passkey login creates (M1-38).
- Redemption of enrollment codes without passkey (M1-37), password and TOTP (M1-39).
- Attestation verification against a metadata service, credential backup policies.

## Boundaries

- Work in a dedicated git worktree on branch `feat/M1-40-identity-passkeys`.
- Conventional Commits with module scope and DCO sign-off (`git commit -s`).
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside Scope, except `docs/DECISIONS.md` and CHANGELOG entries.
- Follow the host rules supplied by the launcher. Tests bind only `127.0.0.1` and port 0;
  PostgreSQL only through `pgtest`.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at
  the end.
- Never read or print secrets. No network calls except package downloads for pinned tools.
- If an element of the core contract (00, 03, 04, 05) named here is missing or different, stop with
  `Verdict: BLOCKED` (P-11).

## Design notes

Relying party configuration: `auth.webauthn_rp_id` (empty means the host of
`server.public_url`), `auth.webauthn_origins` (empty means `[origin of server.public_url]`;
entries may also be non-HTTP origins such as `android:apk-key-hash:...`). Start validation: the RP
ID equals the host of every HTTP(S) origin or is a registrable suffix of it; HTTP origins only for
`localhost`. RP display name = the RP ID. Reason: the product name lives in instance settings
(M1-46), and the display name is only a label. Public key algorithms offered and accepted: ES256
(COSE -7), EdDSA (-8), RS256 (-257, needed by Windows Hello); nothing else.

Ceremony state (03 `auth_challenges`, 08 "2.5"):

- Begin creates a row: `id` = new UUIDv7 (API `challenge_id`), challenge value = 32 bytes from
  `crypto/rand`, stored only as `challenge_hash` = HMAC-SHA-256(token pepper, value);
  `webauthn_session` = `webauthn_session.v1` without the challenge; `expires_at` per the table
  below. go-webauthn's begin functions create the options and session data: put the generated
  challenge into the options (or use the library's challenge option if the pinned version has
  one), store the session data fields named above, never the challenge itself.
- Complete locks the row (`SELECT ... FOR UPDATE`), accepts only `status = issued` and not expired,
  checks that the challenge in `clientDataJSON` has the stored HMAC (`hmac.Equal`), rebuilds
  `webauthn.SessionData` from the row and verifies; then sets `used` (success) or `failed` (failed
  verification) with `finished_at`. Unknown, used or expired rows: `401 unauthenticated` for login
  and enrollment, `400 validation_failed` for registration.

| Purpose (`auth_challenges.purpose`) | Bound to | Lifetime |
|---|---|---|
| `passkey_login` | nothing (the credential names the person) | `auth.login_flow_ttl` |
| `passkey_enroll` | `enrollment_code_id` (checked at begin, redeemed at complete) | `auth.login_flow_ttl` |
| `passkey_register` | `session_id` of the caller | `auth.login_flow_ttl` |
| `step_up` (M1-35) | `session_id`; `passkey_options` use the same value | `auth.step_up_challenge_ttl` |

Assertion checks for every purpose: origin in the list, RP ID hash, user present and user verified
flags set (else reject), signature with the stored COSE key, credential not revoked, person
`active`. Backup flags update `backup_eligible` and `backup_state`.

Sign counter: if stored and received counter are both above zero and received is not greater than
stored, write an audit entry (action of the ceremony, decision `allow`, details
`{passkey_id, sign_count_regression: true, stored, received}`), let the ceremony continue and keep
the higher value. Counters that stay zero (synced passkeys) are not a regression.

Login (`beginPasskeyLogin`, `completePasskeyLogin`):

- Begin: `login_name` optional. With a name of a person who has a local identity and active
  passkeys, `allowCredentials` lists them. For an unknown name or a name without passkeys, return
  one decoy credential ID = first 32 bytes of `HMAC-SHA-256(pepper, "passkey-decoy/v1" || 0x00 ||
  lowercase name)` so the answer has the same shape; without a name, no `allowCredentials`
  (discoverable). Residual risk (document): the number of real passkeys is visible for known names.
- Complete: DPoP login proof without `ath`; verify the assertion; the person must have an active
  identity on the enabled `local` provider (else `401 unauthenticated`). Reason: passkey login of
  an OIDC-only person would bypass `auth.idp_recheck_interval`; their passkeys serve step-up.
  Then `CompleteLogin` (M1-37) with `TrustUnconfirmed`, `AuthMethod = "passkey"`, `IdentityID` =
  the local identity. Every rejection gives the same `401` body.

Enrollment with passkey (`beginEnrollment`, `completeEnrollment`):

- Begin: `EnrollmentCodes.Match(code, first_login, recovery)` read-only check (no attempt count on
  begin); invalid code `401 unauthenticated`. Creation options for the code's person: `user.id` =
  16 bytes of the principal ID, `user.name` = local identity subject or handle,
  `user.displayName` = display name, `excludeCredentials` = active passkeys of the person.
- Complete: load the challenge (row lock) and its `enrollment_code_id`, lock and check the code
  again, verify the creation response; on failure roll back, `RecordFailedAttempt` (own
  transaction, 08 "3.2") and answer `401`. On success insert the passkey (`name` =
  `passkey_name`), `CompleteLogin` with `TrustConfirmed`, `AuthMethod = "enrollment_code"`, then
  `MarkUsed`; `201` `TokenResponse`. `EnrollCompleteRequest` carries no code: the challenge row
  holds it.

Registration (`beginPasskeyRegistration`, `completePasskeyRegistration`, action
`passkey.register`, self): options like enrollment with the calling person and session; complete
requires the session stored with the challenge; `201` `Passkey`. Duplicate `credential_id`: `409
conflict`.

Revoke (`revokePasskey`, `DELETE /users/{user_id}/passkeys/{passkey_id}`, self or owner,
step-up): sets `revoked_at`; already revoked `204`; not visible `404`.

Step-up with passkey: `createStepUpChallenge` adds `passkey_options` (request options with the
person's active passkeys, the step-up challenge value as challenge, UV required) when the person
has at least one active passkey. `completeStepUpWithPasskey` consumes the `step_up` row of M1-35
(same lock and single use), verifies the assertion against the session's person (another person's passkey
fails and counts toward `auth.step_up_fail_limit`), then sets `step_up_at` and `step_up_method =
'passkey'` and updates `last_used_at` and `sign_count`. Works for local and OIDC people.

Conditional step-up (identical shape in M1-39; keep names exactly so the merge is clean):

- Add to `authz.PrincipalSource`: `HasActivePasskey(ctx context.Context, userID <UUID type of
  PrincipalSource>) (bool, error)` (true if a `passkey_credentials` row with `revoked_at IS NULL`
  exists); implement it in the identity adapter; update the authz fakes.
- In the authz file that implements step 8 of 07 "Ablauf", add (or extend, if M1-39 already
  created it) `var stepUpOnlyWithExistingCredential = map[authorize.Action]func(ctx
  context.Context, src PrincipalSource, userID <UUID type>) (bool, error){...}` with the entry for
  `passkey.register`. In step 8: if the action is in the map and the function returns `false`, no
  step-up is required; "human with a session" stays required (else `forbidden`,
  `step_up_not_possible`). Rights logic stays in `authz` (B7).

Virtual authenticator (`virtualauthn`): holds ES256 keys in memory; produces creation responses
(attestation format `none`) and assertion responses as JSON exactly like a browser
(`clientDataJSON` with type, challenge, origin; authenticator data with RP ID hash, flags, counter);
options to drop the UV flag, set the origin, set the RP ID, set or freeze the counter, sign with
another key, mark backup flags and produce discoverable assertions with user handle.

Rulings to record in `docs/DECISIONS.md` (the spec decisions are in 03 R-DM-31 and 08 "2.5"; link
them): RP display name; sign counter handling; decoy credential derivation (`passkey-decoy/v1`).

## Tests (write them first)

All WebAuthn tests use `virtualauthn`; integration tests use `pgtest` and the controllable clock.

Unit:

- `TestCeremonySingleUse`: `issued` and not expired accepted once; `used`, `failed` and expired
  rejected; the stored row never contains the challenge value.
- `TestSignCountRule`: table with zero counters, increase, equal, decrease.
- `TestWebAuthnConfigValidation`: RP ID suffix rule, HTTP origin only for localhost, defaults from
  `server.public_url`.
- `TestVirtualAuthenticatorRoundTrip` against the adapter.

Integration:

- `TestEnrollmentWithPasskey`: code from `bootstrap-owner`, begin, complete; `201`, device
  `active`, passkey stored, code used; a second complete with the same code `401`.
- `TestEnrollmentFailedVerificationCountsAttempt`: wrong origin; `attempts` incremented.
- `TestPasskeyLoginWithName` and `TestPasskeyLoginDiscoverable`: `201`, `auth_method: passkey`,
  new device `pending`, known active device stays active.
- `TestPasskeyLoginRequiresUserVerification`: UV flag missing `401`.
- `TestPasskeyLoginWrongOriginOrRPID`: `401` each.
- `TestPasskeyAssertionReplayRejected`: the same completion body sent twice, second `401`;
  concurrent pair gives one `201`.
- `TestPasskeyLoginExpiredChallenge`: clock past `auth.login_flow_ttl` `401`.
- `TestPasskeyLoginRevokedCredential`: `401`.
- `TestPasskeyLoginOnlyWithLocalIdentity`: person without local identity `401`; the same passkey
  works for step-up.
- `TestPasskeyLoginDecoy`: unknown name returns exactly one credential ID of 32 bytes, stable for
  the same name, different for another name; no error status.
- `TestSignCountRegressionAudited`: lower counter; login succeeds, one audit entry with
  `sign_count_regression`, stored counter unchanged.
- `TestPasskeyRegisterStepUpOnlyWhenOneExists`: first registration without step-up `201`; second
  without fresh step-up `401 step_up_required`; with step-up `201`; personal token `403`.
- `TestPasskeyRegisterBoundToSession`: completion from another session `400`.
- `TestPasskeyRegisterDuplicate`: `409 conflict`.
- `TestPasskeyAlgorithms`: ES256, EdDSA and RS256 accepted, ES384 rejected.
- `TestRevokePasskey`: self with step-up `204`, owner `204`, other person `404`, repeat `204`.
- `TestStepUpWithPasskey`: `createStepUpChallenge` contains `passkey_options`; completion sets
  `step_up_method: passkey`; a challenge used twice fails; another person's passkey fails and
  counts toward `auth.step_up_fail_limit`.
- `TestCredentialOverviewListsPasskeys`.
- `TestNoCeremonyStateInMemory`: begin on one server instance, complete on a second instance with
  the same database and secrets; `201`.

Commands that must pass:

```sh
make api-lint api-checks codegen-check arch migrations-lint config-doc-check docs licenses
cd services/server && go test -race ./internal/identity/... ./internal/authz/... ./internal/platform/config/...
make check
```

## Acceptance criteria

- [ ] Ceremony rows in `auth_challenges` with the purposes of 03; `make api-lint api-checks docs`
      green; spec text changed only where proven wrong.
- [ ] `userVerification = required`, `attestation = none`, RP ID and origins from configuration,
      only ES256, EdDSA and RS256.
- [ ] No ceremony state in process memory; completion on another node works.
- [ ] Every challenge works once and only within its TTL.
- [ ] A decreasing `sign_count` writes an audit entry.
- [ ] `passkey.register` needs step-up once a passkey exists, decided in `authz`.
- [ ] Passkey login creates `pending` devices for unknown devices and only for people with a local
      identity.
- [ ] go-webauthn pinned in `go.mod`, licence allowed (`make licenses`).
- [ ] `make check` green, coverage at or above `quality.coverage.go_min_percent`.

## Definition of done

- Code and tests as above; doc comments on every exported symbol; comments explain why.
- `internal/identity/README.md` (table, use cases, operations, ceremonies), README of
  `adapters/webauthn` and `virtualauthn`; `services/server/CHANGELOG.md` entry under
  "Unreleased".
- Specification, API and configuration changes in the same branch; generated files regenerated;
  `docs/DECISIONS.md` updated.
- `make check` green.
- `make review` and an independent code review over the diff, then the red team per
  `docs/quality/red-team-gate.md` (focus: replay, origin confusion, account enumeration, step-up
  bypass); verdicts in the report.
- Merge locally into the base branch only if review is clean, the red team verdict is `JA` and
  `make check` is green; then `git worktree remove`. If M1-39 merged first, keep both entries of
  `stepUpOnlyWithExistingCredential` and both `PrincipalSource` methods.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then the
commits, the files with line counts, test results with counts, the pinned go-webauthn version, what
is not proven, and follow-ups that could not be done in scope with the reason.
Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
