# 05 Authorization action catalogue

Status: draft M0, integrated after the feedback of all chapters. Normative. This list is the
complete set of actions that `authorize(principal, action, resource) -> Decision` in the
module `authz` knows (B7).
Terms: [00-glossary.md](00-glossary.md). Tables: [03-data-model.md](03-data-model.md).
Sync and events: [04-sync-protocol.md](04-sync-protocol.md). How `authorize` decides:
[07-permissions.md](07-permissions.md).

## 1. Binding rules

1. **One decision point.** Every route, every WebSocket op, every event delivery, every push,
   every search, every file retrieval and every agent invocation calls `authorize` with an action
   from this catalogue. There is no permission logic outside `authz`.
2. **No permission to see gives 404.** If a principal may not read a resource, the answer is
   `404 not_found`, whether it exists or not. `403 forbidden` exists only when the principal may
   read the resource but lacks the required higher level or a capability. Lists, counters,
   search and suggestions filter in the query (P2) and reveal nothing.
   The exception is the list of cost estimates for the reconciliation (`cost.reconcile.read`): it
   evaluates the instance without a filter by visible channels and names neither channel nor
   thread nor content (R-AZ-28).
3. **REST.** Every operation in the OpenAPI document carries `x-authz-action` with exactly one ID
   from this catalogue and `x-rate-limit-class` with the class of this action. The API team
   enforces this with lint rules (`authz-action-required`: field missing; `authz-action-known`: ID
   not in the catalogue). In addition, the server checks at startup that every registered route
   has a known action, and a test counts all routes. The single source of this catalogue for docs,
   lint and code is [actions.yaml](actions.yaml) ([ADR 0049](../adr/0049-machine-readable-action-catalogue-as-the-single-source.md)); the tables of sections 3,
   4, 6 and 7 and the lists in `docs/api/redocly.yaml` are generated from it (`make catalog`),
   and `make catalog-check` keeps them current in the gate.
4. **WebSocket and events.** Every op in the AsyncAPI document carries `x-authz-action`; every
   durable and ephemeral event type carries `x-receive-action`. Sections 6 and 7 are the
   normative tables. They are generated from the same file [actions.yaml](actions.yaml)
   (rule 3). A test fails when a type or an op is registered without an entry.
5. **The catalogue is closed.** An unknown action is a program error: `authorize` denies, the
   request ends with `500`, an alert is raised.
6. **Agent invocation and I1.** Every triggering of an agent (message, follow-up, command,
   interaction, session change, autocomplete) additionally checks `agent.invoke`
   (section 5). The invariants I1 to I3 of the baseline depend on these actions:
    - I1 is checked transactionally on `grant.create`, `grant.revoke`, `grant.expire`,
      `group.member.add`, `group.member.remove`, `group.member.expire`, `group.member.sync`,
      `group.delete`, `owner.add`, `owner.remove`,
      `space.operator.set`, `channel.operator.set`, `agent.billing_mode.set`,
      `agent.billing_mode.set_all`, `agent.credential_holder.set`, `agent.enable`, `user.disable`
      and `user.enable` (check in [03-data-model.md](03-data-model.md),
      section 6.4).
    - Guided flow: `grant.create`, `group.member.add`, `owner.add`, `space.operator.set` and
      `channel.operator.set` optionally accept `resolve_i1 = {switch_to_organization: [agent_ids],
      reason}`. The server then additionally checks `agent.billing_mode.set` for each named agent
      with `authorize`, first switches them to `organization` in the same transaction, performs the
      actual change, checks I1 and writes one audit entry per action with the same
      `correlation_id` ([10-backend-policy.md](10-backend-policy.md), section 11).
      Milestone: the field enters the request schemas with M2-21 ([PLAN.md](../PLAN.md)). Before
      that, the server rejects it like any unknown field with `400 validation_failed`
      ([06-api.md](06-api.md), "Unknown").
    - I2 is part of `agent.invoke` and is checked again on every delivery of an invocation.
    - I3 is checked by the adapter; at run start (`run.start`) the server checks the token again.
7. **Permission version.** Actions with column "Permissions" = `yes` increment
   `permission_version` and write `access.changed` for everyone affected
   ([04-sync-protocol.md](04-sync-protocol.md), section 8).
8. **Read-only channels.** If `channels.read_only` is set, `authorize` denies in this channel
   every action whose column "Permission" requires the level `write` or `trigger` (messages,
   threads, reactions, interactions, commands, uploads, typing, agent invocations), for all
   principals including owners, with `403` and reason `channel_read_only`. Still allowed are
   reading and personal actions, actions with permission `manage` or `owner` (for example
   moderation, `channel.update`, `channel.read_only.set`) and `import.apply`. Setting
   `read_only` writes `run.cancel_requested` with reason `channel_read_only` for active runs in
   the channel (R-AZ-12).
9. **Maintenance mode.** If `server.maintenance_mode` is set to `read_only`
   ([16-operations.md](16-operations.md)), a separate layer before `authorize` answers all actions
   with effect `write` (section 2.7) with `503 maintenance` and `Retry-After`, except for the
   exceptions from [ADR 0065](../adr/0065-maintenance-mode-read-only.md); reading actions and catch-up continue. The layer is not
   permission logic; it knows only the effect from the catalogue.
10. **System actions.** Actions with permission `system` are performed only by the server itself
    (a job or a server command on the host). They cannot be reached via API and gateway. They
    are in the catalogue so that audit entries name a known action and the registration test
    counts them.
11. **Terminal and I6.** The actions from section 4.14 additionally check the guard rules SR10,
    SR11 and SR12 from [07-permissions.md](07-permissions.md). I6 is checked transactionally on `grant.create` with target `host` and on
    `host.manage` ([03-data-model.md](03-data-model.md), section 6.8;
    [24-terminal.md](24-terminal.md), section 7.3) and again on every `terminal.open` and
    `terminal.attach`. The terminal connections are separate WebSocket channels outside the
    gateway; their ops are in section 7.

## 2. Terms of the tables

### 2.1 Column "Permission"

| Value | Meaning |
|---|---|
| `public` | without login; still through `authorize` (principal `anonymous`) |
| `auth` | every active, logged-in principal |
| `human` | every active person |
| `self` | the principal owns the resource (own device, own setting) |
| `read`, `write`, `trigger` | effective minimum level on the target of the resource |
| `+upload`, `+approve_actions`, `+export`, `+manage` | additionally required capability on the target |
| `manage` | capability `manage` on the target (any level) |
| `owner` | the principal is an active owner and acts in a session (rule 2.3 item 1) |
| `agent-self` | the principal is the agent that owns the resource (own run, own session) |
| `app-self` | the principal is the app that owns the resource (own command, own component) |
| `participant` | the person is a participant of the DM channel |
| `+terminal` | capability `terminal` on the host (implicit for owners, unless I6 blocks; [07-permissions.md](07-permissions.md), section 2.12) |
| `assignee` | the person is the host assignee (`hosts.assigned_user_id`) |
| `host-self` | the principal is the host itself, logged in with its authentication key ([24-terminal.md](24-terminal.md), section 4.1) |
| `system` | only the server itself (rule 1.10) |

Several values joined with "or" are alternatives. "and" means both.

### 2.2 Target of a resource

The target determines on which path grants apply:

| Resource | Path |
|---|---|
| `message`, `reaction`, `pin`, `run`, `run_step`, `component`, `interaction`, `agent_session` | thread -> channel -> space -> instance |
| `thread` | channel -> space -> instance |
| `file` with `purpose = 'attachment'` | channel of the message it is attached to; before attaching, `scope_channel_id`, and then only the uploader reads it |
| `file` with `purpose = 'avatar'` | every active, logged-in principal (`auth`) |
| `file` with `purpose = 'branding'` | public like `instance.discovery.read` (logo of the login page) |
| `file` with `purpose = 'gdpr_export'` | only via `gdpr.export.download` |
| `channel` | channel -> space -> instance |
| `space` | space -> instance |
| `instance` | instance |
| DM channel and everything in it | only `dm_participants`; grants do not apply |
| `host`, `terminal_session` | host (for the session: its host); a target of its own outside instance, space and channel, only grants on this host and the owner rule ([07-permissions.md](07-permissions.md), section 2.12) |

An attached file always follows its message, also after the thread is moved;
`files.scope_channel_id` applies only before attaching.

### 2.3 Effective level

1. Owner: level `trigger` and all capabilities on all targets except DM channels, and only in
   a session of a person. A personal API token of an owner acts with the grants of this
   person, without owner permissions (SR5 in [07-permissions.md](07-permissions.md)). Exception:
   the check without a request (section 5) takes the owner status from the current owner list
   without a session (R-AZ-29).
2. Otherwise: the highest level of all valid grants (not revoked, not expired) of the
   principal and its valid groups on the path. Capabilities: union of the
   capabilities of the same grants.
3. DM participants: level `write` and capability `upload` in the DM channel.
4. Operator rule: an agent has at least `write` on every channel whose effective operator it
   is, and on every thread with `threads.operator_agent_id` equal to it, without a grant row.
   The rule follows the operator; on a change nothing remains (R-AZ-13).
5. Agents and apps: at most `write` and as a capability at most `upload` (R-DM-08, R-DM-20).
   Apps additionally need the scope from 2.6.
6. If an agent acts within a run, the acting-for binding applies in addition: at most the
   permissions of the triggering person (SR9 in [07-permissions.md](07-permissions.md)), computed as
   in the check without a request (section 5).

### 2.4 Columns "Owner", "Step-up", "Audit", "Permissions"

- **Owner** `yes`: owners only, regardless of grants.
- **Step-up** `yes`: the session needs a fresh confirmation by the person
  (`auth_sessions.step_up_at` younger than `auth.step_up_max_age`): passkey, step-up device key
  with biometrics or device PIN, password with TOTP or a new OIDC login
  ([08-identity.md](08-identity.md), section 6). Otherwise `401 step_up_required`.
  Only people can perform step-up actions; agent and app tokens and personal tokens
  are rejected (R-AZ-03).
- **Step-up** `conditional`: like `yes` as soon as the person already has credentials of the kind
  that the action sets up (the description names the condition); the first setup needs only the
  session of a person. Reason: the first setup must work without existing credentials, and a
  stolen session thereby gains only a login path whose new devices stay `pending`
  ([08-identity.md](08-identity.md), section 5).
- **Step-up** `fresh`: like `yes`, but the confirmation is younger than `terminal.step_up_max_age`,
  comes from a passkey or a step-up device key (in the web console only a passkey with
  `userVerification = required`) and is not yet consumed; the action consumes it
  (`auth_sessions.step_up_consumed_at`, [08-identity.md](08-identity.md), section 6.3).
  Otherwise `401 step_up_required`.
