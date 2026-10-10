# M1-39: Local accounts with password and TOTP

Status: ready | Depends on: M1-37 | Etappe: M1

## Goal

After this ticket a person with a local account can set a password and enrol TOTP, and can log in
with login name, password and a TOTP code as a switchable fallback for instances without an
identity provider and for native apps without passkeys. Passwords are stored as Argon2id, the TOTP
secret only encrypted with a key from the secret provider, every TOTP code works once, failed
attempts lock only the password path, and unknown names are indistinguishable from wrong
passwords in answer and timing.

## Read first

- `docs/spec/08-identitaet.md`: "Lokale Konten", "Zuordnung einer externen Identität",
  "Step-up-Ablauf", "DPoP", "Schutz der Anmeldung", "Tests" (ID-T20, ID-T21, ID-T24, ID-T27),
  "Entscheidungen dieses Kapitels" (R-ID-09, R-ID-10), "Konfigurationswerte dieses Kapitels".
- `docs/spec/03-datenmodell.md`: Grundsatz 9; tables `local_credentials`, `user_identities`,
  `identity_providers`, `auth_sessions`.
- `docs/spec/05-aktionen.md`: "Spalten Owner, Step-up, Audit, Rechte"; actions
  `auth.login.complete`, `local_credential.set`, `identity.read`.
- `docs/spec/07-rechte.md`: "Prinzipal-Kontext", "Ablauf" step 8.
- `docs/spec/06-api.md`: "Anmeldeabläufe" point 3, "Fehler" point 5 (`404` versus `403
  feature_disabled`), "Endpunkte" (Auth, Me).
- `docs/spec/16-betrieb.md`: "Secret-Provider", "Geheimnisse im Compose-Stack".
- `docs/spec/19-sicherheit.md`: the secrets table (row "Schlüssel für TOTP-Geheimnisse").
- `docs/operations/configuration.md`: section "Geheimnisse".
- `docs/spec/02-architektur.md`: "Schichten im Modul", "Modulregister als Quelle",
  "Konfigurationsladen".
- ADR 0011, ADR 0019.
- `docs/api/openapi.yaml`: `loginWithPassword`, `setMyLocalCredential`, `getMyCredentials`;
  schemas `PasswordLoginRequest`, `LocalCredentialSet`, `LocalCredentialResult`,
  `CredentialOverview`, `Discovery.local_password_enabled`.
- RFC 6238 (TOTP) and RFC 4226 (HOTP): https://www.rfc-editor.org/rfc/rfc6238.html,
  https://www.rfc-editor.org/rfc/rfc4226.html; OWASP Password Storage Cheat Sheet (link in 08).
- Code: `internal/identity` (M1-36 providers and credential overview, M1-37 `CompleteLogin`),
  `internal/authz` (`PrincipalSource`, authorize step 8), `internal/platform/secrets` and
  `bothy secrets init` (M1-09), `internal/platform/config` (M1-07).

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
   `example.com`; example passwords contain `EXAMPLE`. Configuration values only through named keys
   listed in `docs/operations/configuration.md`.
7. Password hash Argon2id in PHC format with parameters `auth.argon2id_*`; values below the OWASP
   floor (19456 KiB, 2 iterations, parallelism 1) stop the start (08 "Lokale Konten", ID-T24).
8. Minimum length `auth.password_min_length`, check against a local list of common passwords, no
   composition rules.
9. TOTP per RFC 6238 with 30-second step, at most one step tolerance, every code accepted only
   once; the server stores the last accepted step (ID-T20, ID-T27).
10. The TOTP secret is stored only AEAD-encrypted; the key lives only in the secret provider and
    the row keeps `totp_key_ref` (03 `local_credentials`, 19).
11. No recovery codes (R-ID-09).
12. `auth.local_password_enabled = false` makes the password login answer `404` (06 "Fehler").
13. Failed attempts per local login name: `auth.login_fail_limit` per `auth.login_fail_window`,
    then the password path only is locked for `auth.login_lock_duration`; passkey, pairing and
    OIDC stay open (R-ID-10). Unknown login name and wrong password give the same answer in the
    same time; the server computes an Argon2id hash for unknown names too (ID-T21).
