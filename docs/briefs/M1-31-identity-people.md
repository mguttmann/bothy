# M1-31: People, owners and own profile

Status: ready | Depends on: M1-20, M1-29 | Etappe: M1

## Goal

After this ticket an owner can create people, change their profile, activate pending or disabled people, list and read the directory, appoint further owners and remove owners, and every person can read their own principal and maintain their own profile. There is no self registration. Owner and status changes are rights changes: they run through the hook of M1-29, so I1 and SR8 hold for every sequence of these actions, and a second owner is impossible while an agent runs on a personal subscription. Ten REST operations become reachable.

## Read first

- `docs/PLAN.md`: section 3, section 7 (M1), rows M1-26, M1-29, M1-31, M1-37, M1-45, M1-46.
- `docs/spec/05-aktionen.md`: 2.3 (owner only in a session), 2.4, 2.5, table 4.2 (`owner.add`, `owner.remove`), table 4.3 (`self.read`, `user.read`, `user.create`, `user.update`, `user.enable`, `self.profile.update`), 6 (`user.updated`), 8 (R-AZ-03, R-AZ-14).
- `docs/spec/07-rechte.md`: 2.7 (Owner), 3 (Prinzipal-Kontext), 4.1, 4.2 step 8 (step-up), 5 (SR1, SR5, SR7, SR8), 11.1 (AZ-M23, AZ-M26, AZ-M35, AZ-M36).
- `docs/spec/08-identitaet.md`: 3.1 (provisioning as `pending`), 3.2 (Einladungs-Code), 4 (Erstinstallation), 11 (Person sperren, for contrast), 18.
- `docs/spec/03-datenmodell.md`: 1 (principles 6, 7, 13), 4.2 `principals`, `users`, `owners`, 9 (`principal.handle_max_chars`).
- `docs/spec/10-backend-richtlinie.md`: 3 (consequence of I1 for owners), 5.1 (row `owner.add`, `owner.remove`, `user.enable`), 5.4.
- `docs/spec/06-api.md`: 2 (Grundregeln), 3.3 (Step-up), 4, 5 (Fehler), 6 (Paginierung), 7 (Idempotenz), 16 (Me, Users).
- `docs/spec/19-sicherheit.md`: SEC-IN-1, SEC-IN-2, SEC-IN-4, SEC-LOG-4, SEC-T07.
- `docs/api/openapi.yaml`: operations `listUsers`, `createUser`, `getUser`, `updateUser`, `enableUser`, `listOwners`, `addOwner`, `revokeOwner`, `getMe`, `updateMyProfile`; schemas `User`, `UserCreate`, `UserCreated`, `UserUpdate`, `UserPage`, `UserStatus`, `ProfileUpdate`, `Me`, `PrincipalRef`, `Owner`, `OwnerAdd`, `OwnerList`, `ReasonBody`, `Problem`, `InvariantViolation`, `UserUpdatedEvent`, `UserPayload`.
- ADRs: 0015, 0017, 0019, 0020, 0021, 0053.
- Merged code: `internal/identity` with `principals`, `users`, `owners`, `Directory` and `PrincipalSource` (M1-26); the rights-change hook, `InvariantError` with `conflicts` and the problem extension (M1-29); `authorize` and the source ports (M1-28); the HTTP chain, problem writer, test authenticator if any (M1-18); idempotency middleware (M1-20); the generated strict server and `services/server/api/operations.yaml` (M1-17). Their names win where they differ from this brief only in naming.
- `docs/quality/gates.md`, `CONTRIBUTING.md`.

## Fixed requirements that apply

