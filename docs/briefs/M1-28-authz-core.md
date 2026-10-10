# M1-28: Rights snapshot, cache and authorize

Status: ready | Depends on: M1-24, M1-25, M1-27 | Etappe: M1

## Goal

After this ticket `authz` implements the one authorization function of B7:
`Authorize(ctx, principal, action, resource)` decides exactly as 07 section 4.2 describes, from a
rights snapshot per principal that is cached per rights version and valid until the earliest
expiry, and `Scope` gives list queries the sorted sets of visible channels and spaces. Owner
rule, DM rule, operator rule, limits for agents and apps, app scopes, read-only channels, "view
as" and step-up are enforced; facts owned by other modules come only through source ports, which
the tests replace with fakes. Every denial is audited. Everything later in M1 (identity, rights
changes, structure, messaging, sync, gateway) calls this function.

## Read first

- `docs/PLAN.md`: section 3 "Arbeitsweise je Ticket"; section 7 (M1) "Teststrategie", "Red
  Team", exit criterion 2, and the rows M1-18, M1-24, M1-25 to M1-30, M1-33, M1-34, M1-44,
  M1-48, M1-49, M1-51 to M1-55, M1-63, M1-65, M1-70, M1-72.
- `docs/spec/07-rechte.md`: sections 1 to 8 completely, section 10 "Beispiele", section 11.1
  "Testmatrix", section 11.2 "Pflicht-Tests", section 12, section 14. Section 9 only to know
  what M1-49 builds on top.
- `docs/spec/05-aktionen.md`: completely; in particular section 1 rules 1, 2, 5, 7, 8 and 10,
  section 2 (2.1 to 2.6), section 4 (every "Recht" cell), section 5 "Agenten-Aufruf".
- `docs/spec/04-sync-protokoll.md`: section 2.2 (`permission_version`), section 8 (8.1, 8.2,
  8.5).
- `docs/spec/03-datenmodell.md`: section 4.3; from section 4.5 the fields of `channels`
  (`kind`, `space_id`, `read_only`, `operator_inherit`, `operator_agent_id`,
  `require_mention`, `thread_require_mention`, `free_response`, `allow_mentioned_agents`),
  `threads` (`kind`, `operator_agent_id`, `created_by`), `dm_participants`, `project_bindings`
  (`ready`); `agents` (`billing_mode`, `credential_holder_id`, `tool_class`,
  `reported_tool_class`); section 6.3; `audit_events` (`decision`, `deny_reason`).
- `docs/spec/02-architektur.md`: sections 4.3 to 4.5, 5, 8.1, 9 (9.1 to 9.3), 11.3, 14.1, 17
  (`perf.authorize_cached_p99`).
- `docs/spec/06-api.md`: section 3.2 point 4, section 3.3, section 4, section 5 (problem codes
  and the extensions `reason` and `hint`), decision R-API-21.
- `docs/spec/08-identitaet.md`: section 6 (step-up), section 12 (personal tokens).
- ADRs `0017-eine-autorisierungsfunktion-404-statt-403-registrierungszwang.md`,
  `0018-freigabe-modell-stufen-zusatzrechte-vererbung-kein-verbot.md`,
  `0021-audit-log-hash-verkettet-und-getrennt-von-telemetrie.md`.
- `docs/observability/audit-vs-telemetry.md`, `docs/security/threat-model.md` (T-AZ-01 to
  T-AZ-04).
- Code of earlier tickets: `internal/kernel/authorize` (M1-05, M1-18 check log, M1-25 generated
  identifiers and registry), `internal/kernel/actor`, `internal/kernel/auditing` (M1-24
  recorder), `internal/kernel/clock`, `internal/kernel/errs`, `internal/authz` (M1-25 catalogue,
  M1-26 `sources.go`, M1-27 domain and store), `internal/identity/app` (M1-26 PrincipalSource),
  `internal/instance` (M1-15 counters), `internal/platform/httpserver` (M1-18),
  `internal/wire`.
- API operations: none are added. Tables: `groups`, `group_memberships`, `grants` (read through
  the M1-27 store); `sequence_counters` row `permission_version` (read through the instance
  port). Action ids: every catalogue action gets a requirement entry; the decision tests use
  `message.read`, `message.create`, `message.delete`, `thread.read`, `space.read`,
  `channel.read`, `channel.update`, `export.channel`, `agent.invoke`, `grant.create`,
  `grant.revoke`, `user.create`, `device.list`, `self.read`.

## Fixed requirements that apply

1. No MVP. Built from scratch directly in its final state; etappen are only the build order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined
   interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface has a docstring or
   doc comment (purpose, parameters, return value, errors, example where useful); comments
   explain why, not what. API documentation is generated from code (OpenAPI from the routes,
   dartdoc, Python API docs in the docs site).
4. Documentation in the repo: README per module/service, an ADR for every architecture decision,
   a CHANGELOG entry per release, docs site kept current. A change without matching
   documentation is not done.
5. Quality gates (SonarQube, linters, type checks, security scanners, test coverage) block the
   merge.
6. Test first: every behaviour gets a test that is red before the code exists (plan section 3).
7. No en dash or em dash in any text you write; white label (examples and test worlds use Alex,
   Sam, Robin, example.com, spaces Support, Marketing, Engineering, Research); configuration only
   through named keys from `docs/operations/configuration.md`.
8. B7, ADR 0017: one function decides; no visible read right gives `not_found`, never
   `forbidden` (R-AZ-01).
9. 07 section 1: default is nothing; time is read once per call and checked on every call;
   fail closed on every error (no allow on a source error, an unknown action or an unknown
   resource kind).
10. 07 section 4.2: the order of the steps and the reasons of section 4.1.
11. 07 section 7 and R-RE-01, R-RE-02: cache per principal and rights version, valid until the
    earliest expiry; every REST request reads the current rights version.
12. 07 sections 2.7 to 2.10, 05 section 2.3: owner only in a session (SR5, R-AZ-14), owners
    never see DMs (SR6), operator rule (R-AZ-13), agents and apps at most `write` and `upload`
    (R-DM-08, R-DM-20), apps need a scope (05 section 2.6).
13. 05 rule 1.8: read-only channels block every action that needs `write` or `trigger`, owners
    included.
