# 07 Permissions and authorization

Status: draft M0. Normative for the module `authz`, for every route, every WebSocket op, every
delivery and every agent invocation.
Terms: [00-glossary.md](00-glossary.md). Tables: [03-data-model.md](03-data-model.md).
Events and catch-up: [04-sync-protocol.md](04-sync-protocol.md). Actions, receive actions
and rate-limit classes: [05-actions.md](05-actions.md). Identity, sessions and step-up:
[08-identity.md](08-identity.md). Threats: [../security/threat-model.md](../security/threat-model.md).
Decisions: ADR 0017 (one authorization function), ADR 0018 (grant model).

This chapter defines how `authorize(principal, action, resource) -> Decision` decides. The
action catalogue (which action needs which level) is in 05 and is not repeated here.
The additions requested during drafting are now in the core contract (section 13).

## 1. Principles

1. **The default is nothing.** Without a valid grant, owner status or DM participation, a principal
   has no level on a target. There are no deny rows; the highest valid level applies.
2. **One truth.** Only `authz` interprets grants. Other modules get decisions or the
   set of visible channels, never grant rows (03, principle 12).
3. **Time is checked on every call.** Expiry dates of grants and group memberships
   take effect from the second of expiry, without a job having to run. Jobs only report the expiry.
4. **Permissions apply at delivery.** What a recipient gets is decided by `authorize` at the
   moment of delivery, not at the moment of the event (04, section 8).
5. **Fail closed.** Every error in `authz` (database unreachable, unknown action,
   unknown resource type) ends in denial, never in permission.
6. **Guard rules rank above grants.** Section 5 names rules that no grant overrides.
7. **Whoever may trigger an agent reaches through it everything the agent can read.** Therefore
   `authz` binds agent invocations to the permissions of the triggering person (guard rule SR9)
   and shows the read horizon of every agent openly (section 6).

## 2. Grant model

### 2.1 Subjects

| Subject | Code | May receive grants | Highest level | Capabilities |
|---|---|---|---|---|
| Person | `user` | yes | `trigger` | all |
| Group | `group` | yes (applies to its valid members) | `trigger` | all |
| Agent | `agent` | yes | `write` (R-DM-08) | only `upload` (R-DM-20) |
| App | `app` | yes | `write` (R-DM-08) | only `upload` (R-DM-20) |

Groups contain only people (R-DM-07). Groups are not acting principals.

Rationale for "agents and apps only `upload`": `approve_actions` on an agent would mean that an
agent could confirm its own approval cards. `manage` would mean that an agent issues grants
and reads channel prompts. `export` would mean that an agent pulls histories in bulk. An agent
needs none of these for its work. The table `grants` enforces this with a check (03,
R-DM-20).

### 2.2 Targets and inheritance

Targets are `instance`, `space` and `channel` (03, `grants.target_kind`). Threads and everything in
them have no grants of their own; they inherit from the channel. Hosts (`host`) are a fourth target
outside this tree (section 2.12). The path of a resource is in 05,
section 2.2.

```text
instance
  └── space
        └── channel (text or forum)
              └── thread
                    └── message, run, run_step, component, interaction, agent_session
DM channel: outside the tree, only dm_participants
```

A grant on a target applies to the target and everything below it. On a channel, the highest
level of all valid grants on the channel, space and instance applies. A channel can therefore never
allow less than its space. Whoever needs "space Support except channel Eskalation" gives grants on
the individual channels instead of on the space (ADR 0018).

### 2.3 Levels

Levels are ascending and include the lower ones: `read` < `write` < `trigger`.

| Level | Meaning (short form; the column "Permission" in 05 is authoritative) |
|---|---|
| `read` | see space, channel, threads, messages, files, runs; search; maintain own read state, notifications |
| `write` | in addition send, edit and delete messages (own), create, close and reopen threads, reactions, drafts, typing |
| `trigger` | in addition trigger agents, operate components, run commands, cancel own runs, start a new agent session or bind one |

`write` never triggers an agent. Only `trigger` does (study C, adopted in Baseline
section 3).

### 2.4 Capabilities

| Capability | Purpose | Also requires |
|---|---|---|
| `upload` | upload and attach files (`file.upload`, attachments to `message.create`) | `write` |
| `approve_actions` | operate components with `required_capability = 'approve_actions'` (approval cards) | `trigger` |
| `export` | export the channel history (`export.channel`) | `read` |
| `manage` | administration of the target: create and change channels, behavior, tags, pins, delete other people's messages, move threads, incoming webhooks, "Who has access?", grants within one's own scope (section 2.6) | any level |
| `terminal` | open a terminal on a host (`terminal.open`, `terminal.attach`); only on the target `host`, only for people and groups (section 2.12) | no level; SR10, SR11 and SR12 |

Capabilities are the union over all valid grants of the principal and its valid
groups on the path (05, section 2.3). A capability never replaces the required level.

### 2.5 Validity of a grant

A grant `g` is valid at time `t` if and only if:

- `g.revoked_at IS NULL`,
- `g.expires_at IS NULL OR g.expires_at > t`,
- the subject is active (`principals.status = 'active'`), for groups: `groups.deleted_at IS NULL`,
- for group grants for person `u`: there is a membership `m` with `m.removed_at IS NULL`
  and `m.expires_at IS NULL OR m.expires_at > t`,
- for app subjects: the installation is `active`.

Boundary: at `expires_at = t` the grant is already invalid. `t` is the server clock, read once per
`authorize` call, so that all checks of one call see the same time.

Grants are never changed, only revoked and created anew (R-DM-09). An extension is
therefore a revocation plus a new grant with a new reason.

### 2.6 Delegation with `manage`

Whoever has `manage` on a target `Z` may create and revoke grants on `Z` and below it, with
these limits (R-AZ-05, made precise):

1. The level of the new grant is at most one's own effective level on the target of the new grant.
2. The capabilities of the new grant are at most one's own effective capabilities there (`manage`
   included; whoever has `manage` may pass it on).
3. Revocation only for grants that one could have created oneself under 1 and 2. Otherwise a
   person with `write` and `manage` could remove the stronger grant of another person that the
   owner set (05 `grant.revoke`, R-AZ-05).
4. Subjects only people and groups. Only the owner creates grants to agents and apps.
   Reason: a grant to an agent extends what its runtime reads and sends to an
   LLM provider. That is a data flow to a processor and a change of the
   read horizon ([20-privacy.md](20-privacy.md), section 6). (05 `grant.create`.)
5. Only the owner creates grants on the instance.
6. Reading (`grant.read`): owners see all grants. Whoever has only `manage` sees the grants on
   targets on which they have `manage`, and below them; filters by target or subject narrow
   this set, they never widen it. A filter on a subject therefore does not show its
   grants outside one's own scope.

Memberships in groups with `source = 'oidc_claim'` are determined solely by the group claim of the
provider ([08-identity.md](08-identity.md), section 3.4); `group.member.add` and
`group.member.remove` on such groups end with `409 conflict` and `reason:
claim_managed_group`, because the next login would otherwise silently overwrite a manual change.

### 2.7 Owner

- Owners have the level `trigger` and all capabilities on every target except DM channels (05,
  section 2.3), independent of grants.
- Only active owners may perform owner actions (column "Owner" in 05).
- At least one active owner always remains (guard rule SR8).
- The first owner is created only by a server command ([08-identity.md](08-identity.md),
  section 4). Further owners only by `owner.add` through an owner with step-up.
- Owners do not read other people's direct messages (R-AZ-02, guard rule SR6).

### 2.8 Direct messages

DM channels have no space and no grants (R-DM-10). Participants with `left_at IS NULL`
have `write` and `upload`. Nobody else sees the channel, not even an owner. There are no agents
in DM channels.

### 2.9 Agents as principals

1. Agents receive grants like people, at most `write` and only `upload` as a capability.
2. **Operator rule.** An agent has at least `write` on every channel whose effective operator it
   is, and on every thread with `threads.operator_agent_id` equal to it, without a grant row.
   Reason: without this rule, every operator change would have to entail a second, easily forgotten
   grant; on a change, the old grant would remain in place. The rule follows the operator
   automatically (05, section 2.3 item 4; R-AZ-13).
3. Agents never trigger by themselves. Their messages have `origin = 'agent'` (guard rule SR4).
4. Agents perform no step-up actions (R-AZ-03).
5. Agents implicitly have all scopes except `users:read` and `costs:reconcile` (05, section
   2.6); their limit is the grants and the operator rule.
6. If an agent acts within a run, the acting-for binding (SR9) applies in addition.

### 2.10 Apps

An app needs both for every action:

1. an effective level from grants (at most `write`) and
2. the scope of the action in `app_installations.granted_scopes` (05, section 2.6) and, when the
   request carries an API token, also in its `api_tokens.scopes` ([08-identity.md](08-identity.md),
   section 12).

An app without an active installation (suspended or uninstalled) has an empty permission state,
like a principal that is not active: `app.suspend` and `app.uninstall` take every visible target
from it, `app.resume` gives them back, and the permission change writes `access.changed` for it.

Visibility follows the level: if an app has a grant on a channel, the channel is visible to it.
If it lacks only the scope, the response is `403` with reason `scope_missing`, not `404`;
the app knows the channel from its grant anyway. Incoming webhooks write as their app, with
`origin = 'app'`, and never trigger.

### 2.11 Tool class of an agent

Permissions govern who reaches an agent. What the agent can then do is determined by its runtime.
So that the server can draw a limit for this, every agent has a tool class:

| Class | Code | What the runtime may do | Worst abuse |
|---|---|---|---|
| Conversation | `chat` | reply, read the thread context, no effect outside the thread | wrong or unwanted text in the thread |
| Scoped | `scoped` | named tools with their own, minimal credentials; effect on the outside only after an approval card | one named, limited effect |
| Host | `host` | shell, free file system or free network access with the permissions of the runtime user (for example a coding agent with tool permission prompts switched off) | takeover of the host on which the runtime runs, and of all secrets reachable there |

Data model (03, `agents`): `tool_class text NOT NULL DEFAULT 'chat' CHECK (tool_class IN
('chat','scoped','host'))` and `reported_tool_class text NULL`. Setting it is the action
`agent.tool_class.set` (Owner, Step-up, Audit, Permissions `yes`).

Configured and reported value as with the backend policy: the invocation token carries `tool_class`
(configured). The adapter reports its actual class in the heartbeat and starts no run if the actual
class is higher than the configured one (reason `tool_class_mismatch`). The order `chat` < `scoped` <
`host` is authoritative here. The protocol is defined in [09-agents.md](09-agents.md); the claim
`tool_class` is in 05, section 5.

### 2.12 Hosts and the `terminal` capability

Normative together with [24-terminal.md](24-terminal.md), section 8.

1. A host is a target of its own. It inherits nothing from instance, space or channel; a grant
   on the instance gives no terminal (R-AZ-24).
2. On a host there is no level and exactly one capability: `terminal`. Recipients are only
   people and groups (03, R-DM-35). Only the owner creates grants there.
