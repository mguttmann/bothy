# M1-26: Principal tables and directory

Status: ready | Depends on: M1-13 | Etappe: M1

## Goal

After this ticket the tables `principals`, `users` and `owners` exist with every check of the
core contract, identity exposes a read-only `Directory` port for other modules, and identity
implements `authz.PrincipalSource`, so that `authz` (M1-28, M1-29) learns kind, status and owner
role of a principal without touching identity tables. Handles and display names are validated
at the domain boundary, including the rejection of control and bidi characters (SEC-T07, part
people), through a shared validator that later tickets reuse for space, channel and thread names.

## Read first

- `docs/PLAN.md`: section 3 "Arbeitsweise je Ticket"; section 7 rows M1-12, M1-13, M1-26 to
  M1-31, M1-37, M1-51, M1-52; section 9 row M3-03.
- `docs/spec/03-datenmodell.md`: section 1 "Grundsätze" (1 IDs, 3 enumerations, 6 case, 7
  deletion, 12 access, 13 no null), section 4.2 tables `principals`, `users`, `owners`, section
  6.3 "Sperrreihenfolge", section 8 R-DM-01 to R-DM-03, section 9 (`principal.handle_max_chars`).
- `docs/spec/07-rechte.md`: section 2.7 "Owner", section 3 "Prinzipal-Kontext", SR5, SR7, SR8.
- `docs/spec/08-identitaet.md`: section 4 "Erstinstallation", section 11 "Person sperren",
  section 12 "API-Tokens" (last bullet: personal token has no owner rights).
- `docs/spec/19-sicherheit.md`: section 6.1 SEC-IN-4 and SEC-IN-6; test SEC-T07.
- `docs/spec/02-architektur.md`: sections 4.1 to 4.5, 5, 6, 7.1 to 7.3, 8.1.
- ADRs 0006, 0007, 0008.
- `docs/operations/configuration.md`: key `principal.handle_max_chars`.
- `docs/api/openapi.yaml` schemas `User`, `PrincipalRef`, `PrincipalKind`, `UserStatus`,
  `UserCreate` (handle pattern): for naming of DTO fields only; no operation is built here.
- Code of earlier tickets: `internal/kernel/*` (M1-05), `internal/platform/pg` and
  `internal/platform/pg/pgtest` (M1-12), `internal/kernel/txn` and the unit of work (M1-13),
  `internal/platform/config` and the typed module configs in `internal/wire` (M1-07),
  `migrations/` and `sqlc.yaml` conventions, `architecture/modules.yaml` and `tools/archgen`
  (M1-05).
- API operations: none. Tables: `principals`, `users`, `owners`. Action ids: none are executed;
  the facts serve `owner`, `human`, `auth` and `self` requirements of 05 later.

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
6. Test first: every behaviour gets a test that is red before the code exists (plan section 3).
7. No en dash or em dash in any text you write; white label (examples use Alex, Sam, Robin and
   example.com); configuration only through named keys from `docs/operations/configuration.md`.
8. 03 `principals`, `users`, `owners`: columns, types, defaults, checks, unique indexes, indexes
   and foreign keys exactly as specified (exception: the avatar foreign key, see Design notes).
9. 03 Grundsatz 6: handles unique without regard to case (`lower(handle)`), email unique without
   regard to case where present.
10. 19 SEC-IN-4 and SEC-T07 (people): display names with control or bidi control characters are
    rejected.
11. 03 Grundsatz 12 and 02 section 4.1: only identity reads and writes its tables; `authz` gets
    facts through `authz.PrincipalSource`.

## Scope

Paths are relative to `services/server/` unless noted.

1. **Migration** `migrations/<next>_identity_principals.sql` (next free sequence number in the
   format M1-12 established; renumber at merge time if a parallel ticket took it): tables
   `principals`, `users`, `owners`; trigger function and triggers described in Design notes;
   Down section according to the M1-12 convention.
2. **Shared text validator** `internal/kernel/textcheck` (new kernel package, no dependencies):
   `func DisplayText(s string) error`. Add it to the kernel packages that domain layers may
   import (component `kernel-core` in the generated go-arch-lint configuration) through the
   source M1-05 uses (`architecture/modules.yaml` or the `tools/archgen` template) and regenerate
   `.go-arch-lint.yml` (02 sections 4.5 and 7.2 already list `kernel/textcheck` in `kernel-core`).