14. 05 section 2.5: a pending device may only use `self.read`, `device.list` for itself,
    `auth.token.refresh`, `auth.logout`.
15. 05 section 2.4 and R-AZ-04: every denial is audited (the M1-24 recorder aggregates).
16. 05 section 5 points 1 to 5, 7 and 8 for message triggers of `agent.invoke`.
17. 02 section 17: `BenchmarkAuthorizeCached` produces the value for `perf.authorize_cached_p99`.

## Scope

Paths are relative to `services/server/` unless they start with `docs/`.

1. **`internal/kernel/authorize`** (hand-written, next to the generated file of M1-25):
   `decision.go` (`Outcome`, `Decision`, `Reason`, `Decision.Err`), `resource.go` (`Resource`,
   `GrantFacts`, `InvokeFacts`), `authorizer.go` (`Authorizer` interface, `Scope` type),
   `version.go` (`WithPermissionVersion`, `WithRequestScope`). If M1-05 or M1-18 already
   declared any of these names, keep the existing name and complete it; if an existing exported
   name has a different meaning, stop with `Verdict: BLOCKED`.
2. **`internal/kernel/actor`**: make sure the request principal carries `ID`, `Kind`,
   `SessionID`, `DeviceID`, `StepUpAt`, `TokenID`, `InstallationID`, `GrantedScopes`, `ViewAs`
   (person id, zero for none) and `ActingFor` (struct `ActingFor{UserID, RunID, ThreadID
   ids.ID; JTI string}`, zero value for none; filled from M2 on) as listed in 07 section 3. Add
   missing fields with doc comments; do not rename existing ones.
3. **`internal/authz/sources.go`**: add `StructureSource`, `AgentPolicySource`,
   `CredentialSource` and their fact types next to `PrincipalSource` (M1-26).
4. **`internal/authz/domain`**: `reasons.go` (complete list), `requirements.go`, `scopes.go`,
   `snapshot.go`, `effective.go`, `visibility.go`, `decide.go`, `guards.go`.
5. **`internal/authz/app`**: `authorizer.go` (implements `authorize.Authorizer`), `cache.go`,
   `scope.go`, `version.go`, `bench_test.go`; extend `VerifyRegistrations` (M1-25) so routes and
   ops that name a deferred action fail the start.
6. **`internal/authz/adapters/fake`**: in-memory fakes of the four sources, exported for the
   tests of this and later modules (test files only).
7. **`internal/instance`**: the read-only port `instance.Versions` with the current rights
   version (see Design notes); implementation in instance's adapter.
8. **`internal/platform/httpserver`**: call `authorize.WithRequestScope` once per request at the
   start of the chain (02 section 9.1 step 1), unless M1-18's request-scoped check-log value can
   carry the memo (then extend that value instead).
9. **`internal/wire`**: build the authz `Authorizer` and register it as the
   `kernel/authorize.Authorizer`; pass the M1-26 `PrincipalSource`, the M1-27 store, the M1-24
   recorder, the instance version reader, the clock, the logger and empty sources for structure,
   agent policy and credentials (file `internal/wire/empty_sources.go`, see Design notes).
10. **Configuration**: schema entries `authz.snapshot_cache_max_entries` (integer, default
    `10000`, minimum `1`) and `auth.step_up_max_age` (duration, default `5m`) if absent; the
    authz config receives both. Regenerate `docs/operations/configuration.md` with the M1-08
    generator.
11. **Specification**: 02 sections 4.3 and 4.4 (`CredentialSource`, empty sources until their
    tickets), 06 section 4 point 4 and R-API-21 (aligned with 07 section 2.10), 07 section 4.2
    (deferred entries, audit of denials), 07 section 7.2 (thread-level operator rule not in list
    filters) and 08 section 12 (token scopes from M2) already state D1 to D3 and D6. Record D4
    and D5 in `docs/DECISIONS.md`.
12. **Documentation**: `internal/authz/README.md` (algorithm, cache, sources, deferred actions,
    how other modules build a `Resource`), package docs, `services/server/CHANGELOG.md` under
    `Unreleased`.

## Out of scope

- Rights changes, the phase hook, `permission_version` increments, `access.changed`, I1, SR8 and
  `VerifyInvariants`: M1-29. `invariant_violation` exists as an outcome here but `Authorize`
  never returns it.
- Expiry jobs and reminders: M1-30.
- "Who has access", preview API and "view as" sessions: M1-49 (the `ViewAs` intersection inside
  `Authorize` is in scope).
- `Authorizer.Observe` and the gateway's version handling: M1-65. If `kernel/authorize` already
  declares `Observe`, remove the declaration; M1-65 adds it with its implementation.
- Following structure changes without a new rights version in the gateway's per-process state
  (`channel.created` and friends, 02 section 11.3): M1-53, M1-62, M1-65. REST needs nothing for
  this: the snapshot holds grants per target and `Scope` builds its lists per request (07 7.2,
  7.4).
- Real implementations of the sources: `StructureSource` (M1-52, M1-53, M1-55), `AgentPolicySource`
  (M1-51; dev environments in V2), `CredentialSource` (M1-33 devices, M1-34 sessions, M1-44
  tokens). App installations as a source: M2.
- The metric `bothy.authz.decisions`: M1-68. Timing equality and leak tests (AZ-P4, AZ-P5,
  AZ-P12): M1-70.
- The exceptions of SR9 and the parsing of the invocation token into `ActingFor`: M2.
- The deferred actions listed in Design notes and `agent.invoke` point 6 (interactions) and SR3
  rule 2 (foreign thread): M2, M3, M8, V2.
- The conditional step-up waivers in the descriptions of `local_credential.set` and
  `passkey.register`: M1-39 and M1-40 (this ticket applies the Step-up column strictly).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-28-authz-core`.
- Conventional Commits with scope (`feat(authz): ...`, `docs(spec): ...`) and DCO sign-off
  (`git commit -s`).
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope above except `services/server/CHANGELOG.md`,
  `docs/DECISIONS.md` and the documentation and specification files named in Scope.
- Follow the host rules supplied by the launcher.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`; remove
  them by their literal path at the end. PostgreSQL runs only as the throwaway container of
  `pgtest` on a random port bound to `127.0.0.1`.