- **Audit** `yes`: allow and deny decisions go into the audit log. `denials`: denials only. Every
  denial of every action in a request or op is logged; filters (receive check at
  delivery and catch-up, check per element of a list) write no denial
  ([07-permissions.md](07-permissions.md), section 4.2). Identical denials (same principal or,
  without login, the same IP pseudonym, same action, same reason) are merged per
  `audit.deny_aggregate_window` into one entry with a counter (R-AZ-04). Because the
  chain is append-only, every process collects the denials in memory and writes one row per key
  at the end of the window, at shutdown or as soon as `audit.deny_aggregate_max_keys`
  keys are open (then the oldest one). A hard crash loses at most the open
  aggregates of one window ([19-security.md](19-security.md), section 12).
- **Permissions** `yes`: see rule 1.7.

### 2.5 Decision

`authorize` returns one of these values. The reason code per denial is given in
[07-permissions.md](07-permissions.md), section 4.1; it is written to the audit log.

| Decision | HTTP | Meaning |
|---|---|---|
| `allow` | | allowed |
| `not_found` | `404` | no read permission on the resource, or the resource is missing |
| `forbidden` | `403` | read permission present, but level, capability or scope missing, or a guard rule blocks (for example `channel_read_only`, `device_pending`) |
| `step_up_required` | `401` | fresh confirmation missing |
| `invariant_violation` | `409` | the change would violate I1 or another invariant |
| `unauthenticated` | `401` | no valid principal |

A device in the state `pending` and a provisioned person in the state `pending`
([08-identity.md](08-identity.md), section 3.1) have tokens, but every action except
`self.read`, `device.list` for its own device, `auth.token.refresh` and `auth.logout` ends with
`403 device_pending` ([08-identity.md](08-identity.md), section 5.2). The gateway rejects
`identify` of a pending device with `4016`. Reason for the person case: with `401` the app would
have to delete its data and could not ask whether the owner has approved it.

Rate limits are checked by a separate layer before `authorize` with the class from the catalogue
(`429`); the maintenance mode by another one (rule 1.9).

### 2.6 Scopes for apps

Besides grants, an app needs a granted scope (`app_installations.granted_scopes`):

| Actions | Scope |
|---|---|
| `message.read`, `thread.read`, `channel.read`, `space.read`, `run.read`, `search.query` | `messages:read` |
| `message.create`, `message.update`, `message.delete`, `reaction.*`, `thread.create` | `messages:write` |
| `file.read` | `files:read` |
| `file.upload` | `files:write` |
| `command.register`, `command.remove`, `command.autocomplete.respond` | `commands:write` |
| `component.update`, `interaction.respond` | `components:write` |
| `user.read` | `users:read` |
| Event subscriptions, `gateway.connect`, `events.catch_up` | `events:subscribe` |
| `cost.reconcile.read`, `cost.reconcile.submit` | `costs:reconcile` |
| `self.read`, `instance.read`, `status.read`, `command.read`, `agent.read` | no scope; only metadata that every logged-in principal with matching grants sees |
| `webhook_in.post` | no scope; token of the incoming webhook |

Agents have all scopes except `users:read` and `costs:reconcile` implicitly; their limit is the
grants and the operator rule.

### 2.7 Effect (`effect`)

Every action has an effect. `write` means: the action can change domain state in the database
or writes an audit entry when allowed (column "Audit" `yes`). `read` means: it does
neither. The maintenance mode uses only this attribute (rule 1.9, [ADR 0065](../adr/0065-maintenance-mode-read-only.md)); the
machine-readable catalogue carries it per action ([ADR 0049](../adr/0049-machine-readable-action-catalogue-as-the-single-source.md)). Exactly these have `read`:

- the actions whose ID ends in `.read` or `.list`, plus `cost.read.thread` and
  `cost.read.aggregate`, except `audit.read` (Audit `yes`);
- `access.explain`, `search.query`, `events.catch_up`, `gateway.connect`, `gateway.focus`,
  `typing.start`, `command.autocomplete`, `command.autocomplete.respond`, `client_metrics.submit`, `console.csp_report`,
  `push.deliver`, `webhook_out.deliver` and `terminal.read_own`.

All others have `write`. Reason for the closed list: a new action without an entry is
`write` and is blocked in maintenance mode (fail closed); the column "Permission" is not suitable
for this, because, for example, `settings.read` and `settings.update` both require `self`.

## 3. Rate-limit classes

Every class has a key (what is counted) and two values
`ratelimit.<class>.per_minute` and `ratelimit.<class>.burst` (token bucket). Default values in
section 10.

<!-- BEGIN GENERATED: rate_limit_classes -->
| Class | Key | Purpose |
|---|---|---|
| `rl.public_auth` | client IP (IPv6: the /64 prefix) | login, refresh, public endpoints; additionally counts every failed check of presented credentials ([02-architecture.md](02-architecture.md), section 9.1) |
| `rl.gateway_connect` | device or token, checked at `identify` and `resume` once the credentials are checked | establish a WebSocket session |
| `rl.read` | principal | read queries |
| `rl.write` | principal | general writes |
| `rl.message` | principal | send and edit messages |
| `rl.trigger` | principal | trigger agents, commands, suggestions |
| `rl.interaction` | principal | use components |
| `rl.upload` | principal | upload files |
| `rl.search` | principal | search |
| `rl.typing` | connection | typing |
| `rl.admin` | principal | administration |
| `rl.export` | principal | export, GDPR packages |
| `rl.agent_runtime` | agent | heartbeat, run patches, acks |
| `rl.webhook_in` | incoming webhook | incoming webhooks |
| `rl.import` | principal | batches of a running import (`import.apply`) |
| `rl.terminal` | principal | open a terminal, attach, connect a stream |
| `rl.host` | host (after the check of the authentication key) | control and data connections of the host service |
<!-- END GENERATED: rate_limit_classes -->

System actions and the internal delivery checks have no class (`none`, the same value as
`x-rate-limit-class` in openapi.yaml).

Reasons for the keys: IPv6 clients usually get a whole /64 prefix; counted per address, a client
could fill any number of buckets by changing addresses. `rl.gateway_connect` counts after the
check of the credentials, so that nobody can drain the bucket of another person's device; before
`identify`, `sync.identify_timeout` limits an open connection ([04-sync-protocol.md](04-sync-protocol.md),
section 6.3).

## 4. Actions

### 4.1 Public and login

<!-- BEGIN GENERATED: actions.public -->
| ID | Description | Resource | Permission | Owner | Step-up | Audit | Permissions | RL class |
|---|---|---|---|---|---|---|---|---|
| `instance.discovery.read` | JWKS, protocol versions, minimal branding, branding logo | instance | public | no | no | no | no | `rl.public_auth` |
| `console.csp_report` | accept a Content Security Policy violation report of the web console, without content in the log ([19-security.md](19-security.md), section 5, rule 6; R-AZ-33) | instance | public | no | no | no | no | `rl.public_auth` |
| `auth.login.begin` | start a login (OIDC, passkey challenge, header login, device pairing) | instance | public | no | no | denials | no | `rl.public_auth` |
| `auth.login.complete` | complete a login (OIDC server code, passkey, enrollment code, password with TOTP, pairing), register the device and device key, create a session | instance | public | no | no | yes | no | `rl.public_auth` |
| `auth.token.refresh` | rotate the refresh token (with grace `auth.refresh_reuse_grace`; in maintenance mode without rotation, ADR 0065); reuse revokes the family | auth_session | public (valid refresh token and DPoP proof of the family) | no | no | denials | no | `rl.public_auth` |
| `auth.logout` | end one's own session; push registrations of this session are deactivated | auth_session | self | no | no | yes | no | `rl.write` |
| `auth.step_up` | provide a fresh confirmation (passkey, step-up device key, password with TOTP, new OIDC login) | auth_session | self | no | no | yes | no | `rl.public_auth` |
| `status.public.read` | status page without login, only if `status.public_enabled` | instance | public | no | no | no | no | `rl.public_auth` |
<!-- END GENERATED: actions.public -->

### 4.2 Instance and administration

<!-- BEGIN GENERATED: actions.instance -->
| ID | Description | Resource | Permission | Owner | Step-up | Audit | Permissions | RL class |
|---|---|---|---|---|---|---|---|---|
| `instance.read` | instance data for clients: branding, language, limits, feature flags evaluated for the requesting principal (including rollout per person and group) | instance | auth | no | no | no | no | `rl.read` |
| `instance.settings.read` | read all instance settings (console) | instance | owner | yes | no | no | no | `rl.read` |
| `instance.settings.update` | change an instance setting | instance | owner | yes | yes | yes | no | `rl.admin` |
| `instance.branding.update` | change the branding (name, logo, color from the category colors) | instance | owner | yes | yes | yes | no | `rl.admin` |
| `instance.bootstrap` | first owner via a server command on the host ([08-identity.md](08-identity.md), section 4) | instance | system | no | no | yes | yes | `none` |
| `feature_flag.read` | read all flags with their rules | instance | owner | yes | no | no | no | `rl.read` |
| `feature_flag.update` | change a flag | instance | owner | yes | yes | yes | no | `rl.admin` |
| `signing_key.rotate` | change server keys: `mode = scheduled` (announce the next key) or `mode = emergency` (all keys of the purpose `retired` at once) | instance | owner | yes | yes | yes | no | `rl.admin` |
| `signing_key.transition` | scheduled state change of a signing key by the job `instance.key_rotation` (create `next`, make it `active`, set `retired`) | instance | system | no | no | yes | no | `none` |
| `identity_provider.read` | read login drivers | instance | owner | yes | no | no | no | `rl.read` |
| `identity_provider.manage` | create, change, switch off login drivers | instance | owner | yes | yes | yes | no | `rl.admin` |
| `owner.add` | make a person an owner; optionally with `resolve_i1` (rule 1.6) | user | owner | yes | yes | yes | yes | `rl.admin` |
| `owner.remove` | withdraw the owner role; at least one active owner remains (SR8) | user | owner | yes | yes | yes | yes | `rl.admin` |
| `owner.recover` | enrollment code for recovery for an active owner via a server command on the host (`bothy setup issue-enrollment-code`, [08-identity.md](08-identity.md), section 4) | user | system | no | no | yes | no | `none` |
| `audit.read` | read and filter the audit log | instance | owner | yes | yes | yes | no | `rl.admin` |
| `audit.verify` | check the hash chain and checkpoints | instance | owner | yes | yes | yes | no | `rl.admin` |
| `stats.read` | read statistics, also the live frame, timeline, SLO traffic light and backend view and the subscription `stats.subscribe` ([25-visualization.md](25-visualization.md)); instance only owners, space with `manage`; always filtered to the visible channels, cost fields only with `cost.read.aggregate` | instance, space | owner or manage | no | no | no | no | `rl.read` |
| `access.explain` | "Who has access?" with origin (direct, group, inherited, operator, indirectly via an agent) | space, channel | owner or manage | no | no | no | no | `rl.read` |
| `access.preview` | dry run of an action with column "Permissions" = `yes`: differences per principal, I1, SR8, hints in plain text; the transaction is rolled back; limits of delegation give `allowed = false` instead of a denial ([07-permissions.md](07-permissions.md), section 9.3) | as the target action | permission of the target action | as target action | no | no | no | `rl.admin` |
| `access.hints.read` | read the lists "Permission hints" and "Expiring soon" (grants, memberships, agent and app tokens) ([07-permissions.md](07-permissions.md), sections 9.4 and 9.5; R-AZ-34); a space only within one's own `manage` scope | instance, space | owner or manage | no | no | no | no | `rl.read` |
| `access.view_as` | "View as person", read-only | user | owner | yes | yes | yes | no | `rl.admin` |
| `access.view_as.end` | end "View as person" stored on one's own console session before its end time ([07-permissions.md](07-permissions.md), section 9.2; R-AZ-32) | auth_session | self | no | no | yes | no | `rl.admin` |
<!-- END GENERATED: actions.instance -->

