# M1-36: identity: identity provider port and provider management (ADR 0019)

Status: ready | Depends on: M1-31, M1-35 | Etappe: M1

## Goal

After this ticket the `identity` module knows its sign-in drivers through one port: the tables
`identity_providers` and `user_identities`, the `ExternalIdentity` that every driver produces, the
mapping of an external identity to a person only through `(issuer, subject)`, linking and
unlinking, and the owner's management of providers with step-up. OIDC (M1-41), the trusted proxy
header (M1-43), local accounts (M1-39) and enrollment (M1-37) plug into this port instead of
inventing their own account mapping, which closes the account takeover through e-mail matching
(T-ID-01, R-ID-02).

## Read first

- `docs/spec/08-identitaet.md`: "2.1 Port", "2.2 OIDC" (what the OIDC driver will need from the
  config), "2.3 Beispiel Microsoft Entra ID" (`tid` allow list, `sub` as subject), "2.4
  Vertrauenswürdiger Proxy-Header" (login header), "2.5 Lokale Konten", "3.1 Zuordnung einer
  externen Identität", "3.3 Weitere Identität verknüpfen und lösen", "3.4 Gruppen aus Claims",
  "15. Tests" (ID-T04), "16. Entscheidungen dieses Kapitels" (R-ID-02).
- `docs/spec/03-datenmodell.md`: "1. Grundsätze" (items 1, 3, 6, 8, 9, 12, 13), "4.2" tables
  `identity_providers`, `user_identities`, `auth_sessions` (`identity_id`), `auth_login_flows`
  (FK to providers), `users`, `devices`.
- `docs/spec/05-aktionen.md`: "4.2" (`identity_provider.read`, `identity_provider.manage`),
  "4.3" (`identity.read`, `identity.link`, `identity.unlink`), "2.4", "2.5".
- `docs/spec/07-rechte.md`: "4.1", "4.2", "4.3 Sichtbarkeit", "SR5".
- `docs/spec/06-api.md`: "3.2 Anmeldeabläufe", "5. Fehler", "7. Idempotenz" (DELETE rules),
  "16. Endpunkte" (Admin, Me, Users).
- `docs/spec/02-architektur.md`: "4.1 Regeln", "5. Schichten im Modul", "8. Transaktionen und
  Unit of Work", "9.2 404 statt 403".
- `docs/spec/19-sicherheit.md`: "6.1" (SEC-IN-1, SEC-IN-2, SEC-IN-4), "10. Geheimnisse" (OIDC
  client access only as `secret_ref`).
- ADR 0019 (`docs/adr/0019-identitaetsanbieter-oidc-proxy-header-lokale-passkey-konten.md`),
  ADR 0011, ADR 0017.
- `docs/api/openapi.yaml`: operations `listIdentityProviders`, `createIdentityProvider`,
  `getIdentityProvider`, `updateIdentityProvider`, `deleteIdentityProvider`, `getMyCredentials`,
  `getUserCredentials`, `unlinkIdentity`; schemas `IdentityProvider`, `IdentityProviderCreate`,
  `IdentityProviderUpdate`, `IdentityProviderList`, `IdentityProviderKind`, `AuthProviderInfo`,
  `Identity`, `Passkey`, `CredentialOverview`, `Problem`.
- `docs/security/threat-model.md`: T-ID-01, T-ID-02.
- Merged code of M1-09 (secret reference syntax of `internal/platform/secrets`), M1-20
  (`Idempotency-Key` middleware), M1-26 (principals, users, display-text check), M1-31 (people,
  owners, `identity` HTTP adapter), M1-33 (devices), M1-34 (`auth_sessions`), M1-35 (step-up),
  M1-24 (audit), M1-28 (`authorize`).
- `services/server/internal/identity/README.md`, `docs/quality/gates.md`, `CONTRIBUTING.md`,
  `docs/PLAN.md` "3. Arbeitsweise je Ticket".

## Fixed requirements that apply

1. No MVP. Built from scratch directly in its final state; etappen are only the build order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined
   interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface has a docstring or
   doc comment (purpose, parameters, return value, errors, example where useful); comments explain
   why, not what. API documentation is generated from code (OpenAPI from the routes, dartdoc,
   Python API docs in the docs site).