1. No MVP. Built from scratch directly in its final state; etappen are only the build order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface has a docstring or doc comment (purpose, parameters, return value, errors, example where useful); comments explain why, not what. API documentation is generated from code (OpenAPI from the routes, dartdoc, Python API docs in the docs site).
4. Documentation in the repo: README per module/service, an ADR for every architecture decision, a CHANGELOG entry per release, docs site kept current. A change without matching documentation is not done.
5. Quality gates (SonarQube, linters, type checks, security scanners, test coverage) block the merge.
6. Test first: red before green, red output kept for the report.
7. No en dash or em dash in any file; White Label (Alex, Sam, Robin, `example.com`).
8. Configuration values only through named keys listed in `docs/operations/configuration.md`; this ticket adds none.
9. No self registration: people are created only by an owner (`user.create`) or, later, by provisioning and bootstrap (08 sections 3.1 and 4).
10. `user.create`, `user.update`, `user.enable`, `owner.add`, `owner.remove` are owner actions with step-up and audit (05 tables 4.2 and 4.3); owner rights only in a session, never through a personal token (SR5, R-AZ-14).
11. `owner.add`, `owner.remove`, `user.enable` are rights changes (column Rechte = ja): `permission_version`, `access.changed`, I1 and SR8 in the same transaction (05 rules 1.6 and 1.7). `owner.add` while an agent has `billing_mode = 'personal'` fails with `409 i1_violation` (07 SR1 consequence, AZ-M23); the last active owner stays (`409 last_owner`, SR8).
12. `user.enable` covers `pending` to `active` and `disabled` to `active` (05 table 4.3; 08 section 3.1).
13. Display texts with control or bidi characters are rejected (SEC-IN-4, SEC-T07); handles follow 03 `principals`.
14. No JSON `null` in responses and events (ADR 0053); every response matches its schema.

## Scope

### 1. Port (root package `internal/identity`)

Extend the ports from M1-26 with DTOs (no domain types in the port):

- `Directory`: `ListUsers(ctx, actor, ListUsersQuery{Q, Status, Cursor, Limit}) (UserPage, error)`, `GetUser(ctx, actor, userID) (User, error)`, `CreateUser(ctx, actor, CreateUserCmd) (User, error)`, `UpdateUser(ctx, actor, userID, UpdateUserCmd) (User, error)`, `EnableUser(ctx, actor, userID, reason) (User, error)`, `GetMe(ctx, actor) (Me, error)`, `UpdateMyProfile(ctx, actor, ProfileCmd) (User, error)`.
- `Owners`: `ListOwners(ctx, actor) ([]OwnerEntry, error)`, `AddOwner(ctx, actor, userID, reason) (OwnerEntry, error)`, `RevokeOwner(ctx, actor, userID, reason) error`.

### 2. Use cases (`internal/identity/app`, files `people.go`, `owners.go`, `me.go`)

Each use case loads the resource, calls `authorize`, runs in one unit of work for writes, writes the audit entry through `auditing.Recorder` where column Audit = ja, and appends `user.updated` where listed.

| Operation | Use case | Action and resource | Rights change | Event |
|---|---|---|---|---|
| `listUsers` | `ListUsers` | `user.read` on instance | no | none |
| `createUser` | `CreateUser` | `user.create` on instance | no | `user.updated` |
| `getUser` | `GetUser` | `user.read` on the user | no | none |
| `updateUser` | `UpdateUser` | `user.update` on the user | no | `user.updated` |
| `enableUser` | `EnableUser` | `user.enable` on the user | yes, affected = the user | `user.updated` |
| `listOwners` | `ListOwners` | `user.read` on instance | no | none |
| `addOwner` | `AddOwner` | `owner.add` on the user | yes, affected = the user | `user.updated` |
| `revokeOwner` | `RevokeOwner` | `owner.remove` on the user | yes, affected = the user | `user.updated` |
| `getMe` | `GetMe` | `self.read` on the caller | no | none |
| `updateMyProfile` | `UpdateMyProfile` | `self.profile.update` on the caller | no | `user.updated` |

Rules per use case:

