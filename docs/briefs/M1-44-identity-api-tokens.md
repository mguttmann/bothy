# M1-44: API tokens with prefix and checksum

Status: ready | Depends on: M1-29, M1-35 | Milestone: M1

## Goal

After this ticket agents, apps and (behind the switch `tokens.personal_enabled`) people
authenticate with long-lived API tokens of the form `bothy_<kind>_<random>_<checksum>`: stored only
as HMAC, shown exactly once, with a mandatory expiry, DPoP-bound for agents by default, revocable
with immediate effect, and recognizable by the secret scanner. ADR 0061 records the format. A
personal token acts with the person's grants but never with owner permissions and never
provides step-up (SR5).

## Read first

- `docs/spec/08-identity.md`: 7.1 "Overview", 7.3 "DPoP", 12 "API tokens for agents, apps
  and tools", 15 (ID-T23, ID-T27), 18 (`auth.dpop_required_for`, `auth.api_token_max_ttl`,
  `auth.personal_token_max_ttl`, `auth.token_pepper_ref`).
- `docs/spec/03-data-model.md`: 1 "Principles" (point 9), 4.2 `api_tokens`, `principals`,
  4.10 `idempotency_keys`, 4.12 `audit_events`, 6.3 "Lock order".
- `docs/spec/05-actions.md` and `docs/spec/actions.yaml`: `token.list`, `token.create`,
  `token.revoke`; 2.1, 2.3 point 1, 2.4, 2.5, 2.6.
- `docs/spec/07-permissions.md`: 3 "Principal context", 4.1 (`token_revoked`, `step_up_not_possible`,
  `owner_only`, `not_self`), 4.2, SR5, SR7, 11.1 AZ-M35.
- `docs/spec/06-api.md`: 3 "Authentication per principal kind", 3.1 "DPoP", 5 "Errors"
  (`feature_disabled`), 6 "Pagination", 7 "Idempotency".
- `docs/spec/16-operations.md`: 6.2 "Feature flags".
- `docs/spec/19-security.md`: 9, 10 (SEC-SEC-3, SEC-SEC-4).
- `docs/spec/02-architecture.md`: 4.3 (`identity` port `Tokens`), 4.4, 4.5 (`kernel/actor`,
  `kernel/idempotency`), 5, 9.
- ADRs 0000 (template), 0001, 0015, 0017, 0020, 0042, 0044.
- `docs/api/openapi.yaml`: operations `listTokens`, `createToken`, `revokeToken`; schemas
  `ApiToken`, `ApiTokenCreate`, `ApiTokenCreated`, `ApiTokenPage`.
- `.gitleaks.toml` rule `bothy-api-token`; `tools/ci/tests/test_gitleaks_rules.py`.
- `docs/security/threat-model.md`: T-TK-02.
- Merged READMEs and code: `internal/identity` (authenticator, HMAC hasher with the pepper, DPoP
  verifier of M1-33 and M1-34), `internal/authz` (guards, permission-change path of M1-29),
  `internal/kernel/idempotency` and its middleware (M1-20), `internal/instance` (port `Flags`, if
  M1-47 is merged), `services/server/api/operations.yaml`.
- `docs/PLAN.md` section 3, section 7 rows M1-34, M1-44, M1-45, M1-47, M1-51;
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
8. White label; every committed token-shaped value carries the marker `EXAMPLE` in its random part
   (CONTRIBUTING.md), all others are built at runtime.
9. Limits only through named configuration keys (`auth.api_token_max_ttl`,
   `auth.personal_token_max_ttl`, `auth.dpop_required_for`, `identity.last_active_granularity`).
10. Format `bothy_<kind>_<random>_<checksum>` with kinds `agt`, `app`, `pat`, `whi`, 256 bits of
    randomness in Base62 and a checksum over the random part (08 12, ADR 0044 for the prefix).
11. Stored only as HMAC-SHA-256 with the server pepper plus `token_prefix`; shown exactly once
    (08 12, 03 point 9, ID-T23).
12. Expiry is mandatory, at most `auth.api_token_max_ttl`, personal tokens at most
    `auth.personal_token_max_ttl` (08 12).
13. Agent tokens are DPoP-bound when `agent` is in `auth.dpop_required_for`; a DPoP-bound token as
    `Bearer` is rejected (08 12, 7.3, RFC 9449 7.2).
14. API tokens never provide step-up; a personal token of an owner has no owner permissions
    (SR5, 05 2.3 point 1, AZ-M35).
15. Personal tokens only with `tokens.personal_enabled`; switched off gives `403
    feature_disabled` (05 `token.create`, 06 5 point 5).