4. Documentation in the repo: README per module/service, an ADR for every architecture decision,
   a CHANGELOG entry per release, docs site kept current. A change without matching documentation
   is not done.
5. Quality gates (SonarQube, linters, type checks, security scanners, test coverage) block the
   merge.
6. Test first: every behaviour starts with a failing test (red output goes into the report).
7. No en dash and no em dash in any text you write.
8. White label: only example.com, example.org and the names Alex, Sam, Robin in examples; provider
   names in tests like "Company SSO".
9. Configuration values only through named keys from `docs/operations/configuration.md`.
10. Drivers deliver an `ExternalIdentity` with exactly the fields of 08 "2.1"; everything after it
    is the same for all drivers (08 "2.1").
11. At most one `trusted_header` and one `local` driver per instance, any number of `oidc`
    (08 "2.1", 03 unique index).
12. Mapping only through `(provider_id, subject)` with `unlinked_at IS NULL`, for OIDC also
    `issuer`; never through e-mail or display name (08 "3.1", R-ID-02, ID-T04).
13. Provider management is owner only with step-up and audit (05 "4.2"); reading is owner only.
14. OIDC client secrets or keys only as `secret_ref` (03 `identity_providers`, 19 "10").
15. `identity.unlink` fails when no sign-in method remains: no identity, no passkey, no active
    device for device pairing (08 "3.3"); self or owner, step-up, audit (05 "4.3").
16. `identity.read` lists identities, passkeys and local credentials without secrets; self or
    owner (05 "4.3").

## Scope

1. Migration `services/server/migrations/<next free number>_identity_providers.sql`: tables
   `identity_providers` and `user_identities` exactly as 03 "4.2" (columns, defaults, CHECKs,
   FKs, unique `lower(name)`, partial unique `(kind) WHERE kind IN ('trusted_header','local')`,
   partial unique `(provider_id, subject) WHERE unlinked_at IS NULL`, index `(user_id)`), plus
   `ALTER TABLE auth_sessions ADD CONSTRAINT ... FOREIGN KEY (identity_id) REFERENCES
   user_identities(id) ON DELETE SET NULL` (deferred from M1-34 because the table did not exist).
2. sqlc queries `internal/identity/adapters/postgres/queries/providers.sql` and `identities.sql`,
   repository.
3. Port `internal/identity/providers.go` (names fixed):
   - `ProviderKind` (`oidc`, `trusted_header`, `local`).
   - `ExternalIdentity{ProviderID ID; Issuer string; Subject string; DisplayName string;
     EmailHint string; Groups []string; AuthTime time.Time; AMR []string}` with the doc comments of
     08 "2.1" (Issuer empty for `trusted_header` and `local`; EmailHint and DisplayName only for
     display).
   - `Provider` (members of the API schema `IdentityProvider`), `CreateProvider`,
     `UpdateProvider` (pointer fields for PATCH semantics), `ProviderInfo{ID, Kind, Name}`.
   - `type Providers interface{ List(ctx, actor.Actor) ([]Provider, error); Create(ctx,
     actor.Actor, CreateProvider) (Provider, error); Get(ctx, actor.Actor, ID) (Provider, error);
     Update(ctx, actor.Actor, ID, UpdateProvider) (Provider, error); Delete(ctx, actor.Actor, ID)
     error; Enabled(ctx, ID) (Provider, error); ListForDiscovery(ctx) ([]ProviderInfo, error) }`.
     `Enabled` returns an enabled provider or `not_found` and is what login drivers call;
     `ListForDiscovery` feeds `getDiscovery` (M1-46).
   - `Resolution{Outcome ResolutionOutcome; UserID ID; IdentityID ID}` with outcomes `matched` and
     `unmatched`.
   - `type Identities interface{ Resolve(ctx, ExternalIdentity) (Resolution, error); Link(ctx,
     userID ID, ExternalIdentity) (Identity, error); MarkLogin(ctx, identityID ID, at time.Time)
     error; Credentials(ctx, actor.Actor, userID ID) (CredentialOverview, error); Unlink(ctx,
     actor.Actor, userID, identityID ID) error }`. `Resolve`, `Link` and `MarkLogin` join the
     ambient unit of work of the calling login flow.