3. **Domain** `internal/identity/domain`: types `Kind` (`user`, `agent`, `app`), `Status`
   (`pending`, `active`, `disabled`, `deleted`), value objects `Handle` and `DisplayName`,
   entities `Principal`, `User`, `Owner` with constructors that validate (see Design notes).
4. **Application** `internal/identity/app`: outgoing port `Store` (interface over the repository),
   the `Directory` implementation and the `authz.PrincipalSource` implementation.
5. **Adapter** `internal/identity/adapters/postgres`: `queries/principals.sql`,
   `queries/owners.sql`, sqlc output in `sqlcgen/`, repository implementing `app.Store`, error
   mapping. Add or complete the identity entry in `sqlc.yaml`.
6. **Port** `internal/identity` (root package): interface `Directory` and its DTOs.
7. **authz source declaration** `internal/authz/sources.go`: `PrincipalSource`, `PrincipalFacts`,
   `PrincipalStatus` (M1-28 adds further sources to this file later).
8. **Configuration**: schema entries `principal.handle_max_chars` and
   `principal.display_name_max_chars` (integers, defaults from 03 section 9, minimum `1`) in the
   identity config of `internal/wire`; regenerate `docs/operations/configuration.md`
   with the M1-08 generator if it exists (`make config-reference` or the target M1-08 named).
9. **Documentation**: `internal/identity/README.md` (status, port, tables, source it implements),
   package docs of `kernel/textcheck`, `services/server/CHANGELOG.md` under `Unreleased`,
   no DECISIONS entry is needed for display names (03 states the rule).

## Out of scope

- Use cases that create or change people, owners, agents or apps, and every HTTP operation
  (`listUsers`, `createUser`, `getMe`, `addOwner` and so on): M1-31, M1-37, M1-51. Validation of
  email, locale (BCP 47) and time zone (IANA) belongs to those use cases.
- `internal/wire` wiring of the Directory and the PrincipalSource into `authz`: M1-28.
- Sessions, devices, tokens, identities: M1-33 and later.
- Pseudonymisation of deleted principals: M7 (DSGVO flows).
- The foreign key `principals.avatar_file_id -> files(id)`: added by the migration that creates
  `files` (M3-03).
- I1 and SR8 checks: M1-29.

## Boundaries

- A new migration needs its line in `migrations.sha256`: run `make migrations-lock` after creating
  the file and before `make check`; never edit a merged migration (M1-01, ADR 0007).
- Work in a dedicated git worktree on the branch `feat/M1-26-identity-principals`.
- Conventional Commits with scope (`feat(identity): ...`) and DCO sign-off (`git commit -s`).
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope above except `services/server/CHANGELOG.md`,
  `docs/DECISIONS.md` and the documentation files named in Scope.
