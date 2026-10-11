# M1-47: Feature flags with registration and cleanup gate

Status: ready | Depends on: M1-44, M1-46 | Milestone: M1

## Goal

After this ticket every feature flag is registered at one place in the server code with key,
kind (`release` or `option`), description, default, owning module and, for `release` flags, the
version `remove_by`. The server evaluates flags per principal with the rollout lists of people
and groups, unknown keys are off, owners change flags over the API with step-up, reason and
audit, and clients get their evaluated flags from `getInstance`. A gate `make flag-check` fails
as soon as the server version reaches the `remove_by` of a registered `release` flag, so no
release flag outlives its date (16 "Feature flags", ADR 0064). The existing gate on personal API
tokens (`tokens.personal_enabled`, M1-44) runs through this evaluation.

## Read first

- `docs/PLAN.md`: section 3 "Way of working per ticket"; section 7 rows M1-27, M1-30, M1-44, M1-46,
  M1-47; exit criterion 9 (ADR 0064, accepted:
  `docs/adr/0064-feature-flag-registration-with-a-cleanup-gate.md`).
- `docs/spec/16-operations.md`: "Environments and feature flags" (6), especially "Feature flags"
  (6.2), and decision R-OP-14.
- `docs/operations/environments-and-flags.md`: section 2 "Feature flags" (2.1 to 2.5).
- `docs/operations/configuration.md`: 1 "Sources" (row Feature flags), 7 "Feature flags".
- `docs/spec/03-data-model.md`: 4.1 table `feature_flags` (rollout schema
  `feature_flag_rollout.v1`); 4.3 `groups`, `group_memberships` (validity of memberships).
- `docs/spec/07-permissions.md`: "Validity of a grant" (2.5: a membership with `expires_at = t`
  is already invalid), "Principal context" (3, personal tokens and `tokens.personal_enabled`).
- `docs/spec/05-actions.md`: 4.2 rows `instance.read`, `feature_flag.read`,
  `feature_flag.update`; 4.3 row `token.create`; 6 row `instance.updated`.
- `docs/spec/06-api.md`: 5 item 5 (`403 feature_disabled`, public endpoints answer `404`), 8
  item 3 (flags take an optional `If-Match`).
- `docs/spec/18-release.md`: 2 "Components", rules 1 and 4 (file `VERSION`, SemVer without `v`, equal to the tag).
- `docs/spec/02-architecture.md`: 4.1 item 4 (source ports), 4.3, 4.4, 5, 7.1 to 7.3.
- `docs/adr/0000-template-madr-4.md`, `docs/adr/0001-adrs-in-madr-format-decisions-as-ruling-log.md`
  (form of the new ADR), `docs/adr/0011-12-factor-configuration-and-secret-provider.md`.
- `docs/api/openapi.yaml`: operations `listFeatureFlags` (`GET /feature-flags`),
  `putFeatureFlag` (`PUT /feature-flags/{key}`), `getInstance`; schemas `FeatureFlag`,
  `FlagRollout`, `FeatureFlagPut`, `FeatureFlagList`, `EvaluatedFlag`, `InstanceInfo`.
- Code of earlier tickets: `internal/instance` (M1-15, M1-46: settings registry, `getInstance`),
  `internal/authz` (M1-27 validity functions of memberships), `internal/identity` (M1-26
  directory, M1-44 API tokens and the current gate on `tokens.personal_enabled`),
  `internal/wire`, `services/server/VERSION`, `Makefile` (target `gates`), M1-05
  `tools/archgen` (location convention for Go tools).
- Tables: `feature_flags`. Action ids: `feature_flag.read`, `feature_flag.update`,
  `instance.read`, `token.create` (only the flag check).

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
7. No en dash or em dash in any text you write; white label (people Alex, Sam, Robin, groups
   Support, Marketing, Engineering, Research); configuration only through named keys from
   `docs/operations/configuration.md`.
8. 16 6.2 "Registration": every flag is registered in code with key, kind `release` or
   `option`, description, default, and for `release` the version `remove_by`; environments doc
   2.2 adds the owning module and says unknown keys in the table are off and shown as orphaned.