3. Effective `terminal(P, H)` for a person `P` on the host `H`:
    - `P` is an active owner in a session, and not (`H.llm_credentials = 'personal'` and
      `P <> H.credential_holder_id`); or
    - a valid grant (2.5) with target `H` and `terminal` to `P` or one of their valid
      groups.

    The exception of the first item (an owner on a host with `personal`) is decided by the
    guard rule SR11 with the reason `i6_holder_only` (section 4.2, step 9; M2-40); the atom
    `+terminal` of the requirement does not repeat it (docs/DECISIONS.md D-M2-39-3).

4. Effective `terminal` is not enough: SR10, SR11 and SR12 (section 5) block afterwards. The preview
   of a grant that does not take effect because of SR10 or for lack of a session user (SR12) says
   so (hint `terminal_not_effective`).
5. Reading a host (`host.read`): owners, the host assignee (`hosts.assigned_user_id`) and
   anyone with a valid grant `terminal` on it, even if SR10 blocks it (so that the
   person sees the reason). Otherwise `404`.

## 3. Principal context

`authorize` receives not just an ID, but the verified context of the request. The
authentication ([08-identity.md](08-identity.md)) builds it; `authz` trusts only this context.

| Field | Content | Source |
|---|---|---|
| `id`, `kind` | principal and kind (`user`, `agent`, `app`, `anonymous`, `host`); `host` is a host, logged in through its login key ([24-terminal.md](24-terminal.md), section 4.1), without a row in `principals` | token, or login of the host |
| `session_id` | `auth_sessions.id` (people only) | `sid` in the access token or console cookie |
| `device_id` | device (people only) | token, DPoP key |
| `step_up_at` | last fresh confirmation of the session | `auth_sessions.step_up_at` |
| `token_id`, `token_scopes` | API token and its scopes (agents, apps, personal tokens) | `api_tokens` |
| `installation_id`, `granted_scopes` | apps only | `app_installations` |
| `acting_for` | triggering person, run, thread and `jti`, if an agent acts within a run | verified invocation token in the request header (guard rule SR9); the header is `Invocation-Token` ([06-api.md](06-api.md), sections 3 and 4) |
| `view_as` | person whose view the owner is currently looking at | session with active "View as person" (section 9.2) |
| `request_time` | one server time per call | clock |

A personal API token (switch `tokens.personal_enabled`) acts as its person, but without a
session. It can therefore never provide step-up.

For the check without a request, the server builds the context itself, not the authentication:
`id` and `kind = user` of the person and `request_time`, without all other fields (05, section 5,
"Check without a request"; R-AZ-29).

## 4. The `authorize` function

### 4.1 Decisions and reasons

`authorize` returns a decision from 05, section 2.5, and on denial a reason code.
The code is in the audit log (`audit_events.deny_reason`), and in error responses only where it
reveals nothing (never with `404`).

| Reason | Decision | Meaning |
|---|---|---|
| `unauthenticated` | `unauthenticated` | no valid principal |
| `principal_inactive` | `unauthenticated` | principal `disabled` or `deleted` |
| `principal_pending` | `forbidden` | provisioned person in state `pending` ([08-identity.md](08-identity.md), section 3.1); problem code `device_pending`; the same actions are allowed as for a waiting device |
| `session_revoked` | `unauthenticated` | session revoked or expired |
| `device_inactive` | `unauthenticated` | device `revoked` or key revoked |
| `device_pending` | `forbidden` | problem code `device_pending`; the device is waiting for approval; only `self.read`, `device.list` for the own device, `auth.token.refresh`, `auth.logout` are allowed (05, section 2.5) |
| `token_revoked` | `unauthenticated` | API token revoked or expired |
| `app_not_installed` | `unauthenticated` | app installation not `active` |
| `no_grant` | `not_found` | no read permission, or the resource is missing; also an app command whose app is not reachable (installation not `active`, no `commands:write`, or no `write` on the channel where one is resolved), answered like an unknown command ([09-agents.md](09-agents.md) section 10.3, M2-25) |
| `level_insufficient` | `forbidden` | level too low |
| `capability_missing` | `forbidden` | capability missing |
| `scope_missing` | `forbidden` | app without the scope |
| `owner_only` | `forbidden` | action only for owners |
| `not_self` | `forbidden` or `not_found` | the resource belongs to someone else (device, setting); `not_found` if the principal may not see it |
| `subject_kind_not_allowed` | `forbidden` | agent or app not allowed as actor or recipient |
| `delegation_exceeds_own_level` | `forbidden` | delegation beyond one's own level |
| `delegation_exceeds_own_capabilities` | `forbidden` | delegation beyond one's own capabilities |
| `step_up_required` | `step_up_required` | fresh confirmation missing |
| `step_up_not_possible` | `forbidden` | token without a session (agent, app, personal token); also `createFileTicket` without a login session, because a download ticket binds to a login session ([12-files.md](12-files.md) section 9.2) |
| `view_as_read_only` | `forbidden` | writing action during "View as person" |
| `i1_violation` | `invariant_violation` | the change would violate I1 |
| `last_owner` | `invariant_violation` | the change would leave no active owner |
| `origin_not_human` | `forbidden` | triggering by an agent, app, import or the system |
| `agent_not_reachable` | `forbidden` | the agent is not the operator, not mentioned, or the mention is not allowed |
| `billing_mode_personal_not_holder` | `forbidden` | I2 |
| `tool_class_owner_only` | `forbidden` | SR2 |
| `dev_environment_not_ready` | `forbidden` | SR3 |
| `dev_environment_foreign_thread` | `forbidden` | SR3, the thread runs in someone else's environment |
| `project_not_ready` | `forbidden` | project binding not ready (05, section 5, item 7) |
| `channel_read_only` | `forbidden` | channel read-only; writing and triggering actions blocked, even for owners (05, rule 1.8); problem code `channel_read_only` |
| `channel_archived` | `forbidden` | channel or its space archived; writing and triggering actions blocked, even for owners (section 4.5); problem code `channel_read_only` with `reason` |
| `run_active_other_agent` | `forbidden` | the thread has an active run of another agent (05, section 5, item 8) |
| `run_active` | `forbidden` | model command during an active run (05 `command.invoke`); also `interaction.create` on a component without run while the card's agent has an active run in the thread, which would need a second run (R-DM-14, [09-agents.md](09-agents.md) section 11.2, M2-15); problem code `agent_invocation_denied` with `reason` |
| `billing_mode_mismatch` | `invariant_violation` | `run.start` with a reported mode not equal to the configured mode (I3); problem code `billing_mode_mismatch` (`409`) |
| `tool_class_mismatch` | `invariant_violation` | the adapter reports a higher tool class than the token; problem code `tool_class_mismatch` (`409`) |
| `invocation_token_invalid` | `forbidden` | invocation token invalid, expired, not the current one, or consumed (`run.start`, `run.token.renew`); `reason` `expired`, `superseded` or `invalid` ([09-agents.md](09-agents.md), section 4.5); also every request with `Invocation-Token` (SR9), with `reason` `invalid` (token, run, agent or person wrong, or the header repeated, empty or sent by a principal that is not an agent) or `run_inactive` (the run of the token has ended) |
| `import_not_running` | `forbidden` | `import.apply` without a running source or with a target outside its channels |
| `malware_detected` | no `authorize` result | reason of the audit entry for `file.upload` on a malware finding ([12-files.md](12-files.md), section 5.2) |
| `component_inactive` | `forbidden` | component used, expired or deactivated; also a component whose run has ended, and a component of an app without an active installation, without `components:write` or without `write` on the channel (M2-25). Answered `409 component_used` without the reason, which stays in the audit, so a lost race answers the same whether `authorize` or the single-use lock decided it (09 section 11.2, M2-15) |
| `component_restricted` | `forbidden` | `restricted_to = 'run_starter'` and the principal is neither the triggering person nor an owner; also `restricted_to = 'owner'` and the principal is not an effective owner ("Always allow", 09 section 11.5, M2-15); problem code `forbidden` with `reason` |
| `acting_for_insufficient` | `not_found` or `forbidden` | acting-for binding: the triggering person does not have the permission; `not_found` when the target is visible with the agent's own rights but not with the intersection (answered like an unknown ID), `forbidden` (problem `forbidden` with `reason: acting_for_insufficient`) when the requirement holds with the agent's own rights but fails with the intersection; if the agent alone already fails, the agent's own reason stays |
| `terminal_not_host_assignee` | `forbidden` | SR10: a non-owner on a host whose assignee they are not; problem code `terminal_denied` with `reason: not_host_assignee` |
| `terminal_server_host_owner_only` | `forbidden` | SR10: server host only for owners; problem code `terminal_denied` with `reason: server_host_owner_only` |
| `i6_holder_only` | `forbidden` | SR11: host with a personal LLM login, the principal is not its holder; problem code `terminal_denied` with `reason: i6_holder_only` |
| `i6_violation` | `invariant_violation` | the change would violate I6; problem code `i6_violation` (`409`) |
| `terminal_no_session_user` | `forbidden` | SR12: the person has no valid mapping on the host to an own session user that the host reports, even as an owner; problem code `terminal_denied` with `reason: no_session_user` |
| `terminal_owner_account_owner_only` | `forbidden` | SR12 with ADR 0073: the person's session user on the host is the owner account (the instance owner's own account with administrator rights), and the person is no active owner; problem code `terminal_denied` with `reason: owner_account_owner_only` |
| `terminal_profile_unknown` | `forbidden` | profile not in `hosts.reported_profiles`, or an index of no working directory of the own session user (`hosts.reported_session_users`); problem code `terminal_denied` with `reason: profile_unknown` |
| `terminal_session_limit` | `forbidden` | `terminal.max_sessions_per_person` or `terminal.max_sessions_per_host` reached; problem code `terminal_denied` with `reason: session_limit` |
| `terminal_step_up_not_fresh` | `step_up_required` | step-up older than `terminal.step_up_max_age`, already consumed or (console) not by passkey; in the console the quick reattach without step-up never applies |
| `host_not_active` | `forbidden` | host not `active` or host service not connected; problem code `host_unavailable` (`409`) |
| `host_auth_failed` | `unauthenticated` | login of a host with an unknown key or a wrong signature |
| `host_duplicate_connection` | `forbidden` | a second live control connection with the same keys of a host whose credentials are valid (24 3.4, M2-43); the host is disabled or falls back to `pairing`, both connections close with `4112` |
| `unknown_action` | error `500` | program error, alert |
| `trusted_header_requires_loopback` | no `authorize` result | Reason of the audit entry of `identity_provider.manage` when enabling the driver `trusted_header` is refused because the public listener is not bound to loopback; problem code `conflict` (`409`) with `reason: trusted_header_requires_loopback` ([08-identity.md](08-identity.md), section 2.4, PLAN W4) |

### 4.2 Flow

