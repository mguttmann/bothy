# M1-54: Operators with invariant I1

Status: ready | Depends on: M1-49, M1-53 | Etappe: M1

## Goal

After this ticket owners set the default operator of a space and the operator of a channel
(inherit, none, agent) with step-up, and every such change is a rights change that `authz` checks
against invariant I1 in the same transaction. The real `chat` implementation of the operator part
of `authz.StructureSource` gives the effective operator its implicit `write` (operator rule) and
tells `authz` exactly which targets each agent serves. An integration test with all real sources,
no fakes, proves I1 for every kind of change that M1 can make.

## Read first

- `docs/DECISIONS.md`: B7, invariant I1 and ruling L22 (no owner exception).
- `docs/spec/03-datenmodell.md`: tables `spaces`, `channels` (effective operator, mention rule),
  `threads` (`operator_agent_id`, index for I1 targets), `agents`, `grants`, `owners`;
  "Transaktionsregeln" 6.3 lock order and 6.4 I1 check; R-DM-08, R-DM-12, R-DM-21.
- `docs/spec/05-aktionen.md`: rules 1.6 and 1.7; "Wirksame Stufe" point 4 (operator rule);
  rows `space.operator.set`, `channel.operator.set`, `agent.read`; section 6 rows `space.updated`,
  `channel.updated`, `access.changed`; decisions R-AZ-10, R-AZ-13, R-AZ-17.
- `docs/spec/07-rechte.md`: 2.9 "Agenten als Prinzipale", 4.4 "Invarianten in der Transaktion",
  SR1, SR5, SR8, "Sofortige Wirkung bei Entzug", "Beispiele" (the world of the tests),
  "Testmatrix" rows AZ-M21 to AZ-M25 and AZ-M29, "Pflicht-Tests" AZ-P7.
- `docs/spec/10-backend-richtlinie.md`: "I1 transaktional" 5.1 to 5.4, "Tests" BR-01 to BR-03.
- `docs/spec/09-agenten.md` 3.1 "Bediener".
- `docs/spec/04-sync-protokoll.md` 8.2 "Was beim Ändern passiert" (payload of `access.changed`).
- `docs/spec/02-architektur.md` 4.3 (chat implements `authz.StructureSource`), 4.4, 8.1 phases.
- `docs/spec/06-api.md` "Autorisierung in der API" point 5, "Nebenläufigkeit".
- `docs/api/openapi.yaml`: operationIds `setSpaceOperator`, `setChannelOperator`; schemas
  `OperatorSet`, `OperatorMode`, `ChannelOperator`, `Space`, `Channel`, `Problem`.
- ADRs 0017, 0018, 0024.
- Code of merged tickets: `internal/authz` (M1-28 `authorize` and snapshot, M1-29
  `VerifyInvariants`, `RightsChange` hook and I1 query), `internal/agents` (M1-51,
  `authz.AgentPolicySource`), `internal/identity` (M1-31 `Owners` port), `internal/chat`
  (M1-52, M1-53).

## Fixed requirements that apply

Owner build rules (verbatim):

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

Note on rule 3: `docs/api/openapi.yaml` is the source and the server interface is generated from
it (ADR 0013); `make codegen-check` keeps both identical.

Rules for every brief:

6. Test first: red, then green, then cleanup; keep the red output for the report.
7. No dashes U+2013 or U+2014 in any file you write.
8. White label: only the example names of the specification.
9. Limits and windows only through named configuration keys.

Product requirements implemented here:

10. Operator setting is owner only, with fresh step-up, audited, and a rights change
    (05 rows `space.operator.set`, `channel.operator.set`; R-AZ-10).
11. A channel inherits the space default, has no operator, or a fixed agent (03 `channels`;
    `OperatorSet`). A space has a default agent or none.
12. Operator rule: the effective operator of a channel and the fixed operator of a thread have at
    least `write` there without a grant; nothing remains after a change (05 2.3 point 4, R-AZ-13).