14. `local_credential.set` needs step-up once credentials exist (05).

## Scope

Precondition (check first): `docs/spec/03-datenmodell.md` table `local_credentials` must contain
a column for the start of the failure window, expected `fail_window_started_at timestamptz NULL`
(requested by the brief writer; without it `failed_count` cannot be tied to
`auth.login_fail_window`). If the column is absent, stop with `Verdict: BLOCKED missing
local_credentials.fail_window_started_at in 03` (P-11). If it exists under another name, use that
name.

Specification first, on this branch, before production code:

1. `docs/api/openapi.yaml`:
   - `LocalCredentialSet`: `password` becomes optional; add optional `totp_code` (string,
     `pattern: ^[0-9]{6}$`); keep `totp_enroll`; `additionalProperties: false`; description lists
     the rules from Design notes ("at least one field; the first call must set `password`").
   - `LocalCredentialResult`: add required `totp_enabled` (boolean).
   - `setMyLocalCredential`: responses add `403` (`feature_disabled`), `409` (`conflict`, no local
     identity), `400`.
   - `loginWithPassword`: description "New devices are `pending` until approved (M1-38)."
2. Regenerate the generated blocks of `docs/spec/06-api.md` (M1-03 generator).
3. `docs/spec/08-identitaet.md`, "Lokale Konten": login name is the `subject` of the local
   identity; TOTP enrolment with confirmation (`totp_enroll`, then `totp_code`); AEAD scheme and
   `auth.totp_key_ref`; otpauth issuer; the meaning of "Anmeldedaten bestehen" for step-up; the
   failure window semantics; rehash on parameter change; source of the common-password list.
   "Konfigurationswerte dieses Kapitels": add `auth.totp_key_ref` with default
   `file:<secrets.dir>/totp_key`. In 07 "Ablauf" step 8 add the sentence: "`passkey.register` und
   `local_credential.set` verlangen Step-up nur, wenn die Person schon einen aktiven Passkey bzw.
   ein eingerichtetes lokales Passwort mit TOTP hat; Mensch mit Sitzung bleibt Pflicht." (M1-40
   adds the identical sentence; keep the text exactly so the merge is clean.)
4. `docs/operations/configuration.md` section "Geheimnisse": row `totp_key`, "erzeugt", service
   `server`, use `auth.totp_key_ref`. Regenerate section 4 with the M1-08 generator.
5. `docs/DECISIONS.md`: rulings listed in Design notes.

Production code under `services/server/`:

- `migrations/<next sequence>_identity_local_credentials.sql`: table `local_credentials` exactly as
  in 03, including the window column from the precondition.
- `internal/identity/adapters/postgres/queries/local_credentials.sql`; regenerated sqlc code.
- `internal/identity/domain/totp.go`: `TOTPCode(secret []byte, step int64, digits int) string`,
  `TOTPStep(t time.Time) int64`, `VerifyTOTP(secret []byte, code string, now time.Time,
  lastStep *int64) (step int64, ok bool)`.
- `internal/identity/domain/lockout.go`: pure function for the failure window and lock.
- `internal/identity/domain/password_rules.go`: length rule on the NFC-normalised password
  counted in Unicode code points.
- `internal/identity/app/local.go`: use cases `SetLocalCredential` and `LoginWithPassword`;
  outbound ports `PasswordHasher`, `SecretSealer`, `CommonPasswords`.
- `internal/identity/adapters/passwordhash`: Argon2id with `golang.org/x/crypto/argon2`.
- `internal/identity/adapters/secretbox`: AES-256-GCM sealer using the secret provider.
- `internal/identity/adapters/passwordpolicy`: embedded list `common_passwords.txt` (truncated
  hashes, Design notes) and its `README.md` with source, commit, licence and checksum.
- `internal/identity/adapters/httpapi`: handlers `LoginWithPassword`, `SetMyLocalCredential`;
  the credential overview from M1-36 fills `local_password_set` and `totp_enrolled`.