16. The secret scanner has a rule for the prefix (SEC-SEC-4).

## Scope

- `services/server/internal/identity/domain/apitoken/`: pure format code: generate, parse,
  checksum, prefix; kinds `agt`, `app`, `pat`, `whi`; the constant `bothy` exists exactly once.
  `testdata/checksum_vectors.json` with vectors whose random part starts with `EXAMPLE`.
- `services/server/internal/identity/` port `Tokens` (02 4.3): `Issue(ctx, IssueTokenCmd)
  (IssuedToken, error)`, `List(ctx, principalID uuid.UUID, page Page) (TokenPage, error)`,
  `Revoke(ctx, tokenID uuid.UUID, by uuid.UUID) error`, `RevokeAllForPrincipal(ctx, principalID
  uuid.UUID, by uuid.UUID) (int, error)`; all join the ambient Unit of Work. `IssuedToken` carries
  the secret only in a type whose `String`, `Format` and `MarshalJSON` print `[redacted]` except
  through one explicit accessor used by the HTTP handler (SEC-LOG-1).
- `services/server/internal/identity/app/`: use cases `CreateToken`, `ListTokens`, `RevokeToken`;
  consumer port `PersonalTokenSwitch` with `PersonalTokensEnabled(ctx context.Context, principalID
  uuid.UUID) (bool, error)`; API token authentication used by the authenticator.
- `services/server/internal/identity/adapters/postgres/`: migration for `api_tokens` exactly as in
  03 (next free goose number), sqlc queries.
- `services/server/internal/identity/adapters/httpapi/`: handlers `listTokens`, `createToken`,
  `revokeToken`.
- The `kernel/actor.Authenticator` implementation of M1-34: the API token path (Design notes);
  export the function `AuthenticateAPIToken` for the gateway `identify` of M1-64.
- `services/server/internal/authz/`: guard for `token.create` (a token for a person only for
  oneself, `forbidden` `not_self`) in the guard table of `authorize` (07 4.2 step 9), with tests.
- One-time secret: the `createToken` handler calls `idempotency.MarkOneTimeSecret(ctx)` of M1-20
  (Design notes).
- `internal/wire`: bindings, including `PersonalTokenSwitch` (Design notes).
- `.gitleaks.toml` rule `bothy-api-token` already requires exactly 6 checksum characters;
  `tools/ci/tests/test_gitleaks_rules.py`: add a runtime token with a valid checksum and a
  cross-check against the Go vectors.
- The specification is in place (planning, spec first): ADR 0061 (format, Base62 alphabet,
  lengths, CRC-32, `token_prefix` with 6 characters), 08 12 (kinds per principal, scopes rule,
  personal tokens only for oneself, `app_not_installed`, one-time secret), 03 `api_tokens`
  (`expires_at` NOT NULL), 06 7 (R-API-26), `openapi.yaml` (`ApiTokenCreate.expires_at` required,
  description of `createToken`). Change them only where the implementation proves them wrong
  (`docs/spec/02-architecture.md` 4.3 if the port names differ). `services/server/api/operations.yaml`:
  the three operations move to the built list; regenerated server code.
- `services/server/internal/identity/README.md`, CHANGELOG entry under "Unreleased" of
  `services/server/CHANGELOG.md`, `docs/DECISIONS.md` entries for the rulings below,
  `docs/security/threat-model.md` T-TK-02 test column with the test names.
- **`authz.CredentialSource`, API tokens part** (02 "Ports, events and sources per module", 07
  "Flow" `check_active`, PLAN row M1-44): implement `TokenActive` and `TokenExpiresAt` of the
  request's API token in the identity adapter of the source port that M1-28 declared, and replace
  the corresponding empty source in `internal/wire` (fail closed until then). Test:
  `TestCredentialSource_Tokens` against PostgreSQL 18 (active, revoked, expired, foreign
  principal).

## Out of scope

- Incoming webhook tokens (`whi`): format support only; issuance, path handling and verification
  belong to M2-27.
- App tokens issued at installation with installation scopes: M2-24 (see Design notes for the
  behavior until then).
- Revoking personal tokens on `user.disable`: M1-45 (uses `RevokeAllForPrincipal`). Revoking agent
  tokens on `agent.disable`: M1-51.
- Closing open gateway connections on revocation (`4012`): M1-65.
- The feature flag registry and `instance.Flags`: M1-47.
- Console listing of tokens that expire soon: M6.

## Boundaries

- A new migration needs its line in `migrations.sha256`: run `make migrations-lock` after creating
  the file and before `make check`; never edit a merged migration (M1-01, ADR 0007).