13. I1 is checked transactionally for both actions; a violation rolls back everything, answers
    `409 i1_violation` with hint, and writes an audit deny entry in its own transaction
    (03 6.4, 10 5.4, 07 4.4).
14. Parallel changes that only together break I1 cannot both commit (BR-03).
15. A thread keeps its operator when the channel operator changes (R-DM-12); I1 still counts that
    channel for the thread's agent (BR-02).

## Scope

Paths relative to `services/server/` unless they start with `docs/`:

1. `internal/chat/domain`: `OperatorAssignment{Mode, AgentID}` with validation, and the pure
   function that computes the effective operator of a channel from its columns and the space
   default.
2. `internal/chat` (port) and `internal/chat/app`: use cases `SetSpaceOperator(ctx,
   SetSpaceOperatorCmd) (Space, error)` and `SetChannelOperator(ctx, SetChannelOperatorCmd)
   (Channel, error)`; commands carry the target ID, the `OperatorAssignment`, the reason and the
   expected version from `If-Match`.
3. `internal/chat/adapters/postgres/queries/operators.sql`: updates of
   `spaces.default_operator_agent_id`, `channels.operator_inherit`, `channels.operator_agent_id`;
   effective operator per channel; fixed operator per thread; served targets of an agent (D5);
   the channels of a space that inherit.
4. The chat adapter implementing `authz.StructureSource`: operator rule data and served targets
   (D5, D6).
5. `internal/chat/adapters/httpapi`: handlers `setSpaceOperator`, `setChannelOperator`.
6. `api/operations.yaml`: enable both operations; regenerate.
7. `internal/authz`, only if D5 or D7 finds a missing capability (served spaces in the I1 query,
   change kinds), with tests. Affected principals come from the `RightsChange` of M1-29 (D3).
8. `docs/api/openapi.yaml` already has `Problem.conflicts` (M1-29 regenerated the code); change it
   only where the implementation proves it wrong.
9. `test/integration/` (cross-module tests with all real modules): a world builder for the world
   of 07 "Beispiele" and "Testmatrix" if none exists yet, and the I1 tests below.
10. Documentation: `internal/chat/README.md`, `services/server/CHANGELOG.md`, `docs/DECISIONS.md`
    (rulings D2, D5 point 4, D8).

## Out of scope

- `resolve_i1` (guided switch to organization inside the same request): M2-21. `OperatorSet` has
  no such member yet; an unknown member is rejected by the request decoder (W7, M1-18).
- Agent invocation, trigger decisions, session end with `agent_changed` on the main thread: M2.
- `channel.behavior.*`, mention switches: M2-02. Project binding and runner readiness: M2-03.
- Thread creation and thread moves: M1-55, M1-56, M1-62 (this ticket reads `threads` only).
- Applying `access.changed` on open gateway connections: M1-65.
- Changing the grants, owners or billing mode APIs themselves: M1-29, M1-31, M1-48, M1-51.

## Boundaries

- A new migration needs its line in `migrations.sha256`: run `make migrations-lock` after creating
  the file and before `make check`; never edit a merged migration (M1-01, ADR 0007).
- Work in a dedicated git worktree on the branch `feat/M1-54-chat-operators`, created from the
  base branch the launcher names. One writer per worktree.
- Conventional Commits with a module scope (`feat(chat): ...`, `test(authz): ...`), every commit
  with DCO sign-off (`git commit -s`). Squash to meaningful commits before the merge.
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope list, except `services/server/CHANGELOG.md`,
  `docs/DECISIONS.md` and the module README named above.
- Follow the host rules supplied by the launcher.
- Tests bind only `127.0.0.1` and port 0; PostgreSQL 18 through `pgtest` on a random port; a
  missing container runtime fails the test (P-09).
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at
  the end by their literal path.
- Never read or print secrets or `.env` files. No network calls except package downloads for
  pinned tools.
- Do not lower a threshold, disable a gate, skip or delete a test.

## Design notes