4. App-level extension points in `internal/identity/app` (unexported types are fine, exported
   only inside the module):
   - `ProviderKindRules` per kind: `Validate(raw json.RawMessage) (json.RawMessage, error)`
     returns the normalized config. M1-43 adds the loopback check of W4 for `trusted_header` to
     this interface; M1-41 uses the OIDC rules.
   - `CredentialSource` per credential kind: contributes to `CredentialOverview` and counts
     remaining sign-in methods for `Unlink`. This ticket registers the sources "identities" and
     "active devices"; M1-39 (local password, TOTP) and M1-40 (passkeys) register theirs.
5. Config validation per kind in `internal/identity/domain/provider` (pure, strict decoding:
   unknown members, duplicate keys and JSON `null` rejected), schemas named as in 03:
   - `identity_provider.oidc.v1`: `issuer` (required; absolute `https` URL without query and
     fragment), `client_id` (required, non-empty), `client_auth` (required: `private_key_jwt` or
     `client_secret_basic`, 08 "2.2" step 5), `secret_ref` (required; a valid secret reference of
     the secret provider holding the private key or the client secret; validated for syntax
     only, never resolved here), `allowed_tenant_ids` (optional list of non-empty strings; required
     and non-empty when the issuer path contains `/common/` or `/organizations/`, 08 "2.3"),
     `groups_claim` (optional claim name; absent means no group sync, 08 "3.4").
   - `identity_provider.trusted_header.v1`: `login_header` (required, an HTTP field name token
     per RFC 9110), `name_header` (optional, same rule).
   - `identity_provider.local.v1`: empty object.
   Secret reference syntax is checked through a small port `SecretRefs{ Valid(ref string) error }`
   implemented in `internal/wire` with the parser of `internal/platform/secrets` (the domain and
   app may not import `platform`).
6. App use cases `internal/identity/app/providers.go`, `identities.go`; adapters
   `internal/identity/adapters/httpapi/providers.go`, `credentials.go` for the eight operations;
   enable them in `services/server/api/operations.yaml` and regenerate (command from
   `services/server/README.md`).
7. Docs: `docs/spec/08-identitaet.md` gets a new subsection "2.6 Konfiguration je Treiber" with
   the three schemas above, and "3.3" gets the exact unlink rule and the rule that unlinking does
   not end sessions; rulings in `docs/DECISIONS.md`; identity README;
   `services/server/CHANGELOG.md`.

## Out of scope

- The drivers themselves: OIDC flows and discovery fetch (M1-41), group sync (M1-42), header
  login and the W4 loopback check at `identity_provider.manage` (M1-43), local password and TOTP
  (M1-39), passkeys (M1-40).
- 08 "3.1" step 2 (link through an enrollment code, M1-37) and step 3 (provisioning as `pending`
  when `allow_provisioning`, M1-41): `Resolve` returns `unmatched` and the caller applies them.
- The neutral denial message and the `deny` audit of a login (`auth.login.complete`) belong to
  the login tickets; this ticket provides the `unmatched` outcome they act on.