- `CreateUser`: new principal (UUIDv7, `kind = 'user'`, `status = 'active'`) and `users` row. `handle` matches `^[a-z0-9][a-z0-9._-]*$`, at most `principal.handle_max_chars`, unique ignoring case (`409 conflict` on collision). `display_name` not empty after trimming and free of control and bidi characters (reuse the validation of M1-26). `email` optional, unique ignoring case (`409 conflict`). `locale` optional, a well-formed BCP 47 tag parsed with `golang.org/x/text/language` and stored in canonical form; when absent the column default of 03 applies. `time_zone` optional, an IANA name accepted by `time.LoadLocation`, never `Local` or empty; the identity domain imports `time/tzdata` because the server image has no zoneinfo. `local_account: true` is answered with `400 validation_failed`, pointer `/local_account`, until M1-37 adds enrollment codes (record this interim rule in `docs/DECISIONS.md` with the note that M1-37 replaces it). Response `201 UserCreated` without `enrollment`.
- `UpdateUser`: fields of `UserUpdate`, same validation; absent fields stay; the handle never changes.
- `EnableUser`: `pending` or `disabled` to `active`, `disabled_at` cleared; `deleted` is `409 invalid_state_transition`; an already active person returns `200` with the unchanged user and an allow audit entry, but no rights change and no event. Enabling never restores devices or sessions revoked by `user.disable` (08 section 11).
- `AddOwner`: the user must be an active principal of kind `user` (`404` if no such user, `409 conflict` if not active or already an owner); insert into `owners` (`granted_by` = caller, `reason`); declare the rights change; I1 and SR8 are verified by the hook, which answers `409 i1_violation` with `conflicts`. Response `201 Owner`.
- `RevokeOwner`: `404` if the user has no active owner row; set `revoked_at`, `revoked_by`; declare the rights change; `409 last_owner` from the hook if no active owner remains, also when owners remove themselves. Response `204`.
- `GetMe`: for humans, agents and apps (an agent and an app read themselves). `principal` from `principals`; `is_owner` true only for a principal with an active owner row; `email`, `locale`, `time_zone` only for humans; `permission_version` = the version the request was authorized with; `session_id` and `device_id` from the actor when present; `step_up_valid_until` = `step_up_at` plus `auth.step_up_max_age` only when that lies in the future. Allowed for a pending device (05 section 2.5).
- `UpdateMyProfile`: `display_name`, `locale`, `time_zone` of the calling human, same validation; no step-up, no audit (05 table 4.3).
- `ListUsers`: every status unless `status` is given; `q` is a case-insensitive prefix match on handle or display name (escape `%` and `_`); keyset order `(lower(handle), id)` ascending; `limit` defaults to `api.page_size_default`, larger values are capped at `api.page_size_max`; opaque cursor bound to the filters, a foreign or stale cursor is `400 cursor_invalid`; no totals (06 section 6). Use the cursor helper of the merged tree if one exists.
- `ListOwners`: active owners ordered by `granted_at`, unpaginated (06 section 6 point 6).
- `GetUser`: an ID that is not a human (agent, app, unknown) is `404`.

### 3. Persistence (`internal/identity/adapters/postgres`)

sqlc queries in `queries/people.sql` and `queries/owners.sql`; no other module reads these tables (03 principle 12). Owner rows locked in ascending `id`.

### 4. HTTP (`internal/identity/adapters/httpapi`)

Implement the ten methods of the generated strict server interface; map DTOs to the generated types; no logic beyond mapping. Move the ten operation IDs from the section `later` into the built section of `services/server/api/operations.yaml` (P-04), so that the route test of AZ-P1 counts them.

### 5. Event `user.updated`

Envelope per 04 section 2.2: `resource = {kind: user, id}` (no `version`, `users` has none), empty `scope`, actor from the request, payload `UserPayload{user: User}` with the state after the change, including `is_owner`. Receive action `user.read` (05 section 6).

### 6. Audit details

IDs and field names only, never values of names, e-mail addresses or locales (SEC-LOG-4; 03 `audit_events.details`): `user.create` `{}`, `user.update` `{changed_fields}`, `user.enable` `{from_status, reason}`, `owner.add` `{reason}`, `owner.remove` `{reason}`.

### 7. Documentation

`services/server/internal/identity/README.md` (operations, use cases, events, rules), `docs/DECISIONS.md` (rulings below and the interim rule for `local_account`), `services/server/CHANGELOG.md` under Unreleased.

## Out of scope