D0. Start check. `make check` green on the fresh worktree, else `Verdict: BLOCKED base branch red`.
M1-49 and M1-53 merged, else `Verdict: BLOCKED dependency missing`.

D1. Request and validation (`OperatorSet {mode, agent_id?, reason}`):

- `mode = agent` requires `agent_id`; `agent_id` with another mode gives `400 validation_failed`
  (`/agent_id`, `not_allowed_for_mode`). `reason` must not be blank.
- Space: `inherit` gives `400` (`/mode`, `inherit_only_for_channels`); `none` sets
  `default_operator_agent_id` NULL; `agent` sets it.
- Channel: `inherit` sets `operator_inherit = true, operator_agent_id = NULL`; `none` sets
  `false, NULL`; `agent` sets `false, agent_id`.
- `agent_id` must be a principal of kind `agent` with status `active` and a row in `agents`
  (check through the `identity` port; chat may use `identity`, 02 4.4), else `400`
  (`/agent_id`, `agent_not_active`). Owners can read every agent, so this leaks nothing.
- `kind = 'dm'` channels cannot reach this code in M1; reject defensively with `400`.

D2. Order of checks: load the target (unknown gives `404`), `authorize` (non-owner without read
`404`, non-owner with read `403 forbidden` with `reason` `owner_only`, owner personal API token `403`, stale step-up
`401 step_up_required`), then `If-Match` (`428` missing, `412 version_conflict` with `current`),
then validation of D1, then state: an archived space or channel gives `409 conflict`
(ruling of M1-53: archived targets are frozen). A request that changes nothing returns `200` with
the current object, no rights change, no version bump, no event; the audit allow entry is written
as for every audited action.

D3. The write transaction: unit of work with the `RightsChange` option (02 8.1), so
`permission_version` is locked first (03 6.3). Affected principals for `access.changed`: every agent
whose effective operator status changes on any channel (old and new operators; for a space the
old and new default for all non-archived channels with `operator_inherit = true`). Pass the change
kind (`space.operator.set` or `channel.operator.set`) and the new operator to
`VerifyInvariants`; the personal agents to check are the new effective operator if its
`billing_mode` is `personal` (10 5.1); `authz` reads the billing mode through
`AgentPolicySource`. Domain rows are locked in ascending `id`.

D4. Events and versions:

- `setChannelOperator`: `channels.version` plus one, `channel.updated` with payload `{channel}`
  whose `operator` carries `mode`, `agent_id`, `effective_agent_id`.
- `setSpaceOperator`: `spaces.version` plus one, `space.updated` with payload `{space}`; then for
  every non-archived channel of the space with `operator_inherit = true`, in ascending `id`:
  `channels.version` plus one and `channel.updated`, because its effective operator changed and
  clients apply a change only with a higher `resource.version` (05 section 6 row `space.updated`;
  04 "Zustellgarantie" point 3).
- No `thread.updated`: thread operators are fixed (R-DM-12); the main thread has no stored operator
  (R-DM-21) and clients read it from `Channel.operator.effective_agent_id`.
- Audit allow details `{target_kind, target_id, old_mode, old_agent_id, new_mode, new_agent_id,
  reason}`.

D5. Served targets of agent A (implemented in chat's `StructureSource` adapter, used by the I1
query of `authz`; SQL pattern from 10 5.3):

1. Non-archived channels whose effective operator is A.
2. Channels with at least one thread `kind = 'thread'` and `operator_agent_id = A`, open or closed,
   whatever the channel's current operator (R-DM-12, BR-02).
3. The spaces of these channels.
4. Non-archived spaces whose `default_operator_agent_id = A`, even without channels (03 6.4 point
   3, 10 5.2 point 3 and its query in 10 5.3). Reason: baseline I1 counts "any target this agent
   serves"; without this point a grant of `trigger` on an empty space would pass, and a later
   `createChannel` there (not an I1 action) would make A serve a channel on which that principal
   has `trigger`. If `authz`'s I1 query only accepts served channels, extend it to these spaces.