- `internal/authz`: conditional step-up for `local_credential.set` (Design notes).
- `internal/identity` adapter of `authz.PrincipalSource`: method `HasEstablishedLocalCredential`.
- Configuration schema (M1-07): `auth.totp_key_ref` (type secret_ref); start validation of the
  Argon2id floors.
- Secret catalogue used by `bothy secrets init` (M1-09; find it with `grep -rn token_pepper
  services/server/internal/platform/secrets services/server/cmd`): add `totp_key`, 32 random
  bytes, service `server`.
- `architecture/modules.yaml`: vendor `xcrypto: [golang.org/x/crypto/argon2]` allowed for
  `identity` adapters; regenerate `.go-arch-lint.yml`.
- `services/server/api/operations.yaml`: enable `loginWithPassword`, `setMyLocalCredential`.
- `internal/identity/README.md`, `services/server/CHANGELOG.md`, `docs/REUSE.md` attribution of
  the password list.

Outbound ports (in `internal/identity/app`, names fixed):

```go
type PasswordHasher interface {
    Hash(password string) (phc string, err error)
    // Verify reports a match and whether the stored parameters differ from the configured ones.
    Verify(phc, password string) (ok bool, needsRehash bool, err error)
    // VerifyDummy spends the same work as Verify for an unknown login name.
    VerifyDummy(password string)
}
type SecretSealer interface {
    Seal(identityID ids.ID, plaintext []byte) (ciphertext []byte, keyRef string, err error)
    Open(identityID ids.ID, ciphertext []byte, keyRef string) ([]byte, error)
}
type CommonPasswords interface { Contains(password string) bool }
```

## Out of scope

- Passkeys and the conditional step-up rule for `passkey.register` (M1-40), device approval of
  the pending devices this login creates (M1-38).
- Step-up with password and TOTP (`step_up_method = 'password_totp'`): 08 foresees it for the
  console, but no API operation exists; it belongs to the console session work (M6) after a
  specification change.
- Discovery field `local_password_enabled` (M1-46).
- Re-encrypting TOTP secrets after a key change (operations task, later).
- Revoking other sessions on password change (not specified).

## Boundaries

- Work in a dedicated git worktree on branch `feat/M1-39-identity-local-accounts`.
- Conventional Commits with module scope and DCO sign-off (`git commit -s`).
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside Scope, except `docs/DECISIONS.md` and CHANGELOG entries.
- Follow the host rules supplied by the launcher. Tests bind only `127.0.0.1` and port 0;
  PostgreSQL only through `pgtest`.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at
  the end.
- Never read or print secrets. No network calls except package downloads for pinned tools and the
  single download of the password list source named in Design notes.
- If an element of the core contract (00, 03, 04, 05) named here is missing or different, stop with
  `Verdict: BLOCKED` (P-11).

## Design notes

Local identity and login name: a person has a local account when an active `user_identities` row
points to the enabled `local` provider (created by `bootstrap-owner` or `createUser` with
`local_account`, M1-37). The login name is that row's `subject`; input is trimmed and lowercased
before lookup. `local_credentials.identity_id` references this identity.

Argon2id (`passwordhash`): salt 16 bytes from `crypto/rand`, key length 32 bytes, PHC string
`$argon2id$v=19$m=<kib>,t=<iterations>,p=<parallelism>$<salt b64>$<hash b64>` (standard Base64
without padding). Parameters from `auth.argon2id_memory_kib`, `auth.argon2id_iterations`,
`auth.argon2id_parallelism`. Verification compares in constant time. `needsRehash` is true when
the stored parameters differ from the configuration; a successful login then stores a new hash in
the same unit of work. `VerifyDummy` verifies against a hash computed once at start with the
current parameters. Start validation (also `bothy config check`): memory below 19456, iterations
below 2 or parallelism below 1 stop the start with a message that names the key.

TOTP:

- Secret: 20 random bytes (RFC 4226 recommends 160 bits). HMAC-SHA-1, 6 digits, step 30 seconds,
  T0 = 0. These constants are held by tests (ID-T27).