```text
authorize(p, action, ref) -> Decision:
  t    := now()                                      # one time per call
  spec := catalog[action]
  if spec missing:          alarm(); return ERROR(unknown_action)

  # 1. Who is acting? (blocking checks)
  if p.kind == anonymous:
      return spec.right == public ? ALLOW : DENY(unauthenticated)
  d := check_active(p, t)                            # status, session, device, token, installation
  if d: return d                                     # unauthenticated with a reason
  if p.status == pending and action not in PENDING_ALLOWED:
      return DENY(forbidden, principal_pending)      # provisioned, 05 section 2.5
  if p.device_state == pending and action not in PENDING_ALLOWED:
      return DENY(forbidden, device_pending)         # 05, section 2.5
  if p.acting_for.status == invalid:                 # SR9, before resolve: unknown and invisible IDs alike
      return DENY(forbidden, invocation_token_invalid, detail invalid)
  if p.acting_for.status == run_inactive and action not in SR9_UNBOUND:
      return DENY(forbidden, invocation_token_invalid, detail run_inactive)   # SR9

  # 2. Permission snapshot (section 7)
  snap := snapshot(p, t)                             # cache per (p, permission_version), valid until snap.valid_until

  # 3. Resource and path to the target
  res := resolve(ref)                                # path thread -> channel -> space -> instance, DM flag, owning principal
  if res missing: return DENY(not_found, no_grant)

  # 4. Effective permissions on the path
  eff := effective(snap, res.path)                   # owner, DM, grants, groups, operator rule,
                                                     # highest level, union of the capabilities,
                                                     # limit for agents and apps
  if p.acting_for.status == bound and sr9_applies(action, p.acting_for, res) and not delivery_mode:
                     eff := eff ∩ effective(snapshot(p.acting_for.user, t), res.path)   # SR9
  if p.view_as:      eff := eff ∩ effective(snapshot(p.view_as, t), res.path)            # section 9.2

  # 5. Visibility: no read permission results in 404
  if not visible(spec, p, eff, res): return DENY(not_found, no_grant | acting_for_insufficient)

  # 6. Requirement from the catalogue (columns "Permission", "Owner")
  d := requirement(spec, p, eff, res)                # level, capability, scope, self, owner, participant
  if d: return d                                     # forbidden with a reason

  # 7. No writing in "View as person"
  if p.view_as and spec.writes: return DENY(forbidden, view_as_read_only)

  # 8. Step-up
  if spec.step_up == yes or (spec.step_up == conditional and has_credentials(p, action)):
      if p.kind != user or p.session_id missing: return DENY(forbidden, step_up_not_possible)
      if p.step_up_at missing or t - p.step_up_at > auth.step_up_max_age:
          return DENY(step_up_required)
  if spec.step_up == conditional and (p.kind != user or p.session_id missing):
      return DENY(forbidden, step_up_not_possible)   # first setup only in a session
  if spec.step_up == fresh:                          # 05 section 2.4; 08 section 6.3
      if p.kind != user or p.session_id missing: return DENY(forbidden, step_up_not_possible)
      if not quick_reattach(p, action, res, t):      # only terminal.attach, never in the console
          if p.step_up_at missing or t - p.step_up_at > terminal.step_up_max_age
             or p.step_up_method not in {passkey, device_key}
             or (p.client_component == console and p.step_up_method != passkey with userVerification)
             or (p.step_up_consumed_at set and p.step_up_consumed_at >= p.step_up_at):
              return DENY(step_up_required, terminal_step_up_not_fresh)
          # the confirmation is consumed in the transaction of the action (08 section 6.3)

  # 9. Guard rules of the action (section 5)
  d := guards[action](p, res, eff, snap, t)           # for example agent.invoke: SR1 (I2), SR2, SR3, SR4
  if d: return d

  return ALLOW
```

SR9 in steps 1 and 4: `SR9_UNBOUND` is `run.start` (also `resumeRun`) and `run.token.renew`; with
a valid token they answer exactly as without the header, also when the run has ended; an invalid
header denies them like every action (step 1). `sr9_applies` is false
for these two actions and for the own exceptions of SR9 (`message.create`, `run.update`,
`run.finish`, `interaction.respond`, `agent_session.report`) when the resolved thread of the
resource is the run's thread and, for a resource of kind `run`, the resource is the run itself;
for every other action it is true (fail closed). In delivery mode (`authorize.Delivery(ctx)`, the
REST poll and the gateway delivery to an agent consumer) decisions and `Scope` ignore `acting_for`,
because an agent consumer receives by its own grants (8.1 row "Agent consumers"); the filter mode
(`authorize.Filter(ctx)`) stays bound. `Scope` with status `invalid` or `run_inactive` returns the
same `invocation_token_invalid` decision.

While a view is stored on a console session, `access.view_as.end`, `auth.logout` and
`auth.step_up` on the person's own session and `self.read` on the person themselves are decided
without `view_as` (R-AZ-32; steps 4 and 7 would otherwise deny them).

After that, not in `authorize`, come the audit and the rate-limit counting according to the columns
in 05. Rate limits are checked by a separate layer before `authorize` (05, section 2.5).

Order and reason:

- Blocking checks before everything else: a disabled principal never gets any information about
  resources.
- Visibility before requirement: whoever may not read gets `404`, even if they also lack
  a level.
- Requirement before step-up: whoever does not have the permission anyway is not asked for
  confirmation. Otherwise the step-up prompt would reveal that the action would be possible.
- Guard rules last: they need the full context (level, run, thread) and apply to owners too,
  where they say so.

Guard rules of the terminal in step 9 (M2-40), in this order, the first violation decides: SR10
server host (`terminal_server_host_owner_only`), SR10 assignee (`terminal_not_host_assignee`), SR11
(`i6_holder_only`), SR12 (`terminal_no_session_user`, then `terminal_owner_account_owner_only`). Reason: the order of section 5 and
[24-terminal.md](24-terminal.md), section 6.1 step 2; a server host is closed to every non-owner,
assigned or not, so its reason names what would help (docs/DECISIONS.md D-M2-40-3). The guard rules
run after the step-up: whoever has no fresh step-up learns `step_up_required` first. The atom
`+terminal` of the requirement is effective `terminal` after 2.12 item 3 without the I6 exception
for owners; an owner who is not the holder of a personal login fails at SR11. M2-44
(`host_not_active`, `terminal_profile_unknown`) and M2-46 (`terminal_session_limit`) extend the
list after it.

Exception to the order: if the requirement of an action is `All(HumanSession, ...)` (`host.read`,
`host.manage`, from M2-40 also `terminal.*`), `authorize` checks right after step 1 and before the
rights state, resolution and visibility whether a person acts in a session; agents, apps and
personal tokens get `forbidden` with `subject_kind_not_allowed`. Reason: 05 section 4.14 rejects
them with this reason, and the visibility before it would turn that into `404`. The answer depends
only on the principal, never on the resource, so it reveals nothing about a host (AZ-P4;
docs/DECISIONS.md D-M2-39-4).

Requirements that need facts of a later milestone (runs, sessions and commands of the
agents, components, files, webhooks, import) stand until then as deferred entries in the
catalogue of the code: they behave like an unknown action (`500`, alert), and the
startup check rejects every route, op or event type with such an action. The ticket
that delivers the facts replaces the entry in the same change. Reason: the catalogue is closed
from M1 on (AZ-P3), and a requirement that cannot be checked yet must never allow. `conditional` in
the column "Step-up" (05, section 2.4) counts as `yes` until the ticket that delivers the condition.
`has_credentials(p, action)` is a question to `PrincipalSource`: for `passkey.register` a
passkey of the person with `revoked_at IS NULL`, for `local_credential.set` a row in
`local_credentials` with `totp_enabled_at IS NOT NULL` (an established password path). The rule
itself stays in `authz` (B7).

Actions with two targets (`thread.move`: thread and target channel, permission "manage on source and
target"; `agent_session.bind`: thread and the thread in which the reference is current, permission
"trigger and `agent.invoke` on the thread and, when the reference is current in another thread, also on that thread",
09 section 9.4) carry the second target in the resource reference (`ref.target`). `resolve` resolves both
paths; visibility and requirement apply on both. Without read permission on the target the response
is `404`, the same as for an unknown ID. Reason: a second check in the use case would be permission
logic outside of `authz` (05, rule 1.1).

Audit according to the column "Audit" in 05: `authorize` passes every denial and every error to
`kernel/auditing`; they are written after the rollback in a separate transaction and aggregated
per `audit.deny_aggregate_window` (AZ-P13). Filters are exempt: the
receive check per recipient during delivery and catch-up (gateway, `GET /api/v1/events`, push,
webhooks) and the check per element of a list, search or snapshot (7.4, rule 3). They
run in filter mode (`authorize.Filter(ctx)`, 02 section 4.5) and write no denial.
Reason: otherwise every event would write a row for every non-recipient; what is checked is
requests and ops. Allow decisions of an action with audit `yes`
are written by its use case in its transaction with its details, because only the use case knows
the details; a test of the use case finds exactly one entry.

### 4.3 Visibility

`visible(spec, p, eff, res)` is true if one of the following holds:

- The resource is `instance` and `p` is logged in (`auth`), or the action is `public`.
- `eff.level >= read` on the path of the resource.
- The resource is a space and `p` has `read` on at least one of its channels (05, `space.read`).
- The resource is an agent and `p` is an owner or has `read` on a channel that the agent operates
  (05, `agent.read`).
- The resource belongs to `p` itself (own device, own session, own setting, own
  privacy request), or `p` is an owner and the action allows `owner`.
- DM channel or content in it: `p` is a participant.
- File with `purpose = 'avatar'` or `'branding'`: `p` is logged in (05, 2.2); `gdpr_export` never
  through `file.read`. A `file` built from an event row without scope is visible only to its
  audience (`audience_principal_id`).
- The resource is a host and 2.12, item 5 holds; for a terminal session: `p` is its person
  or an owner.
- Command (M2-16, 09 section 10.3): `p` is its owner or an effective owner, or sees its scope
  target (`read` on the channel, the space visible, any principal for `instance`; without command
  facts the scope of the event, for the receive check of `command.changed`). As an item of the list
  of a channel C: `p` has `read` on C, C is no dm channel, the scope covers C, the owner is active
  and C's effective operator, or an agent guest (`allow_mentioned_agents`, its own level at least
  `write`, billing mode `organization`), or an app with level at least `write`.

### 4.4 Invariants in the transaction

`authorize` checks whether someone may attempt a change. Whether the result is allowed is checked by
`authz.VerifyInvariants(tx, change)` in the same transaction after the change and before the commit:

1. Lock order according to 03, section 6.3 (`permission_version` first).
2. Write the change.
3. Determine the affected agents and owners (03, section 6.4).
4. Compute I1 (SR1), I6 (SR11, for every host with `llm_credentials = 'personal'`, M2-40) and
   SR8 (last owner).
5. Violated: rollback, response `409` with the problem code of the reason (`i1_violation`,
   `i6_violation` or `last_owner`, decision `invariant_violation`) and a hint, an audit entry with
   `decision = 'deny'` in a separate transaction. With several violations the answer names
   `last_owner` before `i1_violation` before `i6_violation`, and `conflicts` lists the entries of
   that code (docs/DECISIONS.md D-M2-40-6).
6. Otherwise: `access.changed` per affected principal, audit, commit.

### 4.5 Archived spaces and channels

`space.archive` and `channel.archive` are final; 05 has no reverse action. The data stays
for retention and export.

1. In an archived channel and in every channel of an archived space, the effective
   level of every principal, owners included, is at most `read`. Actions whose permission needs
   `write` or `trigger` on a target in it end with `forbidden` and reason `channel_archived`
   (HTTP `403 channel_read_only` with `reason`). Reading stays possible for everyone who could read
   before.
2. Administrative changes to an archived space or channel (rename, move,
   operator, tags) pass `authorize`, but `chat` rejects them with `409 conflict`.