- Never read or print secrets. No network calls except package downloads for pinned tools.
- A gap in the core contract (chapters 00, 03, 04, 05) that blocks a decision below ends the
  ticket with `Verdict: BLOCKED` (plan P-11).

## Design notes

### Public contract in `kernel/authorize`

```go
type Outcome string // "allow", "not_found", "forbidden", "step_up_required",
                    // "invariant_violation", "unauthenticated"
type Reason string  // no constants here: the one closed list of 07 4.1 is kernel/auditing.Reason
                    // (M1-24, R-DM-28); authz converts those constants. Kernel ports never import
                    // each other (02 "go-arch-lint"), so the list cannot live in both packages.

type Decision struct { Outcome Outcome; Reason Reason }
func (d Decision) Allowed() bool
func (d Decision) Err() error // nil for allow; otherwise a kernel/errs error, mapping below

type Resource struct {
    Kind      ResourceKind // generated by M1-25
    ID        ids.ID       // zero for instance
    SpaceID   ids.ID       // path hints; authz resolves the path itself (see Resolution)
    ChannelID ids.ID
    ThreadID  ids.ID
    OwnerID   ids.ID       // principal the resource belongs to: author of a message or reaction,
                           // creator of a thread, holder of a device, session, token, draft or
                           // setting, starter of a run, the agent or app itself, the principal
                           // itself for user and principal resources
    TargetChannelID ids.ID // thread.move: destination channel
    HasAttachments  bool   // message.create
    RestrictedTags  bool   // thread.tags.set
    Grant     *GrantFacts  // grant.create (the new grant) and grant.revoke (the existing grant)
    Invoke    *InvokeFacts // agent.invoke
    PreviewOf Action       // access.preview: the action being previewed
}
type GrantFacts struct {
    SubjectKind string; Level string; Caps []string
    TargetKind string; TargetSpaceID, TargetChannelID ids.ID
}
type InvokeFacts struct {
    AgentID          ids.ID
    TriggerOrigin    string // messages.origin of the trigger: human, agent, app, import, system
    MentionsAgent    bool
    ActiveRunAgentID ids.ID // zero: no active run in the thread (set by runs from M2)
}

type Scope struct {
    ChannelIDs           []ids.ID // ascending, for "channel_id = ANY($1)"; built per request
    SpaceIDs             []ids.ID // ascending; spaces with at least one visible channel or a grant
    FullyVisibleSpaceIDs []ids.ID // ascending; spaces with read on the space itself (07 7.2)
    InstanceWide         bool     // owner or read on the instance (07 7.2)
    PermissionVersion    int64
}

type Authorizer interface {
    Authorize(ctx context.Context, p actor.Principal, a Action, r Resource) (Decision, error)
    Scope(ctx context.Context, p actor.Principal, a Action) (Scope, error)
}

func WithPermissionVersion(ctx context.Context, v int64) context.Context // pin (gateway, tests)
func WithRequestScope(ctx context.Context) context.Context               // memo for one request
```

`Authorize` returns an `error` only for failures (unknown or deferred action, unknown resource
kind, contradictory path hints, source errors); policy results are always a `Decision`. Use the
request-principal type name that `kernel/actor` already has; `actor.Principal` above stands for
it.

### Decision to error mapping (`Decision.Err`, 06 section 5)

| Outcome | Reason | errs kind | Problem code | Extension |
|---|---|---|---|---|
| `not_found` | any | `not_found` | `not_found` | none; the reason never leaves the server |
| `forbidden` | `scope_missing` | `forbidden` | `scope_missing` | `reason` |
| `forbidden` | `device_pending` | `forbidden` | `device_pending` | `reason` |
| `forbidden` | `channel_read_only` | `forbidden` | `channel_read_only` | `reason` |
| `forbidden` | every other reason | `forbidden` | `forbidden` | `reason` |
| `step_up_required` | `step_up_required` | `step_up_required` | `step_up_required` | none |
| `unauthenticated` | `principal_inactive`, `session_revoked`, `device_inactive` | `unauthenticated` | `session_revoked` | none |
| `unauthenticated` | `unauthenticated`, `token_revoked`, `app_not_installed` | `unauthenticated` | `unauthenticated` | none |
| `invariant_violation` | `i1_violation`, `last_owner`, `billing_mode_mismatch`, `tool_class_mismatch` | `invariant_violation` | the reason | `reason`; `hint` for `i1_violation` (set by M1-29) |

Use the `reason` extension of the problem body if M1-18 provides it; otherwise add an optional
reason field to `kernel/errs` that the problem renderer writes as `reason`.

### Sources (`internal/authz/sources.go`)

```go
type ChannelKind string // "text", "forum", "dm"
type ChannelFacts struct {
    ID, SpaceID ids.ID                  // SpaceID zero for dm
    Kind ChannelKind
    ReadOnly bool
    EffectiveOperatorID ids.ID          // operator_inherit ? spaces.default_operator_agent_id : operator_agent_id; zero = none
    RequireMention, ThreadRequireMention, FreeResponse, AllowMentionedAgents bool
    ProjectBound, ProjectReady bool     // a project_bindings row exists; its ready flag
    DMParticipants []ids.ID             // dm only: users with left_at IS NULL, ascending
}
type ThreadFacts struct {
    ID, ChannelID ids.ID
    Main bool                           // threads.kind = 'main'
    OperatorAgentID ids.ID              // threads.operator_agent_id; zero for main threads
    CreatedBy ids.ID
}
type StructureSource interface {
    Space(ctx context.Context, id ids.ID) (ok bool, err error)
    Channel(ctx context.Context, id ids.ID) (ChannelFacts, bool, error)
    Thread(ctx context.Context, id ids.ID) (ThreadFacts, bool, error)
    Channels(ctx context.Context) ([]ChannelFacts, error)              // every non-dm channel
    ChannelsOperatedBy(ctx context.Context, agentID ids.ID) ([]ids.ID, error)
    DMChannelsOf(ctx context.Context, userID ids.ID) ([]ids.ID, error)
}

type AgentPolicy struct {
    BillingMode        string // "personal", "organization"
    CredentialHolderID ids.ID // set exactly for personal
    ToolClass          string // "chat", "scoped", "host"
    ReportedToolClass  string // "" when no heartbeat reported one
}
type AgentPolicySource interface {
    AgentPolicy(ctx context.Context, agentID ids.ID) (AgentPolicy, bool, error)
    DevEnvironmentReady(ctx context.Context, userID, channelID ids.ID) (bool, error) // SR3; false until V2
}

type CredentialRef struct { PrincipalID, SessionID, DeviceID, TokenID ids.ID }
type DeviceState string // "pending", "active", "revoked", "unknown"
type CredentialFacts struct {
    SessionActive    bool      // session exists, belongs to PrincipalID and DeviceID, revoked_at IS NULL
    SessionExpiresAt time.Time
    DeviceState      DeviceState
    TokenActive      bool      // token exists, belongs to PrincipalID, revoked_at IS NULL
    TokenExpiresAt   time.Time // zero: none
}
type CredentialSource interface {
    Credentials(ctx context.Context, ref CredentialRef) (CredentialFacts, error)
}
```