5. The instance (always).
Guest channels (`allow_mentioned_agents`) never count (10 5.2).

D6. Operator rule data for the snapshot and `authorize` (07 7.2 step 4): for agent principal P,
the channel IDs where P is the effective operator (non-archived) and the thread IDs with fixed
operator P. `authorize` computes the rule from the freshly resolved path as well (M1-28, 07 7.2), so a
channel that starts inheriting P is writable for P at once. On archived channels the operator rule
gives nothing beyond `read` (M1-53 D2).

D7. I1 error answer: `409`, problem code `i1_violation`, `reason: i1_violation`, `hint` meaning
"switch the agent to organization first", and `conflicts`: per agent `agent_id`,
`credential_holder_id` (absent if the agent has none), `principal_ids` (the other principals that
would gain `trigger`; schema `InvariantViolation` in `openapi.yaml`, 10 5.4), produced by the
mapping M1-29 built (`Problem.conflicts`). Names of other principals never appear in the body.

D8. Ruling recorded in `docs/DECISIONS.md`: `OperatorSet` does not accept `resolve_i1` before
M2-21, although 05 rule 1.6 names it for both actions; the guided flow is an M2 feature.

## Tests (write them first)

Integration with real sources for M1-49 (PLAN row M1-54): `explainSpaceAccess`,
`explainChannelAccess`, `previewGrant` and `viewAsUser` against real spaces, channels, operators
and agents (no fakes): an operator gives a source `operator`, a personal agent that reads another
channel gives `via_agent` with level `none`, the preview of a `trigger` grant on a space whose
default operator is a personal agent shows the I1 violation, and view-as lists exactly the
channels the person reads.

Unit (`internal/chat/domain`):

- `TestOperatorSet_Validation`: table over modes, missing and superfluous `agent_id`, `inherit` on
  a space.
- `TestEffectiveOperator`: inherit with and without space default, none, agent.

Integration with all real sources (`test/integration/`, PostgreSQL 18, real `identity`, `authz`,
`agents`, `chat`, `audit`, `sync`; world of 07 "Beispiele": owner, Alex, Sam, Robin; spaces
Support, Marketing, Engineering, Research; agents Lotse (organization, operator of Support), Texter
(organization, operator of Marketing), Runner (organization), Assistent (personal, credential
holder = owner, operator of Research); grants G-1 to G-6 created through the `authz` admin port, not
through SQL):

- `TestAZ_M21_GrantTriggerInPersonalSpace`: `grant.create` Sam `trigger` on Research gives
  `i1_violation`.
- `TestAZ_M22_SwitchToPersonalWithForeignTrigger`: `agent.billing_mode.set` personal while Alex
  has `trigger` on a served channel gives `i1_violation`.
- `TestAZ_M23_SecondOwnerWithPersonalAgent`: `owner.add` Alex gives `i1_violation`.
- `TestAZ_M24_ChannelOperatorSetViolatesI1`: `channel.operator.set` Assistent on Support/Allgemein
  where Alex has `trigger` gives `409 i1_violation` with `hint` and `conflicts`.
- `TestAZ_M25_GroupMemberAddViolatesI1`.
- `TestAZ_M29_OperatorRuleGivesWrite`: Texter writes in Marketing/Kampagnen without a grant
  (`authorize(message.create)` allowed; test through `authorize` because messages come in M1-56).
- `TestBR_01_OperatorActions`: both actions with a violating change give `409`, all rows unchanged
  (versions, operator columns, `permission_version`), and one audit deny entry with
  `deny_reason = i1_violation` exists after the rollback.
- `TestBR_02_ClosedThreadWithFixedPersonalOperator`: insert through the chat repository a closed
  thread `kind = 'thread'` with `operator_agent_id = Assistent` in a channel now operated by Lotse;
  `grant.create` Sam `trigger` on that channel gives `i1_violation`.