3. Archiving is a permission change (column "Permissions" in 05): increase the permission version,
   and `access.changed` with an entry in `changed` (level `read`) for everyone whose level was
   above it.
4. The sidebar lists (`listSpaces`, `listSpaceChannels`) do not show archived items;
   single fetch, history and search still return them to those entitled to them.
5. I1 considers only non-archived targets ([10-backend-policy.md](10-backend-policy.md),
   section 5.3), because nobody has `trigger` there.

Reason: archiving should freeze a history, not hide it; reading without writing is the
behavior that people know from archived channels, and an owner loses nothing by it.

### 4.5a Deleted spaces, channels and threads

Addendum of 2026-11-01 ([delete.md](../design/delete.md), D-DEL-1 to D-DEL-11; 03 R-DM-57).

1. `space.delete` (owner, step-up) and `channel.delete` (`manage`, step-up) archive the item in
   the same permission change and mark it deleted; everything of 4.5 applies. Their restores
   (`space.restore`, `channel.restore`) need the same right and pass `authorize` on the archived
   item like the other administrative actions of 4.5 point 2; `chat` allows them only before
   `deleted_at` plus `chat.delete_undo_window` (`409 conflict`, reason `undo_window_passed`
   after it), and a channel of a deleted space only after the space is restored (`409 conflict`,
   reason `space_deleted`). A dm channel is never deleted (`409 conflict`, reason `dm_channel`).
2. `thread.delete` and `thread.restore` need `write` and (creator or `manage`), without step-up;
   in an archived or read-only channel nobody deletes a thread (4.5 point 1). A main thread is
   never deleted alone (`409 conflict`, reason `main_thread`).
3. `authorize` answers `not_found` for every action on a deleted thread, its messages and its
   runs except `thread.read`, `thread.delete`, `thread.restore` and the agent-self actions with
   which the runner ends a run the deletion cancelled (`run.update`, `run.finish`,
   `agent_session.report`). `thread.read` stays for the
   deletion state (`getThreadDeletion`) and for the receivers of its events; `getThread` and the
   history answer `404 not_found` themselves, every thread list leaves the thread out, and
   nothing new goes into it.
4. Moving the project folder with `trash_project_folder` also needs `channel.project.bind`
   (owner, step-up) and an ok folder check of the serving runner (`409 conflict`, reason
   `project_folder_refused` otherwise). `DeletionInfo.project_folder` is shown to owners only.
5. `channel.project.trash` is recorded from the heartbeat of the runner the move was asked from
   (`agent-self` on the channel); other runners' results are ignored.
6. Every delete and restore is audited with `trash_project_folder` (D-DEL-9); the purge after the
   undo window has no actor and writes no entry.

Reason: deletion follows the right that creates and archives the item; the archive state brings
every list, write path and I1 rule along without a second hidden state.

### 4.6 Resolving mentions

A mention token in the content ([03-data-model.md](03-data-model.md), `messages`,
content format) becomes a mention on send and edit only if the following holds:

| Mentioned | Condition |
|---|---|
| Person | `authorize(author, user.read, person)` allows, or the person may read the thread of the message (`authorize(person, thread.read, thread)`) |
| Agent | `authorize(author, agent.read, agent)` allows |

Otherwise the token stays text. Reason for the second condition for people: agents have no
scope `users:read` and do not see the directory (05, section 2.6); their mention should still
reach the person who reads the thread ([13-push-notifications.md](13-push-notifications.md),
section 2.1). The author thereby learns only whether a handle may read the thread they themselves
write in; the directory stays hidden. A person without read permission on the channel whom a person
mentions gets a row in `mentions`, but never a notification (8.1, row Push).

## 5. Protection rules that no grant overrides

Every guard rule has a number, an enforcement point and a test (section 11).

### SR1 Backend policy (I1, I2)

- **I1, transactional.** For every agent with `billing_mode = 'personal'`, the set of
  principals with effective `trigger` on any target that the agent operates is exactly
  `{credential_holder_id}` (Baseline section 5; check 03, section 6.4). Checked in the
  transaction of every action from 05, rule 1.6.
- **Consequence.** Owners have `trigger` everywhere. Therefore `personal` works only if the
  `credential_holder` is the only active owner and nobody else has `trigger` on the operated
  targets. If a second owner is created, `owner.add` fails with `i1_violation` as long as an
  agent is on `personal`. Under this consequence, "exactly" and "at most" are the same condition,
  because the only owner always has `trigger` themselves.
- **I2, before every invocation.** `agent.invoke` checks on acceptance and immediately before every
  delivery: with `personal`, the triggering person is the `credential_holder`, otherwise
  `billing_mode_personal_not_holder`, fail closed, audit (05, section 5, item 4; 04, section 15.3).

### SR2 Tool class

An agent with `tool_class = 'host'` may only be triggered by:

- an active owner, or
- a person for whom SR3 proves a ready dev environment of their own in which the run
  is executed.

Otherwise `tool_class_owner_only`. Checked in `agent.invoke` on acceptance and before every delivery.
A grant with `trigger` on a channel with a `host` operator is not rejected; the preview
(section 9.3) says in plain text that it does not enable triggering. Reason for the check at the
invocation instead of at creation: the readiness of an environment changes without a permission
transaction, and group memberships change often. The invocation is the only place where all facts
are current.

### SR3 Dev isolation

For channels with a project binding (`project_bindings`), the following applies in addition to 05,
section 5, item 6:

1. A run of a non-owner runs only in a dev environment of this person that is ready
   (`dev_environments`, [21-dev-isolation.md](21-dev-isolation.md), section 6). Otherwise
   `dev_environment_not_ready`.
2. The agent session of a thread is bound to the environment in which its first run ran
   (`agent_sessions.dev_environment_id`, 03). If another person triggers in this thread,
   that is `dev_environment_foreign_thread`. Reason: a session reads and writes the files
   of its environment; a different triggering person would work in the environment of another person.
3. Owner runs run in the environment of the owner. That is today's single-person operation.
4. The invocation token carries `dev_environment_id`; the runner starts only in exactly this
   environment.

As long as there is no ready environment, every dev project can therefore be triggered only by
owners, regardless of which grants exist.

### SR4 Only humans trigger

- Only messages with `origin = 'human'` trigger agents; agents, apps, imports, system messages
  and incoming webhooks never do (05, section 5, item 1).
- Agents and apps have at most `write` and only `upload` as a capability (section 2.1).
- Reason: bot loops are ruled out, and the trigger set from I1 stays human.

### SR5 Administration only by humans with confirmation

- Step-up actions only with a person's session and a fresh confirmation (R-AZ-03).
- Owner actions only by active owners, even if a grant `manage` on the instance exists.
- Owner status takes effect only in a session. A personal API token of an owner acts with the
  grants of this person, without owner permissions. Reason: a stolen token without device binding
  must never open the whole instance ([08-identity.md](08-identity.md), section 12; 05,
  section 2.3; R-AZ-14).
- Exception: in the check without a request (before a delivery to an agent, at
  `run.token.renew`, at the run from deferred follow-ups, before handing an app command or an
  interaction to an app, and for the permissions of the triggering person in SR9), the owner
  status comes from the current owner list without a session; the permission state computed this
  way is never the own permission state of the principal of a request, not even of a personal
  token (05, section 5, "Check without a request"; R-AZ-29).

### SR6 Direct messages only for participants

No owner status, no grant and no "View as person" opens a DM channel (Ruling
L21). The only way in the product is the GDPR export of a person: `gdpr.request.create` for another
person, approval and download, each by an owner with step-up and audit; the package
contains only this person's own messages ([20-privacy.md](20-privacy.md),
section 5.2). Other ways, such as an order by an authority, lie outside the product
(R-AZ-02).

### SR7 Disabled means no permissions

A principal with status `disabled` or `deleted`, a revoked session, a revoked
device, a revoked token or an app installation that is not active results in `unauthenticated`,
before grants are read. A person in state `pending` and a waiting device result in
`forbidden`, except for the self-service actions (4.1). Grants of disabled or waiting principals
stay in place but do not take effect.

### SR8 Last owner

No transaction may remove the last active owner: neither `owner.remove` nor `user.disable`
on an owner if afterwards no owner with `principals.status = 'active'` remains. Code
`last_owner` (03 `owners`).

### SR9 Acting-for binding (confused deputy)

An agent that uses the API within a run attaches the verified invocation token of the run
(a header; its name is defined in [06-api.md](06-api.md)). Then the following applies to every
action that reads content or writes into a channel:

```text
eff(agent in run, target) = eff(agent, target) ∩ eff(triggering person, target)
```

`∩` means: the lower level, the intersection of the capabilities. Exceptions, for which only the
permissions of the agent apply: replies in the thread of the run, patches to its own run,
`run.finish`, `interaction.respond`, `agent_session.report`. The token is checked on every request
(signature, `aud`, run active); the permissions of the person are computed fresh, not read from
the token. A revocation therefore also takes effect immediately on tool calls of a running agent.

Reason: without this rule, a person with `trigger` on channel A can ask an agent that may also
read channel B to summarize B. The agent is then a "confused deputy". The
SDKs ([09-agents.md](09-agents.md)) automatically bind the token to every API call during a run.
What the runtime knows outside the server API (its own memory, earlier sessions) is not
covered by the rule; for that there is the read horizon hint (section 6) and the threat
T-AI-02 in the threat model.

The header is `Invocation-Token` with the compact JWS of `EventEnvelope.invocation_token` of the
run ([06-api.md](06-api.md), section 4 point 8). The binding check on every request: the
signature, `typ`, `kid` and `iss` of the token, `aud` equal to the authenticated agent, then the
run of the claim `run_id`, which must exist, belong to this agent, be triggered by `sub` (for the
tokens of trigger kind `message` and `command`; an interaction or follow-up token names the person
who acted, and the binding always takes the person from the run, [09-agents.md](09-agents.md)
section 4.5) and lie in the claimed thread; a token without `run_id` (autocomplete, a command without an active run) is
invalid, because SR9 binds a run. An active run (`queued`, `running`, `waiting_input`) binds; an
ended one answers `invocation_token_invalid` with `reason: run_inactive`, every other failure
`reason: invalid`, also for the header sent by a principal that is not an agent. Expiry and the
currency of the `jti` are not checked: a run outlives `agent.invocation_token_ttl`, the binding
always intersects with the run's triggering person, and the token never adds a right, so a leaked or older token gives its
holder nothing the agent token alone would not give (docs/DECISIONS.md D-M2-14-3). The exceptions
above hold only for the own thread and run: the resolved thread of the resource is the run's
thread, and for a resource of kind `run` it is the run itself, so `run.finish` of an earlier run in
the same thread is bound; `run.start` and `run.token.renew` are not bound at all, every other action
is (section 4.2). The rights of the triggering person are computed at the request like the re-check
of R-AZ-29: a principal of kind `user` with the person's ID and no session, device, token,
`view_as` or `acting_for`, liveness from the principal status only (a `disabled`, `deleted` or
`pending` person has no rights, SR7, so every bound action ends with `acting_for_insufficient`),
owner status from the current owner list without a session. What SR9 does not cover: the server
cannot tell a request inside a run that omits the header from one outside a run (AZ-M45, hint H1
of section 6; the SDKs and reference adapters always send it); knowledge a runtime keeps across
runs is outside the API (section 6, T-AI-01); an owner who triggered with a personal API token is
intersected as owner (R-AZ-29).