- Verification accepts the steps `current - 1`, `current`, `current + 1`; the matched step must be
  greater than `totp_last_step` (so a code works once and older codes never again). Compare codes
  in constant time.
- otpauth URI: `otpauth://totp/<issuer>:<login name>?secret=<Base32 without padding>&issuer=
  <issuer>&algorithm=SHA1&digits=6&period=30`, issuer = host of `server.public_url`, all parts
  URL-escaped. Reason for the host instead of the product name: the display name lives in
  instance settings (M1-46), which this ticket must not depend on.
- Sealing (`secretbox`): AES-256-GCM, key = 32 bytes resolved from `auth.totp_key_ref` through
  the secret provider (a different length stops the start), nonce 12 random bytes, AAD =
  `"totp/v1" || 0x00 || identity_id (16 bytes)`, stored layout `0x01 || nonce || ciphertext and
  tag`. The row stores the ref that sealed it in `totp_key_ref`; `Open` resolves that ref, so a
  new key applies to new rows while old rows stay readable (19).

Common passwords (`passwordpolicy`): source is the SecLists file
`Passwords/Common-Credentials/10k-most-common.txt` (github.com/danielmiessler/SecLists, MIT
licence) at a commit you pin; download it once with `curl` into a `mktemp -d` folder, verify that
the repository's `LICENSE` at that commit is MIT (if not, stop with `BLOCKED`), and generate
`common_passwords.txt` with one line per entry: the first 16 hex characters of
`SHA-256(lowercase(NFC(password)))`, sorted, unique. Commit only the generated file, not the
source. Reason for hashes: no word list in the repository (white-label scan, readability) and a
small file. Record source URL, commit, SHA-256 of the source file and the generation command in
the adapter README; a Go test checks the file is sorted, unique and well-formed. `Contains`
hashes the candidate the same way.

`SetLocalCredential` (`setMyLocalCredential`, action `local_credential.set`, self):

- `auth.local_password_enabled = false`: `403 feature_disabled` (authenticated endpoint).
- No active local identity on an enabled `local` provider: `409 conflict` with `reason:
  no_local_identity`.
- Body rules: at least one of `password`, `totp_enroll`, `totp_code`; if no row exists,
  `password` is required. Violations: `400 validation_failed` with `errors[]`.
- `password`: shorter than `auth.password_min_length` gives code `too_short`; listed gives
  `common_password`; else new hash, `password_changed_at = now`.
- `totp_enroll: true`: new secret, sealed; `totp_enabled_at` and `totp_last_step` NULL (the
  enrolment is pending until confirmed); response `totp_secret_uri` once.
- `totp_code`: verified against the current secret; success sets `totp_enabled_at = now` and
  `totp_last_step`; failure gives `400 validation_failed` code `totp_invalid`.
- Response `200` `LocalCredentialResult` with `password_set`, `totp_enabled`, optional
  `totp_secret_uri`. Audit per catalogue (`ja`), details without any secret.
- The response with `totp_secret_uri` is a one-time secret: mark it with the mechanism from M1-37
  (`idempotency.MarkOneTimeSecret`) in case a client sends `Idempotency-Key`.

Conditional step-up (identical shape in M1-40; keep names exactly so the merge is clean):

- "Credentials exist" for `local_credential.set` means: a `local_credentials` row of the person
  with `totp_enabled_at IS NOT NULL` (an established password path). Reason: the first set-up and
  its confirmation must work without step-up, and a stolen session that adds a password only gains
  a login whose new devices stay `pending`.
- Add to `authz.PrincipalSource`: `HasEstablishedLocalCredential(ctx context.Context, userID
  <UUID type of PrincipalSource>) (bool, error)`; implement it in the identity adapter; update the
  authz fakes.
- In the authz file that implements step 8 of 07 "Ablauf", add (or extend, if M1-40 already
  created it) `var stepUpOnlyWithExistingCredential = map[authorize.Action]func(ctx
  context.Context, src PrincipalSource, userID <UUID type>) (bool, error){...}` with the entry for
  `local_credential.set`. In step 8: if the action is in the map and the function returns `false`
  for the resource's person, step-up is not required; the check "human with a session" (else
  `forbidden`, `step_up_not_possible`) stays. Rights logic stays inside `authz` (B7).