- Work in a dedicated git worktree on the branch `feat/M1-44-identity-api-tokens`, created from the
  base branch the launcher names.
- Conventional Commits with the module as scope (`feat(identity): ...`, `docs(adr): ...`) and a DCO
  sign-off on every commit (`git commit -s`).
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the Scope list, except the docs and changelog entries named in this
  brief.
- Follow the host rules supplied by the launcher.
- Tests bind only `127.0.0.1` and port 0 (P-10). PostgreSQL only through `internal/platform/pg/pgtest`;
  without a Docker-compatible runtime the test fails, it never skips (P-09).
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at
  the end.
- Never read or print secrets; never commit a token-shaped value without `EXAMPLE`.
- No network calls except package downloads for pinned tools.
- A gap in the core contract (00, 03, 04, 05, `docs/spec/actions.yaml`) ends the ticket with
  `Verdict: BLOCKED` (P-11); never build a deviation.

## Design notes

**Preconditions (check before writing code; any failure: `Verdict: BLOCKED <which>`).**

1. M1-29 (permission-change path: `permission_version` first, `access.changed`; with M1-24 audit)
   and M1-35 (step-up for `token.create`) are merged: both are dependencies of this ticket.
2. The DPoP verifier (M1-33) and the HMAC hasher with `auth.token_pepper_ref` (M1-34) exist.

Tests obtain step-up through the M1-35 flow (or its test helper), not by writing columns.

**Format** (08 12 and ADR 0061):

- `bothy_` + kind + `_` + random + `_` + checksum; kinds `agt` (agent), `app` (app), `pat`
  (personal), `whi` (incoming webhook). Total length 60 characters.
- Random: 32 bytes from `crypto/rand`, read as a big-endian unsigned integer, written in Base62
  with the alphabet `0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz`, left-padded
  with `0` to 43 characters (62^43 > 2^256).
- Checksum: CRC-32 (IEEE, `hash/crc32.ChecksumIEEE`) over the 43 ASCII bytes of the random part,
  written in the same Base62 alphabet, left-padded to 6 characters. Public precedent: CRC32,
  Base62 with leading-zero padding, last 6 characters
  (https://github.blog/engineering/platform-security/behind-githubs-new-authentication-token-formats/,
  retrieved 2026-10-10) [verified]. The checksum rejects typos and random matches without a database
  call; it is not a security control.
- Parse: exactly `^bothy_(agt|app|pat|whi)_[0-9A-Za-z]{43}_[0-9A-Za-z]{6}$`, checksum equal, the
  random part decodes to a value below 2^256.
- `token_prefix` = `bothy_<kind>_` plus the first 6 characters of the random part.
- `token_hash` = HMAC-SHA-256(pepper of `auth.token_pepper_ref`, the full token string).

**Kinds and principals.** `agt` only for principals of kind `agent`, `app` only for `app`, `pat`
only for `user`; `whi` is never issued by `createToken` and never accepted by the authenticator.

**`createToken`** (`POST /api/v1/principals/{principal_id}/tokens`, `token.create`, step-up,
`rl.admin`, `Idempotency-Key`):

1. Load the principal; missing gives `404`. Resource descriptor: kind `principal`, owner = the
   principal.