### 4.3 People, identity, devices, tokens

<!-- BEGIN GENERATED: actions.identity -->
| ID | Description | Resource | Permission | Owner | Step-up | Audit | Permissions | RL class |
|---|---|---|---|---|---|---|---|---|
| `self.read` | read one's own principal (`GET /me`), for people, agents and apps; also for a pending device | principal | auth and self | no | no | no | no | `rl.read` |
| `user.read` | read the profile and directory of people | user | human; apps with scope | no | no | no | no | `rl.read` |
| `user.create` | create people (no self-registration), for a local account with an enrollment code | instance | owner | yes | yes | yes | no | `rl.admin` |
| `user.update` | change the profile of another person | user | owner | yes | yes | yes | no | `rl.admin` |
| `user.disable` | disable a person: revoke sessions and devices, cancel runs; never the last active owner (SR8) | user | owner | yes | yes | yes | yes | `rl.admin` |
| `user.enable` | lift the disabling or activate a provisioned account (`pending -> active`) | user | owner | yes | yes | yes | yes | `rl.admin` |
| `self.profile.update` | change one's own profile (display name, avatar, language, time zone) | user | self | no | no | no | no | `rl.write` |
| `identity.read` | read the identities, passkeys and local credentials of a person, without secrets | user | self or owner | no | no | no | no | `rl.read` |
| `identity.link` | link a further identity | user | self | no | yes | yes | no | `rl.admin` |
| `identity.unlink` | unlink an identity; rejected if no login path remains | user | self or owner | no | yes | yes | no | `rl.admin` |
| `local_credential.set` | set a local password and set up TOTP; step-up as soon as a password path with TOTP is set up | user | self | no | conditional | yes | no | `rl.admin` |
| `passkey.register` | add a passkey; step-up as soon as an active passkey exists | user | self | no | conditional | yes | no | `rl.admin` |
| `passkey.revoke` | revoke a passkey | passkey | self or owner | no | yes | yes | no | `rl.admin` |
| `device.list` | read devices; a pending device reads itself | user, device | self or owner | no | no | no | no | `rl.read` |
| `device.approve` | approve a pending device; one's own device from an active own device, with a confirmation code or pairing code ([08-identity.md](08-identity.md), section 5.3) | device | owner or self | no | yes | yes | yes | `rl.admin` |
| `device.revoke` | revoke a device; connections close with `4012` | device | self or owner | no | yes | yes | yes | `rl.admin` |
| `device.expire` | revoke a device without confirmation after `auth.device_pending_ttl`, an expired confirmed pairing or a device without contact after `auth.device_inactive_revoke_after` (job `identity.auth_cleanup`) | device | system | no | no | yes | yes | `none` |
| `session.list` | read login sessions | user | self or owner | no | no | no | no | `rl.read` |
| `session.revoke` | end a session; its child sessions end as well | auth_session | self or owner | no | no | yes | yes | `rl.admin` |
| `session.extension.create` | create a child session for the share extension of one's own app with its own refresh family (the Notification Service Extension uses the preview token, not a session); replaces an active child session of the same component | auth_session | self | no | no | yes | no | `rl.write` |
| `token.list` | read API tokens (prefix only) | principal | owner; self for own tokens | no | no | no | no | `rl.read` |
| `token.create` | create an API token for an agent or app; a personal token only for oneself and only with the flag `tokens.personal_enabled` | principal | owner; self for own token | no | yes | yes | no | `rl.admin` |
| `token.revoke` | revoke an API token | api_token | owner; self for own token | no | no | yes | yes | `rl.admin` |
<!-- END GENERATED: actions.identity -->

### 4.4 Groups and grants

<!-- BEGIN GENERATED: actions.grants -->
| ID | Description | Resource | Permission | Owner | Step-up | Audit | Permissions | RL class |
|---|---|---|---|---|---|---|---|---|
| `group.read` | read groups and members; one's own groups also without being owner | group | owner or self | no | no | no | no | `rl.read` |
| `group.create` | create a group | instance | owner | yes | yes | yes | no | `rl.admin` |
| `group.update` | rename a group | group | owner | yes | yes | yes | no | `rl.admin` |
| `group.delete` | delete a group (soft); its grants are revoked | group | owner | yes | yes | yes | yes | `rl.admin` |
| `group.member.add` | add a member, optionally with expiry and `resolve_i1`; not in groups with `source = oidc_claim` | group | owner | yes | yes | yes | yes | `rl.admin` |
| `group.member.remove` | remove a member; not in groups with `source = oidc_claim` | group | owner | yes | yes | yes | yes | `rl.admin` |
| `group.member.expire` | report an expired membership (job `authz.expiry_sweep`): permission version, `access.changed`, cancellation of affected runs | group | system | no | no | yes | yes | `none` |
| `group.member.sync` | when an OIDC login completes, align a person's memberships in groups with `source = oidc_claim` of their provider with the group claim; one audit entry per login with the groups ([08-identity.md](08-identity.md), section 3.4) | user | system | no | no | yes | yes | `none` |
| `grant.read` | read the grants of a target or subject; with `manage` only grants on targets within one's own `manage` scope ([07-permissions.md](07-permissions.md), section 2.6); grants on hosts only owners | space, channel, host, principal | owner or manage | no | no | no | no | `rl.read` |
| `grant.create` | create a grant. With `manage` only up to one's own level, only one's own capabilities, only to people and groups, never on the instance (R-AZ-05). Grants to agents and apps only owners; agents and apps never `trigger`, as a capability only `upload`. On a host (`host`) only owners, only to people and groups, without a level, exactly `terminal`; I6 (rule 1.11). Optionally `resolve_i1` | instance, space, channel, host | owner or manage (on `host` only owner) | no | yes | yes | yes | `rl.admin` |
| `grant.revoke` | revoke a grant; with `manage` only grants that one could have created oneself under R-AZ-05 | grant | owner or manage | no | yes | yes | yes | `rl.admin` |
| `grant.expire` | report an expired grant (job `authz.expiry_sweep`): permission version, `access.changed`, cancellation of affected runs | grant | system | no | no | yes | yes | `none` |
<!-- END GENERATED: actions.grants -->

Expiry takes effect in every `authorize` from the second of expiry; the two system actions only
make sure that clients clear their cache and runs end (R-AZ-15).

### 4.5 Spaces, channels, sidebar

<!-- BEGIN GENERATED: actions.structure -->
| ID | Description | Resource | Permission | Owner | Step-up | Audit | Permissions | RL class |
|---|---|---|---|---|---|---|---|---|
| `space.read` | see a space; applies with `read` on the space or on at least one of its channels | space | read | no | no | no | no | `rl.read` |
| `space.create` | create a space | instance | owner | yes | no | yes | no | `rl.admin` |
| `space.update` | change name, icon, color | space | manage | no | no | yes | no | `rl.admin` |
| `space.reorder` | global order of the spaces | instance | owner | yes | no | yes | no | `rl.admin` |
| `space.archive` | archive a space | space | owner | yes | yes | yes | yes | `rl.admin` |
| `space.operator.set` | set the default operator (I1); optionally `resolve_i1` | space | owner | yes | yes | yes | yes | `rl.admin` |
| `sidebar.update` | change one's own sidebar state (collapse, own order) | space | self and read | no | no | no | no | `rl.write` |
| `channel.read` | see a channel, read its header data | channel | read | no | no | no | no | `rl.read` |
| `channel.create` | create a channel | space | manage | no | no | yes | no | `rl.admin` |
| `channel.update` | change name, topic, the channel default for notifications | channel | manage | no | no | yes | no | `rl.admin` |
| `channel.reorder` | order of the channels in the space | space | manage | no | no | yes | no | `rl.admin` |
| `channel.behavior.read` | read prompt, skills, mention rules, backfill | channel | manage | no | no | no | no | `rl.read` |
| `channel.behavior.update` | change prompt, skills, mention rules, `auto_thread`, backfill | channel | manage | no | yes | yes | no | `rl.admin` |
| `channel.operator.set` | set the operator or switch on inheritance (I1); optionally `resolve_i1` | channel | owner | yes | yes | yes | yes | `rl.admin` |
| `channel.project.bind` | set or change the project binding of a forum | channel | owner | yes | yes | yes | no | `rl.admin` |
| `channel.read_only.set` | set or lift read-only (rule 1.8), with a reason; when it is set, active runs in the channel are cancelled | channel | manage | no | yes | yes | no | `rl.admin` |
| `channel.archive` | archive a channel | channel | manage | no | yes | yes | yes | `rl.admin` |
| `forum_tag.manage` | create, change, delete the tags of a forum | channel | manage | no | no | yes | no | `rl.admin` |
| `dm.create` | create a direct message with other people; only with the flag `dm.enabled` | instance | human | no | no | no | no | `rl.write` |
| `dm.leave` | leave a DM | channel | participant | no | no | no | no | `rl.write` |
<!-- END GENERATED: actions.structure -->

### 4.6 Threads and agent sessions