Structure facts and agent policy facts are read fresh per call (see Resolution and Guards);
principal, credential, group and grant facts are cached with the snapshot.

### Requirement table (`domain/requirements.go`)

Every catalogue action has exactly one entry `{Source string, Expr Expr}`. `Source` repeats the
05 "Recht" text that the expression was written against; a test compares it with
`domain.LookupAction(id).Right`, so any change of 05 fails until the entry is reviewed. Atoms:

| Atom | True when | Reason when it decides a failure |
|---|---|---|
| `Public` | always, also for anonymous | |
| `Auth` | the principal is not anonymous | |
| `Human` | kind `user` | `subject_kind_not_allowed` |
| `AppKind` | kind `app` | `subject_kind_not_allowed` |
| `Self` | `r.OwnerID` equals the principal | `not_self` |
| `AgentSelf`, `AppSelf` | kind `agent` (`app`) and `Self` | `subject_kind_not_allowed` if the kind differs, else `not_self` |
| `Owner` | effective owner (snapshot) | `owner_only` |
| `Participant` | resolved channel is `dm` and the principal is in `DMParticipants` | `no_grant` (not visible) |
| `Level(l)` | effective level on the path is at least `l`; on a `space` resource `Level(read)` means "space visible" (07 section 4.3) | `level_insufficient` |
| `Cap(c)` | effective capabilities on the path contain `c` | `capability_missing` |
| `CapIf(c, fact)` | the descriptor fact is false, or `Cap(c)` | `capability_missing` |
| `TargetCap(c)` | `Cap(c)` on the path of `r.TargetChannelID`; an invisible target gives `not_found` | `capability_missing` |
| `ThreadOperator` | kind `agent` and the principal operates the thread (fixed operator, or the channel's effective operator for a main thread) | `subject_kind_not_allowed` |
| `ChannelOperator` | kind `agent` and the principal is the channel's effective operator | `subject_kind_not_allowed` |
| `ReadOnOperatedChannel` | `r` is agent A and the principal has at least `read` on a channel that A operates | `no_grant` |
| `GroupMember` | `r` is group G and the principal has a membership in effect in G | `not_self` |
| `Preview` | the requirement of `r.PreviewOf` holds on the same resource (no step-up); `PreviewOf` must be an action with Rights = `ja`, otherwise an internal error | reason of the inner failure |
| `All(...)`, `Any(...)` | conjunction, disjunction | `All`: first failing part; `Any`: reason of its first alternative |
| `ByKind{kind: expr}` | expression chosen by `r.Kind`; a kind not listed is an internal error | |

Mapping from the distinct 05 texts to expressions (apply literally; "scope" is checked
separately for apps). The texts are shown without their inner backticks; copy the exact text
from `docs/spec/actions.yaml` into `Source`. Overrides and deferred entries below win over this
table.

| 05 "Recht" text | Expression |
|---|---|
| `owner` | `Owner` |
| `manage` | `Cap(manage)` |
| `self` | `Self` |
| `read` / `write` / `trigger` | `Level(read)` / `Level(write)` / `Level(trigger)` |
| `self oder owner`, `owner oder self`, `owner; self bei eigenem`, `owner; self bei eigenen`, `self (Betroffene Person) oder owner` | `Any(Self, Owner)` |
| `agent-self`, `agent-self (Runner der Umgebung)` | `AgentSelf` |
| `auth`, `auth (Mensch mit aktivem Gerät, Agent, App mit events:subscribe)` | `Auth` |
| `owner oder manage` | `Any(Owner, Cap(manage))` |
| `self und read` | `All(Self, Level(read))` |
| `public`, `public (gültiges Refresh-Token und DPoP-Beweis der Familie)` | `Public` |
| `self oder manage` | `Any(Self, Cap(manage))` |
| `human` | `Human` |
| `agent-self bzw. app-self mit Scope`, `agent-self bzw. app-self`, `agent-self bzw. app-self, Eigentümer des Befehls`, `Autor der Nachricht (agent-self bzw. app-self)` | `Any(AgentSelf, AppSelf)` |
| `write und self`, `write und Autor` | `All(Level(write), Self)` |
| `self und write` | `All(Self, Level(write))` |
| `write und +upload` | `All(Level(write), Cap(upload))` |
| `write und (Ersteller oder manage oder Bediener-Agent)` | `All(Level(write), Any(Self, Cap(manage), ThreadOperator))` |
| `trigger und Regeln aus 5` | `Level(trigger)` plus the `agent.invoke` guard |
| `trigger und (Auslöser des Laufs oder manage)` | `All(Level(trigger), Any(Self, Cap(manage)))` |
| `read und +export` | `All(Level(read), Cap(export))` |
| `read bzw. self` | `ByKind{channel: Level(read), user: Self}` |
| `participant` | `Participant` |
| `owner; Bereich mit manage` | `ByKind{instance: Owner, agent: Owner, space: Any(Owner, Cap(manage))}` |
| `owner oder agent-self oder read auf einem Kanal, den der Agent bedient` | `Any(Owner, AgentSelf, ReadOnOperatedChannel)` |
| `manage auf Quelle und Ziel` | `All(Cap(manage), TargetCap(manage))` |
| `human; Apps mit Scope` | `Any(Human, AppKind)` |
| `auth und self` | `All(Auth, Self)` |
| `agent-self und wirksamer Bediener` | `ChannelOperator` |
| `App mit Scope costs:reconcile` | `AppKind` |
| `(write und Autor) oder manage` | `Any(All(Level(write), Self), Cap(manage))` |
| `Recht der Zielaktion` | `Preview` |
| `system` | not evaluable: `Authorize` returns an internal error (code `internal`) |

Overrides by action id (they win over the text rule): `message.create`:
`All(Level(write), CapIf(upload, HasAttachments))`; `thread.tags.set`:
`All(Level(write), CapIf(manage, RestrictedTags))`; `group.read`: `Any(Owner, GroupMember)`;
`stats.read`: `ByKind{instance: Owner, space: Any(Owner, Cap(manage))}`; `grant.read`:
`ByKind{space: Any(Owner, Cap(manage)), channel: Any(Owner, Cap(manage)), principal: Owner}`.

**Deferred entries** (07 section 4.2): the requirement of these actions depends on facts that only
a later etappe defines. Their entry is `Deferred("<etappe>")`; `Authorize` and `Scope` answer an
internal error with cause `unknown_action` (as for an unknown action, because no build of this
etappe may reach them), and `VerifyRegistrations` rejects any route or op that names one:
`agent_session.reset`, `agent_session.bind`, `run.followup.create`, `command.autocomplete`,
`command.invoke`, `interaction.create`, `run.start`, `webhook_out.deliver`, `webhook_in.post`
(M2); `file.read`, `file.transcript.submit` (M3); `import.apply` (M8). The ticket that builds one
of them replaces its entry and removes it from the golden list.

### App scopes (`domain/scopes.go`, 05 section 2.6)

A hand-written table from action id to scope: `message.read`, `thread.read`, `channel.read`,
`space.read`, `run.read`, `search.query` need `messages:read`; `message.create`,
`message.update`, `message.delete`, `reaction.add`, `reaction.remove`, `thread.create` need
`messages:write`; `file.read` `files:read`; `file.upload` `files:write`; `command.register`,
`command.remove`, `command.autocomplete.respond` `commands:write`; `component.update`,
`interaction.respond` `components:write`; `user.read` `users:read`; `gateway.connect`,
`events.catch_up` `events:subscribe`; `cost.reconcile.submit` `costs:reconcile`; `self.read`,
`instance.read`, `status.read`, `command.read`, `agent.read`, `webhook_in.post` need no scope.
For an app principal: action in the table with a scope not in `p.GrantedScopes` gives
`forbidden scope_missing`; action not in the table gives `forbidden subject_kind_not_allowed`.
Agents are not scope-checked: the only scopes they lack (`users:read`, `costs:reconcile`) belong
to actions whose requirement already excludes agents. Token scopes (`api_tokens.scopes`) are not
evaluated in this ticket (ruling D6).

### Visibility (`domain/visibility.go`, 07 section 4.3 per resource kind, ruling D4)

| Resource kind | Visible when |
|---|---|
| `instance` | the principal is authenticated, or the action is `Public` |
| `space` | effective level on the space path is at least `read`, or at least `read` on one of its channels |
| `channel`, `thread`, `message`, `reaction`, `pin`, `run`, `run_step`, `component`, `interaction`, `agent_session`, `file`, `incoming_webhook`, `command` | dm channel: the principal is a participant (owners included only as participants); otherwise effective level on the path at least `read` |
| `grant` | effective owner, or level at least `read` on the path of the grant's target (instance target: owner only) |
| `agent` | effective owner, the agent itself, or `read` on a channel the agent operates |
| `user`, `principal` | the principal is a human or an app, or the resource is the principal itself |
| `group` | effective owner, or a member in effect |
| `device`, `auth_session`, `api_token`, `passkey`, `push_registration`, `draft`, `gdpr_request`, `dev_environment` | `r.OwnerID` is the principal, or the principal is effective owner and the requirement contains `Owner` |
| `app`, `webhook_delivery`, `import_source` | effective owner |

A resource kind missing from this table is an internal error. Invisible gives `not_found` with
reason `no_grant` (`not_self` for the principal-owned kinds; `acting_for_insufficient` when only
the `ActingFor` intersection removed the visibility).

### Resolution (fresh per call)

The path comes from `StructureSource`, never from the caller alone: `ThreadID` gives the thread,
its channel and the channel's space; `ChannelID` gives channel and space; `SpaceID` is checked
with `Space`. A hint that contradicts the source (for example `ChannelID` not equal to the
thread's channel) is an internal error. A missing thread, channel or space gives `not_found`
`no_grant`. For `grant` resources the path is the target in `r.Grant`.

### Snapshot and effective rights (`domain/snapshot.go`, `domain/effective.go`)

The snapshot holds the result of 07 section 7.2 steps 1, 3 and 7 for one principal at one rights
version; propagation to channels (step 2), the operator rule (step 4), DMs (step 5) and the bot
limits (step 6) are applied when a path is evaluated, so a channel created after the snapshot is
judged correctly by `Authorize` (ruling D5; AZ-P8 proves equivalence with the 07 definition):

```text
Snapshot {
  PrincipalID, PermissionVersion, ComputedAt, ValidUntil
  Liveness  (PrincipalFacts, CredentialFacts)
  Owner     bool                       // IsOwner and kind user and SessionID set and status active
  Instance  (Level, Caps)              // max level and union of caps of grants in effect
  Spaces    map[space](Level, Caps)    // direct grants on spaces
  Channels  map[channel](Level, Caps)  // direct grants on channels
  Groups    []groupID                  // memberships in effect, groups not deleted
  ValidUntil = NextExpiry(computedAt, every expires_at of the grants and memberships read,
                           SessionExpiresAt, TokenExpiresAt)
}
```

Grants in effect: the principal's own grants (`Store.ActiveGrantsOfPrincipal`) and, for people
only, the grants of their groups in effect (`ActiveMembershipsOfUser`, `ActiveGrantsOfGroups`),
filtered with `Grant.InEffect(t)` and `Membership.InEffect(group, t)` from M1-27.

Effective rights on a path (instance, space S, channel C, optional thread T):
1. dm channel: participant gives `write` and `{upload}`; anyone else nothing (owners too, SR6).
2. owner: `trigger` and all capabilities.
3. otherwise: highest level of `Instance`, `Spaces[S]`, `Channels[C]`; union of their caps.
4. agents: at least `write` when the principal is `C.EffectiveOperatorID`, or when T is a
   non-main thread with `OperatorAgentID` equal to the principal (07 section 2.9 point 2).
5. agents and apps: level capped at `write`, caps intersected with `{upload}` (07 section 2.1;
   this also neutralises a grant whose `subject_kind` does not match the real kind).
6. `p.ViewAs` set: intersect with the effective rights of that person (snapshot without session,
   so never owner); `p.ActingFor` set: intersect with the effective rights of the triggering
   person (no SR9 exceptions yet: stricter until M2). Intersection: lower level, intersection of
   capabilities, owner only if both are owner.

### Cache (`app/cache.go`)

- Key: `(PrincipalID, SessionID, DeviceID, TokenID, PermissionVersion)`; snapshots of `ViewAs`
  and `ActingFor` people use zero credential ids.
- Hit only when the key version equals the current version and `t < ValidUntil` (strict: at
  `ValidUntil` the entry is rebuilt, so `expires_at = t` is already invalid without a job).
- LRU bounded by `authz.snapshot_cache_max_entries`; `container/list` and a mutex from the
  standard library; no new dependency.
- The visible lists for `Scope` are never stored in the cache entry: they are built per request
  from the snapshot and the current structure (`StructureSource.Channels`, `ChannelsOperatedBy`,
  `DMChannelsOf`) and memoised only in the request scope (`WithRequestScope`), so a channel or
  space created without a new rights version appears at once (07 7.2 and 7.4 rule 1).

### Rights version (`app/version.go`)

Order of sources: a version pinned with `WithPermissionVersion`; else the memo of the request
scope (filled by the first read in the request); else a fresh read. The read goes through the
instance port `instance.Versions` (02 "Ports, Ereignisse und Quellen je Modul"): create it with
`PermissionVersion(ctx) (int64, error)`, implemented with `SELECT value FROM sequence_counters
WHERE name = 'permission_version'` in the ambient transaction when present. Do not use the
`Counters` port: its phase guard (M1-15) rejects calls outside the phase hooks. A read failure is
an error of kind `unavailable`, never an allow.

### Algorithm (`domain/decide.go` pure; `app/authorizer.go` gathers facts)

```text
Authorize(ctx, p, a, r):
  spec := catalog.Spec(a)                  # M1-25; unknown -> internal unknown_action
  req  := requirements[a]                  # deferred -> internal unknown_action; system -> internal
  t    := clock.Now()                      # once
  if p anonymous: return Public(req) ? allow : unauthenticated/unauthenticated
  v    := version(ctx); snap := cache(p, v, t)
  liveness: status != active -> unauthenticated/principal_inactive
            SessionID set and (not SessionActive or t >= SessionExpiresAt) -> session_revoked
            DeviceID set and DeviceState in {revoked, unknown} -> device_inactive
            TokenID set and (not TokenActive or (TokenExpiresAt set and t >= TokenExpiresAt)) -> token_revoked
            kind app and InstallationID zero -> app_not_installed
  DeviceState pending and a not in {self.read, device.list on r.Kind device with r.ID == p.DeviceID,
                                    auth.token.refresh, auth.logout} -> forbidden/device_pending
  path := resolve(r)                       # not found -> not_found/no_grant
  eff  := effective(snap, path, p)         # including ViewAs and ActingFor intersections
  if not visible(r, eff, req): not_found with its reason
  if fail := eval(req, ...): forbidden with the atom's reason
  if p kind app: scope check (scope_missing or subject_kind_not_allowed)
  if path channel ReadOnly and eval(req with Level(write) and Level(trigger) forced false) fails:
      forbidden/channel_read_only           # 05 rule 1.8: manage and owner paths stay open
  if p.ViewAs set and writes(a): forbidden/view_as_read_only
  if spec.StepUp:
      kind != user or SessionID zero -> forbidden/step_up_not_possible
      StepUpAt zero or t - StepUpAt > auth.step_up_max_age -> step_up_required/step_up_required
  guards: agent.invoke, grant.create, grant.revoke (below)
  return allow
After deciding (app layer, not in decide): record (a, outcome) in the request check log of M1-18;
for every outcome other than allow call the M1-24 recorder for a denial outside the business
transaction (it aggregates per audit.deny_aggregate_window); internal errors are recorded with
decision "error". A recorder failure is logged and never changes the decision.
```

`decide.go` stays pure: it receives a fact bundle, and the app layer gathers what the action
needs before calling it (for `agent.read` the channels the agent operates and their facts; for
`agent.invoke` the agent's principal facts, its policy, its own effective level on the channel
and `DevEnvironmentReady`; for `thread.move` the target channel; for `grant.revoke` the target
path of the grant).

`writes(a)` is false for every action with rate-limit class `rl.read` or `rl.search` and for
`access.preview`, `audit.read`, `audit.verify`, `instance.discovery.read`,
`status.public.read`, `gateway.connect`, `push.deliver`, `webhook_out.deliver`; true for all
others. A golden test pins the resulting set.

### Guards (`domain/guards.go`)

`agent.invoke` (05 section 5; descriptor `r.Invoke` required, else internal error), in order:
1. principal kind `user` and `TriggerOrigin == "human"`, else `origin_not_human`.
2. path channel is not `dm`, else `agent_not_reachable` (no agents in DMs; `Level(trigger)` and
   the read-only rule ran before).
3. agent A (`Invoke.AgentID`) exists with kind `agent` and status `active` (`PrincipalSource`)
   and has a policy (`AgentPolicySource`), and is reachable: either A operates the thread (fixed
   operator of a non-main thread, otherwise the channel's effective operator) and the mention
   rule holds (`FreeResponse`, or main thread and not `RequireMention`, or non-main thread and
   not `ThreadRequireMention`, or `MentionsAgent`); or A is a guest: `MentionsAgent`,
   `AllowMentionedAgents`, A's own effective level on the channel is at least `write`, and A's
   billing mode is `organization`. Otherwise `agent_not_reachable`.
4. A's billing mode `personal` and the principal is not `CredentialHolderID`:
   `billing_mode_personal_not_holder` (I2).
5. the higher of `ToolClass` and `ReportedToolClass` (order `chat` < `scoped` < `host`) is
   `host`, the principal is not effective owner and `DevEnvironmentReady` is false:
   `tool_class_owner_only` (SR2).
6. `ProjectBound` and not `ProjectReady`: `project_not_ready`; `ProjectBound`, principal not
   owner and `DevEnvironmentReady` false: `dev_environment_not_ready` (05 point 7, SR3 rule 1).
7. `ActiveRunAgentID` set and not A: `run_active_other_agent` (05 point 8).
Point 6 of 05 (interactions) and SR3 rule 2 (foreign thread) come with M2 and V2.

`grant.create` (`r.Grant` required; its target must equal the resource, else internal error):
`domain.CheckCreate(Delegator{IsOwner: snapshot owner after intersections, Level, Caps:
effective rights on the resource path}, shape)`. `grant.revoke` (`r.Kind == grant`):
`domain.CheckRevoke` with the existing grant's shape and the effective rights on its target
path. A non-empty reason gives `forbidden` with that reason.

### `Scope(ctx, p, a)`

Allowed only for actions whose requirement is `Level(read)` on a channel-path kind or that are
`space.read`; other actions are an internal error. Liveness and pending rules as in `Authorize`
(a failing principal gets an error of the mapped kind, not an empty scope). Returns the channels
where the effective level is at least `read` (owner: all non-dm channels; dm channels of
participation; agents: plus operated channels), filtered by the app scope of `a` for apps (a
missing scope gives empty lists), after the `ViewAs` intersection; `SpaceIDs` are the visible
spaces. Records `(a, allow)` in the check log.

### Empty sources in `internal/wire` (ruling D2)

Until their owning modules have tables, `internal/wire/empty_sources.go` provides sources that
report the truth of an instance without those tables: no spaces, channels or threads (`ok`
false, empty lists), no agent policy (`ok` false, `DevEnvironmentReady` false), no live session,
device or token (`SessionActive` false, `DeviceState` `unknown`, `TokenActive` false). Each type
carries a doc comment naming the ticket that replaces it (M1-52, M1-53, M1-55; M1-51; M1-33,
M1-34, M1-44). Every request with credentials is therefore `unauthenticated` until M1-34, which
is correct because no credential can exist before it.

### Rulings (D1 to D3, D5 and D6 are in the spec; D4 goes to `docs/DECISIONS.md`)

- D1: `authz.CredentialSource` is a fourth source port; identity implements it. Reason: 07
  section 4.2 checks session, device and token on every call and 04 section 8.5 caches that
  state per rights version; these facts live in identity tables and `PrincipalSource` (M1-26)
  covers only the principal row.
- D2: empty sources in `internal/wire` until the owning modules exist (text above).
- D3: deferred requirement entries (list above) behave like unknown actions and cannot be
  registered.
- D4: visibility per resource kind (table above) as the reading of 07 section 4.3 for resources
  outside the structure tree.
- D5: the snapshot keeps direct grant maxima per target and applies propagation at evaluation
  time; 07 section 7.2 states this (`instance_level`, `level_by_space`, `level_by_channel`,
  `instance_wide`, `fully_visible_space_ids`), so it is in the spec now.
- D6: app scopes come from `p.GrantedScopes` (07 section 3); token scopes are not evaluated until
  M2 defines how they combine with installation scopes; missing read scope on a visible channel
  is `403 scope_missing` (07 section 2.10), and 06 is aligned.

## Tests (write them first)

Pure tests run without a database; the test world and fakes live in
`internal/authz/adapters/fake` and `internal/authz/app/world_test.go`. The clock is
`kernel/clock` with a controllable fake. Test names carry the AZ id where one applies.

1. **Catalogue closure** (`internal/authz/domain`):
   - `TestRequirementsCoverCatalogueExactly` (no missing, no extra entry);
   - `TestRequirementSourceMatchesCatalogue` (`Source` equals the 05 text from M1-25);
   - `TestDeferredActionsGolden` (exactly the twelve ids above);
   - `TestScopeTableActionsExist`, `TestWritesSetGolden`;
   - `TestReasonsMatchChapter07`: reads `docs/spec/07-rechte.md`, extracts the first-column
     codes of the table in section 4.1 (between the headings "### 4.1" and "### 4.2"), and
     compares with the `kernel/auditing.Reason` constants that `authz` uses (R-DM-28 keeps the list
     closed by this test; M1-24 has the same test on its side).
2. **Mapping**: `TestDecisionErrMapping` covers every row of the mapping table.
3. **Test world** of 07 section 11.1 built with the fakes: spaces Support (channels Allgemein,
   Eskalation), Marketing (Kampagnen, Intern), Engineering (forum Projekt-A, text Werkzeuge with
   operator Runner), Research (Notizen); agents Lotse (default operator of Support,
   organization, chat), Texter (Marketing, organization, scoped), Runner (Projekt-A and
   Werkzeuge, organization, host), Assistent (Research, personal, holder Owner, scoped); grants
   G-1 to G-6 exactly as in 07 sections 10 and 11.1; group Leser with member Sam (membership
   with expiry) and G-4; app Tracker installed with scope `messages:write`; DM Alex-Sam; the
   owner acts in a session with a fresh step-up unless a case says otherwise.
4. **Matrix** (`internal/authz/app`), one test function per id with the expected outcome and
   reason of 07 section 11.1: `TestAZM01` to `TestAZM14`, `TestAZM18`, `TestAZM27`,
   `TestAZM28`, `TestAZM29`, `TestAZM31` to `TestAZM34`, `TestAZM40`, `TestAZM41`, `TestAZM50`.
   For AZ-M04 and AZ-M11 the trigger message mentions Lotse.
5. **Mandatory tests**:
   - `TestAZP3_UnknownActionFailsClosed` through `Authorize` (internal error, no allow, one
     ERROR log record, recorder called with decision `error`);
   - `TestAZP8_CacheMatchesReference`: an independent slow reference in the test file computes
     decisions straight from the world (no cache, no snapshot type) following 07 sections 2.5,
     2.7 to 2.10 and 4.2; random worlds from a seeded `math/rand/v2` source (seed printed on
     failure, overridable with `BOTHY_TEST_SEED`) with random grants, groups, memberships,
     expiries, owners, disabled principals, operators and DMs; interleaved mutations (each
     raises the version), clock advances (including landing exactly on an expiry) and queries
     over `message.read`, `message.create`, `thread.read`, `channel.read`, `space.read`,
     `channel.update`, `export.channel`, `file.upload`; decisions, reasons and `Scope` channel
     sets must be equal;
   - `TestAZP10_ExpiryWithoutJob`: warm cache, advance the clock to exactly `expires_at` of a
     grant and of a membership, no version change: `not_found`.
6. **Rules** (`internal/authz/app` and `domain`):
   - owner: `TestSR5_OwnerOnlyInSession` (same owner with a personal token: no owner rights,
     `user.create` gives `forbidden owner_only`, `identity.link` on the own user gives
     `step_up_not_possible`);
     `TestSR6_OwnerNeverSeesDM`;
   - operator rule on a main thread and on a fixed-operator thread after the channel's operator
     changed; bot limits (agent grant read from a mismatching `subject_kind` row is capped);
   - read-only: owner `message.create` gives `channel_read_only`; a member with `manage`
     deleting a foreign message is allowed; a reader gets `level_insufficient`;
   - pending device: `message.read` gives `device_pending`, `self.read` and `device.list` on the
     own device are allowed, `device.list` on another device is `not_found`;
   - step-up: age equal to `auth.step_up_max_age` allowed, one nanosecond more gives
     `step_up_required`; an agent calling `token.create` for itself gives
     `step_up_not_possible`;
   - view as: `TestViewAs_IntersectsAndIsReadOnly` (owner as Alex: Marketing `not_found`,
     Support `message.create` gives `view_as_read_only`, DM of Alex stays `not_found`);
   - acting for: an agent with read on Engineering acting for Alex gets `not_found`
     `acting_for_insufficient` on Engineering;
   - `agent.invoke`: `TestAgentInvoke_*` for each guard step (origin, dm, inactive agent,
     mention rule cases, guest allowed and guest refused for personal or lacking `write`, I2
     refusal and holder allowed, SR2 refusal and owner allowed, project not ready, dev
     environment not ready, active run of another agent);
   - grant guards: `TestGrantCreateGuard` (AZ-M31 through `Authorize`, plus non-owner on
     instance target gives `owner_only`) and `TestGrantRevokeGuard`;
   - `TestScope_*`: owner excludes DMs and includes his DM participation; app without
     `messages:read` gets empty lists; view as intersects; ascending order; a channel added to
     the fake structure of a space with a space grant appears in the next request's `Scope`
     without a new rights version (cached snapshot unchanged).
7. **Cache and version**: version change invalidates; `ValidUntil` boundary; LRU never exceeds
   `authz.snapshot_cache_max_entries` (run with a small value); one version read per request
   scope however many `Authorize` calls (count reads on a fake); pinned version wins.
8. **Audit and check log**: the fake recorder sees one denial call per non-allow decision and
   none for allow; the M1-18 check log contains `(action, outcome)` for `Authorize` and `Scope`.
9. **Registration**: `VerifyRegistrations` rejects a route and an op naming a deferred action.
10. **Wire**: `TestWireAuthorizerWithEmptySources` (PostgreSQL 18 through `pgtest`, one active
    user seeded with plain SQL in the test file): the wired authorizer answers `not_found`
    `no_grant` for that user without credential ids reading an unknown channel, and
    `unauthenticated` `session_revoked` for the same user with a session id.
11. **Instance**: an integration test against PostgreSQL 18 reads the seeded value through
    `instance.Versions`.
12. **Benchmark** `BenchmarkAuthorizeCached` (`internal/authz/app/bench_test.go`): warm cache,
    in-memory fakes, `message.read` on a visible channel; records each iteration's duration and
    reports the 99th percentile with `b.ReportMetric(p99, "p99-ns")`. It asserts nothing; M1-72
    compares it with `perf.authorize_cached_p99`.

Commands that must pass:

```sh
cd services/server && go test -race ./internal/kernel/... ./internal/authz/... ./internal/instance/... ./internal/wire/... ./internal/platform/httpserver/...
cd services/server && go test -race -run '^TestAZ' -v ./internal/authz/...
cd services/server && go test -run '^$' -bench BenchmarkAuthorizeCached -benchtime 20000x ./internal/authz/app
make codegen-check
make arch
make semgrep
make check
```

## Acceptance criteria

- [ ] AZ-M01 to AZ-M14, AZ-M18, AZ-M27 to AZ-M29, AZ-M31 to AZ-M34, AZ-M40, AZ-M41, AZ-M50 are
      green with fake sources under their ids.
- [ ] AZ-P3, AZ-P8 and AZ-P10 are green under their ids; AZ-P1 and AZ-P2 from M1-25 stay green.
- [ ] Every catalogue action has exactly one requirement entry whose source text equals 05; the
      deferred list is exactly the twelve ids named here.
- [ ] The reason constants equal the codes of 07 section 4.1.
- [ ] No source error, unknown action, deferred action or unknown resource kind can produce an
      allow (tests per case).
- [ ] Every non-allow decision reaches the audit recorder; allow decisions do not (the use case
      writes them, 07 section 4.2). AZ-P13 runs end to end: a denied request through the HTTP
      chain leaves exactly one aggregated row in `audit_events` (with the M1-24 recorder against
      PostgreSQL 18).
- [ ] `BenchmarkAuthorizeCached` reports `p99-ns`.
- [ ] 02 sections 4.3 and 4.4, 06 section 4 point 4, 07 sections 4.2 and 7.2 still match the
      build; `docs/DECISIONS.md` has D4 and D5.
- [ ] `authz.snapshot_cache_max_entries` and `auth.step_up_max_age` are in the schema and the
      regenerated configuration reference.
- [ ] `make check` is green.

## Definition of done

Code, tests and documentation are complete: module README updated, doc comments on every public
symbol, an ADR if you took an architecture decision beyond this brief and rulings D1 to D6,
CHANGELOG entry under `Unreleased` in `services/server/CHANGELOG.md`. `make check` is green. An
independent review and the red team (`docs/quality/red-team-gate.md`, with the M1 "Red Team"
points 1, 2 and 5 of `docs/PLAN.md` in mind) have run over the diff with their verdicts recorded
in the report. Merge locally into the base branch only if both are green; then remove the
worktree with `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then the
commits, the files with line counts, the test results with counts (including the red run of each
new test before the code and the benchmark's `p99-ns` value with the machine description), what
is not proven, and follow-ups that could not be done in scope with the reason. Then write
`{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