- Follow the host rules supplied by the launcher.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`; remove
  them by their literal path at the end. PostgreSQL runs only as the throwaway container of
  `pgtest` on a random port bound to `127.0.0.1`.
- Never read or print secrets. No network calls except package downloads for pinned tools.

## Design notes

**Tables** (03 section 4.2, verbatim): `principals` with `kind CHECK (kind IN
('user','agent','app'))`, `status` default `'active'` with its check, `handle CHECK (handle ~
'^[a-z0-9][a-z0-9._-]*$')`, unique index on `lower(handle)`, index `(kind, status)`;
`users` with PK `principal_id`, FK to `principals(id)` ON DELETE RESTRICT, `locale` default
`'de'`, `time_zone` default `'UTC'`, partial unique index on `lower(email) WHERE email IS NOT
NULL`; `owners` with FKs `user_id -> users(principal_id)`, `granted_by`, `revoked_by ->
principals(id)`, all ON DELETE RESTRICT, partial unique index `(user_id) WHERE revoked_at IS
NULL`, `reason text NOT NULL`. IDs are UUIDv7 from `kernel/ids`; no database defaults for ids.

**Avatar foreign key.** `principals.avatar_file_id` is created as a nullable `uuid` without the
foreign key, because `files` does not exist before M3-03. The migration carries a comment naming
M3-03 as the place that adds `FOREIGN KEY (avatar_file_id) REFERENCES files(id) ON DELETE SET
NULL`.

**Triggers** (SQLSTATE `23514` so the adapter maps them like a check violation):

- `identity_users_kind_check` (BEFORE INSERT OR UPDATE OF principal_id ON users): the referenced
  principal has `kind = 'user'` (03 `users`: "per Trigger geprüft").
- `identity_principals_kind_immutable` (BEFORE UPDATE OF kind ON principals): rejects any change
  of `kind` (03: "nie änderbar").

**Validation.**

- `kernel/textcheck.DisplayText` rejects: invalid UTF-8; every rune of Unicode category Cc
  (C0, DEL, C1, including tab and newline); the bidi controls U+061C, U+200E, U+200F, U+202A to
  U+202E and U+2066 to U+2069. It returns a typed error that names the rule (`invalid_utf8`,
  `control_character`, `bidi_control`) and the rune offset, never the text. Letters with
  accents, emoji and U+200D (zero width joiner) pass. It does not judge length or emptiness,
  because thread titles may be empty.
- `domain.NewHandle(s string, maxChars int)`: matches `^[a-z0-9][a-z0-9._-]*$` (Go `regexp`,
  SEC-IN-6), length 1 to `maxChars` (`principal.handle_max_chars`). Upper case is rejected, not
  folded: the database check and the OpenAPI pattern are lower case only.
- `domain.NewDisplayName(s string, maxChars int)`: `textcheck.DisplayText`, at least one rune that
  is not Unicode white space, and at most `maxChars` runes (`principal.display_name_max_chars`;
  03 `principals.display_name`).
- `domain.NewOwner(...)` requires a non-blank `reason` (03: "Pflicht, steht im Audit").
- Validation errors are `kernel/errs` errors of kind `bad_request` with code
  `validation_failed` and a field error (`/handle`, `/display_name`, `/reason`) in the form
  M1-18 renders as `errors[]`.

The rule "not blank" is in 03 (`principals.display_name`). Reason: a blank name makes a person
unidentifiable in mentions, member lists and the audit view.

**Port `identity.Directory`** (read only, for other modules; it does not call `authorize`
because callers authorize their own actions):

```go
type PrincipalView struct {
    ID           ids.ID
    Kind         actor.Kind // user, agent, app
    Status       Status     // identity.Status: pending, active, disabled, deleted
    Handle       string
    DisplayName  string
    AvatarFileID ids.ID     // zero value: no avatar
    CreatedAt    time.Time
}
type UserView struct {
    PrincipalView
    Email    string // "": none
    Locale   string
    TimeZone string
    IsOwner  bool   // active owners row (revoked_at IS NULL)
}
type OwnerView struct {
    User      PrincipalView
    GrantedBy ids.ID // zero value: first owner (server command)
    GrantedAt time.Time
    Reason    string
}
type Directory interface {
    Principal(ctx context.Context, id ids.ID) (PrincipalView, error)                 // errs not_found
    Principals(ctx context.Context, ids []ids.ID) (map[ids.ID]PrincipalView, error)  // unknown ids absent
    PrincipalByHandle(ctx context.Context, handle string) (PrincipalView, error)     // lower(handle) match; errs not_found
    User(ctx context.Context, id ids.ID) (UserView, error)                           // errs not_found also for non-users
    ActiveOwners(ctx context.Context) ([]OwnerView, error)                          // revoked_at IS NULL, ordered by granted_at, id
}
```

Use the principal kind type of `kernel/actor` (M1-05). If `kernel/actor` has none, add
`actor.Kind` with the values `user`, `agent`, `app`, `anonymous` and doc comments.

**Source `authz.PrincipalSource`** (declared in `internal/authz/sources.go`, implemented in
`internal/identity/app`):

```go
// PrincipalStatus is the lifecycle state of a principal as authz sees it.
type PrincipalStatus string // "pending", "active", "disabled", "deleted"

type PrincipalFacts struct {
    ID      ids.ID
    Kind    actor.Kind
    Status  PrincipalStatus
    IsOwner bool // an owners row with revoked_at IS NULL exists; independent of Status
}