9. 16 6.2 "Evaluation" and environments doc 2.3: value = global value, inverted when the
   principal or one of its valid groups is in `rollout`; unknown keys are off; the server
   evaluates, never the client; agents and apps get the global value (rollout holds only people
   and groups).
10. 16 6.2 "Cleanup rule": a gate fails when the server's `VERSION` reaches `remove_by` of a
    registered `release` flag.
11. 05 4.2: `feature_flag.read` and `feature_flag.update` are owner only; update needs step-up
    and is audited on allow and deny; environments doc 2.4 requires a reason.
12. 05 6 and 16 6.2: `instance.updated` names only the changed flag keys.
13. 16 6.2 "Limit": flags switch functions only, never security limits, thresholds or secrets.
14. configuration.md 7: the option flags `dm.enabled`, `voice.enabled`,
    `tokens.personal_enabled`, all off by default.
15. 06 5 item 5 and 05 `token.create`: with `tokens.personal_enabled` off for the caller,
    creating a personal token answers `403 feature_disabled`.

## Scope

Paths are relative to `services/server/` unless they start with `docs/` or `Makefile`.

1. **Specification** (in place from planning, spec first; change it only where the
   implementation proves it wrong): `docs/api/openapi.yaml` `FeatureFlag` with `kind`,
   `description`, `default`, `remove_by`, `module`, `orphaned`, `version`, `updated_by`,
   `updated_at` and `FeatureFlagPut` with required `enabled` and `reason` and without
   `description`; ADR 0064; 03 `feature_flags` (no `description` column, version `0` for a
   registered key without a row); 02 4.3 (`instance` declares `FlagGroupSource` and
   `FlagUserSource`, implemented by `authz` and `identity`); 02 "6. Code layout"
   (`tools/flagcheck`). To do here: `docs/quality/gates.md` row `flag-check`; `docs/DECISIONS.md`
   the rulings under Design notes, in the file's existing format.
2. **Port** `internal/instance/flags.go` (port package, imports only `kernel`):
    ```go
    type FlagKind string // FlagKindRelease = "release", FlagKindOption = "option"
    type FlagDefinition struct {
        Key, Description, Module, RemoveBy string // RemoveBy only for release
        Kind    FlagKind
        Default bool
    }
    const (
        FlagDMEnabled             = "dm.enabled"
        FlagVoiceEnabled          = "voice.enabled"
        FlagTokensPersonalEnabled = "tokens.personal_enabled"
    )
    func RegisteredFlags() []FlagDefinition // the one registry, sorted by key
    type FlagSubject struct { PrincipalID ids.ID; Kind string } // user, agent, app
    type EvaluatedFlag struct { Key string; Enabled bool }
    type Flags interface {
        Enabled(ctx context.Context, s FlagSubject, key string) (bool, error)
        EvaluateAll(ctx context.Context, s FlagSubject) ([]EvaluatedFlag, error)
    }
    type FlagGroupSource interface { // implemented by authz
        ValidGroupIDs(ctx context.Context, userID ids.ID, at time.Time) ([]ids.ID, error)
        ExistingGroupIDs(ctx context.Context, groupIDs []ids.ID) ([]ids.ID, error)
    }
    type FlagUserSource interface { // implemented by identity
        ExistingUserIDs(ctx context.Context, principalIDs []ids.ID) ([]ids.ID, error)
    }
    ```
    02 5 allows only the port package and `domain`, `app`, `adapters` in a module (PLAN row M1-47),
    so the parts live in those layers.

3. **Domain** `internal/instance/domain/flags.go` (pure): `FlagSpec` (same fields as the port
   definition; `app` converts), `CheckRegistry(specs []FlagSpec, modules []string) []string`,
   `Evaluate(global bool, inRollout bool) bool`, `InRollout(r Rollout, kind string, principalID
   ids.ID, validGroups []ids.ID) bool`, `NormalizeRollout(r Rollout) Rollout` (sorted, unique),
   `CompareSemVer(a, b string) (int, error)` (precedence after semver.org 2.0.0 section 11, build
   metadata ignored), `ReachedRemoveBy(version, removeBy string) (bool, error)`.