`LoginWithPassword` (`loginWithPassword`, action `auth.login.complete`, `rl.public_auth`):

1. `auth.local_password_enabled = false` or no enabled `local` provider: `404 not_found`.
2. DPoP login proof without `ath` (M1-33 verifier).
3. One unit of work: find the local identity by login name, the person (must be `active`) and
   `local_credentials` with `SELECT ... FOR UPDATE`.
4. Exactly one Argon2id computation per request: `Verify` when a row exists, else
   `VerifyDummy`; never skip it (also not when locked), so time does not reveal accounts or locks.
5. Reject with the same `401 unauthenticated` body when: no row, person not active,
   `locked_until > now`, `totp_enabled_at IS NULL`, wrong password, or TOTP not accepted.
6. Counter (only when a row exists and the account is not locked): if `fail_window_started_at` is
   NULL or `now >= fail_window_started_at + auth.login_fail_window`, start a new window
   (`fail_window_started_at = now`, `failed_count = 1`), else `failed_count + 1`. When
   `failed_count >= auth.login_fail_limit`: `locked_until = now + auth.login_lock_duration`,
   `failed_count = 0`, `fail_window_started_at = NULL`, audit entry deny with details `{lock:
   true}`. The counter update commits; every rejection is audited (aggregation by M1-24).
7. Success: `totp_last_step` = matched step, reset counter fields and `locked_until`, rehash if
   needed, then `CompleteLogin` (M1-37) with `TrustUnconfirmed`, `AuthMethod = "password_totp"`,
   `IdentityID` = the local identity. New devices are `pending`; a known active device with the
   same key stays active. `201` `TokenResponse`.

Configuration: new `auth.totp_key_ref`; used `auth.local_password_enabled`,
`auth.password_min_length`, `auth.argon2id_memory_kib`, `auth.argon2id_iterations`,
`auth.argon2id_parallelism`, `auth.login_fail_limit`, `auth.login_fail_window`,
`auth.login_lock_duration`, `server.public_url`, `authz.not_found_timing_tolerance` (tests).

Rulings to record in `docs/DECISIONS.md`: TOTP confirmation step and the API change; meaning of
"credentials exist"; conditional step-up in authz through `PrincipalSource`; issuer from
`server.public_url`; AEAD scheme and AAD; password list source and hashed storage; failure window
column; one Argon2id computation per request.

## Tests (write them first)

Tests use the Argon2id floor parameters (19456, 2, 1) to stay fast; one test uses the defaults.

Unit:

- `TestIDT27_TOTPVectorsRFC6238`: the SHA-1 vectors of RFC 6238 Appendix B with 8 digits (secret:
  the ASCII string "1234567890" repeated twice, 20 bytes, as in the RFC; T = 59 gives `94287082`,
  1111111109 gives `07081804`, 1111111111 gives `14050471`, 1234567890 gives `89005924`,
  2000000000 gives `69279037`, 20000000000 gives `65353130`); step size 30 seconds and tolerance
  of exactly one step.
- `TestIDT20_TOTPCodeOnlyOnce`: same code twice in the same step, second rejected; a code of the
  previous step after a newer step was used, rejected.
- `TestTOTPWindowEdges`: steps -2 and +2 rejected, -1 and +1 accepted.
- `TestLockoutWindow`: table over window start, expiry, limit reached, lock expiry.
- `TestPasswordRules`: NFC normalisation, code-point length, list hit.
- `TestPHCFormatAndRehash`: parse, parameter mismatch gives `needsRehash`.
- `TestSecretBoxBindsIdentity`: a ciphertext opened with another identity ID fails; a wrong
  version byte fails; tampering fails.
- `TestCommonPasswordsFile`: sorted, unique, 16 hex characters per line; a known listed password
  (taken from the source file during generation and recorded in the test) is found.
- `TestIDT24_Argon2BelowFloorStopsStart`: each of the three keys below its floor makes config
  validation fail and names the key; floor values pass.