### SR10 Terminal only on own host

A person who is not an active owner gets `terminal.open` and `terminal.attach` only on a
host that

- is assigned to them (`hosts.assigned_user_id = P`), or
- is their own ready dev environment (`hosts.dev_environment_id` with `dev_environments.user_id =
  P` and readiness according to [21-dev-isolation.md](21-dev-isolation.md), section 6.2; milestone
  "Multi-user dev environment").

Otherwise `terminal_not_host_assignee`. On a server host (`hosts.declared_server_host` or
`hosts.reported_is_server_host`) only owners (`terminal_server_host_owner_only`); the host service
additionally checks this against the claim `own` of the terminal session token. Checked on every
open and attach and on every change of `assigned_user_id`, `dev_environment_id` or the owner status:
if the condition no longer holds, the sessions end (`access_revoked`). If `updateHost` changes or
clears the assignee, the same transaction ends the sessions of the previous assignee on the host,
unless they are an active owner; sessions of owners and of the new assignee stay (M2-40). Only the
previous assignee can lose SR10 through this change; SR10 refused every other non-owner before
(docs/DECISIONS.md D-M2-40-10). Reason:
[21-dev-isolation.md](21-dev-isolation.md) and ADR 0046 reject the OS user as the sole boundary
between people; a shared home directory shares secrets and LLM logins (Ruling T1).

### SR11 Terminal and LLM login (I6)

If `hosts.llm_credentials = 'personal'`, `terminal.open` and `terminal.attach` allow only
`hosts.credential_holder_id`, not even another owner (`i6_holder_only`). The transaction rejects
grants on such hosts to subjects other than the holder, including to groups
(`i6_violation`, 03 section 6.8). Distinction from SR1: I1 has no owner exception, I6 has one,
because a terminal is access to one specific host and not an inherited level
([24-terminal.md](24-terminal.md), section 7.3; ADR 0068).

### SR12 Own session user (Ruling T12)

`terminal.open` and `terminal.attach` allow only a person `P` who has a valid assignment to a
session user on the host (`host_session_users` with `user_id = P` and
`revoked_at IS NULL`, whose `os_user` is in `hosts.reported_session_users`), owners included;
otherwise `terminal_no_session_user`. Reason for this error: otherwise only the host service would
reject a name that the host does not know, as tampering (`4104`, 24 section 5.3). A session always runs
as the session user of its person. A session user belongs to at most one person, also after the
end of the assignment (03, `host_session_users`); the transaction of `host.manage` rejects an
assignment that would violate this (`409 conflict` with `reason: session_user_taken`). Every assignment
and every end of one raises the permission version; the end terminates the person's sessions on the host
in the same transaction (`access_revoked`), and so does a report of the host that no longer lists
that name. SR12 replaces neither SR10 nor SR11 and applies in addition.

Owner account (ADR 0073, FEAT-terminal-admin-user): a host in the owner account mode reports its
one session user with `owner_account = true`; it is the instance owner's own account with
administrator rights. An assignment of it carries `host_session_users.owner_account = true`; only an
active owner creates one, only for himself (`403 forbidden` with `reason: owner_account_owner_only`),
and the flag must equal the report (`400 validation_failed`). The assignment counts only while the
host reports the name with the same flag (a report with another flag ends its sessions like a
vanished name), and `terminal.open` and `terminal.attach` with it additionally require an active
owner, otherwise `terminal_owner_account_owner_only`. Losing the owner role ends the sessions in the
owner account (`access_revoked`).
Reason: this way the credentials of a session user belong to exactly one person; a shared
session user would make every subscription login in it usable for all terminal users of the host
([24-terminal.md](24-terminal.md), sections 2.2 and 7.4; ADR 0068).

## 6. Read horizon of agents

The read horizon `H(A)` of an agent is the set of channels that A may read (grants plus
operator rule). The trigger set `T(A)` is all people with effective `trigger` on a target that
A operates, for whom no guard rule blocks triggering.

Hint H1: for every person `P` in `T(A)` and every channel `K` in `H(A)` that `P` may not read,
`P` can learn content from `K` through A if the runtime of A keeps knowledge across
runs. SR9 prevents this only for read access through the server API.

H1 is a hint, not a block. It appears:

- in the preview of every permission change (section 9.3),
- in "Who has access?" as the source "indirectly via agent A" (section 9.1),
- in the "Permission hints" list of the admin console.

Reason for a hint instead of a block: a hard rule would also have to apply on `channel.create` (a new
channel in a space with an agent grant enlarges `H(A)`) and would make creating channels
fail unexpectedly. Only the instance operator knows whether a runtime keeps knowledge. The
recommended configuration is one agent per audience: the people in `T(A)` may read at least everything
that A reads. Then `H1` is empty.

## 7. Filtering in SQL: permission cache with version counter

### 7.1 Decision

Lists, counters, search, suggestions and catch-up filter in the database query with the set of
visible channels from a permission cache in `authz`. There is no materialized view of effective
permissions in the database (R-DM-19). The rejected alternative and the reasons are in ADR 0018.

In short: a materialized view would be a second truth that would still have to check expiry times in every
query (`valid_until > now()`), would write many rows on every group change and would silently return
wrong results on a bug in its maintenance. A cache that is built from
the same function as `authorize` cannot diverge without the differential test
(AZ-P8) finding it.

### 7.2 Permission state (snapshot)

```text
RightsSnapshot {
  principal_id, permission_version, computed_at, valid_until
  owner: bool
  instance_level, instance_caps                  # from grants on the instance
  level_by_space:   map[space_id]level           # from grants on spaces
  caps_by_space:    map[space_id]set
  level_by_channel: map[channel_id]level         # from grants on channels; DM channels the principal takes part in
  caps_by_channel:  map[channel_id]set
  instance_wide: bool                            # owner or read on the instance
  fully_visible_space_ids: sorted list           # spaces with read on the space
  visible_channel_ids:     sorted list           # channels with read on the channel, DMs the principal takes part in
  visible_space_ids:       sorted list           # spaces with at least one visible channel
  provenance (only for access.explain, not in the cache)
}
```

Construction:

1. Valid grants of the principal, for people also those of their valid groups, all with
   the time `t` from section 2.5, per target (instance, space, channel): highest level, union
   of the capabilities.
2. Owner: `trigger` and all capabilities on the instance (never applies to DM channels).
3. DM channels the principal takes part in, with `write` and `upload`.
4. `valid_until` = smallest `expires_at` of all grants and memberships used (or
   infinity).

The snapshot stores levels per target, not expanded per channel. `authorize` forms the effective level on a
resource only at call time, from the path that `resolve` returns (channel, space,
instance): highest level along the path, plus the operator rule from the resolved effective
operator (section 2.9), the limit for agents and apps (2.1) and the boundary of archived
targets (4.5). In the same way `Scope` forms the lists for queries per request (7.4). Reason: a new channel
or space does not change the permission version (05, `channel.create` and `space.create` have "Permissions"
`no`); with expanded channel lists in the cache it would stay invisible until the next permission change to everyone
with a grant on the space or instance. Catch-up uses the same names directly in
its query, because `event_log` carries the column `space_id` (04, section 7.2).

Limit: the permission snapshot knows targets, not threads. The operator rule at thread level
(section 2.9, item 2: `threads.operator_agent_id`) therefore applies in `authorize` for the single
thread, but not in list filters (7.4). An agent that operates a thread in an
otherwise unreadable channel does not see it in lists. This includes too little and
reveals nothing. How agents read threads is decided by milestone M2 (agent platform).

### 7.3 Validity of the cache

A cache entry is valid as long as both hold:

- `permission_version` of the entry equals the current permission version, and
- `request_time < valid_until`.

Every REST request reads the current permission version in its read transaction
(`SELECT value FROM sequence_counters WHERE name = 'permission_version'`, one row through the
primary key). Gateway processes use the version from the event log in log order
(04, section 8.1). So every request after the commit of a permission change sees the new
state, also in another process. There is no read without the current version.

The version counter is global. Every permission change invalidates all entries; only those who
make a request afterwards are recomputed. Permission changes are rare, requests frequent; a counter
per principal would make the case "group with many members" expensive and is therefore not needed.

### 7.4 Query pattern

```sql
-- Message search, $1 = visible channels from Scope (formed per request, 7.2),
-- $2 = search expression, built by the search module (14-search.md)
SELECT m.id, m.thread_id, m.channel_id, ts_rank(m.search_vector, $2::tsquery) AS rank
  FROM messages m
 WHERE m.channel_id = ANY($1::uuid[])
   AND m.deleted_at IS NULL
   AND m.search_vector @@ $2::tsquery
 ORDER BY rank DESC, m.id DESC
 LIMIT $3;

-- Unread counter per channel, $1 = requested channels, intersected with Scope
SELECT t.channel_id,
       sum(t.last_message_seq - greatest(coalesce(r.last_read_seq, 0), t.unread_floor_seq)) AS unread
  FROM threads t
  LEFT JOIN read_states r ON r.thread_id = t.id AND r.user_id = $2
 WHERE t.channel_id = ANY($1::uuid[])
 GROUP BY t.channel_id;
```

Rules:

1. Every query over content, threads, channels, spaces, runs, files or mentions has the
   predicate `channel_id = ANY($visible)` (or `space_id = ANY($visible_spaces)`). The lists
   are formed by `Authorizer.Scope` per request from the permission snapshot and the current structure
   (`StructureSource`): channels with their own grant and DM channels the principal takes part in, all channels of the
   spaces in `fully_visible_space_ids`, with `instance_wide` all channels except DMs, for agents the
   channels they operate under the operator rule. With `view_as` or `acting_for` (SR9) `Scope` intersects the
   lists with those of that person, as `authorize` intersects the permissions. New channels and spaces thus appear without a new
   permission version. A lint in the repository layer rejects reading sqlc queries on these tables
   without this predicate, unless they are on a reviewed exception list (resolution for
   `authorize`, loading a resource that has already been checked, jobs with system permission such as retention,
   log tail). The lint is `tools/lint/sql_visibility.py` (gate `sql-visibility`, AZ-P12); the
   reviewed exception list with a category and a reason per query is
   `tools/lint/sql_visibility.yaml`. The list of cost estimates per run for the reconciliation (`cost.reconcile.read`)
   stays without this predicate: it checks the action once against the instance, without a check
   per entry (rule 3), reads only `cost_entries` and names neither channel nor thread nor content
   (05, rule 1.2 and R-AZ-28). Writing queries only hit rows of a resource that `authorize` has checked
   before (for example the copies of `channel_id` when moving); the lint does not check them.
   Thread-scoped tables without `channel_id` (`runs`, `run_steps`; a run follows its thread when
   the thread moves, 03 `threads`) follow one of two forms instead (D-M2-05-9): (a) the query reads
   the rows of exactly one thread or run whose visibility was checked first (`getRun`,
   `listRunSteps`, `listThreadRuns`, the active run of a thread at the acceptance; exception
   category `after_authorize`, the load before `authorize` in `getRun` `authorize_resolution`),
   or (b) for the active-run lists (`listActiveRuns`, `Snapshot.active_runs`) it reads the active
   runs, resolves their threads through the port of `chat`, keeps only threads whose channel is
   in `Scope.ChannelIDs` and runs `authorize` per row (category `thread_scope`). The gauge query
   of `bothy.run.active` has no principal (`system_job`).