4. **Application** `internal/instance/app/flags.go`: implements `instance.Flags`; use cases
   `ListFlags(ctx, actor)` and `PutFlag(ctx, cmd PutFlagCmd)`; `GetInstanceInfo` of M1-46 fills
   `features` with `EvaluateAll` for the caller.
5. **Adapter postgres**: migration `migrations/<next>_feature_flags.sql` (table exactly as 03
   4.1), `queries/flags.sql`, sqlc output.
6. **Adapter httpapi**: strict-server methods `ListFeatureFlags`, `PutFeatureFlag`.
7. **Source port implementations**: `internal/authz/adapters/flagsource` (valid groups through
   the M1-27 validity functions and the authz store; existing groups = not deleted),
   `internal/identity/adapters/flagsource` (principals of kind `user` with status other than
   `deleted`); wired in `internal/wire`.
8. **Personal tokens.** M1-44 decides `tokens.personal_enabled` through its consumer port
   `PersonalTokenSwitch`, bound to the default-off `app.PersonalTokensDefault{}`. Implement the
   switch in `internal/identity/adapters/instanceflags` over
   `instance.Flags.Enabled(ctx, caller, instance.FlagTokensPersonalEnabled)`, rebind it in
   `internal/wire`, and delete the default binding.
9. **Gate tool** `tools/flagcheck` (Go `main` in the server module, beside `tools/archgen`;
   follow its location convention): flags `-version-file`, `-config-doc`, `-modules`; checks in
   Design notes. `Makefile`: target `flag-check`, part of `gates`; it prints `SKIP flag-check no
   server module` when `services/server/go.mod` is not tracked (gates.md "Skipping").
10. **Enable list** `api/operations.yaml`: `listFeatureFlags`, `putFeatureFlag`.
11. **Documentation**: `internal/instance/README.md` (registry, evaluation, source ports, how to
    add and remove a flag), `docs/operations/environments-and-flags.md` 2.2 (where the registry
    lives, the `flag-check` command), `services/server/CHANGELOG.md` under `Unreleased`.

## Out of scope

- Using `dm.enabled` (V3, DMs) and `voice.enabled` (M3 transcription): their features check the
  flag when they are built.
- Console screens for flags: M6-09 and M4.
- Instance settings, branding, discovery: M1-46.
- Deleting a flag's row when its code is removed: done by the change that removes the flag
  (environments doc 2.5 item 2); no release flag exists in M1.

## Boundaries

- A new migration needs its line in `migrations.sha256`: run `make migrations-lock` after creating
  the file and before `make check`; never edit a merged migration (M1-01, ADR 0007).
- Work in a dedicated git worktree on the branch `feat/M1-47-feature-flags`.
- Conventional Commits with module scope (for example `feat(instance): feature flag registry`)
  and DCO sign-off (`git commit -s`).
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope above except `services/server/CHANGELOG.md`,
  `docs/DECISIONS.md` and `docs/adr/README.md` (by `make adr-index`).
- Follow the host rules supplied by the launcher.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`; remove
  them by their literal path at the end. PostgreSQL runs only as the throwaway container of
  `pgtest` on a random port bound to `127.0.0.1`.
- Never read or print secrets. No network calls except package downloads for pinned tools.

## Design notes

**Registry content in M1** (configuration.md 7):

| Key | Kind | Default | Module | Description (English, shown in the console) |
|---|---|---|---|---|
| `dm.enabled` | option | false | `chat` | Direct messages between people |
| `tokens.personal_enabled` | option | false | `identity` | Personal API tokens |
| `voice.enabled` | option | false | `files` | Transcription of voice messages |

Registry rules (checked by `CheckRegistry`, used by `flag-check` and a unit test): key matches
`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$` (fits the `KeyPath` pattern), keys unique, kind is
`release` or `option`, description not blank, module is a key under `modules:` in
`architecture/modules.yaml`, `release` has a valid `remove_by` (SemVer without `v`), `option`
has none. A flag never encodes a number, limit or secret (16 6.2 "Limit"); the README says so
next to the registry, because a check cannot see intent.

**Evaluation.**

```text
Enabled(s, key):
  def := registry[key];  if missing: return false            # unknown keys are off
  row := feature_flags[key] (one primary-key read)
  global := row exists ? row.enabled : def.Default
  in := s.Kind == "user" and (s.PrincipalID in row.rollout.user_ids
         or any(ValidGroupIDs(s.PrincipalID, t)) in row.rollout.group_ids)
  return global XOR in