- `TestBR_03_ParallelChangesSerialized`: transaction A grants Sam `trigger` on a channel operated by
  an organization agent; transaction B sets that channel's operator to Assistent; start both behind
  a barrier; exactly one commits, the other gets `409 i1_violation`; repeat with the order swapped.
- `TestI1_SpaceDefaultOperatorServesSpace`: (a) empty space with default Assistent, then
  `grant.create` Sam `trigger` on it gives `i1_violation`; (b) grant first, then
  `setSpaceOperator` Assistent gives `i1_violation`; (c) after a valid setup, `createChannel` in the
  space keeps I1 (the I1 query over all personal agents returns exactly the holder).
- `TestSetOperator_RightsAndPreconditions`: non-owner with read `403`, outsider `404` identical to
  an unknown ID, stale step-up `401`, personal token of the owner `403`, `428`, `412` with `current`,
  archived target `409`, inactive or non-agent `agent_id` `400`.
- `TestSetSpaceOperator_InheritingChannelsUpdated`: inheriting channels get a version bump and
  `channel.updated` with the new `effective_agent_id`; channels with own operator unchanged.
- `TestSetOperator_AccessChangedForOldAndNewOperator`: old operator loses implicit `write`
  (`removed` or `changed`), new operator gains it (`added` or `changed`), `permission_version` plus
  one.
- `TestSetOperator_NoOp`: same assignment gives `200`, no event, no version change.
- `TestAZ_P7_OperatorKinds`: the AZ-P7 case table of M1-29 runs green against the real sources for
  both operator actions.

Commands that must pass:

```
cd services/server && go test -race ./internal/chat/... ./internal/authz/... ./test/integration/...
make codegen-check
make arch
make api-lint
make api-checks
make register-check
make check
```

## Acceptance criteria

- [ ] `setSpaceOperator` and `setChannelOperator` are enabled, owner only, step-up, `If-Match`,
      audited, and answer as D2.
- [ ] Every operator change bumps `permission_version`, runs `VerifyInvariants`, and writes
      `access.changed` for old and new operators.
- [ ] AZ-M21 to AZ-M25, AZ-M29, BR-01, BR-02, BR-03 are green with all real sources.
- [ ] Served targets include spaces whose default operator is the agent (D5 point 4), proven by
      `TestI1_SpaceDefaultOperatorServesSpace`.
- [ ] The I1 problem body carries `hint` and `conflicts`, and no foreign names.
- [ ] `channel.updated` and `space.updated` carry the operator; inheriting channels get their own
      event with a higher version.
- [ ] Explanation, preview and view-as are green with real sources.
- [ ] Rulings recorded in `docs/DECISIONS.md`; `make check` is green.

## Definition of done

- Code, tests and documentation in the same change: `internal/chat/README.md` current (operator
  use cases, source methods), doc comments on every public symbol, comments explain why.
- No architecture decision beyond the rulings; otherwise an ADR (MADR 4, next free number).
- `services/server/CHANGELOG.md` entry under "Unreleased".
- `make check` green on the final commit.
- Local review (`make review` plus a code review over the diff); red team by a fresh read-only
  reviewer with `docs/quality/red-team-gate.md` over `git diff <base>...HEAD`, with special
  attention to "can any sequence of grant, group, owner, operator and policy changes break I1,
  also in parallel"; record the verdict line. On `NEIN` fix, test, re-check the delta.
- Merge locally into the base branch only with clean review, `Urteil: JA` and green
  `make check`; then remove the worktree.

## Report

Write `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits and files with line counts.
3. Test results with counts, red output of the first failing tests, list of the specification IDs
   now green (AZ-M21 to AZ-M25, AZ-M29, BR-01 to BR-03).
4. Review and red team verdicts.
5. What is not proven (for example cross-process effect on open connections, M1-65).
6. Only real owner decisions, each with a recommendation.

Then write `{{DONE_PATH}}` with `exit=0 sha=<merge sha>` or `exit=1 <reason>`.