- `disableUser` and the revocation of devices, sessions and tokens: M1-45.
- Enrollment codes, `local_account`, `createEnrollmentCode`: M1-37.
- Applying the instance setting `locale.default` to new people: M1-46 wires it; until then the column default of 03 (`de`), which equals the documented default of `locale.default`, applies.
- Avatars (`setMyAvatar`, `deleteMyAvatar`): M3.
- Identities, passkeys and credentials of a person: M1-36, M1-40.
- `resolve_i1` on `addOwner`: M2-21.
- Real authentication with DPoP and sessions: M1-34 and M1-35. This ticket tests with a test authenticator.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-31-identity-people`, created from the base branch named by the launcher. One writer per worktree.
- Conventional Commits with the module as scope (for example `feat(identity): manage people and owners`) and a DCO sign-off on every commit (`git commit -s`). Squash into meaningful commits before the merge.
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope of this brief, except `docs/DECISIONS.md`, `services/server/CHANGELOG.md` and the generated files named here.
- Follow the host rules supplied by the launcher. Tests bind only `127.0.0.1` and port 0 (P-10); PostgreSQL through `pgtest` (P-09), never skipped.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets. No network calls except package downloads for pinned tools.
- No gate may be weakened, no threshold lowered, no test deleted or skipped.

## Design notes

- Step-up, owner-only and session-only checks are decisions of `authorize` (07 section 4.2 steps 6 and 8); the use cases never inspect the actor for them.
- Production wiring: no agents and no structure exist yet, so the empty sources of M1-28 and M1-29 are wired; I1 is exercised in tests with a fake `AgentPolicySource` and a fake `StructureSource` (plan row: AZ-M23 with a fake agent source).
- Ruling: `createUser` emits `user.updated`, because the catalogue has no `user.created` and clients treat the event as an upsert of a directory entry.
- Ruling: `addOwner` for a non-active person is `409 conflict`; an owner must be able to act at once.
- Ruling: `enableUser` on an active person is an audited no-op with `200`.
- Ruling: `revokeOwner` for a person without an active owner row is `404`, because the owner role is the resource.
- Problem codes: `400 validation_failed` with `errors[]`, `400 cursor_invalid`, `401 unauthenticated`, `401 step_up_required`, `403 forbidden` (`owner_only`, `step_up_not_possible`, `subject_kind_not_allowed`), `404 not_found`, `409 conflict`, `409 invalid_state_transition`, `409 i1_violation`, `409 last_owner`, `422 idempotency_key_reuse` (M1-20).
- Response validation: use the response validator of the merged tree if one exists; otherwise add a test-only helper in `services/server/test/apitest` that validates each response body against its component schema in `docs/api/openapi.yaml` with `github.com/santhosh-tekuri/jsonschema/v6` (Apache-2.0, JSON Schema 2020-12 as used by OpenAPI 3.1), pinned in `go.mod`, checked by `make licenses`.
- Test authentication: use the test authenticator of M1-18 if it exists; otherwise add the test-only package `internal/kernel/actor/actortest` with builders for: owner in a session with fresh or old step-up, owner with a personal API token, human without owner role, human on a pending device, agent, anonymous.

## Tests (write them first)

HTTP-level integration tests through the real chain (M1-18 to M1-20) with the test authenticator, PostgreSQL 18 through `pgtest`, controllable clock, fake `AgentPolicySource` and `StructureSource` where I1 matters. Every response is validated against its schema.

1. No self registration: anonymous `POST /api/v1/users` is `401`; a human without owner role gets `403` (`owner_only`); no other built operation inserts a person (assert the users table after calling all ten operations as non-owner).
2. Step-up: `createUser`, `updateUser`, `enableUser`, `addOwner`, `revokeOwner` without fresh step-up are `401 step_up_required` (AZ-M36 shape); with an owner's personal token `403 step_up_not_possible` (AZ-M35 shape, SR5); `getMe` and `updateMyProfile` need no step-up.
3. `TestAZ_M23_AddOwnerWithPersonalAgent`: with the fake source reporting agent Assistent as personal with the owner as holder, `addOwner` for Alex is `409 i1_violation` with `conflicts` naming the agent, the holder and Alex; nothing changed; one deny audit row.
4. Last owner: `revokeOwner` of the sole owner, also by that owner, is `409 last_owner`.
5. Owner race: two owners, two concurrent `revokeOwner` calls removing each other: exactly one `204`, one `409 last_owner`.
6. `addOwner` happy path without personal agents: `201`, `user.updated` with `is_owner = true`, `access.changed` for the new owner (the fake structure has channels), audit `owner.add` with reason, `listOwners` contains the person.
7. `enableUser`: `pending` to `active`; `disabled` to `active` with `disabled_at` cleared; `deleted` is `409 invalid_state_transition`; already active is `200` without event; enabling a disabled second owner while the fake agent source reports a personal agent is `409 i1_violation`; enabling a person with a `trigger` grant on a channel served by the personal agent is `409 i1_violation`.
8. Validation: handle with upper case or leading dot, handle longer than `principal.handle_max_chars`, duplicate handle in other case, duplicate e-mail, display name with a bidi override or control character (SEC-T07), unknown time zone, `Local`, malformed locale, `local_account: true`: each `400 validation_failed` with the pointer of the field, or `409 conflict` for the duplicates.
9. `getMe`: human owner with fresh step-up (all fields, `step_up_valid_until` set), human with old step-up (field absent), agent principal (no `email`, `is_owner = false`), pending device allowed.
10. `updateMyProfile`: changes own fields and emits `user.updated`; an agent gets `403`; invalid values `400`.
11. `listUsers`: stable keyset pages while rows are inserted between requests; cursor reused with another filter is `400 cursor_invalid`; `q` prefix on handle and display name, with `%` and `_` treated literally; `status` filter; `limit` above `api.page_size_max` is capped; no `total` member; an agent caller gets `403`.
12. `getUser`: unknown ID and the ID of an agent principal both give the same `404` body.
13. Idempotency: `createUser` repeated with the same `Idempotency-Key` returns the stored response with `Idempotent-Replay: true` and creates one person; the same key with another body is `422 idempotency_key_reuse`; repeating after a `401 step_up_required` with the same key succeeds once step-up is fresh.
14. Audit: entries for `user.create`, `user.update`, `user.enable`, `owner.add`, `owner.remove` with the details of Scope 6; a search of all audit `details` and captured logs for the test display names and e-mail addresses finds nothing.
15. AZ-P1: the route test counts the ten new operations, each with its action from `actions.yaml`.

Commands that must pass:

- `cd services/server && go test -race ./internal/identity/... ./internal/authz/... ./test/...`
- `make codegen-check`, `make arch`, `make api-lint`, `make licenses`, `make docs`
- `make check`

## Acceptance criteria

- [ ] The ten operations are reachable, in `operations.yaml`, and every response matches its schema.
- [ ] No self registration (test 1).
- [ ] Owner actions only by owners in a session with fresh step-up (test 2).
- [ ] AZ-M23 green with a fake agent source; last owner and owner race green.
- [ ] `user.enable` covers `pending` and `disabled` and checks I1.
- [ ] `user.updated` emitted for create, update, enable, owner changes and own profile.
- [ ] SEC-T07 for people green; no names or e-mail addresses in audit details or logs.
- [ ] README, DECISIONS and CHANGELOG updated; doc comments on every exported symbol.
- [ ] `make check` green.

## Definition of done

- Code, tests and documentation as listed in Scope; the module README is current (operations, ports, events, tables).
- A doc comment on every exported symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- An ADR for every architecture decision taken here (none expected; rulings go to `docs/DECISIONS.md`).
- CHANGELOG entry under Unreleased in `services/server/CHANGELOG.md`.
- `make check` green on the final commit.
- Independent review of the diff (`make review` plus a code review by a fresh reviewer) and the red team of `docs/quality/red-team-gate.md` over the diff, verdict recorded in the report. On `NEIN`: fix, add a test per finding, re-check the delta.
- Merge locally into the base branch only when the review is clean, the red team says `JA` and `make check` is green; then `git worktree remove`.

## Report

Write `{{REPORT_PATH}}`:

- first line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`;
- commits (sha and subject), files changed with line counts, test results with counts (zero skipped), the red output of the first run of each new test file, what is not proven, follow-ups that could not be done in scope with the reason.

Then write `{{DONE_PATH}}` with `exit=0 sha=<merge sha>` or `exit=1 <reason>`.