```

`t` is read once per call from `kernel/clock`. `ValidGroupIDs` returns groups that are not
deleted and in which the person has a membership in effect at `t` (`expires_at = t` is already
invalid, 07 2.5). `EvaluateAll` reads all rows in one query, calls `ValidGroupIDs` at most once,
returns registered flags only, sorted by key. Orphaned rows evaluate to off and never appear in
`EvaluateAll`. No cache: one indexed read per call keeps changes in other processes effective at
once; a cache would need the event log and buys nothing at this size. Errors are returned, not
turned into "off", so a database fault shows as `5xx` instead of silently disabling a feature.

**Rollout** (`feature_flag_rollout.v1`): `{user_ids?: [uuid], group_ids?: [uuid]}`, stored
normalized (sorted, unique, empty arrays omitted, `{}` when both empty). The API returns the same
shape (`rollout: {}` when empty).

**`listFeatureFlags`** (`feature_flag.read`, owner): every registered flag (row values, or the
default with `version = 0`, no `updated_at`, no `updated_by`), then every orphaned row
(`orphaned: true`, `description` from the row, no `kind`, `default`, `module`, `remove_by`), all
sorted by key.

**`putFeatureFlag`** (`feature_flag.update`, owner, step-up, audit, `IfMatchOptional`):

- key not registered (also orphaned): `404 not_found`;
- `If-Match` present and not the current version (`0` without row): `412 version_conflict` with
  `current`;
- body `{enabled, rollout?, reason}`; an absent `rollout` means empty (PUT replaces);
- unknown or non-person IDs in `rollout.user_ids` (via `FlagUserSource`): `400
  validation_failed`, pointer `/rollout/user_ids/<index>`, code `unknown_user`; unknown or
  deleted groups (via `FlagGroupSource`): pointer `/rollout/group_ids/<index>`, code
  `unknown_group`;
- same `enabled` and same normalized rollout as stored: no write, no event, `200`;
- otherwise upsert: `version` 1 on insert, `version + 1` on update, `description` copied from the
  registry, `updated_by`, `updated_at`; append `instance.updated` with `changed_keys = [key]`;
  audit allow with `details = {key, enabled, rollout, reason}` (IDs only, no names);
- response `200` `FeatureFlag` with `ETag: "<version>"`.

**`getInstance`** now returns `features = EvaluateAll(caller)` (agents and apps get global
values). Its ETag (hash of the body, M1-46) changes with the evaluation, as intended.

**`flag-check`** (`tools/flagcheck`): reads `VERSION`, `RegisteredFlags()`, the module keys of
`architecture/modules.yaml` and the table of `docs/operations/configuration.md` section 7.
Fails with one line per finding, prefixed `flag-check:`, exit code 1:

1. any `CheckRegistry` finding;
2. a `release` flag with `CompareSemVer(VERSION, remove_by) >= 0` (a pre-release such as
   `1.4.0-beta.1` has not reached `1.4.0`), message `remove flag <key>: VERSION <v> reached
   remove_by <r>`;
3. the configuration.md table differs from the registry in key, kind or default (`on` = true,
   `off` = false).
Prints `flag-check: ok (<n> flags)` otherwise. The tool takes the registry as a function
argument internally, so tests run it on fixture registries without touching the real one.

**ADR 0064** exists (accepted). Keep it current: if the implementation changes a rule or the
confirmation (`make flag-check`, `TestCheckRegistry*`, `TestFlagCheck*`, `TestEvaluate*`), update
the ADR in the same change and run `make adr-index`.

**Rulings for `docs/DECISIONS.md`:**

1. The registry owns key, kind, default, module and description; `FeatureFlagPut` has no
   `description` and requires `reason`.
2. A flag without a row has its default and `version = 0`.
3. Flag evaluation reaches groups and people through source ports declared by `instance`
   (`FlagGroupSource`, `FlagUserSource`), because `instance` may not depend on `authz` or
   `identity` (02 4.4).
4. No flag cache; errors are not mapped to "off".

## Tests (write them first)

1. `internal/instance/domain`: `TestCheckRegistry` (bad key, duplicate, unknown kind, blank
   description, unknown module, release without or with invalid `remove_by`, option with
   `remove_by`), `TestCompareSemVer` (table from semver.org 11 including pre-release order and
   build metadata), `TestReachedRemoveBy`, `TestEvaluate`, `TestInRollout` (person direct,
   person via group, agent with its ID in `user_ids` stays global), `TestNormalizeRollout`.
2. `internal/instance`: `TestRegisteredFlagsValid` (the real registry passes `CheckRegistry`).
3. `tools/flagcheck`: `TestFlagCheck_ReleaseReachedRemoveByFails`,
   `TestFlagCheck_PreReleaseBeforeRemoveByPasses`, `TestFlagCheck_ConfigDocMismatchFails`,
   `TestFlagCheck_RealRepositoryPasses`.
4. Integration against PostgreSQL 18 with real authz, identity, audit and event log and a
   controllable clock:
    - `TestFlags_GlobalOnOff`, `TestFlags_PersonInRolloutInverts`,
      `TestFlags_GroupInRolloutInverts`, `TestFlags_ExpiredMembershipDoesNotInvert` (membership
      of Sam in group Research expires: at `t = expires_at` Sam gets the global value),
      `TestFlags_RemovedMembershipDoesNotInvert`, `TestFlags_DeletedGroupDoesNotInvert`,
      `TestFlags_UnknownKeyOff`, `TestFlags_OrphanedRowOffAndListed`,
      `TestFlags_AgentGetsGlobalValue`.
    - `TestListFeatureFlags_OwnerOnly` (Sam `403 forbidden` with `reason` `owner_only`), defaults with `version = 0`.
    - `TestPutFeatureFlag_StepUpReasonAudit`, `TestPutFeatureFlag_UnknownKey404`,
      `TestPutFeatureFlag_OrphanedKey404`, `TestPutFeatureFlag_RolloutValidation` (agent ID in
      `user_ids`, unknown group), `TestPutFeatureFlag_IfMatch412`,
      `TestPutFeatureFlag_EventNamesOnlyKey`, `TestPutFeatureFlag_NoChangeNoEvent`.
    - `TestGetInstance_FeaturesEvaluatedForCaller` (Alex and Sam see different values with a
      rollout for Sam).
    - `TestPersonalToken_FlagOffForbidden`, `TestPersonalToken_FlagOnAllowed`,
      `TestPersonalToken_RolloutForSamOnly` (through `createToken` with fresh step-up).
5. Every response passes the response validation of the test build.

Commands that must pass:

```sh
cd services/server && go test -race ./internal/instance/... ./internal/authz/... ./internal/identity/... ./tools/...
make flag-check
make codegen-check
make api-lint
make docs
make check
```

## Acceptance criteria

- [ ] Evaluation is correct globally, per person and per group, including an expired membership
      at the exact second; unknown and orphaned keys are off.
- [ ] `make flag-check` is part of `make gates` and fails when `VERSION` reaches `remove_by` of a
      registered `release` flag (fixture test), and when configuration.md 7 drifts.
- [ ] Changes need an owner with fresh step-up and a reason, are audited on allow and deny, and
      write `instance.updated` with only the changed key.
- [ ] `getInstance` returns the flags evaluated for the caller.
- [ ] `tokens.personal_enabled` decides `createToken` for personal tokens through
      `instance.Flags`; no stand-in remains.
- [ ] ADR 0064 matches the implementation and names its confirmation.
- [ ] `make check` is green.

## Definition of done

Code, tests and documentation are complete: module README updated, doc comments on every public
symbol, ADR 0064 current, CHANGELOG entry under `Unreleased` in `services/server/CHANGELOG.md`.
`make check` is green. An independent review and the red team (`docs/quality/red-team-gate.md`)
have run over the diff with their verdicts recorded in the report. Merge locally into the base
branch only if both are green; then remove the worktree with `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then the
commits, the files with line counts, the test results with counts (including the red run of each
new test before the code), what is not proven, and follow-ups that could not be done in scope
with the reason. Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