2. `authorize(actor, token.create, principal)`; the new `authz` guard rejects a token for a person
   other than the actor with `forbidden` reason `not_self` (ruling: a personal token is the
   person's own credential; an owner minting one for someone else could act as that person).
3. Kind from the principal kind. For `app` principals answer `409 conflict` with `reason:
   app_not_installed` until M2-24 replaces this with the installation check (apps cannot exist in
   M1).
4. Kind `pat`: `PersonalTokenSwitch.PersonalTokensEnabled(ctx, actor)` false gives `403
   feature_disabled`.
5. Body: `name` 1 or more characters without control, format or bidi characters; `scopes` must be
   empty for `agt` (agents hold their scopes implicitly, 05 2.6) and for `pat` (ruling: personal
   tokens act with the person's grants; 07 knows scopes only for apps), else `400
   validation_failed` with pointer `/scopes`; `expires_at` required (08 12), later than
   now and not later than now plus `auth.api_token_max_ttl` (`agt`, `app`) or
   `auth.personal_token_max_ttl` (`pat`), else `400` with pointer `/expires_at`; `dpop_jkt`
   optional, exactly 43 base64url characters (RFC 7638 SHA-256 thumbprint), required when the kind
   is in `auth.dpop_required_for` (values `agent`, `app`, `personal`; register the allowed values in
   the configuration schema if missing), else `400` with pointer `/dpop_jkt`.
6. Generate, insert (`created_by` = actor), audit allow with details `{token_id, kind,
   principal_id, dpop_bound, expires_at}` (never secret, never hash). `token.create` changes no
   permissions.
7. `201 ApiTokenCreated` with `secret`; header `Cache-Control: no-store`.

**Idempotency with a one-time secret.** 03 point 9 forbids storing the token. The handler calls
`idempotency.MarkOneTimeSecret(ctx)` (M1-20): the stored row holds no part of the secret, and a
replay with the same key and body answers `409 conflict` with `reason: one_time_secret` and
`Idempotent-Replay: true` (06 7, R-API-26); the owner revokes the unknown token in the list.

**`listTokens`** (`GET /api/v1/principals/{principal_id}/tokens`, `token.list`, `rl.read`):
`authorize` on the principal (owner, or the principal itself); tokens with `revoked_at IS NULL`
(expired ones included, so they can be seen and revoked), ordered by `created_at` descending then
`id`, cursor pagination per 06 6; schema `ApiToken`, never hash or secret.

**`revokeToken`** (`DELETE /api/v1/tokens/{token_id}`, `token.revoke`, `rl.admin`): load the
token (missing: `404`); resource kind `api_token` with owner = `principal_id`; `authorize`; already
revoked: `204` without a new audit entry; otherwise in a permission-change Unit of Work (M1-29
path): `revoked_at = now`, `revoked_by` = actor, audit allow, `access.changed` for the token's
principal as the path computes it. `204`.

**`RevokeAllForPrincipal`** revokes every active token of a principal in the caller's Unit of
Work and returns the count; the caller owns the permission change and the audit (M1-45,
M1-51).

**Authentication** (shared authenticator, 02 9 step 4):

- `Authorization: Bearer <t>` or `Authorization: DPoP <t>` where `t` starts with `bothy_` takes the
  API token path; everything else stays on the access-token path of M1-34.
- Parse and checksum fail, kind `whi`: `401 unauthenticated` without a database call.
- Lookup by `token_hash`; missing, `revoked_at` set or `expires_at <= now`: `401
  unauthenticated` (audit reason `token_revoked` for the last two); principal kind not matching
  the kind: `401`.
- `dpop_jkt` set: scheme must be `DPoP` and the proof must verify with the M1-33 verifier
  (`htm`, `htu`, `iat`, `jti`, nonce, `ath` = base64url SHA-256 of the token, key thumbprint equal
  to `dpop_jkt`), else `401 invalid_dpop_proof` with `WWW-Authenticate: DPoP
  error="invalid_dpop_proof"` (or `use_dpop_nonce` with a fresh `DPoP-Nonce`). `dpop_jkt` NULL and
  scheme `DPoP`: `401 unauthenticated`.
- Actor: `id` = principal, `kind` from the principal, `token_id`, `token_scopes`, no session, no
  device. Status checks of the principal stay in `authz` (SR7). No cache of token state:
  revocation must work on the next request in every process (AZ-P9).
- `last_used_at`: after a successful authentication, one conditional update `WHERE last_used_at IS
  NULL OR last_used_at < now - identity.last_active_granularity`; a failure is logged with the
  token ID and never fails the request.

**`PersonalTokenSwitch` binding.** M1-47 (feature flags) depends on this ticket and comes later.
Bind `app.PersonalTokensDefault{}` in `internal/wire`, which returns the registered default `false`
(16 6.2: unknown or unregistered flags are off); M1-47 registers `tokens.personal_enabled` and
rebinds `PersonalTokenSwitch` over its `Flags` port. Tests use a fake switch for both values.

**ADR 0061** exists (accepted). Keep it current: if the implementation changes a rule or the
confirmation (`TestFormat_*`, the checksum vectors in Go and Python, the gitleaks rule test),
update the ADR in the same change and run `make adr-index`.

## Tests (write them first)

Unit tests (`domain/apitoken`):

- `TestFormat_RoundTrip` for every kind; length 60; prefix and kind positions.
- `TestFormat_ChecksumVectors`: reads `testdata/checksum_vectors.json` (random parts starting with
  `EXAMPLE`, at least one checksum with leading `0` padding) and checks every checksum.
- `TestParse_Rejects`: wrong prefix, unknown kind, random part of 42 and 44 characters, a
  non-Base62 character, a single changed checksum character, a random part at or above 2^256.
- `TestGenerate_ReadsExactly32BytesFromCryptoRand` (ID-T27, injected reader) and
  `TestTokenPrefix`.

Integration tests (PostgreSQL 18, real HTTP chain on `127.0.0.1:0`, steerable clock, fake
`PersonalTokenSwitch`):

- `TestIDT23_OnlyHashStored`: after `createToken` the full secret appears in no row of
  `api_tokens`, `idempotency_keys` and `audit_events`; `token_hash` is the HMAC.
- `TestIDT23_ShownOnce`: `listTokens` never returns the secret; a replay with the same
  `Idempotency-Key` gives `409 conflict` with `reason: one_time_secret`.
- `TestIDT23_WrongChecksumRejectedWithoutLookup` (repository fake counts calls),
  `TestIDT23_ExpiredRejected`, `TestIDT23_RevokedRejected`.
- `TestIDT23_DPoPBoundAgentTokenWithoutProof`: `Bearer` gives `401 invalid_dpop_proof`; a proof
  with another key gives `401`; a valid proof succeeds on `listTokens` for the agent's own
  principal.
- `TestCreateToken_AgentWithoutJKT400`, `TestCreateToken_ExpiryRequiredAndBounded` (both maxima),
  `TestCreateToken_ScopesRejectedForAgentAndPersonal`, `TestCreateToken_AppPrincipal409`,
  `TestCreateToken_StepUpRequired` (owner session without fresh step-up gives `401
  step_up_required`).
- `TestCreateToken_PersonalDisabled403` (`feature_disabled`) and
  `TestCreateToken_PersonalForOtherPerson403` (`not_self`).
- `TestAZM35_PersonalTokenCannotStepUp`: Sam's personal token calling `createToken` for Sam gives
  `403` with reason `step_up_not_possible`.
- `TestPersonalTokenOfOwnerHasNoOwnerRights`: the owner's personal token calling `createToken` for
  an agent gives `403` with reason `owner_only`.
- `TestRevokeToken_Immediate`: the next request with the token gives `401`; a second `DELETE`
  gives `204`; a token of another principal without permissions gives `404`;
  `permission_version` raised and audit entry written.
- `TestRevokeAllForPrincipal` and `TestLastUsedGranularity`.
- `TestAPITokenNeverLogged`: log and audit capture contain no secret.
- Route count (AZ-P1) and response validation pass with the three operations.

Python tests (`tools/ci/tests/test_gitleaks_rules.py`):

- `test_bothy_token_with_valid_checksum_is_detected`: builds a token at runtime with
  `secrets.token_bytes(32)`, Base62 and `zlib.crc32`, and asserts the rule fires.
- `test_checksum_vectors_match_go`: recomputes every vector of the Go `testdata` file in Python.
- The existing sample for `bothy-api-token` keeps passing with the 6-character checksum.

Commands that must pass:

```sh
cd services/server && go test -race ./internal/identity/... ./internal/authz/... ./internal/kernel/...
cd tools && uv run --locked pytest ci/tests/test_gitleaks_rules.py
make gitleaks
make codegen-check
make arch
make api-lint
make adr-index
make docs
make check
```

## Acceptance criteria

- [ ] Preconditions verified and named in the report.
- [ ] ID-T23 green in all its parts; T-TK-02 in the threat model names the tests.
- [ ] Format exactly as specified; Go and Python agree on the checksum vectors; the gitleaks rule
      detects a runtime token and requires 6 checksum characters.
- [ ] No secret in the database, the idempotency store, audit, logs or problem bodies.
- [ ] Expiry mandatory and bounded; agent tokens DPoP-bound by default; `Bearer` with a bound token
      rejected.
- [ ] Personal tokens: switch respected (`403 feature_disabled`), only for oneself, no owner
      permissions, no step-up (AZ-M35).
- [ ] Revocation effective on the next request; `RevokeAllForPrincipal` available for M1-45 and
      M1-51.
- [ ] The implementation matches ADR 0061, 03, 06, 08 and `openapi.yaml` (changed only where proven
      wrong); `operations.yaml` and DECISIONS updated; `make check` green.

## Definition of done

- Code and tests as above; every public symbol has a doc comment (purpose, parameters, return
  value, errors, example where useful); comments explain why.
- `internal/identity/README.md` current (token format, port `Tokens`, authentication path);
  CHANGELOG entry under "Unreleased"; ADR 0061 current; `docs/DECISIONS.md` entries.
- `make check` green.
- Independent review (`make review` and a code review over the diff) and the red team gate over
  the diff, verdict recorded in the report; red team item 4 of the M1 section (personal token with
  owner permissions, step-up with a token without session) applies directly.
- Merge locally into the base branch only if review and red team are green and `make check` is
  green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then the
commits, the files with line counts, the test results with counts (including the red runs), what
is not proven, and follow-ups that could not be done in scope with the reason (at least: the
`PersonalTokenSwitch` rebinding by M1-47, app tokens in M2-24). Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or
`exit=1 <reason>`.