Integration (`pgtest`, controllable clock):

- `TestPasswordLoginHappyPath`: set password, enrol and confirm TOTP, log in; `201`,
  `auth_method: password_totp`, new device `pending`.
- `TestPasswordLoginKnownDeviceStaysActive`.
- `TestIDT21_UnknownNameTimingEqualsWrongPassword`: interleaved samples (at least 31 each) of
  unknown login name and known name with wrong password; equal status and body (except
  `correlation_id`); medians differ by less than `authz.not_found_timing_tolerance`.
- `TestLockedAccountStillHashes`: while locked, a correct password is rejected with the same body
  and the hasher was called once (counting fake).
- `TestLockoutOnlyPasswordPath`: after the limit the password login is rejected for
  `auth.login_lock_duration`; redeeming an enrollment code (M1-37) still works.
- `TestPasswordDisabledGives404`: `auth.local_password_enabled = false`: login `404`,
  `setMyLocalCredential` `403 feature_disabled`.
- `TestTOTPSecretEncryptedAtRest`: `totp_secret_ciphertext` does not contain the secret bytes;
  `totp_key_ref` equals the configured ref.
- `TestTOTPEnrolmentNeedsConfirmation`: login before confirmation rejected; after `totp_code`
  accepted.
- `TestLocalCredentialStepUpOnlyWhenEstablished`: first set and confirm without step-up `200`;
  later change without fresh step-up `401 step_up_required`; with step-up `200`; personal token
  `403`.
- `TestSetLocalCredentialValidation`: `too_short`, `common_password`, empty body, first call
  without password, no local identity `409`.
- `TestPasswordAndTOTPNeverLogged`: captured logs and audit details contain neither password,
  TOTP code, secret nor otpauth URI.
- `TestCredentialOverviewFlags`: `local_password_set` and `totp_enrolled` in `getMyCredentials`.

Commands that must pass:

```sh
make api-lint api-checks codegen-check arch migrations-lint config-doc-check docs licenses
cd services/server && go test -race ./internal/identity/... ./internal/authz/... ./internal/platform/config/...
make check
```

## Acceptance criteria

- [ ] Precondition checked; migration matches 03.
- [ ] ID-T20, ID-T21, ID-T24 green and named after their IDs; the TOTP part of ID-T27 green.
- [ ] Password login answers `404` when switched off; unknown names and wrong passwords are
      indistinguishable in body and timing.
- [ ] Lock after `auth.login_fail_limit` failures within `auth.login_fail_window` affects only the
      password path.
- [ ] TOTP secrets are encrypted with a key from the secret provider; `bothy secrets init` creates
      `totp_key`.
- [ ] Step-up for `local_credential.set` only once a local credential is established, decided in
      `authz`.
- [ ] New dependency `golang.org/x/crypto` pinned in `go.mod`, licence allowed (`make licenses`).
- [ ] `make check` green, coverage at or above `quality.coverage.go_min_percent`.

## Definition of done

- Code and tests as above; doc comments on every exported symbol; comments explain why.
- READMEs (`internal/identity`, `adapters/passwordpolicy`), `services/server/CHANGELOG.md` entry
  under "Unreleased", `docs/REUSE.md` attribution.
- Specification, API and configuration changes in the same branch; generated files regenerated;
  `docs/DECISIONS.md` updated. No ADR unless you take an architecture decision beyond this brief
  (then the next free number).
- `make check` green.
- `make review` and an independent code review over the diff, then the red team per
  `docs/quality/red-team-gate.md` (focus: guessing, lockout abuse, timing, secrets at rest);
  verdicts in the report.
- Merge locally into the base branch only if review is clean, the red team verdict is `JA` and
  `make check` is green; then `git worktree remove`. If M1-40 merged first, keep both entries of
  `stepUpOnlyWithExistingCredential` and both `PrincipalSource` methods.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then the
commits, the files with line counts, test results with counts, the pinned password-list commit and
checksum, what is not proven, and follow-ups that could not be done in scope with the reason. Then
write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