<!-- BEGIN GENERATED: actions.threads -->
| ID | Description | Resource | Permission | Owner | Step-up | Audit | Permissions | RL class |
|---|---|---|---|---|---|---|---|---|
| `thread.read` | read a thread and its history | thread | read | no | no | no | no | `rl.read` |
| `thread.create` | create a thread from a message or forum post; tags at creation are treated as with `thread.tags.set` (tags with `restricted` need `manage`) | channel | write | no | no | no | no | `rl.message` |
| `thread.title.update` | change the title; sets `title_source = manual`. The operator agent may set it only with `auto`; the source then stays `auto` | thread | write and (creator or manage or operator agent) | no | no | no | no | `rl.write` |
| `thread.close` | set a thread to "Done" | thread | write | no | no | no | no | `rl.write` |
| `thread.reopen` | reopen a thread (also implicitly through a new message) | thread | write | no | no | no | no | `rl.write` |
| `thread.move` | move to another channel; target with the same operator | thread | manage on source and target | no | no | yes | no | `rl.admin` |
| `thread.tags.set` | set tags; tags with `restricted` need `manage` | thread | write | no | no | no | no | `rl.write` |
| `agent_session.read` | current and earlier session references, model, costs | thread | read | no | no | no | no | `rl.read` |
| `agent_session.reset` | start a new session in the thread (for example `/new`) | thread | trigger and `agent.invoke` | no | no | no | no | `rl.trigger` |
| `agent_session.bind` | bind an existing external session to the thread (resume) | thread | trigger and `agent.invoke` | no | no | yes | no | `rl.trigger` |
| `agent_session.report` | the adapter reports the external session reference, model, `started` or `bind_failed`; the first report creates the row if `run.start` carried no reference (compare-and-swap) | agent_session, run | agent-self | no | no | no | no | `rl.agent_runtime` |
<!-- END GENERATED: actions.threads -->

### 4.7 Messages, reactions, pins, templates