2. Totals (`total`), facets and "more results" count only within this predicate.
3. After the SQL filter, `authorize` also runs for every result with the read action (for example
   `message.read`). The SQL filter is the performance, `authorize` the truth. A difference
   between the two discards the element, writes a log without IDs and content and increments
   `bothy.authz.errors` with `bothy.reason = filter_mismatch`; the alert
   `BothyAuthzError` depends on it ([signals.md](../observability/signals.md)).
4. Suggestions (mentions, commands, channel quick switcher) follow the same rules: a person whom
   the asker may not see does not appear, and the count reveals nothing.
5. Large instances: if the list gets so long that the query planner switches to a bad plan, the remedy is a temporary
   table from the same snapshot in the same transaction, never a permanent permission table.
   The load test `perf/catchup` (`perf.catchup_page_p99`, [02-architecture.md](02-architecture.md),
   section 17) also runs with a large set of visible channels and pins the limit.
6. The examples above show the result, not the place of the query: if the tables are in
   different modules (here `threads` in `chat`, `read_states` in `sync`), the code assembles the
   parts through ports (02, section 4.1); no module reads foreign tables.

## 8. Delivery filter per recipient and immediate effect

### 8.1 Delivery filter

| Path | Check per recipient | Source |
|---|---|---|
| Durable events (gateway, `GET /api/v1/events`) | `authorize(recipient, receive_action, resource)`; personal events only to `audience_principal_id`. An app as the audience of a personal event (`command.invoked`, `interaction.created`, and the ephemeral `command.autocomplete_requested`) passes the receive action `agent.events.consume` in filter mode with an active installation and `events:subscribe` (another principal's event `not_self`, an installation that is not `active` `app_not_installed`, a missing `events:subscribe` `scope_missing`); in request mode, the consumer operations of agents, an app stays `subject_kind_not_allowed` (M2-25) | 05, section 6; 04, section 7; 03 `event_log.receive_action` |
| Ephemeral events (typing, `run.updated`, presence) | receive action and focus | 05, section 6; 04, sections 11, 12 |
| Agent consumers | like people, according to their own grants, plus personal events to the agent; before `agent.invocation`, `agent.followup`, `interaction.created`, `command.invoked` additionally `agent.invoke` for the triggering person; the header `Invocation-Token` (SR9) does not narrow these decisions (delivery mode, 4.2) | 04, section 15 |
| Push | `push.deliver` (read permission on the thread), notification preference, quiet hours, not the author; mentions without read permission produce nothing | 05; [13-push-notifications.md](13-push-notifications.md) |
| Outgoing webhooks | `webhook_out.deliver`: receive action of the event for the app according to grants and scope, subscription to the type; personal events of the app by the receive rule for apps of the row "Durable events" | 05 |
| File fetch | `file.read` on every fetch through the server, also with a file ticket; no permanent links to clients | [12-files.md](12-files.md), [ADR 0048](../adr/0048-file-download-only-through-the-server-with-short-lived-tickets.md) |
| Search, counters, lists | SQL filter plus `authorize` per element (section 7.4) | this chapter |

### 8.2 Immediate effect on revocation

A revocation (grant revoked, membership removed, owner removed, principal disabled,
device or session revoked, operator changed) takes effect in this order:

1. **Commit.** The transaction raises `permission_version` as its first step (03, section 6.3)
   and writes `access.changed` per affected principal as well as the audit entry.
2. **REST.** Every request after the commit reads the new version and recomputes the snapshot
   (section 7.3). There is no time window with the old state.
3. **Open connections.** Every gateway process re-evaluates the connections of the affected principal
   when it reaches `access.changed` in the log: focus on removed targets ends, buffered
   events of these targets are discarded, the event goes out immediately (04, section 8.3).
   On a session, device or principal revocation it closes with `4012` (04, section 8.5).
4. **Active runs.** If a person loses `trigger` on the thread of an active run that they
   triggered, the same transaction writes `run.cancel_requested` with reason `access_revoked`
   (R-SY-11). After `agent.cancel_grace` the server sets the run to `cancelled` itself. Tool
   calls of the agent already fail before that because of SR9.
5. **Queued invocations.** I2 before every delivery makes the run `failed`
   (04, section 15.3).
6. **Components.** An interaction checks `trigger` on click; open cards are not
   rewritten.
7. **Terminal sessions.** If a person loses the effective `terminal` on a host, if
   SR10 or SR11 applies or if their assignment ends under SR12, the same transaction ends their sessions
   there (`access_revoked` or `i6_violation` respectively); the server closes the connections and instructs the host service to end the
   PTY. Disabling the person, revoking a device or login session, and logout end the
   sessions that depend on them ([24-terminal.md](24-terminal.md), section 8.3).
8. **Tokens.** Access tokens stay formally valid until they expire, but `authorize` checks
   session and permissions on every request. Invocation tokens expire after `agent.invocation_token_ttl`
   and are checked again at `run.start`.

### 8.3 Expiry by time

An expiry is not a write. The decision is still immediately correct, because every call
checks the time and `valid_until` invalidates the cache. So that clients clear their cache and runs
end, the job `authz.expiry_sweep` runs every `authz.expiry_sweep_interval`. It finds grants and
memberships with `expires_at` up to now whose expiry has not been reported yet
(`expiry_reported_at IS NULL`, 03), at most `authz.expiry_sweep_batch_max` rows per transaction,
and writes per batch in one transaction: raise the permission version, set `expiry_reported_at`,
`access.changed`, `grant.changed` with `change = expired` or `group.changed` with
`change = member_expired` respectively, `run.cancel_requested` for affected runs, an audit entry with
system action `grant.expire` or `group.member.expire` respectively (05, R-AZ-15). Because the selection depends on the
stored flag and not on the time of the last run, every expiry is reported exactly once,
also after an outage of the worker. The largest delay in clearing the client cache
is `authz.expiry_sweep_interval`; the delivery of new content ends immediately.

## 9. Requirements for the admin interface

### 9.1 "Who has access?" (`access.explain`)

For a space or channel the server returns per principal: effective level, capabilities and the
source as a chain. Source kinds:

| Source | Example in plain text |
|---|---|
| direct | "Grant for Alex on space Support, until 12-31, reason: stand-in" |
| group | "via group Support-Team (member until 11-30)" |
| inherited | "inherited from space Support" |
| owner | "Owner" |
| operator | "Operator of this channel" (agent) |
| DM | "Participant" |
| indirectly via agent | "can learn content of this channel through agent Lotse" (hint H1) |
| blocked by guard rule | "trigger, but triggering blocked: dev environment not ready (SR3)" |

In the API the source kinds are called `owner`, `direct`, `group`, `inherited`, `operator`,
`dm_participant`, `via_agent` and `guard_blocked` (schema `AccessSource`, with agent, rule,
expiry and reason of the grant); an entry only from `via_agent` has the level `none`.
Disabled principals appear struck through (`inactive`) with the level that their grants
would give. The view needs `owner` or `manage` (05).

### 9.2 "View as person" (`access.view_as`)

- Only owners, with step-up, audit at start and end.
- The server sets `view_as` in the principal context for at most `authz.view_as_max_duration`.
- Effective permissions are the intersection of owner and person. DM channels of the person stay
  invisible (SR6). All writing actions end with `view_as_read_only`; after expiry
  every decision ends with `step_up_required`.
- The console shows a banner with name and remaining time throughout.
- Purpose: check what a person sees and can trigger, without knowing their credentials.
- Stored view (M6-01, R-DM-49, R-AZ-32): `viewAsUser` called from a console session stores the
  target person and the start on the session (`auth_sessions.view_as_user_id`,
  `view_as_started_at`) in the transaction of its audit entry (`target_user_id`, `started_at`,
  `expires_at`). Every later request of that session carries `view_as` until `endViewAs`
  (`access.view_as.end`, audit `ended_reason = ended`) or until `authz.view_as_max_duration` has
  passed; the first request after that ends the view (audit `ended_reason = expired`) and answers
  `401 step_up_required`. Starting a new view while one is stored is a write and is denied; the
  console ends the current view first. Called from any other session, `viewAsUser` stays a
  snapshot that lives only in that request. `getConsoleSession` shows the stored view with its
  end.

### 9.3 Plain-text preview

Every action with column "Permissions" = `yes` has a preview without effect: the action
`access.preview` (05; permission like the target action, without step-up, without audit, `rl.admin`). For
grants the route is `POST /api/v1/grants/preview`; previews of further actions come with
their routes. The server executes
the change in a transaction, computes differences and invariants and rolls back. The
response contains:

- per affected principal: added, removed and changed levels and capabilities,
- the result of I1 and SR8 (whether the change would be rejected, with the hint "switch to
  organization first"),
- hints: SR2 and SR3 block triggering, SR10 or SR12 block a terminal
  (`terminal_not_effective`), H1 read horizon, a grant to a group reaches
  n people, expiry date missing for persons outside of groups (API: `hints`),
- a text from templates (no model generates it), in the language of the caller (primary subtag of
  `users.locale`, `de` or `en`, otherwise `de`, as in 01 "Languages"; API: `text`, `text_locale`).
  Dates are in the time zone of the caller (`users.time_zone`). The templates are a catalogue
  per language in the code of `authz`; a golden test holds the example below byte for byte.

For grants, permission of the target action means: read permission on the target (otherwise `404`) and `owner` or
`manage` on it (otherwise `403`). The limits of delegation (2.6: own level, own capabilities,
agents and apps only by owners, instance only by owners) let the preview succeed: `200` with
`allowed = false` and `denial_reason` with the reason from 4.1. Reason: the preview should show
why a grant would fail.

Example (locale `de`):

<!-- language-check: off -->

```text
Sam erhält bis 31.12.2026 die Stufe "Auslösen" im Kanal Marketing/Kampagnen.
Sam kann damit: Nachrichten lesen und schreiben, Threads anlegen, Agent Texter auslösen.
Dateien hochladen: nein. Freigabe-Karten bestätigen: nein.
Hinweis: Texter liest auch Marketing/Intern. Sam darf Marketing/Intern nicht lesen, kann aber über Texter Inhalte daraus erfahren.
```

<!-- language-check: on -->

A grant on a host (M2-40) shows as its effect, per person reached, the host with `before_level` and
`after_level` `none` and `capabilities_added: [terminal]`; a violation of I6 is listed in
`invariant_violations` with `allowed = false`. `terminal_not_effective` names `principal_id` for a
person whom SR10 or SR12 would deny on the host after the change, and `group_id` and `count` for a
group (members who would be denied; without such members no hint); with an I6 violation of the same
host it is left out. The text has no line on capabilities and levels in the tree: summary, denial,
I6, `terminal_not_effective` (for a person with the reason of the first violated rule: server host,
assignee, session user), group, missing expiry date (docs/DECISIONS.md D-M2-40-11). Example (Sam is
not the assignee of R1):

<!-- language-check: off -->

```text
Sam erhält das Zusatzrecht "Terminal" auf dem Rechner R1.
Hinweis: Die Freigabe wirkt für Sam nicht. Auf R1 öffnen nur Owner und die Person, der der Rechner zugewiesen ist, ein Terminal.
Hinweis: Die Freigabe für Sam hat kein Ablaufdatum.
```

<!-- language-check: on -->

The console shows the preview before every confirmation; the step-up follows afterwards.

Preview with `resolve_i1` (guided flow, [10-backend-policy.md](10-backend-policy.md)
section 11; M2-21): the preview is a dry run of the whole request. After `access.preview` for the
grant target, every listed agent needs `access.preview` with the target action
`agent.billing_mode.set` in ascending id (owner, never step-up, never audit; a visible agent and a
caller who is not an owner give `403` with `owner_only`, an unknown or invisible agent the `404`
of an unknown id). In the rolled-back transaction the agents are switched to `organization` before
the grant is applied, so `allowed`, `effects`, `invariant_violations` and `hints` describe both
parts. `text` starts with one line per agent the request switches from `personal` ("First ... is
switched to Organization; the organization key then bears the costs.") and, when `organization` is
not in the agent's `billing_modes_supported`, the line "The adapter of ... does not report an
organization key yet. Invocations fail until it is switched." (German catalogue accordingly;
docs/decisions.d/M2-21.md D-M2-21-8).

### 9.4 Expiry reminder

The job `authz.expiry_reminder` runs with `authz.expiry_sweep_interval` and finds grants and
memberships that expire within `authz.expiry_reminder_lead` and have not been reminded yet
(column `expiry_reminded_at` on `grants` and `group_memberships`, 03), at most
`authz.expiry_sweep_batch_max` per transaction. It writes the personal event
`access.expiring` (05) to active recipients, without duplicates: for grants to a person, agent
or app, to the subject and to `granted_by`; for grants to a group, to all active
owners and to `granted_by` (a group is not a principal that receives personal events);
for memberships, to the member and to `added_by`. For grants on the instance
`target` is missing, because a target can only be a space or a channel. The console shows the list "Expiring soon" with
"Extend" (revocation plus a new grant with a new reason). There are no emails (Baseline
section 4).

The list "Expiring soon" is the operation `listExpiringAccess` (`GET /api/v1/access/expiring`,
action `access.hints.read`, R-AZ-34). It holds grants that are not revoked and memberships that
are not removed (of groups that are not deleted) whose `expires_at` lies in
`(now, now + authz.expiry_reminder_lead]`, whether reminded or not, and for owners on the
instance the API tokens of agents and apps that are not revoked and expire in the same window;
personal tokens and token values never appear. The entries are ordered by expiry, then grant
before membership before token, then ID. With `space_id` a person with `manage` on the space
sees only grants on the space and its channels and the memberships of groups that hold such a
grant in effect, and no tokens. "Extend" opens the grant wizard with subject, target, level and
capabilities of the old grant, an empty reason and a new expiry; confirming creates the new
grant first and then revokes the old one, so the access never lapses in between. A token is
renewed by creating a new one (its secret shown once) and revoking the old one.

### 9.5 Further obligations

- Revoking takes effect immediately and says so in the confirmation.
- Every grant and every revocation needs a reason (03, `grants.reason` and `grants.revoke_reason`); deleting a group revokes its grants with the reason `group_deleted` (M1-48).
- List "Permission hints": H1, grants to disabled principals, `trigger` without possible
  triggering (SR2, SR3), agents with `personal` and their trigger set. It is the operation
  `listPermissionHints` (`GET /api/v1/access/hints`, action `access.hints.read`, R-AZ-34),
  computed per request with the snapshot code of `authorize`, never a second rule set, and it
  never names content:

    - `h1_read_horizon`: for every agent A, every person P in T(A) and every channel K in H(A)
      that P cannot read (section 6), the same function as the source `via_agent` of section 9.1.
    - `grant_to_disabled`: a grant in effect whose subject person, agent or app is `disabled` or
      `deleted` (SR7); a pending person's grants take effect and do not count.
    - `trigger_blocked`: a grant in effect with level `trigger` to an active person or a group
      that reaches at least one channel with an operator, where on every such channel SR2 (an
      operator of tool class `host`) or SR3 (a project binding) keeps the subject from
      triggering because it is no owner and has no ready dev environment of its own there; the
      hint names the blocking rules.
    - `agent_personal`: every active agent with `billing_mode = personal`, its credential holder
      and its trigger set of SR1; I1 asks for exactly the holder.

    The hints come in this order of kinds, within a kind by agent, person and channel (H1), by
    grant ID or by agent ID, and a page carries the names of the principals, groups, spaces and
    channels it names. Without `space_id` the list covers the instance and needs the owner role;
    with `space_id` the space and `manage` there (a space without read gives `404`, with read but
    without `manage` `403`): H1 over the channels of the space for every agent, the grants on the
    space and its channels, and the personal agents that serve a target in the space.

- Filterable log from the audit log (`audit.read`), [ADR 0021](../adr/0021-hash-chained-audit-log-separate-from-telemetry.md).

## 10. Examples

Starting point for all examples (fictional): spaces Support, Marketing, Engineering, Research. One
owner. Agents: Lotse (operator of Support, `organization`, `chat`), Texter (operator of
Marketing, `organization`, `scoped`), Runner (operator of the forum Engineering/Projekt-A with
project binding, `organization`, `host`), Assistent (operator of Research, `personal`,
`credential_holder` = owner, `scoped`).

### 10.1 Support only for Alex

| Grant | Subject | Target | Level | Capabilities | Expiry | Reason |
|---|---|---|---|---|---|---|
| G-1 | Alex | space Support | `trigger` | `upload`, `approve_actions` | none | Support lead |

Result: Alex sees only Support, can trigger Lotse and confirm its approval cards.
Marketing, Engineering and Research are `404` for Alex; search and counters show nothing there.
Lotse is `organization`: that is required as soon as Alex has `trigger` (I1).

### 10.2 Marketing only for Sam

| Grant | Subject | Target | Level | Capabilities | Expiry | Reason |
|---|---|---|---|---|---|---|
| G-2 | Sam | space Marketing | `trigger` | `upload` | none | Marketing |

Result: Sam triggers Texter. Sam cannot confirm approval cards of Texter with `approve_actions`;
that stays with the owner. If Texter also reads a channel that Sam may not read,
the preview shows hint H1.

### 10.3 A dev project for Robin with expiry

| Grant | Subject | Target | Level | Capabilities | Expiry | Reason |
|---|---|---|---|---|---|---|
| G-3 | Robin | channel Engineering/Projekt-A | `trigger` | `upload` | 2026-12-31 | project work |

Result until dev isolation exists: Robin reads and writes in the forum, but every triggering ends with
`dev_environment_not_ready` (SR3) or `tool_class_owner_only` (SR2). The preview says so before
the grant is created. As soon as a ready environment is registered for Robin, Robin's threads run there.
Robin can read threads of the owner but not trigger them (`dev_environment_foreign_thread`). From
2026-12-31 the channel is `404` for Robin, without a job having to run.

### 10.4 Owner has everything

The owner sees all spaces and channels and triggers all agents, also Assistent (`personal`, the owner is
the `credential_holder`). The owner does not see DMs between Alex and Sam. If the owner gave Alex `trigger` on
Research, the server would reject it with `i1_violation`: "switch to organization first".

## 11. Tests

### 11.1 Test matrix

World of the matrix: as in section 10, plus group Leser (member Sam, membership with expiry) with
grant G-4 (`read` on Support/Allgemein), grant G-5 (Lotse, `write`, `upload`, space
Support), grant G-6 (app Tracker, `write`, channel Support/Eskalation, scope only
`messages:write`), channel Engineering/Werkzeuge (text, operator Runner, without project binding), one
DM between Alex and Sam. The clock is controllable in tests.

| ID | Principal | Target | Action | Condition | Expected |
|---|---|---|---|---|---|
| AZ-M01 | Owner | Research/Notizen | `message.read` | | `allow` |
| AZ-M02 | Owner | DM Alex-Sam | `message.read` | | `not_found` (SR6) |
| AZ-M03 | Alex | Support/Allgemein | `message.create` | G-1 on space | `allow` (inherited) |
| AZ-M04 | Alex | Support/Allgemein | `agent.invoke` (Lotse) | | `allow` |
| AZ-M05 | Alex | Marketing/Kampagnen | `message.read` | | `not_found` |
| AZ-M06 | Alex | instance | `search.query` "campaign" | 3 hits only in Marketing | 0 hits, no counters |
| AZ-M07 | Sam | Support/Allgemein | `message.read` | membership in Leser valid | `allow` (group) |
| AZ-M08 | Sam | Support/Allgemein | `message.create` | only G-4 | `forbidden` `level_insufficient` |
| AZ-M09 | Sam | Support/Allgemein | `message.read` | clock = expiry of the membership | `not_found` (boundary, without a job) |
| AZ-M10 | Sam | Support/Allgemein | `message.read` | membership removed | `not_found` |
| AZ-M11 | Alex | Support/Eskalation | `agent.invoke` | G-1 `trigger` on space and additionally `read` on channel | `allow` (highest applies, overlapping) |
| AZ-M12 | Alex | Support/Eskalation | `message.create` | G-1 revoked, `read` on channel remains | `forbidden` `level_insufficient` |
| AZ-M13 | Alex | Support/Allgemein | `message.read` | G-1 revoked | `not_found` |
| AZ-M14 | Robin | Engineering/Projekt-A | `message.create` | G-3 valid | `allow` |
| AZ-M15 | Robin | Engineering/Projekt-A | `agent.invoke` | no dev environment | `forbidden` `dev_environment_not_ready` |
| AZ-M16 | Robin | Engineering/Projekt-A | `agent.invoke` | own environment ready | `allow`, token with `dev_environment_id` |
| AZ-M17 | Robin | thread of the owner in Projekt-A | `agent.invoke` | own environment ready | `forbidden` `dev_environment_foreign_thread` |
| AZ-M18 | Robin | Engineering/Projekt-A | `thread.read` | clock after 2026-12-31 | `not_found` |
| AZ-M19 | Owner | Engineering/Projekt-A | `agent.invoke` | | `allow` |
| AZ-M20 | Alex | Engineering/Werkzeuge | `agent.invoke` (Runner, `host`) | `trigger` grant on channel | `forbidden` `tool_class_owner_only` |
| AZ-M21 | Owner | Research/Notizen | `grant.create` Sam `trigger` | Assistent `personal` | `invariant_violation` `i1_violation` |
| AZ-M22 | Owner | Assistent | `agent.billing_mode.set` to `personal` | Alex has `trigger` on an operated channel | `invariant_violation` `i1_violation` |
| AZ-M23 | Owner | Alex | `owner.add` | Assistent `personal` | `invariant_violation` `i1_violation` |
| AZ-M24 | Owner | Support/Allgemein | `channel.operator.set` Assistent | Alex has `trigger` | `invariant_violation` `i1_violation` |
| AZ-M25 | Owner | group with `trigger` on Research | `group.member.add` Sam | Assistent `personal` | `invariant_violation` `i1_violation` |
| AZ-M26 | Owner | the owner itself | `user.disable` | only owner | `invariant_violation` `last_owner` |
| AZ-M27 | Lotse | Support/Allgemein | `message.create` | G-5 | `allow` |
| AZ-M28 | Lotse | Marketing/Kampagnen | `message.create` | agent in a foreign space | `not_found` |
| AZ-M29 | Texter | Marketing/Kampagnen | `message.create` | no grant, operator | `allow` (operator rule) |
| AZ-M30 | Lotse | Support/Allgemein | message mentions Texter | `origin = agent` | no run (SR4) |
| AZ-M31 | Owner | Lotse | `grant.create` `trigger` | subject is an agent | `forbidden` `subject_kind_not_allowed` |
| AZ-M32 | Tracker | Support/Eskalation | `message.create` | G-6, scope `messages:write` | `allow` |
| AZ-M33 | Tracker | Support/Eskalation | `message.read` | scope `messages:read` missing | `forbidden` `scope_missing` |
| AZ-M34 | Tracker | Support/Allgemein | `message.create` | no grant | `not_found` |
| AZ-M35 | Sam with `trigger` + `manage` on Marketing, personal token | Marketing | `grant.create` Robin `read` | token without a session | `forbidden` `step_up_not_possible` |
| AZ-M36 | Owner | Marketing | `grant.create` | step-up older than `auth.step_up_max_age` | `step_up_required` |
| AZ-M37 | Alex with `write` + `manage` on Support | Support/Allgemein | `grant.create` Sam `trigger` | | `forbidden` `delegation_exceeds_own_level` |
| AZ-M38 | Alex with `trigger` + `manage` on Support | Support/Allgemein | `grant.create` Sam `read` + `export` | Alex without `export` | `forbidden` `delegation_exceeds_own_capabilities` |
| AZ-M39 | Alex with `write` + `manage` on Support | grant of the owner for Sam (`trigger`) | `grant.revoke` | | `forbidden` `delegation_exceeds_own_level` |
| AZ-M40 | Alex | any | any | Alex disabled | `unauthenticated` `principal_inactive` |
| AZ-M41 | Alex | any | any | session revoked, token not yet expired | `unauthenticated` `session_revoked` |
| AZ-M42 | Alex | card with `restricted_to = run_starter`, run by the owner | `interaction.create` | | `forbidden` `component_restricted` |
| AZ-M43 | Sam | card with `approve_actions` in Marketing | `interaction.create` | Sam without `approve_actions` | `forbidden` `capability_missing` |
| AZ-M44 | Lotse in a run of Alex | Engineering/Projekt-A | `message.read` | Lotse additionally has `read` on Engineering | `not_found` `acting_for_insufficient` (SR9) |
| AZ-M45 | Lotse without a run context | Engineering/Projekt-A | `message.read` | as in M44 | `allow`, hint H1 in the console |
| AZ-M46 | Owner as Alex | Marketing/Kampagnen | `message.read` | "View as person" | `not_found` |
| AZ-M47 | Owner as Alex | Support/Allgemein | `message.create` | "View as person" | `forbidden` `view_as_read_only` |
| AZ-M48 | Alex | Marketing message with a mention of Alex | `push.deliver` | | no push, mention stored silently |
| AZ-M49 | Tracker | event from Support/Allgemein | `webhook_out.deliver` | subscription to `message.created` | not delivered |
| AZ-M50 | Sam | Support/Allgemein | `message.read` | group Leser deleted | `not_found` |
| AZ-M51 | Alex | Support/Allgemein | `agent.invoke` | revocation between acceptance and delivery | run `failed` `access_revoked`, no delivery (I2) |
| AZ-M52 | Alex | Marketing/Kampagnen | `GET` message with a foreign ID | | same as a random ID (status, body, headers) |
| AZ-M53 | incoming webhook of Tracker | Support/Eskalation | `webhook_in.post` with a mention of Lotse | channel with `free_response` | message `origin = app`, no run (SR4) |
| AZ-M54 | Alex | host R1 (assignee Alex, `organization`) | `terminal.open` | grant `terminal` to Alex, fresh step-up | `allow` |
| AZ-M55 | Sam | host R1 | `terminal.open` | grant `terminal` to Sam | `forbidden` `terminal_not_host_assignee` (SR10) |
| AZ-M56 | Owner | host R2 (`personal`, holder of the login Robin, Robin is a second owner) | `terminal.open` | | `forbidden` `i6_holder_only` (SR11) |
| AZ-M57 | Owner | host R2 | `grant.create` group Leser `terminal` | | `invariant_violation` `i6_violation` |
| AZ-M58 | Alex | host R1 | `terminal.open` | step-up already used for another opening | `step_up_required` `terminal_step_up_not_fresh` |
| AZ-M59 | Lotse | host R1 | `terminal.open` | agent token | `forbidden` `subject_kind_not_allowed` |
| AZ-M60 | Alex | host R3 (without assignee) | `terminal.open` | grant on the instance with `trigger` and all capabilities, no grant on R3 | `not_found` (host not visible) |
| AZ-M61 | Owner | host R1 | `terminal.open` | fresh step-up, no valid assignment of the owner to a session user on R1 | `forbidden` `terminal_no_session_user` (SR12) |

### 11.2 Required tests

| ID | Test | Fails if |
|---|---|---|
| AZ-P1 | Route test: counts all registered HTTP routes | a route has no known `x-authz-action` (05, rule 3) |
| AZ-P2 | Op and event test | a WebSocket op or an event type is registered without an entry in 05, section 6 or 7 respectively |
| AZ-P3 | Catalogue closed | an unknown action does not end with `500` and an alert |
| AZ-P4 | 404 equality | the response for a foreign ID and for an unknown ID differs in status, body or headers, or the median response time deviates by more than `authz.not_found_timing_tolerance`. Routes whose resource the requester may always see (for example `user.read` for people) have no foreign invisible instance and are listed with a reason as "not applicable" in the test's list |
| AZ-P5 | Counters and search reveal no foreign channels | in randomly generated worlds (property test) unread counters, mention counters, search hits, `total`, facets, suggestions, "Running now" or the snapshot take into account a channel without read permission |
| AZ-P6 | Adapter without a valid token does nothing | the reference adapter or an SDK adapter starts the runtime with a missing token, a wrong signature, a wrong `aud`, an expired token, a repeated `jti`, a differing `billing_mode` or a differing `tool_class` (conformance test of the SDKs, [09-agents.md](09-agents.md)) |
| AZ-P7 | I1 for every kind of change | for one of the actions from 05, rule 1.6, a change that violates I1 is committed; one case per action |
| AZ-P8 | Differential test of the cache against a reference | `authorize` with the cache decides differently in random worlds than a slow reference without a cache |
| AZ-P9 | Immediate revocation | after the commit of a revocation in process A, a request to process B is still allowed. Process means a separate server instance with its own pool, log tail, permission cache and hub on the same database; two such instances in one test binary suffice, because they share no memory |
| AZ-P10 | Expiry without a job | with `authz.expiry_sweep` switched off, an expired grant still takes effect |
| AZ-P11 | Acting-for binding | an agent in the run of a person reads or writes content that the person may not read or write respectively |
| AZ-P12 | SQL predicate | a reading sqlc query on content tables exists without `= ANY($visible)` and without an entry in the exception list (section 7.4, rule 1) |
| AZ-P13 | Denials in the audit | a denial of a request or op is missing from the audit log or is not aggregated there (`audit.deny_aggregate_window`), or a filter (section 4.2) writes a denial |

## 12. Decisions of this chapter

| No. | Decision | Reason |
|---|---|---|
| R-RE-01 | Permission cache with a global version counter and `valid_until`; no materialized view. | One truth (R-DM-19); expiry by time is correct without a write. |
| R-RE-02 | Every REST request reads the current permission version. | Revocation takes effect across processes without a time window; the cost is one row per request. |
| R-RE-03 | Agents and apps get only `upload` as a capability. | No self-approval, no administration, no mass export by bots. |
| R-RE-04 | Operator rule: the effective operator implicitly has `write`. | Follows the operator without a second grant; nothing is left over after a change. |
| R-RE-05 | Tool class `host` only for owners or an own ready dev environment, checked at invocation. | Study C P6(b); readiness changes without a permission transaction. |
| R-RE-06 | Acting-for binding: an agent in a run has at most the permissions of the triggering person. | Closes the confused deputy through the server API. |
| R-RE-07 | Read horizon as a hint, not as a block. | A block would make `channel.create` fail unexpectedly; only the instance operator knows the memory of a runtime. |
| R-RE-08 | `manage` revokes only what it could have created itself; grants to agents and apps only by the owner. | No permission escalation through delegation; agent grants are data flows to processors. |
| R-RE-09 | The expiry job only reports; the decision never depends on it. | Study C: expiry is checked on every call; clients still clean up. |
| R-RE-10 | Preview as a dry run in the real transaction with rollback. | The same logic as the real write; plain text from templates, not from a model. |
| R-RE-11 | Host as its own target with only `terminal`; SR10, SR11 and SR12 as guard rules. | A terminal is remote access to a host; neither inheritance nor delegation may open it, and no account on the host is shared between people (Ruling T1, T2, T12). |

## 13. Alignment with the core contract

The additions requested during drafting are decided (integration of 2026-10-10):

| No. | Addition | Decision, location |
|---|---|---|
| K-RE-1 | Check: agents and apps get only `upload` as a capability | adopted, 03 `grants`, R-DM-20 |
| K-RE-2 | Operator rule in "Effective level" | adopted, 05 section 2.3 item 4, R-AZ-13 |
| K-RE-3 | `agents.tool_class`, `agents.reported_tool_class`, `agent.tool_class.set`, claim `tool_class` | adopted, 03, 05 |
| K-RE-4 | `dev_environments`, `dev_environment_id` on sessions and runs, claim, actions | adopted as the milestone "Multi-user dev environment", 03, 05 section 4.13 |
| K-RE-5 | SR8 also for `user.disable` | adopted, 03 `owners`, 05 |
| K-RE-6 | `grant.revoke` within R-AZ-05; grants to agents and apps only by owners | adopted, 05 section 4.4, R-AZ-05 |
| K-RE-7 | Action `access.preview` | adopted, 05 section 4.2 |
| K-RE-8 | System actions `grant.expire`, `group.member.expire`, event `access.expiring`, `expiry_reminded_at` | adopted, 03, 05 |
| K-RE-9 | Reason codes from 4.1 as values of `audit_events.deny_reason` | adopted without a database check: list closed by a test (R-DM-28) |
| K-RE-10 | Owner permissions only with a session | adopted, 05 section 2.3, R-AZ-14; current state: the exception is the check without a request (05 section 5, R-AZ-29) |

## 14. Configuration values of this chapter

| Name | Default | Meaning |
|---|---|---|
| `authz.expiry_sweep_interval` | `1m` | interval of the job that reports expiries and writes reminders |
| `authz.expiry_reminder_lead` | `48h` | how long before expiry the reminder comes (Study C: two days) |
| `authz.expiry_sweep_batch_max` | `500` | rows per transaction of the jobs `authz.expiry_sweep` and `authz.expiry_reminder` |
| `authz.view_as_max_duration` | `15m` | maximum duration of "View as person" |
| `authz.snapshot_cache_max_entries` | `10000` | permission snapshots in memory per process (LRU) |
| `authz.not_found_timing_tolerance` | `20ms` | allowed deviation of the median in AZ-P4 |