type PrincipalSource interface {
    // Principal returns the facts of one principal; ok is false when the id is unknown.
    Principal(ctx context.Context, id ids.ID) (PrincipalFacts, bool, error)
    // ActiveOwnerIDs returns the users with an active owners row whose principal status is
    // active, ascending by id. I1 and SR8 (M1-29) need this set.
    ActiveOwnerIDs(ctx context.Context) ([]ids.ID, error)
}
```

**Transactions.** Reads join the ambient transaction of `kernel/txn` when one exists, otherwise
use the pool (M1-13 convention). Repository writes (insert principal and user, insert owner) are
only used by tests in this ticket and by later use cases; their doc comments state that callers
run them inside a unit of work and, for owners, as a rights change (M1-29).

**Error mapping in the adapter.** Unique violation on the handle index: kind `conflict`, code
`conflict`, field `/handle`, without echoing the value. Unique violation on the email index:
same with `/email`. Unique violation on the active owner index: `conflict`. Check violations,
trigger violations (`23514`) and foreign key violations (`23503`) on writes: kind `bad_request`,
code `validation_failed`, with the column as field. Never put SQL text or values into the error.

## Tests (write them first)

All database tests use `pgtest` against PostgreSQL 18; without a Docker-compatible runtime they
fail with a clear message (P-09).

1. `internal/kernel/textcheck`: `TestDisplayText` table with one case per bidi control listed
   above, U+0000, U+0009, U+000A, U+007F, U+0085, U+009F, invalid UTF-8 bytes, and passing cases
   `Alex`, `Zoë`, an emoji, a name with U+200D.
2. `internal/identity/domain`:
    - `TestNewHandle` table: valid `alex`, `a`, `robin.s-1`, a name of exactly `maxChars`; invalid
      empty, `-alex`, `.alex`, `Alex`, `al ex`, `alëx`, a name of `maxChars + 1` (run with
      `maxChars` 32 and 5).
    - `TestSECT07_DisplayNameRejectsControlAndBidi` (SEC-T07, people) and
      `TestNewDisplayName_RejectsBlank` (spaces, U+3000, empty), `TestNewDisplayName_MaxChars`
      (exactly the limit passes, one rune more fails; runes, not bytes).
    - `TestNewOwner_RequiresReason`.
3. `internal/identity/adapters/postgres` (integration):
    - migration applies on an empty database and a second `up` changes nothing;
    - `TestHandleUniqueCaseInsensitive`: second insert of `alex` gives `conflict` on `/handle`; a
      raw SQL insert of `Alex` fails on the check constraint;
    - `TestEmailUniqueCaseInsensitive`: `alex@example.com` and `Alex@example.com` conflict;
      two users without email are fine;
    - `TestUsersRowRequiresUserKind`: a `users` row for an agent principal fails (trigger);
    - `TestPrincipalKindImmutable`: `UPDATE principals SET kind = 'agent'` fails;
    - `TestOneActiveOwnerRowPerUser`: a second active row conflicts, after setting `revoked_at`
      a new row is accepted; an owner row for an agent principal fails (no `users` row).
4. `internal/identity/app` (integration through the real repository):
    - `TestPrincipalSource_Facts`: user, agent and app principals report kind and status; owner
      true for an active row, false after revocation, ok false for an unknown id;
    - `TestPrincipalSource_ActiveOwnerIDs`: excludes revoked owners and owners whose principal is
      `disabled`; sorted ascending;
    - `TestDirectory`: `Principal` not found gives kind `not_found`; `Principals` omits unknown
      ids; `PrincipalByHandle` finds `alex`; `User` on an agent id gives `not_found`;
      `ActiveOwners` ordering.
5. Architecture: `make arch` green with `identity/domain` importing `kernel/textcheck`.

Commands that must pass:

```sh
cd services/server && go test -race ./internal/kernel/textcheck/... ./internal/identity/...
make migrations-lint
make codegen-check
make arch
make check
```

## Acceptance criteria

- [ ] The three tables match 03 section 4.2 column by column; the only deviation is the missing
      avatar foreign key, documented in the migration and in the identity README.
- [ ] Handle rules hold in the domain and in the database; uniqueness ignores case.
- [ ] SEC-T07 for display names of people is green under that name.
- [ ] `PrincipalSource` returns kind, status and owner role; `ActiveOwnerIDs` matches its
      definition.
- [ ] `Directory` returns the documented errors and never exposes SQL details.
- [ ] `kernel/textcheck` is importable from domain layers and listed in 02 section 4.5.
- [ ] `principal.handle_max_chars` is in the configuration schema with default `32` and appears
      in the regenerated configuration reference.
- [ ] Display names over `principal.display_name_max_chars` runes are rejected (test).
- [ ] `make check` is green.

## Definition of done

Code, tests and documentation are complete: module README updated, doc comments on every public
symbol, an ADR if you took an architecture decision beyond this brief, CHANGELOG entry under
`Unreleased` in `services/server/CHANGELOG.md`. `make check` is green. An independent review and
the red team (`docs/quality/red-team-gate.md`) have run over the diff with their verdicts
recorded in the report. Merge locally into the base branch only if both are green; then remove
the worktree with `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then the
commits, the files with line counts, the test results with counts (including the red run of each
new test before the code), what is not proven, and follow-ups that could not be done in scope with
the reason. Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