<!-- BEGIN GENERATED: actions.messages -->
| ID | Description | Resource | Permission | Owner | Step-up | Audit | Permissions | RL class |
|---|---|---|---|---|---|---|---|---|
| `message.read` | read a message | message | read | no | no | no | no | `rl.read` |
| `message.create` | send a message; with attachments additionally `+upload`. If it triggers an agent, `agent.invoke` applies in addition. | thread | write | no | no | no | no | `rl.message` |
| `message.update` | edit one's own message, with `If-Match` | message | write and author | no | no | no | no | `rl.message` |
| `message.delete` | delete one's own message; others' only with `manage` (moderation) | message | (write and author) or manage | no | no | yes for others' | no | `rl.message` |
| `message.revisions.read` | read earlier versions | message | read | no | no | no | no | `rl.read` |
| `reaction.add` | add a reaction | message | write | no | no | no | no | `rl.write` |
| `reaction.remove` | remove one's own reaction | reaction | write and self | no | no | no | no | `rl.write` |
| `pin.create` | pin a message | message | manage | no | no | no | no | `rl.write` |
| `pin.delete` | unpin | message | manage | no | no | no | no | `rl.write` |
| `mention.list` | read one's own mentions (filtered by `message.read`) | user | self | no | no | no | no | `rl.read` |
| `template.read` | read templates (one's own and those of the channel) | channel, user | read or self | no | no | no | no | `rl.read` |
| `template.manage` | maintain one's own templates (resource `user`); channel templates with `manage` (resource `channel`) | user, channel | self or manage | no | no | no | no | `rl.write` |
<!-- END GENERATED: actions.messages -->

### 4.8 Files and search

<!-- BEGIN GENERATED: actions.files -->
| ID | Description | Resource | Permission | Owner | Step-up | Audit | Permissions | RL class |
|---|---|---|---|---|---|---|---|---|
| `file.upload` | upload a file for a channel (upload session, parts, completion); avatar and logo go through `self.profile.update`, `agent.update` or `instance.branding.update` respectively | channel | write and +upload | no | no | yes on malware detection | no | `rl.upload` |
| `file.read` | file, thumbnail, transcript or file ticket; only after the scan; path per `purpose` as in 2.2 | file | read | no | no | no | no | `rl.read` |
| `file.delete` | delete one's own unattached file; attached ones via `message.delete` | file | self or manage | no | no | no | no | `rl.write` |
| `file.transcribe` | start the transcription of a voice message by the server (flag `voice.enabled`) | file | write | no | no | no | no | `rl.upload` |
| `file.transcript.submit` | save the transcript of a voice message (`transcript_source = 'agent'`), with `If-Match` | file | agent-self: operator of the thread whose message the file is attached to, and read | no | no | no | no | `rl.agent_runtime` |
| `search.query` | full-text search; hits filtered by `message.read`, `thread.read`, `file.read` | instance | auth | no | no | no | no | `rl.search` |
<!-- END GENERATED: actions.files -->

### 4.9 Personal data, notifications, sync

<!-- BEGIN GENERATED: actions.personal -->
| ID | Description | Resource | Permission | Owner | Step-up | Audit | Permissions | RL class |
|---|---|---|---|---|---|---|---|---|
| `read_state.update` | advance the read state | thread | self and read | no | no | no | no | `rl.write` |
| `read_state.mark_unread` | reset the read state, with `If-Match` | thread | self and read | no | no | no | no | `rl.write` |
| `draft.read` | read own drafts | user | self | no | no | no | no | `rl.read` |
| `draft.write` | save or delete a draft, also in read-only and archived channels (sending checks `message.create`) | thread, channel | self and read | no | no | no | no | `rl.write` |
| `settings.read` | read own personal data: settings, read states, sidebar, notification levels, quiet hours | user | self | no | no | no | no | `rl.read` |
| `settings.update` | change an own setting | user | self | no | no | no | no | `rl.write` |
| `notification_pref.update` | notification level for a space, channel or thread | space, channel, thread | self and read | no | no | no | no | `rl.write` |
| `quiet_hours.update` | maintain quiet hours | user | self | no | no | no | no | `rl.write` |
| `push.register` | register the push target of one's own active device; returns the preview token once | device | self | no | no | no | no | `rl.write` |
| `push.unregister` | remove a push target | push_registration | self | no | no | no | no | `rl.write` |
| `push.deliver` | internal check before every push, every notification via the gateway and every retrieval via the preview endpoint (P3); hints without a thread (`security`, `admin`) use the recipient rule of R-AZ-30 instead | thread | read | no | no | no | no | `none` |
| `client_metrics.submit` | send performance data of the app without contents and without IDs, only with `privacy.client_metrics` ([15-client-architecture.md](15-client-architecture.md)) | instance | human | no | no | no | no | `rl.write` |
| `sync.snapshot.read` | snapshot for a resync | instance | auth | no | no | no | no | `rl.read` |
| `events.catch_up` | `GET /api/v1/events`; every event additionally with its receive action | instance | auth | no | no | no | no | `rl.read` |
| `gateway.connect` | WebSocket `identify` or `resume` | instance | auth (person with an active device, agent, app with `events:subscribe`) | no | no | denials | no | `rl.gateway_connect` |
| `gateway.focus` | focus threads; unreadable ones are silently ignored | thread | read | no | no | no | no | `rl.read` |
| `typing.start` | show typing; only people, agents show typing through their run (04, section 12) | thread | human and write | no | no | no | no | `rl.typing` |
<!-- END GENERATED: actions.personal -->

### 4.10 Agents, commands, runs, interactions

<!-- BEGIN GENERATED: actions.agents -->
| ID | Description | Resource | Permission | Owner | Step-up | Audit | Permissions | RL class |
|---|---|---|---|---|---|---|---|---|
| `agent.read` | read the agent card (name, mode badge, tool class, presence); the agent reads itself, including the configured mode of its backend policy | agent | owner or agent-self or read on a channel that the agent operates | no | no | no | no | `rl.read` |
| `agent.create` | create an agent | instance | owner | yes | yes | yes | yes | `rl.admin` |
| `agent.update` | change name, description, avatar, `operator_config` | agent | owner | yes | yes | yes | no | `rl.admin` |
| `agent.disable` | disable an agent; tokens and connections end, runs are cancelled | agent | owner | yes | yes | yes | yes | `rl.admin` |
| `agent.enable` | lift the disabling of an agent (I1) | agent | owner | yes | yes | yes | yes | `rl.admin` |
| `agent.billing_mode.set` | switch the backend policy of an agent, with a reason (I4, I1) | agent | owner | yes | yes | yes | yes | `rl.admin` |
| `agent.billing_mode.set_all` | switch the backend policy of several agents in one transaction, with a reason (I4, I1); to `personal` only with an explicit list | instance | owner | yes | yes | yes | yes | `rl.admin` |
| `agent.credential_holder.set` | set `credential_holder` (I1) | agent | owner | yes | yes | yes | yes | `rl.admin` |
| `agent.tool_class.set` | set the tool class (`chat`, `scoped`, `host`), with a reason ([07-permissions.md](07-permissions.md), section 2.11) | agent | owner | yes | yes | yes | yes | `rl.admin` |
| `agent.invoke` | trigger agents; check in section 5 | thread (with agent) | trigger and rules from 5 | no | no | denials | no | `rl.trigger` |
| `agent.heartbeat` | report state, reported mode, supported modes, tool class and active runs | agent | agent-self | no | no | yes on a change of the reported mode or the tool class | no | `rl.agent_runtime` |
| `agent.events.consume` | receive and acknowledge events as a consumer (`ack`) | agent | agent-self | no | no | no | no | `rl.agent_runtime` |
| `agent.config.read` | read prompt, skills, project binding of the channels that the agent operates | channel | agent-self and effective operator | no | no | no | no | `rl.read` |
| `agent.triggerers.read` | set of the people who may trigger this agent, with `permission_version` (allow list in the adapter) | agent | agent-self | no | no | no | no | `rl.agent_runtime` |
| `command.read` | see the command list and options (built-in commands, those of the operator, of allowed guests and of apps with `write`) | channel | read | no | no | no | no | `rl.read` |
| `command.register` | register or change a command; it may apply only to targets that its owner operates or on which the owner has `write` | command | agent-self or app-self with scope | no | no | yes | no | `rl.admin` |
| `command.remove` | remove a command | command | agent-self or app-self with scope | no | no | yes | no | `rl.admin` |
| `command.autocomplete` | fetch suggestions for an option from the owner of the command | channel | trigger and `agent.invoke` without a run (for app commands `required_level`) | no | no | no | no | `rl.trigger` |
| `command.autocomplete.respond` | deliver suggestions for a request (gateway op or REST) | command | agent-self or app-self, owner of the command | no | no | no | no | `rl.agent_runtime` |
| `command.invoke` | run a command. Agent commands check `required_level` and `agent.invoke`; model commands during an active run: denial `run_active`. App commands check `required_level` and scope, without `agent.invoke` and I2. The control command `stop` additionally checks `run.cancel` | channel, thread | `required_level` of the command and rules as described | no | no | denials | no | `rl.trigger` |
| `run.read` | read a run, its steps and metrics | run | read | no | no | no | no | `rl.read` |
| `run.overview.read` | read "Running now", filtered by `run.read` | instance | auth | no | no | no | no | `rl.read` |
| `run.context.read` | read the context of one's own active run (schema `run_context.v1` with a trust mark per message, [09-agents.md](09-agents.md), section 4.6) | run | agent-self | no | no | no | no | `rl.agent_runtime` |
| `run.start` | start a run with a valid token: `queued -> running` and `waiting_input -> running` (CAS); the same token repeated returns the result again as long as the run is `running`; a consumed token does not resume a waiting run | run | agent-self and token valid | no | no | denials | no | `rl.agent_runtime` |
| `run.token.renew` | fresh invocation token for one's own run in `queued` or `waiting_input` with the current unconsumed token, even an expired one (`runs.started_jti` NULL or not equal to `jti`, in SQL `IS DISTINCT FROM`), or the token before the last renewal (`runs.renewed_from_jti`, lost response); repeats the checks before a delivery (agent.invoke, I2, SR2, SR3) with current permissions ([09-agents.md](09-agents.md), section 4.5.1) | run | agent-self and token current | no | no | denials | no | `rl.agent_runtime` |
| `run.update` | live box patch of one's own run | run | agent-self | no | no | no | no | `rl.agent_runtime` |
| `run.finish` | finish a run or let it wait: `completed`, `failed`, `cancelled`, `waiting_input` | run | agent-self | no | no | no | no | `rl.agent_runtime` |
| `run.cancel` | cancel a run | run | trigger and (triggering person of the run or manage) | no | no | yes | no | `rl.trigger` |
| `run.followup.create` | follow-up to an active run of the same agent (via `message.create`) | thread | write and `agent.invoke` | no | no | denials | no | `rl.message` |
| `component.update` | deactivate or change own components | component | author of the message (agent-self or app-self) | no | no | no | no | `rl.write` |
| `interaction.create` | use a component (single-use rule per message); respect `required_capability` and `restricted_to` (`any`, `run_starter`, `owner`); for components of an agent additionally `agent.invoke` | component | trigger | no | no | yes for `approve_actions`, otherwise denials | no | `rl.interaction` |
| `interaction.respond` | the owner acknowledges or answers an interaction | interaction | agent-self or app-self | no | no | no | no | `rl.agent_runtime` |
<!-- END GENERATED: actions.agents -->

### 4.11 Apps and webhooks

<!-- BEGIN GENERATED: actions.apps -->
| ID | Description | Resource | Permission | Owner | Step-up | Audit | Permissions | RL class |
|---|---|---|---|---|---|---|---|---|
| `app.read` | read apps, manifests, installations | app | owner | yes | no | no | no | `rl.read` |
| `app.install` | install an app, grant scopes | instance | owner | yes | yes | yes | yes | `rl.admin` |
| `app.scopes.update` | change granted scopes | app | owner | yes | yes | yes | yes | `rl.admin` |
| `app.suspend` | switch an app off without losing its granted scopes: tokens blocked, deliveries paused, commands hidden, components `disabled` | app | owner | yes | yes | yes | yes | `rl.admin` |
| `app.resume` | switch a suspended app back on | app | owner | yes | yes | yes | yes | `rl.admin` |
| `app.uninstall` | uninstall an app; tokens and subscriptions end | app | owner | yes | yes | yes | yes | `rl.admin` |
| `webhook_out.read` | read outgoing subscriptions and deliveries | app, agent | owner | yes | no | no | no | `rl.read` |
| `webhook_out.manage` | create, change, delete an outgoing subscription of an app or an agent, rotate the HMAC secret | app, agent | owner | yes | yes | yes | no | `rl.admin` |
| `webhook_out.redeliver` | repeat a delivery | webhook_delivery | owner | yes | no | yes | no | `rl.admin` |
| `webhook_out.deliver` | internal check before every delivery: receive action of the event for the app or the agent | app, agent | grants and scope of the receiver | no | no | no | no | `none` |
| `webhook_out.transition` | automatic state change of an outgoing subscription by the consumer `webhooks` or the job `apps.webhook_deliver`: `disabled` after `410`, after `webhook.max_consecutive_failures` failures in a row, after a delivery that fails until `webhook.retry_max_age`, or on a gap in the event log; `paused` by the delivery quota `webhook.max_pending_per_subscription` ([11-extensions.md](11-extensions.md), sections 6 and 7; [webhooks.md](../api/webhooks.md), sections 4 and 5) | app, agent | system | no | no | yes | no | `none` |
| `webhook_in.read` | read the incoming webhooks of a channel (without token) | channel | manage | no | no | no | no | `rl.read` |
| `webhook_in.create` | create an incoming webhook for a channel | channel | manage | no | yes | yes | no | `rl.admin` |
| `webhook_in.revoke` | revoke an incoming webhook | incoming_webhook | manage | no | yes | yes | no | `rl.admin` |
| `webhook_in.post` | message via an incoming webhook; writes as the app with `origin = app` | channel | public with token; the app needs write | no | no | denials | no | `rl.webhook_in` |
<!-- END GENERATED: actions.apps -->

### 4.12 Import, costs, privacy, status

<!-- BEGIN GENERATED: actions.operations -->
| ID | Description | Resource | Permission | Owner | Step-up | Audit | Permissions | RL class |
|---|---|---|---|---|---|---|---|---|
| `import.read` | read import sources, progress, errors | import_source | owner | yes | no | no | no | `rl.read` |
| `import.manage` | create a source, start, pause, cancel, complete, ownership list per channel (the dry run only reads and needs `import.read` and read permissions) | import_source | owner | yes | yes | yes | no | `rl.admin` |
| `import.apply` | write batches with origin keys of a running source (items of a thread, files, cursor); only in channels of this source, also read-only ones ([22-import-migration.md](22-import-migration.md), section 8) | import_source | owner | yes | no | denials | no | `rl.import` |
| `cost.read.thread` | costs of a run and a thread (header line, live box) | thread | read | no | no | no | no | `rl.read` |
| `cost.read.aggregate` | costs per space, agent, instance | instance, space, agent | owner; space with manage | no | no | no | no | `rl.read` |
| `price.read` | read the price table | instance | owner | yes | no | no | no | `rl.read` |
| `price.manage` | maintain prices with source and retrieval time | instance | owner | yes | yes | yes | no | `rl.admin` |
| `cost.reconcile` | reconcile estimates with the invoice by hand | instance | owner | yes | yes | yes | no | `rl.admin` |
| `cost.reconcile.read` | read the estimates per run of a period for the reconciliation: agent, provider, currency, amount of the estimate before the reconciliation and whether reconciled; only entries with `billing_mode = 'organization'`, without channel, thread and content and without a filter by visible channels ([10-backend-policy.md](10-backend-policy.md), section 12.4; R-AZ-28) | instance | owner or app with scope `costs:reconcile` | no | no | no | no | `rl.read` |
| `cost.reconcile.submit` | the reconciliation app submits `reconciled` entries for existing runs; deletes nothing ([10-backend-policy.md](10-backend-policy.md), section 12.4; [ADR 0055](../adr/0055-cost-reconciliation-as-its-own-app-admin-key-never-in-the-server.md)) | instance | app with scope `costs:reconcile` | no | no | yes | no | `rl.admin` |
| `export.channel` | export the channel history | channel | read and +export | no | yes | yes | no | `rl.export` |
| `gdpr.request.create` | export or deletion for oneself; for others only the owner | user | self or owner | no | yes | yes | no | `rl.export` |
| `gdpr.request.read` | read a request and its state | gdpr_request | self or owner | no | no | no | no | `rl.read` |
| `gdpr.request.decide` | approve or reject a request; the export of one's own data is approved by the system when `privacy.export_requires_approval = false` | gdpr_request | owner | yes | yes | yes | no | `rl.admin` |
| `gdpr.export.download` | download the export package | gdpr_request | self (data subject) or owner | no | yes | yes | no | `rl.export` |
| `status.read` | read the status page | instance | auth | no | no | no | no | `rl.read` |
| `status.manage` | maintain components and incidents | instance | owner | yes | no | yes | no | `rl.admin` |
<!-- END GENERATED: actions.operations -->

### 4.13 Dev environments (milestone)

These belong to the milestone "Multi-user dev environment"; their routes are created with it
([21-dev-isolation.md](21-dev-isolation.md)).

<!-- BEGIN GENERATED: actions.dev -->
| ID | Description | Resource | Permission | Owner | Step-up | Audit | Permissions | RL class |
|---|---|---|---|---|---|---|---|---|
| `dev_environment.read` | read an environment, its state and check results | dev_environment | owner or self | no | no | no | no | `rl.read` |
| `dev_environment.manage` | create, approve, suspend, retire an environment | dev_environment | owner | yes | yes | yes | yes | `rl.admin` |
| `dev_environment.report` | the runner reports check results and readiness | dev_environment | agent-self (runner of the environment) | no | no | yes | no | `rl.agent_runtime` |
<!-- END GENERATED: actions.dev -->

### 4.14 Hosts and terminal

Normative together with [24-terminal.md](24-terminal.md). Milestone M2.

<!-- BEGIN GENERATED: actions.terminal -->
| ID | Description | Resource | Permission | Owner | Step-up | Audit | Permissions | RL class |
|---|---|---|---|---|---|---|---|---|
| `host.read` | read hosts: name, state, platform, profiles, fingerprint, LLM declaration, check results, valid assignments and reported names of the session users; owners all, others the hosts whose assignee they are or on which they have a `terminal` grant | host | owner or assignee or +terminal | no | no | no | no | `rl.read` |
| `host.manage` | create hosts, change them (name, assignee), issue a pairing code (also re-pairing), confirm a fingerprint, declare the LLM login, assign session users and end assignments (T12; a name already taken yields `409 conflict` with `reason: session_user_taken`, a name that the host does not report yields `400 validation_failed` with `errors[].code = session_user_not_on_host`), disable, re-enable, retire; I6 (rule 1.11) | instance, host | owner | yes | yes | yes | yes | `rl.admin` |
| `host.pair` | the host service redeems a pairing code and reports its public keys, profiles and the names of its session users | host | public (valid pairing code) | no | no | yes | no | `rl.public_auth` |
| `host.connect` | control connection and data connections of the host service; reports (profiles, names of the session users, check results, start and end of sessions) | host | host-self | no | no | denials; yes on a change of server host, check result or reported session users | no | `rl.host` |
| `terminal.open` | open a terminal session; SR10, SR11 (I6), SR12 (without a valid assignment of a session user whose name the host reports, `403 terminal_denied` with `reason: no_session_user`), limits; for the profile `claude` only a working directory of one's own session user; issues the terminal session token with the session user | host | human and +terminal | no | fresh | yes | no | `rl.terminal` |
| `terminal.attach` | attach to one's own session that is not closed, also from another device; new token. Step-up fresh, except from the same device in the same login session within `terminal.quick_reattach_window` after the detach; this exception never applies in the web console | terminal_session | self and +terminal | no | fresh | yes | no | `rl.terminal` |
| `terminal.stream` | open the device's stream connection to one's own session; only the device of the current token | terminal_session | self | no | no | denials | no | `rl.terminal` |
| `terminal.read_own` | read own terminal sessions; owners read all | terminal_session, host | self or owner | no | no | no | no | `rl.read` |
| `terminal.close` | end a session; one's own, owners any | terminal_session | self or owner | no | no | yes | no | `rl.write` |
| `terminal.terminate` | end a session by the server: revocation, idle, maximum duration, detach beyond `terminal.detach_grace`, disabling, logout, state change of the host, end of the process | terminal_session | system | no | no | yes | no | `none` |
<!-- END GENERATED: actions.terminal -->

Agents, apps and personal tokens reach none of these actions except `host.pair` and
`host.connect`, which have no principal from `principals` (`subject_kind_not_allowed`, SR5).
There is no scope for apps.

## 5. Agent invocation `agent.invoke`

`authorize(P, agent.invoke, {thread T, agent A, trigger_message M | interaction I | command C})`
is `allow` if and only if all items hold. It runs when the request is accepted and once more
immediately before every delivery to the adapter (I2). Which message triggers an agent at all is
decided by [09-agents.md](09-agents.md), section 3.4.

1. P is an active person. Agents, apps, imports and system messages never trigger
   (`messages.origin = 'human'`).
2. P effectively has `trigger` on the channel of T. DM channels have no agents. The channel
   is not read-only (rule 1.8).
3. A is active and one of the two:
    - A is the operator of T: `threads.operator_agent_id` for `kind = 'thread'`, the effective
      operator of the channel in the main thread and for new threads; the mention rule of the
      channel is met ([03-data-model.md](03-data-model.md), `channels`); or
    - A is mentioned in M, `channels.allow_mentioned_agents` is `true`, A itself has at least
      `write` on the channel, and A has `billing_mode = 'organization'` (an agent with `personal`
      is never a guest agent, because I1 does not count guest channels among its targets).
4. Backend policy: with `agents.billing_mode = 'personal'`, `P = credential_holder_id` applies
   (I2). Otherwise denial with reason `billing_mode_personal_not_holder`, fail closed, audit.
5. Tool class (SR2): if the higher of `agents.tool_class` and `agents.reported_tool_class`
   equals `host`, P is an active owner, or SR3 proves a ready dev environment of P's own.
   Otherwise `tool_class_owner_only`.
6. For an interaction: the component is active and not expired; P has `required_capability`
   (for example `approve_actions`); `restricted_to = 'run_starter'`: P is the triggering person
   of the run or an owner; `restricted_to = 'owner'`: P is an owner. The single-use rule sets the
   component to `used` in the same transaction and the other `single_use` components of the
   message to `disabled`.
7. For forum channels with a project binding: `project_bindings.ready = true` (otherwise
   `project_not_ready`) and SR3 from [07-permissions.md](07-permissions.md) (dev environment, milestone).
8. The thread has at most one active run. If there is one of A, triggering by a message becomes
   a follow-up (`run.followup.create`); if it belongs to another agent, denial
   `run_active_other_agent`. Control commands go to the agent immediately as `command.invoked`;
   model commands are denied during an active run (`run_active`).

App commands and components of apps do not check `agent.invoke` and I2, because no agent
runtime is involved; what is checked is `required_level`, `required_capability`,
`restricted_to`, the grants and the scope of the app ([11-extensions.md](11-extensions.md),
section 9).

**Check without a request.** In these cases the server evaluates the permissions of a person
without a request of this person (R-AZ-29):

- `agent.invoke` immediately before every delivery of `agent.invocation`, `agent.followup`,
  `interaction.created` and `command.invoked` to an agent
  ([04-sync-protocol.md](04-sync-protocol.md), section 15.3 step 2, section 15.4 item 2 and
  section 15.6), at `run.token.renew` (04 section 15.3 step 3) and for the new run from deferred
  follow-ups (04 section 15.4 item 4);
- `command.invoke` and `interaction.create`, when the server checks an app command or the use of
  a component of an app again with the permissions of that moment before handing it to the app
  ([11-extensions.md](11-extensions.md), section 9; R-SY-02);
- the permissions of the triggering person in the acting-for binding (rule 2.3 item 6, SR9 in
  [07-permissions.md](07-permissions.md));
- `push.deliver` before each hint decision and each push send, and the recipient rule of R-AZ-30
  for hints without a thread ([13-push-notifications.md](13-push-notifications.md),
  sections 4.2 and 12).

The following applies:

- The person is a principal of kind `user` without session, device, token, `view_as` and
  `acting_for`. Only the server itself builds this principal, never the authentication of a
  request.
- Blocks come only from the person's status (SR7): `disabled` or `deleted` gives
  `principal_inactive`, `pending` gives `principal_pending`; in the acting-for binding such a
  person has no permissions. Sessions, devices and tokens of the person do not count.
- The owner status comes from the current owner list (`owners` with `revoked_at IS NULL`),
  without a session (exception to rule 2.3 item 1 and R-AZ-14); everything else from rule 2.3
  applies unchanged. A permission state computed this way is never the own permission state of
  the principal of a request, not even of a personal API token of the same person; in the
  acting-for binding it only narrows.
- `authorize` with this principal and an action other than `agent.invoke`, `command.invoke`,
  `interaction.create` or `push.deliver` is a programming error and ends like an unknown action
  (rule 1.5).
- If such a check writes an audit entry, this person is the actor, without device and session.

Consequences on `allow`: run `queued`, live box anchor, `trigger_state`, event to the agent,
invocation token on delivery ([04-sync-protocol.md](04-sync-protocol.md), section 15). The
token is a JWS with the header `alg = EdDSA`, `kid` and `typ = agent-invocation+jwt` (RFC 8725,
section 3.11). Claims:

| Claim | Content |
|---|---|
| `iss` | `instance.instance_id` |
| `aud` | principal ID of the agent |
| `sub` | principal ID of the triggering person |
| `jti`, `iat`, `exp` | UUIDv7 of the token, issuance, expiry (`iat + agent.invocation_token_ttl`) |
| `trigger_kind` | `message`, `followup`, `interaction`, `command` or `autocomplete` |
| `run_id` | run; missing for `trigger_kind = autocomplete` and for `command.invoked` when the owner of the control command has no active run in the thread at execution time (`command.invoke`); otherwise payload and token of `command.invoked` carry this run, never the run of another agent (R-AZ-25) |
| `message_id` or `interaction_id` or `command_id` | triggering object |
| `thread_id`, `channel_id`, `space_id` | location; for `trigger_kind = autocomplete`, `thread_id` is the thread of the request, without one the main timeline of the text channel; in a forum channel without a thread there is no token ([04-sync-protocol.md](04-sync-protocol.md), section 15.7; R-AZ-26) |
| `level` | effective level of `sub` on the channel at delivery time |
| `billing_mode` | configured mode from `agents.billing_mode` at delivery time (I3) |
| `tool_class` | configured tool class from `agents.tool_class` at delivery time |
| `dev_environment_id` | dev environment of the run, only in the dev environment milestone |
| `permission_version` | permission version at delivery time |

The adapter starts no run if its reported mode is not equal to `billing_mode`
(`billing_mode_mismatch`) or its actual tool class is higher than `tool_class`
(`tool_class_mismatch`).

Consequences on denial: the message is stored if `message.create` was allowed, but without a
run; `trigger_state = failed` and `trigger_state_reason` with the reason, so that the person sees
it. On a denial only before delivery: run `failed` with a reason, `trigger_state = failed`.

## 6. Durable event types and receive action

A receiver gets an event only if `authorize(receiver, receive_action, resource)`
allows it. Personal events (`audience = self`) go only to the named principal.

<!-- BEGIN GENERATED: events.durable -->
| Event type | Resource | Receive action | Note |
|---|---|---|---|
| `message.created` | message | `message.read` | echo with `client_msg_id` to the author |
| `message.updated` | message | `message.read` | edit, `trigger_state`, component state |
| `message.deleted` | message | `message.read` | tombstone |
| `reaction.added` | message | `message.read` | |
| `reaction.removed` | message | `message.read` | |
| `pin.added` | message | `thread.read` | |
| `pin.removed` | message | `thread.read` | |
| `thread.created` | thread | `thread.read` | |
| `thread.updated` | thread | `thread.read` | title, tags, operator; during an import once per batch instead of `message.created` per message |
| `thread.closed` | thread | `thread.read` | |
| `thread.reopened` | thread | `thread.read` | |
| `thread.moved` | thread | `thread.read` | applies to: target channel |
| `thread.moved_out` | channel | `channel.read` | applies to: source channel; receivers without access to the target remove the thread |
| `agent_session.changed` | thread | `thread.read` | new session, rebinding, binding after an import |
| `run.state_changed` | run | `run.read` | with summary |
| `space.created` | space | `space.read` | |
| `space.updated` | space | `space.read` | name, icon, order, operator; if the default operator changes, a `channel.updated` with a new version follows for each inheriting, non-archived channel, because its effective operator changes |
| `space.archived` | space | `space.read` | |
| `channel.created` | channel | `channel.read` | |
| `channel.updated` | channel | `channel.read` | name, topic, order, operator, `read_only`, `default_notification_level`; changes to forum tags (only the new version, tags via REST) |
| `channel.settings_changed` | channel | `channel.read` | only `settings_version`; contents via `channel.behavior.read` or `agent.config.read` |
| `channel.archived` | channel | `channel.read` | |
| `command.changed` | command | `command.read` | registered, changed, removed |
| `file.updated` | file | `file.read` | scan result, thumbnail, transcript |
| `user.updated` | user | `user.read` | |
| `agent.updated` | agent | `agent.read` | also on creation (there is no separate type); including a change of the backend policy and of the tool class; also reaches the agent itself (change of its configured values) |
| `instance.updated` | instance | `instance.read` | branding, switches (keys only, values via REST) |
| `status.updated` | instance | `status.read` | |
| `grant.changed` | grant | `grant.read` | for administrators; the subject gets `access.changed` |
| `group.changed` | group | `group.read` | |
| `access.changed` | principal | self | section 8 in 04 |
| `access.expiring` | principal | self | a grant or membership expires within `authz.expiry_reminder_lead`; to the subject (for groups, the person) and to the granter |
| `session.revoked` | auth_session | self | additionally close with `4012` |
| `device.pending` | device | self | a new device of the human waits for approval; written by `identity` in the transaction that registers the device with state `pending`; payload only `device_id` and `platform` (R-AZ-31) |
| `read_state.updated` | thread | self | |
| `draft.updated` | draft | self | |
| `draft.deleted` | draft | self | |
| `settings.updated` | user | self | |
| `sidebar.updated` | space | self | |
| `notification_pref.updated` | user | self | |
| `quiet_hours.updated` | user | self | |
| `gdpr.request_updated` | gdpr_request | `gdpr.request.read` | |
| `import.progress` | import_source | `import.read` | |
| `agent.invocation` | run | self (agent) and `agent.events.consume` | before delivery, `agent.invoke` for the triggering person |
| `agent.followup` | run | self (agent) and `agent.events.consume` | before delivery, `agent.invoke` for the triggering person |
| `run.cancel_requested` | run | self (agent) and `agent.events.consume` | |
| `interaction.created` | interaction | self (agent or app, owner of the component); agent additionally `agent.events.consume`, app `events:subscribe` | before delivery to agents, `agent.invoke` |
| `command.invoked` | command | self (agent or app, owner of the command); agent additionally `agent.events.consume`, app `events:subscribe` | before delivery to agents, `agent.invoke` |
| `agent.config_changed` | channel | self (agent) and `agent.config.read` | prompt, skills, project binding changed |
| `host.updated` | host | `host.read` | also on creation; state, assignee, LLM declaration, profiles, check results, assignments of the session users; `key_changed` on re-pairing; never keys |
| `terminal_session.changed` | terminal_session | self | to the person of the session: state, device, `close_reason`; never contents |
<!-- END GENERATED: events.durable -->

Ephemeral events:

<!-- BEGIN GENERATED: events.ephemeral -->
| Event type | Resource | Receive action | Additional condition |
|---|---|---|---|
| `typing.started` | thread | `thread.read` | thread in focus |
| `run.updated` | run | `run.read` | thread in focus |
| `agent.presence_changed` | agent | `agent.read` | |
| `host.presence_changed` | host | `host.read` | |
| `stats.live` | instance, space | `stats.read` | only to connections with a `stats.subscribe` subscription; content filtered to the visible channels, costs only with `cost.read.aggregate` ([25-visualization.md](25-visualization.md), section 5) |
| `notification.created` | thread; for a hint without a thread the recipient (`user`) | self and `push.deliver`; for a hint without a thread: self and the recipient rule of R-AZ-30 | only to connections of the receiver, instead of a push to this device ([13-push-notifications.md](13-push-notifications.md), section 6.3) |
| `command.autocomplete_requested` | command | self (owner of the command); agent additionally `agent.events.consume` | carries a token with `trigger_kind = autocomplete`; only to one gateway connection of the owner |
<!-- END GENERATED: events.ephemeral -->

Outgoing webhooks and push use the same receive action (`webhook_out.deliver`,
`push.deliver`).

## 7. WebSocket ops and action

<!-- BEGIN GENERATED: ws_ops -->
| Op | Direction | Action |
|---|---|---|
| `identify` | client to server | `gateway.connect` |
| `resume` | client to server | `gateway.connect` |
| `heartbeat` | client to server | `gateway.connect` (the session must be valid, consumes nothing from `rl.gateway_connect`); for agents additionally `agent.heartbeat` |
| `reauth` | client to server | `gateway.connect` (consumes nothing from `rl.gateway_connect`) |
| `focus` | client to server | `gateway.focus` per thread |
| `typing.start` | client to server | `typing.start` |
| `typing.stop` | client to server | `typing.start` |
| `ack` | agent to server | `agent.events.consume` |
| `run.update` | agent to server | `run.update` |
| `command.autocomplete_result` | agent or app to server | `command.autocomplete.respond` |
| `stats.subscribe`, `stats.unsubscribe` | client to server | `stats.read` on the instance or space respectively |
| `hello`, `ready`, `resumed`, `cursor`, `catchup.complete`, `resync_required`, `heartbeat_ack`, `reauth_required`, `error` | server to client | none of its own; part of the session from `gateway.connect` |
| `dispatch` | server to client | receive action of the event type (section 6) |
<!-- END GENERATED: ws_ops -->

Terminal channels ([24-terminal.md](24-terminal.md); [asyncapi.yaml](../api/asyncapi.yaml)):

<!-- BEGIN GENERATED: channel_ops -->
| Channel | Op | Direction | Action |
|---|---|---|---|
| device stream `/terminal/sessions/{session_id}/stream` | `hello` | server to device | `terminal.stream` |
| | `identify` | device to server | `terminal.stream` |
| | binary frames (Noise) | both | `terminal.stream` (the session must be valid) |
| | `error` | server to device | `terminal.stream` |
| control `/hosts/connect` | `hello`, `ready`, `heartbeat_ack`, `error`, `session.open`, `session.close` | server to host | `host.connect` |
| | `identify`, `heartbeat`, `report`, `session.started`, `session.ended` | host to server | `host.connect` |
| data `/hosts/sessions/{session_id}/stream` | `hello`, `identify`, binary frames (Noise) | both | `host.connect` (the session belongs to the host) |
| | `error` | server to host | `host.connect` |
<!-- END GENERATED: channel_ops -->

## 8. Decisions of this chapter

| No. | Decision | Reason |
|---|---|---|
| R-AZ-01 | `403` only with an existing read permission, otherwise `404`. | Baseline B7; reveals no existence and stays understandable for those entitled. |
| R-AZ-02 | DM channels are readable only by participants, not even by owners. The only way in the product is the GDPR export of a person (`gdpr.request.create` for others, owner, step-up, audit); it contains only that person's own messages. Other ways (an order by an authority) lie outside the product. | Confidentiality between people (ruling L21); a default that the owner confirms. |
| R-AZ-03 | Step-up actions only by people in a session. | A token cannot provide a confirmation by the person; administration by token would be a bypass. |
| R-AZ-04 | All denials of requests and ops go into the audit, identical denials aggregated per window; filters during delivery and in lists write none. | Study C requires a record of every denial; without aggregation, probing fills the log; a filter is not a request, otherwise every event would write one row per non-receiver. |
| R-AZ-05 | `manage` may assign grants only up to its own level, with its own capabilities (`manage` included), only to people and groups and not on the instance; revocation only within the same bounds. | "delegable" according to the baseline, without an escalation of permissions beyond one's own grant; grants to agents and apps are data flows to processors and stay with the owner. |
| R-AZ-06 | Layout actions (create, sort, rename a space) without step-up; permissions, identity, agents, costs, export, deletion with step-up. | Step-up where damage arises; otherwise it gets clicked away out of habit. |
| R-AZ-07 | `run.cancel` only by the triggering person, `manage` or owner. | With organization agents, people should not cancel each other's runs. |
| R-AZ-08 | `pin.create` needs `manage`. | Everyone sees pinned items; that is moderation. |
| R-AZ-09 | Channel behavior (prompt, skills) is readable only with `manage` and never in events. | Prompts can contain internal notes; read permission on the channel is not enough. |
| R-AZ-10 | Only owners set the operator, the project binding and the tool class. | All three affect I1 or the tool power of the agent. |
| R-AZ-11 | Internal delivery checks (`push.deliver`, `webhook_out.deliver`) are actions of their own. | Rule 1.1: deliveries too run through the catalogue and are testable. |
| R-AZ-12 | The read-only setting of a channel blocks all `write` and `trigger` actions for all principals, including owners; only `import.apply` writes. | A read mirror has exactly one leading location; no agent may be triggered in the mirror. Grants cannot restrict owners. |
| R-AZ-13 | Operator rule: the effective operator has `write` implicitly; `space.operator.set` and `channel.operator.set` require no grant of its own for the agent. | Follows the operator without a second, easily forgotten grant; on a change no remnant stays. Replaces the proposal to require a grant when setting the operator. |
| R-AZ-14 | Owner permissions only in a session, never through a personal API token; the exception is the check without a request (R-AZ-29). | A stolen token without device binding must never open the whole instance. |
| R-AZ-15 | Expirations, the first installation, the recovery on the host, the sync of claim groups, the automatic revocation of devices and scheduled key rotations are system actions in the catalogue (`grant.expire`, `group.member.expire`, `group.member.sync`, `instance.bootstrap`, `owner.recover`, `device.expire`, `signing_key.transition`). | Every audit entry names a known action; expirations reach clients and runs via `access.changed`; these operations are security relevant and belong in the audit, without faking an action by a person. |
| R-AZ-16 | A waiting device gets tokens, but may only read itself (`403 device_pending`). | The app can wait for the approval without a new login; it reaches contents only after that. |
| R-AZ-17 | `resolve_i1` carries out "switch first, then grant" in one transaction, with both checks. | The guided flow of the console needs no intermediate state in which I1 would be violated. |
| R-AZ-18 | App commands and app components without `agent.invoke` and I2. | No agent runtime, no subscription; the limit is the grants, the scope and `required_level`. |
| R-AZ-19 | Guest agents only with `billing_mode = 'organization'`. | Guest channels do not count among the I1 targets; a personal guest could otherwise be triggered by strangers. |
| R-AZ-20 | Triggering another agent during an active run is denied (`run_active_other_agent`), not turned into a follow-up. | A follow-up belongs to the run of its agent; a foreign agent would otherwise get text that was not meant for it. |
| R-AZ-21 | `settings.read` covers all of one's own personal data; no separate read action per kind. | All are `self`, with no difference in permissions; fewer actions, same effect. |
| R-AZ-22 | Health has no action; liveness and readiness are on the internal listener outside the public API (ruling L2, [16-operations.md](16-operations.md)). | An operations contract, not a product API; no auth bypass in the public listener. |
| R-AZ-23 | Terminal actions only for people in a session, with step-up `fresh` per opening; the host acts only through `host.pair` and `host.connect`. | The owner requires a passkey when opening; a terminal is remote access to a host ([24-terminal.md](24-terminal.md), R-TM-07). |
| R-AZ-24 | Hosts are a target of their own outside the tree; grants there only by owners. | A grant on the instance or a space must never silently open a host; `manage` is meaningless on hosts. |
| R-AZ-25 | The token of `command.invoked` carries `run_id` only if the owner of the control command has an active run in the thread at execution time, and then exactly this run, like the payload; otherwise the claim is missing (section 5). | Control commands run without a run and still go to the agent with a token ([09-agents.md](09-agents.md), section 10.4, D-AG-12); the adapter learns which of its own runs the command concerns, and never a foreign one. The token starts, resumes and renews no run (`run.start`, `run.token.renew`), because its `jti` is never in `runs.invocation_jti`: only deliveries of `agent.invocation`, `agent.followup` and `interaction.created` and their renewal enter a token there ([04-sync-protocol.md](04-sync-protocol.md), section 15.3). |
| R-AZ-26 | Autocomplete of an agent command without a thread in the request: in a text channel, token and payload name the main timeline as `thread_id`. In a forum channel the server checks as for a new thread of the channel; if the check passes, it does not ask the agent, issues no token and returns an empty list at once ([04-sync-protocol.md](04-sync-protocol.md), section 15.7). | Every token names a thread as its location (section 5), and a forum has no main timeline (R-DM-11). A token with only a channel would be a new form with another location that every check of a token would have to know (adapter, server, acting-for binding); without a token the surface stays smaller. The check still runs, so that the empty list reveals nothing and denials go into the audit (R-AZ-01, R-AZ-04). Typing costs nothing and never hangs (R-AG-14); the client then shows the static choice values, if there are any ([09-agents.md](09-agents.md), section 10.5). |
| R-AZ-27 | Automatic state changes of outgoing subscriptions (`disabled` after `410`, after `webhook.max_consecutive_failures`, after `webhook.retry_max_age` and on a gap; `paused` by the delivery quota) are the system action `webhook_out.transition`: per change one audit entry without an actor in the transaction of the change, with the subscription, the old and new state and the cause as a code. Changes by the owner stay `webhook_out.manage`; the disabling at uninstallation is covered by the entry of `app.uninstall`. | Rule 1.10: every audit entry names a known action, and [webhooks.md](../api/webhooks.md), section 4, requires the disabling in the audit log; as with R-AZ-15, a system action does not pretend to be an action of a person. One action for all causes, because the cause is in the details (D-W-104). |
| R-AZ-28 | `cost.reconcile.read` gives the reconciliation app with scope `costs:reconcile` and the owner in a session the estimates per run in the mode `organization` for the requested agents and the period: run, agent, provider, currency, amount of the estimate before the reconciliation and whether reconciled. Without step-up, with the effect `read` (section 2.7); allows without audit, denials with audit (R-AZ-04). The list evaluates the instance and does not filter by visible channels (exception to rule 1.2); it never names channel, thread or content, and the instance is the only check, without a check per entry; a run ID opens nothing, because whoever wants to read the run still needs `run.read` on its path. Agents, personal tokens and people without owner status do not reach it (sections 2.3 and 2.6). | The app distributes the daily amount in proportion to the estimates and submits it per run ([10-backend-policy.md](10-backend-policy.md), section 12.4, items 3 and 4; [ADR 0055](../adr/0055-cost-reconciliation-as-its-own-app-admin-key-never-in-the-server.md)); without a read action it knows neither the estimates nor the run IDs, and `cost.read.aggregate` returns only sums and reaches no app (section 2.3 item 5). Reading is weaker than `cost.reconcile.submit`, which the same scope already allows for every run of the instance; a filter by visible channels would leave an app without grants only an empty list. Personal amounts stay out, because they are never an invoice (03 `cost_entries`); DM channels have no agents and therefore no runs ([07-permissions.md](07-permissions.md), section 2.8). Like `cost.read.aggregate` and `price.read`, reading needs no step-up; R-AZ-06 requires it for changes to costs, and an app cannot perform it (R-AZ-03; D-W-104). |
| R-AZ-29 | Check without a request (section 5): if the server evaluates the permissions of a person without the person's request (`agent.invoke` before every delivery to an agent, at `run.token.renew` and for the run from deferred follow-ups; `command.invoke` and `interaction.create` before handing over to an app; the permissions of the triggering person in the acting-for binding), this person is a principal without session, device and token: blocks come only from the person's status, the owner status from the current owner list (exception to R-AZ-14). Only the server builds this principal, only for these cases, and the permission state computed this way is never the own permission state of the principal of a request. | None of these cases has a request, session, device or token of the person; with R-AZ-14 alone no invocation of an owner would pass the repeated check (AZ-M19, every agent with `personal` or `host`), and no run of an owner would read its own context in the acting-for binding. The acceptance has already applied R-AZ-14 to the real request; afterwards the server evaluates the same person with the permissions of the moment, and a block or the loss of the owner status takes effect at once. A request with a personal token still gets no owner permissions. Remainder: an owner who triggered through grants with a personal token is evaluated as an owner afterwards; in the acting-for binding the run then reaches as far as the permissions of the agent. A permission of another person never arises from this (D-W-104). |
| R-AZ-30 | Hints without a thread (kinds `security` and `admin`, [13-push-notifications.md](13-push-notifications.md), section 2) are not checked with `push.deliver` on a thread. Instead the server evaluates, at the time of each decision and each send, the recipient rule of 13 section 2 with the rights of that moment: the recipient is an active human; `security` goes only to the other active devices of the human whose new device waits for approval; `admin` goes only to the owners or administrators that [07-permissions.md](07-permissions.md) and [20-privacy.md](20-privacy.md) name for the reminder. `notification.created` for such a hint has the recipient (`user`) as its resource and goes only to connections of that recipient (self). The payload names only the kind and the target ID, as for every hint. | `push.deliver` has the resource type `thread`, and these kinds have no thread, so without a rule no check would be defined for them and the push path could not be built (M3-17, M3-19, M3-23). The recipient rule is the same check the chapter already states, evaluated at send time like `push.deliver` (P3), so a revoked device or a lost owner status stops the hint at once; it grants no read right on any content (D-W-108). |
| R-AZ-31 | The durable personal event `device.pending` (resource `device`, receive action self, audience the human who owns the device) marks a device that waits for approval; `identity` writes it in the transaction that registers the device with state `pending`, with only `device_id` and `platform`. The check of `push.deliver` and the recipient rule of R-AZ-30 are checks without a request (section 5, R-AZ-29). | Hints of the kind `security` ([13-push-notifications.md](13-push-notifications.md), section 2) arise in the worker consumer `push` from the event log (13 section 12 point 1, ADR 0051), and no event marked a waiting device, so the kind had no source. The hint check evaluates a person who sent no request, exactly like the cases of R-AZ-29 (D-W-108). |
| R-AZ-32 | New action `access.view_as.end` on the person's own console session (`auth_session`, self, without step-up, audit always, `rl.admin`): it ends "View as person" stored on the console session (03 `auth_sessions`, R-DM-49) before `authz.view_as_max_duration`; `access.view_as` from a console session starts the stored view. While a view is stored, the actions `access.view_as.end`, `auth.logout`, `auth.step_up` and `self.read` on the person's own session are decided without `view_as` ([07-permissions.md](07-permissions.md) section 4.2 steps 4 and 7 would otherwise deny them); every other decision of the session follows 07 section 9.2, and the first request after the end time gets `step_up_required`. The module `identity` owns the action, because the state lives on `auth_sessions`. | [07-permissions.md](07-permissions.md) section 9.2 requires audit at start and end and operations to start and end the view; ending must not need a fresh step-up, which `access.view_as` requires, so that an owner can always leave the view (D-W-100 extended to M5 and M6). |
| R-AZ-33 | New public action `console.csp_report` on the instance (no login, without step-up and audit, `rl.public_auth`, owned by `instance`): the endpoint of the web console's `report-to` accepts violation reports, logs only the directive, the disposition and the blocked origin and stores nothing; its effect is `read` (05 2.7), so reports are accepted in maintenance mode as well ([19-security.md](19-security.md), section 5, rule 6). | Browsers send reports without the session cookie in many cases, so the endpoint cannot require a session; every operation needs an action from the catalogue (05 section 1), and the rate class of public endpoints bounds abuse (D-W-123). |
| R-AZ-34 | New read action `access.hints.read` on the instance (owner) or a space (`manage`, within one's own `manage` scope; without step-up, audit `no`, `rl.read`, owned by `authz`): the lists "Permission hints" (H1, grants to disabled principals, `trigger` blocked by SR2 or SR3, agents in mode `personal` with their trigger set) and "Expiring soon" (grants, memberships, and for owners only agent and app tokens, within `authz.expiry_reminder_lead`) of [07-permissions.md](07-permissions.md), sections 9.4 and 9.5. | 07 section 9.5 requires the list "Permission hints" in the console, and `access.explain` and `grant.read` do not have the instance as a target; a read action of its own keeps the hints out of the audit and limits a manager to the targets they administer (D-W-123). |

## 9. Tests that hold this catalogue

1. Route test: every registered route has a known `x-authz-action` and its
   rate-limit class.
2. Event test: every registered event type and every op has an entry in 6 or 7 respectively.
3. Case table: principal x target x action x expected decision, including expired,
   revoked and overlapping grants, groups with expiry, DM, owner, owner with a
   personal token, agent with the operator rule, app, waiting device, read-only channel.
4. 404 test: for every reading route with a foreign ID, the response equals the one for a
   non-existent ID (status, body, timing within bounds).
5. I1 test: every action from rule 1.6 is checked with a change that would violate I1,
   and with `resolve_i1`, which makes it permissible.
6. I2 test: a permission revocation between acceptance and delivery leads to run `failed`, no
   invocation.
7. System actions: no route and no op names an action with permission `system`.
8. Terminal: every op of the terminal channels has an entry in section 7; the cases AZ-M54 to
   AZ-M61 from [07-permissions.md](07-permissions.md) and TM-T07 to TM-T10, TM-T17 and TM-T28 (a) from
   [24-terminal.md](24-terminal.md) run in the case table.

## 10. Configuration values of this chapter

| Name | Default | Meaning |
|---|---|---|
| `audit.deny_aggregate_window` | `60s` | aggregation window for identical denials |
| `audit.deny_aggregate_max_keys` | `10000` | at most this many open aggregations per process; limits the memory during floods without login |
| `status.public_enabled` | `false` | status page without login |
| `ratelimit.rl.public_auth.per_minute` / `.burst` | `30` / `10` | per client IP |
| `ratelimit.rl.gateway_connect.per_minute` / `.burst` | `12` / `4` | per device or token |
| `ratelimit.rl.read.per_minute` / `.burst` | `600` / `100` | per principal |
| `ratelimit.rl.write.per_minute` / `.burst` | `120` / `30` | per principal |
| `ratelimit.rl.message.per_minute` / `.burst` | `60` / `20` | per principal |
| `ratelimit.rl.trigger.per_minute` / `.burst` | `30` / `10` | per principal |
| `ratelimit.rl.interaction.per_minute` / `.burst` | `60` / `20` | per principal |
| `ratelimit.rl.upload.per_minute` / `.burst` | `30` / `10` | per principal |
| `ratelimit.rl.search.per_minute` / `.burst` | `60` / `20` | per principal |
| `ratelimit.rl.typing.per_minute` / `.burst` | `30` / `5` | per connection |
| `ratelimit.rl.admin.per_minute` / `.burst` | `60` / `20` | per principal |
| `ratelimit.rl.export.per_minute` / `.burst` | `2` / `2` | per principal |
| `ratelimit.rl.agent_runtime.per_minute` / `.burst` | `1200` / `200` | per agent |
| `ratelimit.rl.webhook_in.per_minute` / `.burst` | `60` / `20` | per incoming webhook |
| `ratelimit.rl.import.per_minute` / `.burst` | `120` / `30` | per owner for `import.apply` |
| `ratelimit.rl.terminal.per_minute` / `.burst` | `20` / `5` | per person |
| `ratelimit.rl.host.per_minute` / `.burst` | `120` / `30` | per host |