- `beginIdentityLink`, `completeIdentityLink` (M1-41); `revokePasskey` (M1-40).
- Discovery output (M1-46) beyond `ListForDiscovery`.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-36-identity-providers`.
- Conventional Commits with scope `identity` (or `docs` where fitting), each signed off
  (`git commit -s`).
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope above except `services/server/CHANGELOG.md`,
  `docs/DECISIONS.md` and generated references.
- Follow the host rules supplied by the launcher.
- Tests bind only `127.0.0.1` and port 0; PostgreSQL 18 as a throwaway container via the M1-12
  helper; no Docker runtime means failure, never a skip.
- Stop only processes you started, by PID; temporary directories only via `mktemp -d`, removed at
  the end.
- Never read or print secrets; secret references in tests point to nothing real.
- No network calls except package downloads for pinned tools; no call to any identity provider.
- A gap in the core contract ends the ticket with `Verdict: BLOCKED` (P-11).

## Design notes

Provider management (actions from 05; `authorize` with resource `instance`):

| Operation | Action | Rules |
|---|---|---|
| `listIdentityProviders`, `getIdentityProvider` | `identity_provider.read` | owner only (non-owners `403 forbidden` reason `owner_only`); unknown id `404` |
| `createIdentityProvider` | `identity_provider.manage` | owner, step-up, `Idempotency-Key` (M1-20), audit; `201` |
| `updateIdentityProvider` | `identity_provider.manage` | `kind` cannot change; absent members stay; `config`, if given, is validated against the stored kind; `updated_at = now`; `200` |
| `deleteIdentityProvider` | `identity_provider.manage` | `409 conflict` with `hint` "disable the provider instead" while any `user_identities` row references it (FK `RESTRICT`, also unlinked rows); otherwise the row is deleted (`auth_login_flows` cascade); `204`; a repeated delete is `404` because providers have no tombstone |

- `name` is trimmed, non-empty, passes the display-text check of M1-26 (SEC-IN-4); a second
  provider with the same name ignoring case gives `409 conflict`; a second `trusted_header` or
  `local` provider gives `409 conflict`. Map the unique violations of PostgreSQL to these
  conflicts; never leak constraint names.
- Validation errors are `400 validation_failed` with `errors[]` pointers into `/config/...`.
- `enabled = false` stops new sign-ins through the provider (the drivers call `Enabled`); existing
  sessions continue until they end or an owner revokes them. Record as ruling (the spec does not
  say more).
- Responses never contain secret values; `config` contains `secret_ref` names only, exactly as
  stored after normalization. Audit details name the provider id, kind and the names of changed
  members, never config values.
- No event type exists for providers (02 "4.7"); do not invent one.

Mapping (`Resolve`, 08 "3.1" steps 1 and 4):

1. The provider must exist and be enabled; otherwise `unmatched`.
2. Look up `user_identities` by `provider_id` and `subject` with `unlinked_at IS NULL`; for
   `oidc` additionally `issuer` must equal the stored `issuer` (a multi-tenant provider has one
   issuer per tenant).
3. Found: `matched` with user and identity. The user's status is not decided here; `authorize`
   (`principal_inactive`) and the login tickets handle it.
4. Not found: `unmatched`. `EmailHint`, `DisplayName`, `users.email` and `email_at_link` are never
   read for matching. A test proves that a known e-mail with an unknown subject stays `unmatched`
   (ID-T04).

Linking (`Link`): provider enabled; insert a row with `issuer` only for `oidc`, `email_at_link` =
`EmailHint` if not empty, `linked_at = now`; an already linked subject (active row) gives
`409 conflict`; a subject linked before and unlinked since may be linked again. The caller writes
the `identity.link` audit entry.

Credential overview (`getMyCredentials` resource `user` = caller, `getUserCredentials` resource
`user` = path id; action `identity.read`; self or owner; others and unknown ids `404`; a pending
device `403 device_pending`): `identities` are the active rows with `provider_kind` from the
provider, `issuer` and `email_at_link` only when set, `last_login_at` when set; `passkeys` is
`[]` and `local_password_set` and `totp_enrolled` are `false` until M1-40 and M1-39 register their
sources; these values are correct while those tables do not exist.

Unlink (`unlinkIdentity`, action `identity.unlink`, resource `user` = path id; self or owner,
step-up, audit):

1. Load the identity by `user_id` and `identity_id`; missing or not visible `404` (same body).
2. Already unlinked: `204` without change (DELETE semantics of 06 "7").
3. Count the sign-in methods that remain after unlinking, through every registered
   `CredentialSource`: active identities at enabled providers other than this one; active devices
   of the person (device pairing, 08 "5.3"); later passkeys. Zero gives `409 conflict` with
   `reason` `no_login_method_left` and a `hint`; the person keeps the identity.
4. Otherwise set `unlinked_at = now`; audit allow; `204`. Sessions started with this identity
   continue (`auth_sessions.identity_id` stays); ending them is `session.revoke` (M1-45). Record
   as ruling.

Concurrency: two unlinks of different identities of one person in parallel must not both pass the
count. Lock the person's active `user_identities` rows `FOR UPDATE` in ascending `id` (03 "6.3")
before counting.

## Tests (write them first)

Unit tests:

- `TestProviderConfig_OIDC`: valid config normalized; missing `issuer`, `client_id`,
  `client_auth`, `secret_ref`; `http` issuer; issuer with query; inline `client_secret` member;
  unknown member; `null` value; duplicate key; invalid secret reference syntax;
  `/organizations/` issuer without `allowed_tenant_ids`.
- `TestProviderConfig_TrustedHeader`: missing `login_header`; header name with space or colon.
- `TestProviderConfig_Local`: `{}` accepted; any member rejected.
- `TestResolve_IDT04_EmailNeverMatches`: person with `users.email` equal to `EmailHint` and a
  different subject: `unmatched`.

Integration tests against PostgreSQL 18 with real Authenticator (M1-34), real step-up (M1-35),
real `authz` and audit:

- `TestProvidersMigration_Constraints`: unique name ignoring case, one `trusted_header`, one
  `local`, many `oidc`; partial unique subject; FK `auth_sessions.identity_id` present with
  `ON DELETE SET NULL`.
- `TestProviders_OwnerWithStepUp`: create, read, update, delete as owner with fresh step-up.
- `TestProviders_WithoutStepUp`: manage operations `401 step_up_required`; read works.
- `TestProviders_NonOwner`: `403 forbidden`.
- `TestProviders_SecondTrustedHeaderOrLocal`: `409 conflict`.
- `TestProviders_KindImmutable` and `TestProviders_ConfigValidatedOnUpdate`.
- `TestProviders_DeleteWithIdentities`: `409 conflict`; without identities `204`; again `404`.
- `TestProviders_Idempotency`: same `Idempotency-Key` returns the stored `201`.
- `TestProviders_AuditNoConfigValues`: audit details without config values.
- `TestResolve_MatchedAndIssuer`: same subject with a different issuer is `unmatched`; disabled
  provider `unmatched`; unlinked identity `unmatched`.
- `TestLink_Conflicts`: active subject twice `409`; relink after unlink allowed.
- `TestCredentials_SelfOwnerOther`: self and owner `200`, other person and unknown id `404`,
  pending device `403 device_pending`, no `null` members, `passkeys` empty.
- `TestUnlink_LastMethod`: only identity and no active device: `409 conflict` reason
  `no_login_method_left`; with an active device: `204`; with a second identity at a disabled
  provider only: `409`; already unlinked: `204`; step-up missing: `401 step_up_required`; owner
  unlinks for another person: `204`; non-owner for another person: `404`.
- `TestUnlink_ConcurrentLastTwo`: a person with exactly two identities and no device; two parallel
  unlinks: exactly one succeeds.
- `TestListForDiscovery`: only enabled providers, only id, kind, name.

Commands that must pass:

```sh
cd services/server && go test -race ./internal/identity/...
make arch
make codegen-check
make migrations-lint
make docs
make check
```

## Acceptance criteria

- [ ] Migration creates `identity_providers` and `user_identities` exactly as 03 and adds the FK
      on `auth_sessions.identity_id`.
- [ ] `ExternalIdentity`, `Providers` and `Identities` exist with the names of "Scope".
- [ ] Config schemas of the three kinds are enforced strictly and documented in 08 "2.6"; secrets
      only as `secret_ref`, never resolved or returned as values.
- [ ] At most one `trusted_header` and one `local` provider; names unique ignoring case.
- [ ] Provider management only for owners with step-up, audited; reading only for owners.
- [ ] ID-T04 green: mapping never uses e-mail or display name.
- [ ] Unlink refuses to remove the last sign-in method, also under concurrency.
- [ ] Credential overview conforms to `CredentialOverview` without `null` members.
- [ ] Rulings in DECISIONS.md (provider disable keeps sessions, unlink keeps sessions, codes).
- [ ] `make check` green.

## Definition of done

Code, tests and docs complete: identity README (status, ports `Providers` and `Identities`,
extension points, tables, actions, test ids), a doc comment on every public symbol, comments
explain why, ADR 0019 stays as is unless you take an architecture decision beyond it, the
CHANGELOG entry under Unreleased of `services/server/CHANGELOG.md`, `make docs` green with the new
08 subsection. `make check` green. Independent review (`make review` plus a code review of the
diff) and red team gate (`docs/quality/red-team-gate.md`) over the diff with both verdicts in the
report; merge locally into the base branch only if both are clean and `make check` is green; then
`git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then the
commits, the files with line counts, the test results with counts (including the red output of
the first failing tests), what is not proven, and follow-ups that could not be done in scope, each
with its reason. Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
