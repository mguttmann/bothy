# M2-06: Invocation token with EdDSA and JWKS

Status: ready | Depends on: M2-05 | Milestone: M2

## Goal

After this ticket the module `runs` issues and verifies invocation tokens (invariant I3): compact JWS with the header `alg = EdDSA`, `kid` and `typ = agent-invocation+jwt` and exactly the claims of 05 section 5, signed with the `active` server key of purpose `invocation` (M1-32), so that every adapter can check them through the public JWKS (`getJwks`) but none can forge one (R-SY-08, ADR 0023, L24). Every call issues a new token with a new `jti`; nothing is stored. A scheduled key rotation fails no token at any moment, and an emergency rotation withdraws the old key on the server at once. A reference verifier, written independently with the Go standard library, performs the adapter check of 09 4.5 "Check in the adapter" and rejects every token with another `alg` or `typ`. Delivery (M2-08) puts the tokens into envelopes and stores their `jti`; run start and renewal (M2-09) use the server-side verifier built here. Without this ticket no adapter could tell a decision of the server from a forged event, and I3 (the adapter compares its actual billing mode with the claim) would have nothing to compare against.

## Read first

- `docs/PLAN.md`: section 3 "Way of working per ticket"; section 8 "M2 Agent platform, extensions and terminal" (Goal, Scope, Out of scope, Exit criteria items 1 and 10, Test strategy, Red team first paragraph: "forging or reusing the invocation token", Risks), block "agents and extensions", rows M2-05, M2-06, M2-07, M2-08, M2-09, M2-14, M2-15, M2-16, M2-17, M2-29, M2-31; section 18.1 row 5 and section 18.2 row "I3 signed invocation token, check in the adapter"; V2-02 (adds `dev_environment_id` later).
- `docs/spec/09-agents.md`: 4.5 "Invocation token" completely (Header, Claims, Check in the adapter points 1 to 10, the `reason` table, "Keys and rotation"); 4.5.1 "Fresh token for a waiting run" (step 1 checks the signature without expiry); 4.3 "Connection and cursor" (acceptance and `ack` after the check, newest token replaces the stored one); 4.4 "Events to the adapter" (which events carry a token); 10.4 "Execution"; 10.5 "Autocomplete"; 16.1 "Test doubles"; 16.2 "Required scenarios" (AG-08, AG-22, AG-28); 21 "Configuration values of this chapter".
- `docs/spec/05-actions.md`: 5 "Agent invocation `agent.invoke`" (header and the normative claim table); 4.10 "Agents, commands, runs, interactions" (rows `run.start`, `run.token.renew`).
- `docs/spec/04-sync-protocol.md`: 2.2 "Envelope" (`invocation_token` is a delivery field), 15.2, 15.3 "Invocation and run start", 15.6 "Webhook as transport", 15.7 "Autocomplete", 17 "Decisions of this chapter" (R-SY-07, R-SY-08, R-SY-10, R-SY-17, R-SY-19).
- `docs/spec/08-identity.md`: 7.1 "Overview" (row Invocation token), 7.2 "Access token" (`typ = at+jwt`, `auth.clock_leeway` for `exp`, `nbf`, `iat`).
- `docs/spec/03-data-model.md` (core contract, read only): `server_signing_keys`; `runs` (`invocation_jti`, `started_jti`, `renewed_from_jti`, `trigger_kind`, `triggering_command_id`).
- `docs/spec/10-backend-policy.md`: 3 "Invariants" (I3), 7 "Adapter check (I3)" (an unknown actual mode never equals the claim).
- `docs/spec/07-permissions.md`: "SR9 Acting-for binding (confused deputy)" (the token in later API requests, M2-14), the row `invocation_token_invalid` of the problem code table, 11.2 "Required tests" (AZ-P6).
- `docs/spec/06-api.md`: 5 "Errors" (row `invocation_token_invalid`).
- `docs/spec/19-security.md`: 9 "Logs without secrets and without content" (SEC-LOG-2: invocation tokens never appear), 10 "Secrets" (signing key row, SEC-SEC-3).
- `docs/spec/02-architecture.md`: 4.4 "Allowed dependencies" (row `runs`, reason "sign the token"), 4.5 "Cross-cutting ports" (`kernel/jose`, `kernel/ids`, `kernel/clock`, `kernel/errs`), 5 "Layers within a module", 6 "Code layout" (test helpers in `internal/<module>/adapters/<module>test`, cross-module tests in `test/`).
- `docs/spec/11-extensions.md`: 11.2 "Hand-written layer" (row `invocation`: the SDK verifier that M2-29 and M2-30 build after this reference).
- `docs/spec/23-quality.md`: "Documentation in code".
- `docs/spec/24-terminal.md`: 5.3 "Terminal session token" (only its header `typ = terminal-session+jwt`, which an invocation verifier must reject).
- `docs/api/openapi.yaml`: operations `getJwks` (schema `Jwks`, media type `application/jwk-set+json`), `getDiscovery` (`instance_id`, `jwks_uri`), `startRun`, `resumeRun`, `renewInvocationToken`; schemas `EventEnvelope` (`invocation_token`), `AgentInvocationPayload`, `AgentFollowupPayload`, `InteractionCreatedPayload`, `CommandInvokedPayload`, `CommandAutocompleteRequestedPayload`, `RunStart`, `InvocationTokenRenew`, `InvocationTokenGrant`, `Level`, `BillingMode`, `ToolClass`, `SigningKeyPurpose`; problem code `invocation_token_invalid` in `x-problem-codes` of `Problem`.
- ADRs: 0023 (signed invocation token, its "Confirmation"), 0066 (signing keys), 0067 (JOSE profile with the standard library), 0052 (one envelope; tokens are never stored), 0053 (no JSON `null`), 0026 (SDKs), 0008 (module rules).
- `docs/DECISIONS.md`: D-L24, D-I3, D-W-15, D-W-44, D-W-58, D-W-100, section 10 "Rulings of the build milestones" (row format `D-M2-06-<n>`).
- `docs/security/threat-model.md`: TB4, the row "Forged invocation to the adapter", T-TK-04, T-TK-05. `docs/RISKS.md`: RK-09.
- `docs/operations/configuration.md`: `agent.invocation_token_ttl`, `auth.clock_leeway`, `agent.jwks_max_age`, `signing_key.invocation_rotation_interval`, `signing_key.cache_ttl`, `signing_key.rotation_check_interval`, `run.queued_max_age` (server); `adapter.invocation_token_leeway`, `adapter.jwks_refresh_min_interval` (adapter).
- `docs/observability/signals.md`: it names no metric for issuing or verifying tokens; the only invocation metric, `bothy.agent.invocation.denied`, belongs to M2-08.
- M1 briefs and merged code this ticket builds on: M1-32 (`instance.Keys` with `Signer`, `VerificationKey`, `PublicKey`, `JWKS`, `Rotate`; `instance.Signer`; `instance.PublicKey`; job `instance.key_rotation`; handler of `getJwks`), M1-33 (`internal/kernel/jose`: `ParseCompactJWS`, `JWS.Verify`, `SignCompact`, `jose.Signer`; test helper `identitytest.SignJWS`), M1-34 (access tokens with `typ = at+jwt`, the adapter from `instance.Signer` to `jose.Signer`), M1-15 (`instance.State.Load`, `Record.InstanceID`), M1-12 (`pgtest`), M1-18 (`httpservertest`), M1-22 (`egress.NewLoopback`, Semgrep rule `bothy-go-egress-only`), M1-07 and M1-08 (configuration schema and reference), M1-09 (redacting secret type), M1-51 (agent principals). Already on the base branch: `internal/kernel/ids` (`ids.ID`, `ids.NewGenerator`), `internal/kernel/clock` (`clock.Clock`, `clock.NewFake`), `internal/kernel/errs` (`errs.CodeInvocationTokenInvalid`, `WithReason`), `internal/runs/README.md`; the module skeleton and runs of M2-05.
- Implied dependencies: the header names only M2-05. M1-32, M1-33, M1-34, M1-15, M1-12, M1-18, M1-22 and M1-51 are in its closure (M2-05, M2-04, M1-56 and so on down to M1-35, M1-34 and M1-32) and therefore merged before this brief starts.

## Fixed requirements that apply

Owner's fixed build rules (apply verbatim):

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

Rules for every brief:

6. Test first: write the failing test, run it and keep the red output for the report, then make it green, then clean up.
7. No en dash and no em dash in any file you write; the `text-rules` pre-commit hook rejects them. Use a hyphen or rephrase.
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory.
9. Limits, windows and thresholds only through named configuration keys listed in `docs/operations/configuration.md`; code comments and docs name the key, never the number.
10. Specification first: a change to behaviour, API, event, permission or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`) is not edited by this ticket.

Product requirements implemented here:

11. Header exactly `alg = EdDSA`, `kid` (key ID of the signing key) and `typ = agent-invocation+jwt`; explicit typing keeps every other JWS of the server (access tokens `at+jwt`, terminal session tokens `terminal-session+jwt`) from passing as an invocation token (05 section 5; 09 4.5 "Header"; RFC 8725 sections 3.1 and 3.11; 08 7.2; 24 5.3 "Terminal session token").
12. Claims exactly as the normative list of 05 section 5, with the origins of 09 4.5: `iss` = `instance.instance_id`; `aud` = principal ID of the agent; `sub` = principal ID of the triggering human; `jti` = new UUIDv7; `iat`, `exp` = `iat` + `agent.invocation_token_ttl`; `trigger_kind` (`message`, `followup`, `interaction`, `command`, `autocomplete`); `run_id`, absent for `trigger_kind = autocomplete` and for `command.invoked` when the owner of the control command has no active run in the thread (05 section 5, R-AZ-25; this issuer refuses that second case until M2-16 adds it, Design note 2); exactly one triggering object `message_id`, `interaction_id` or `command_id`; `thread_id`, `channel_id`, `space_id`; `level`; `billing_mode`; `tool_class`; `dev_environment_id` only in the dev environment milestone (V2-02, never set here); `permission_version`. A schema test holds the claims equal to 05 section 5 (ADR 0023 "Confirmation").
13. Signed with the `active` key of purpose `invocation` (`server_signing_keys`, EdDSA only by the check of 03) through `instance.Keys` (M1-32, ADR 0066); the JWS is assembled through `internal/kernel/jose` (ADR 0067). A new token on every call (R-SY-07); the token itself is never stored, cached, logged or put into the event log; only its `jti` goes back to the caller for `runs.invocation_jti` (R-SY-17, ADR 0052, 04 15.3 step 2).
14. Rotatable: during a scheduled rotation (`signing_key.invocation_rotation_interval`; a `next` key published at least `agent.jwks_max_age` before it becomes `active`; the old key `retiring` until `agent.invocation_token_ttl` plus `auth.clock_leeway` have passed) no token fails at an adapter or at the server inside its validity; after an emergency rotation (`mode = emergency`) the old key verifies nothing on the server (09 4.5 "Keys and rotation", 19 section 10, AG-22).
15. The adapter check of 09 4.5 "Check in the adapter" exists as a reference verifier; a token with another `alg` or another `typ` is rejected by it (PLAN row M2-06; 09 4.5 points 1 and 2).
16. Server-side verification for `startRun`, `resumeRun` and `renewInvocationToken` (used by M2-09): `alg`, `typ`, `kid` of purpose `invocation`, signature, `iss`, `aud`, run; expiry optional (09 4.5.1 step 1). Every failure is `403 invocation_token_invalid` with `reason` `invalid`, or `expired` when only `exp` has passed (06 5 "Errors"; 07 problem code table; 09 4.5 `reason` table).
17. A token never appears in logs, traces, metrics, audit details, events or error responses (SEC-LOG-2, SEC-SEC-3).

## Scope

All paths under `services/server/` unless noted.

Create:

- `internal/runs/domain/invocation.go`: the pure token rules (Design notes 1 to 3 and 5): types `TriggerKind`, `InvocationClaims`, `InvocationGrant`, `InvocationExpectation{Issuer, Audience, RunID ids.ID; IgnoreExpiry bool}`; constants for the header values; `NewInvocationClaims(g InvocationGrant, jti ids.ID, now time.Time, ttl time.Duration) (InvocationClaims, error)`; `(InvocationClaims) Payload() ([]byte, error)`; `ParseInvocationToken(token string) (ParsedInvocationToken, error)`; `CheckInvocationClaims(c InvocationClaims, e InvocationExpectation, now time.Time, leeway time.Duration) error`. Imports only `kernel-core` packages (02 section 5).
- `internal/runs/app/invocation.go`: the service `InvocationTokens` with `Issue` and `Verify` (Design notes 4 and 5), its config struct `InvocationTokenConfig{TTL, ClockLeeway time.Duration}` and the value type `InvocationToken` (Design note 8).
- `internal/runs/adapters/runstest/refverifier.go` (create the package `runstest` if M2-05 did not): the reference verifier `ReferenceVerifier` (Design note 6) with its own unit tests in `refverifier_test.go`.
- `test/integration/invocationtoken/` (component `testsupport`): the cross-module integration tests of this ticket (real `instance.Keys`, the real `getJwks` handler, the reference verifier, two server processes).
- A binary exclusion test in `internal/runs/adapters/runstest` that runs `go list -deps ./cmd/bothy` and fails if `runstest` appears (the pattern of M1-33 for `identitytest`).

Change:

- `internal/wire`: construct `app.InvocationTokens` once per process where the `runs` services of M2-05 are built, with `instance.Keys`, `instance.State`, the process clock, the ID generator and `InvocationTokenConfig` from `agent.invocation_token_ttl` and `auth.clock_leeway` (both keys are already in the schema; add the wiring only if missing). No HTTP route, no subscription, no job.
- `internal/runs/README.md` (English, like the existing file): section on invocation tokens (claims per trigger kind, issuing, verification order, error reasons, what is never stored or logged, the reference verifier and its use by later tickets and SDKs).
- `docs/adr/0023-signed-invocation-token-and-check-in-the-adapter.md`: name this ticket's tests in "Confirmation" (schema test, reference verifier, AG-22 token level); then `make adr-index`.
- `docs/security/threat-model.md`: add the reference verifier and server verifier tests of this ticket to the test column of T-TK-05 next to AZ-P6.
- `docs/DECISIONS.md` section 10: one row per ruling of Design note 11 (`D-M2-06-1` and following).
- `services/server/CHANGELOG.md` under "Unreleased".

This ticket adds no migration, enables no operation in `api/operations.yaml`, adds no configuration key, writes no event, no audit entry and no job, and records no metric: issuing a token is part of a delivery whose audit and metrics belong to M2-08, and the signal catalogue names no token metric.

## Out of scope

- Fresh token per delivery in the envelope (`invocation_token`), storing the `jti` in `runs.invocation_jti`, `billing_mode_expected`, `agent.invoke` with I2, SR2 and SR3 before delivery, the events `agent.invocation` and `agent.followup`: M2-08.
- `startRun`, `resumeRun`, `renewInvocationToken`, the `superseded` reason (current `jti`, consumed token, `runs.renewed_from_jti`), `409 already_started`, `billing_mode_mismatch` and `tool_class_mismatch` at the server, AG-08 (run level), AG-28, AG-29, AG-31, AG-32: M2-09.
- The token in API requests of an agent during a run (`acting_for`, SR9) and any port that exposes verification to other modules: M2-14.
- Tokens for `interaction.created`, `command.invoked` and `command.autocomplete_requested` deliveries: M2-15, M2-16 and M2-17 (they call `Issue`).
- The SDK verifiers (`invocation` module of 11 11.2) and AZ-P6: M2-29 and M2-30; reference adapters: M2-33 and M2-34.
- AG-22 at run level (scheduled and emergency half with real deliveries, `agent.ack_timeout` and `run.start`): M2-31, which runs AG-01 to AG-30 (M2 exit criterion 1).
- Signing keys, their states, the JWKS operation, the rotation job and `rotateSigningKey`: M1-32 (merged). Terminal session tokens (purpose `terminal`): M2-44. `dev_environment_id`: V2-02.
- Delivery to agents over webhooks (04 15.6, fresh token per delivery): not owned by this ticket, and the plan names no owner yet; list it as a follow-up in the report if it is still open.
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M2-06-invocation-token`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(runs): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL runs only as a throwaway container on a random port through `pgtest` (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d` (the key store directory of the tests with mode 0700), removed at the end.
- Never read or print secrets; test signing keys are generated by the tests through M1-32 and never leave the temporary directory. No network calls except package downloads for pinned tools and Go modules. No real agent runtime and no model call: nothing in this ticket starts one.
- No new third-party Go module: tokens go through `internal/kernel/jose` (ADR 0067), the reference verifier uses the standard library only.
- The reference verifier builds no HTTP client, transport or dialer (Semgrep rule `bothy-go-egress-only` covers `runstest`, which is not a `_test.go` file); callers pass the client.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100). Where the merged code of an M1 ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. Placement and layers (02 sections 4.4 and 5). Domain `internal/runs/domain/invocation.go` holds the claim rules, encoding, parsing and claim checks and imports only `kernel-core` (`kernel/jose`, `kernel/ids`, `kernel/errs`). The app service `internal/runs/app/invocation.go` uses the port of `instance` (allowed edge `runs -> instance`, reason "sign the token") and no adapter. The level, billing mode and tool class are string types local to the runs domain with exactly the values of the openapi enums `Level`, `BillingMode` and `ToolClass`; the domain imports no `agents` type. Ruling for docs/DECISIONS.md: the service stays inside the module (no type in the root package `internal/runs`) because every consumer of this milestone up to M2-17 is inside `runs`; a ticket with a consumer in another module (M2-14) adds the port with its own DTOs (02 section 5 rule 1).

2. Claims per trigger kind. The issuer accepts an `InvocationGrant{AgentID, TriggeredBy ids.ID; TriggerKind; RunID, MessageID, InteractionID, CommandID ids.ID; ThreadID, ChannelID, SpaceID ids.ID; Level, BillingMode, ToolClass string; PermissionVersion int64}` and builds the claims after this table (05 section 5: `run_id` absent for `trigger_kind = autocomplete` and for `command.invoked` without an active run of the owner, R-AZ-25; "`message_id` or `interaction_id` or `command_id`"; 09 4.4; 03 `runs.triggering_command_id` refers to `commands(id)`):

    | `trigger_kind` | `run_id` | triggering object (exactly one) | source of the object |
    |---|---|---|---|
    | `message` | required | `message_id` | triggering message |
    | `followup` | required | `message_id` | the follow-up message (`AgentFollowupPayload.message_id`) |
    | `interaction` | required | `interaction_id` | the interaction |
    | `command` | required | `command_id` | the command (`commands.id`) |
    | `autocomplete` | absent | `command_id` | the command of the request (`CommandAutocompleteRequestedPayload.command_id`) |

    `iss`, `aud`, `sub`, `jti`, `iat`, `exp`, `trigger_kind`, `thread_id`, `channel_id`, `space_id`, `level`, `billing_mode`, `tool_class` and `permission_version` are always present; `dev_environment_id` is never written here (V2-02). A grant that breaks the table (missing or extra run or object, unknown enum value, zero ID, negative `permission_version`) is a programming error of the caller: `Issue` returns an internal error and signs nothing (fail closed). Ruling for docs/DECISIONS.md: the object column for `followup` and `autocomplete`. A control command (09 10.4, `commands.control = true`: `command.invoked` with a token, "No run") gets a token without `run_id` when its owner has no active run in the thread (05 section 5, R-AZ-25); M2-16 adds that grant (field `Control`), and until then this issuer refuses a `command` grant without `RunID` (fail closed), so the `command` row above stays as it is. An autocomplete request outside a thread never needs a token without `thread_id` (05 section 5 row `thread_id`, R-AZ-26, D-W-104): in a text channel the main thread is the place, and in a forum channel without a thread the server asks no agent and issues no token. So the issuer keeps refusing a zero `thread_id` for every trigger kind.

3. Encoding. Ruling for docs/DECISIONS.md:
    - Header JSON exactly `{"alg":"EdDSA","kid":<KeyID() of the signer>,"typ":"agent-invocation+jwt"}`, no other member. If the signer reports another algorithm than `EdDSA`, `Issue` fails with an internal error (03 allows only `EdDSA` for `invocation`).
    - Payload: one JSON object, members in the order of the table of 05 section 5; IDs as canonical lower-case UUID strings; `iat` and `exp` as NumericDate integers in seconds (RFC 7519 section 2), `iat` = issuing time truncated to whole seconds, `exp` = `iat` + `agent.invocation_token_ttl` truncated to whole seconds; `aud` a single string, never an array; `permission_version` a JSON integer; absent claims are omitted (`omitzero`), never `null` (ADR 0053).
    - The compact form comes from `jose.SignCompact` with an adapter from `instance.Signer` (context-bound) to `jose.Signer`; reuse the adapter M1-34 built for access tokens if it is exported, otherwise add a small unexported one in the app package.

4. `Issue(ctx, g InvocationGrant) (IssuedInvocationToken, error)` with `IssuedInvocationToken{Token InvocationToken; JTI ids.ID; KeyID string; IssuedAt, ExpiresAt time.Time}`:
    a. Build and validate the claims (note 2) with `jti` from the ID generator (UUIDv7, R-DM-01) and `now` from the clock.
    b. `iss` is `Record.InstanceID` from `instance.State.Load`; load it once per process and keep it (M1-15: `instance_id` never changes); a failed load is returned and retried on the next call.
    c. `Keys.Signer(ctx, <purpose invocation>)` returns the `active` key, read fresh on each call (M1-32), so a promotion in another process takes effect on the next token.
    d. Sign and return. `ExpiresAt` feeds `InvocationTokenGrant.expires_at` in M2-09.
    `Issue` starts no transaction and takes no lock; it may run inside the caller's unit of work (M2-08). Its only database access is the signer's read of the active key row (and the first `Load`). Errors of the key store pass through with the kind M1-32 gives them (`503 unavailable` when the key store is unusable).

5. `Verify(ctx, token string, v VerifyInvocation) (domain.InvocationClaims, error)` with `VerifyInvocation{Audience ids.ID; RunID ids.ID; IgnoreExpiry bool}`, in this order; every failure in steps a to f gives `reason` `invalid`:
    a. Parse through `ParseInvocationToken`: `jose.ParseCompactJWS(token, []string{"EdDSA"}, "agent-invocation+jwt")` plus a byte-exact comparison of `typ` (09 4.5 point 2 says "exactly"; no case folding, no `application/` prefix) and a non-empty `kid`. The profile of ADR 0067 already rejects `none`, `HS*`, `crit`, `jku`, `x5u`, `x5c` and `zip`; a header `jwk` is never used.
    b. Key: `Keys.VerificationKey(ctx, <purpose invocation>, kid)`. Not found covers an unknown `kid`, a `retired` key and a key of another purpose, so a JWS signed with an `access_token` or `terminal` key never verifies here even if its header claimed the invocation type (if the merged `VerificationKey` does not filter by purpose, compare `PublicKey.Purpose` in `Verify`). `next` keys are accepted because the cached state of one process may lag a promotion in another by up to `signing_key.cache_ttl`; no `next` key ever signs.
    c. Signature through `kernel/jose` with that key.
    d. Payload: decode into `InvocationClaims`; members required by note 2 present and of the right JSON type; enums known. Ruling for docs/DECISIONS.md: unknown members are ignored (RFC 7519 section 4), so a token of a newer server version stays verifiable during an upgrade; the issuer still writes exactly the set of 05.
    e. `iss` equals the own `instance_id`; `aud` equals `v.Audience`; if `v.RunID` is set, `run_id` equals it (09 4.5 `reason` table: "run wrong").
    f. `iat` not later than now plus `auth.clock_leeway`; `exp` after `iat`.
    g. Unless `v.IgnoreExpiry`: now at or after `exp` plus `auth.clock_leeway` gives `reason` `expired` (RFC 7519 section 4.1.4: the current time must be before the expiration time). Ruling for docs/DECISIONS.md: the server applies `auth.clock_leeway` to its own invocation tokens as 08 defines it for `exp` and `iat`; the retiring window of 09 4.5 "Keys and rotation" point 2 already contains this margin, so a token accepted here always has a published key.
    So `expired` comes only when everything else holds (09 4.5: "`exp` passed, otherwise valid and current"). Whether the `jti` is current or consumed (`superseded`) is M2-09's step after `Verify`. `IgnoreExpiry` serves `renewInvocationToken` (09 4.5.1 step 1) and later SR9 (07: "signature, `aud`, run active").
    Errors: `errs.New(errs.CodeInvocationTokenInvalid, <detail>).WithReason("invalid" | "expired")`, kind forbidden (`403`). The detail names the failed step and at most the `kid`, never the token or a segment of it.

6. Reference verifier (PLAN "reference verifier"). Ruling for docs/DECISIONS.md: it is test-support code in `internal/runs/adapters/runstest` that implements the adapter check of 09 4.5 independently of the server path, with the Go standard library only (`encoding/base64` with `RawURLEncoding` in strict mode, `encoding/json`, `crypto/ed25519`), not with `internal/kernel/jose`, so that a defect shared by issuer and server verifier cannot hide (the same idea as G10's "independent verification" for webhooks, 06 15). ADR 0067 governs the JWS paths of the server; this code stands in for the external adapter and never enters the binary.
    - API: `NewReferenceVerifier(ReferenceOptions{JWKSURL string; Client *http.Client; Clock clock.Clock; Issuer, AgentID ids.ID; Leeway time.Duration /* adapter.invocation_token_leeway */; RefreshMinInterval time.Duration /* adapter.jwks_refresh_min_interval */}) (*ReferenceVerifier, error)`; `Check(ctx, token string, e ReferenceExpectation) (ReferenceClaims, error)` with `ReferenceExpectation{RunID, ThreadID, MessageID, InteractionID, CommandID ids.ID; ActualBillingMode, ActualToolClass string}` (zero values are not compared); counters `Fetches()` and `ForcedReloads()` for the tests. Every option is required; the verifier has no default value for a configuration key.
    - JWKS: fetched with `GET` from `JWKSURL` through the given client; cached for the `max-age` of the response's `Cache-Control` (absent: refetch on the next use); on a `kid` missing from the cache it reloads once, and at most once per `RefreshMinInterval` (09 4.5 point 3). Only keys with `kty = OKP`, `crv = Ed25519` and a 32-byte `x` are usable; `alg` and `use`, when present, must be `EdDSA` and `sig`.
    - Checks in 09 order, with this precedence: points 1 and 2 (`alg` exactly `EdDSA`, `typ` exactly `agent-invocation+jwt`, no `crit`), 3 (`kid` known, after at most one reload, else `ErrRefUnknownKey`: the adapter neither accepts nor acknowledges), 4 (signature, `iss`, `aud`), 6 (`run_id`, `thread_id` and the triggering object equal the expectation), each failure `ErrRefInvalid`; then 8 (`billing_mode` equals the actual mode; an actual mode `unknown` never matches, 10 section 7) as `ErrRefBillingModeMismatch` and 10 (actual tool class not above `tool_class` in the order `chat` < `scoped` < `host`) as `ErrRefToolClassMismatch`; then 5 (expired when now is at or after `exp` plus `Leeway`, alone `ErrRefExpired`; `iat` later than now plus `Leeway` is `ErrRefInvalid`); then 7 (`jti` seen before and not yet expired: `ErrRefReplay`, kept until `exp` plus `Leeway`). Point 9 (allow list of triggerers via `agent.triggerers.read`, 17.7) is not part of it: it belongs to adapters that keep such a list and to the SDK (M2-29).
    - It is the oracle for later tickets: M2-08 and M2-09 check delivered tokens with it in their Go tests; the SDK verifiers of M2-29 and M2-30 must agree with it on the cases of this ticket's tests.

7. Rotation (AG-22). This ticket relies on M1-32 for the key states, the publication of a `next` key at least `agent.jwks_max_age` before use, the `retiring` window (`agent.invocation_token_ttl` plus `auth.clock_leeway`), the per-process cache with `signing_key.cache_ttl` and the emergency transition; it adds nothing to them and proves at token level that they suffice: an adapter whose JWKS cache honours `Cache-Control: max-age` (= `agent.jwks_max_age`) never meets an unknown `kid` during a scheduled rotation, and a second server process verifies every new token at once because it already knows the key as `next`. Ruling for docs/DECISIONS.md: this ticket proves AG-22 at token level (scheduled: no token is rejected inside its validity by any adapter or server process; emergency: the old key is rejected at once on the server and a new token verifies after at most one reload); the run-level scenario of both halves is M2-31's. Two gaps of the spec that can still fail runs are recorded in the report and not built around: the renewal of an expired token after its key went `retired` (09 4.5.1 step 1 has no time bound, the retiring window ends after `agent.invocation_token_ttl` plus `auth.clock_leeway`, `run.queued_max_age` is longer), and a token of the old key that an adapter with a still-valid JWKS cache accepted and acknowledged before an emergency rotation.

8. Secret handling (SEC-LOG-2, SEC-SEC-3). `InvocationToken` wraps the compact string; `String`, `GoString`, `Format`, `LogValue`, `MarshalText` and `MarshalJSON` render `[redacted]` (the behaviour of the redacting secret type of M1-09); only `Reveal() string` returns the value, and M2-08 calls it when it fills the envelope. Logs and errors of this ticket may name `kid` and `jti`, never the token, its signature or its payload. `Verify` receives the token as a plain string from a request body (M2-09) and copies it nowhere.

9. Configuration: no new key. `agent.invocation_token_ttl` and `auth.clock_leeway` come through `InvocationTokenConfig` from `internal/wire`. The adapter keys `adapter.invocation_token_leeway` and `adapter.jwks_refresh_min_interval` are options of the reference verifier, set by the tests; the server never reads them. 09 4.5 "Keys and rotation" point 2 requires `adapter.invocation_token_leeway` not above `auth.clock_leeway`; the AG-22 test sets it equal to `auth.clock_leeway` to test the edge.

10. Concurrency and cost. `Issue` and `Verify` are safe for concurrent use; the cached `instance_id` is guarded by a `sync.Once`-like pattern that allows a retry after an error. Issuing costs one signature and one indexed read; this matters for `perf.invocation_delivery_p99`, which M2-36 measures.

11. Rulings for `docs/DECISIONS.md` (one row each with reason): note 1 (service inside the module, no root port yet); note 2 (triggering object for `followup` and `autocomplete`; incomplete grants fail closed); note 3 (header with exactly three members, NumericDate seconds with truncation, `aud` as a string, absent claims omitted); note 5 (purpose-bound key lookup with `next` accepted, unknown claims ignored, byte-exact `typ`, `auth.clock_leeway` on the server, every `invalid` cause before `expired`); note 6 (independent reference verifier with the standard library, precedence of its results); note 7 (AG-22 split between this ticket and M2-31); note 8 (redacting token type).

## Tests (write them first)

Unit (`internal/runs/domain`):

- `TestInvocationClaims_MatchSpec` (the schema test of ADR 0023): reads `docs/spec/05-actions.md` section 5 and `docs/spec/09-agents.md` 4.5 (locate the repository root by walking up to the directory that contains `docs/spec`), extracts the backticked names of the first column of both claim tables and asserts that both sets are equal and equal the JSON member names of `InvocationClaims`; asserts that the header constants equal `EdDSA` and `agent-invocation+jwt` as both chapters state.
- `TestInvocationClaims_ShapePerTriggerKind`: for each of the five kinds, `Payload()` decoded with `UseNumber` has exactly the members of note 2; every ID is a canonical lower-case UUID string; `iat` and `exp` are integers with `exp - iat` equal to the TTL in seconds; `level`, `billing_mode` and `tool_class` take only the enum values read from `docs/api/openapi.yaml` (`Level`, `BillingMode`, `ToolClass`); no member is `null`; `dev_environment_id` is absent.
- `TestNewInvocationClaims_PresenceRules`: table with a missing `run_id` for each kind except `autocomplete`, a `run_id` with `autocomplete`, no triggering object, two triggering objects, the wrong object for the kind, zero `thread_id`, `channel_id` or `space_id`, an unknown enum value, a negative `permission_version`: each returns an error.
- `TestParseInvocationToken_HeaderRules`: `alg` other than `EdDSA` (`none`, `HS256`, `ES256`, `RS256`, `Ed25519`, `eddsa`, missing) and `typ` other than `agent-invocation+jwt` (missing, `JWT`, `at+jwt`, `terminal-session+jwt`, `application/agent-invocation+jwt`, `Agent-Invocation+JWT`, with a trailing space), empty `kid`, `crit` present: each rejected.
- `TestCheckInvocationClaims_Order`: a token that is expired and has a foreign `aud` gives `invalid`, not `expired`; `iat` exactly at now plus leeway passes and one second later fails; `exp` plus leeway one second after now passes and exactly now gives `expired`, so a key that went `retired` after its window never had a valid token left.
- `FuzzParseInvocationToken`: seed corpus of a valid token and every case above; the seeds run in every `go test`; the parser never panics and never returns a token whose header breaks note 5a.

Unit (`internal/runs/app`, fake `instance.Keys` and `instance.State`, fake clock):

- `TestIssue_SignsWithInvocationKey`: the fake records that only the purpose `invocation` is requested; the header `kid` equals the signer's `KeyID()`; `iss` equals the instance ID; two calls give two different UUIDv7 `jti`; `ExpiresAt - IssuedAt` equals `agent.invocation_token_ttl` truncated to seconds; `instance.State.Load` runs once for many calls.
- `TestIssue_RefusesIncompleteGrant`: a grant of `TestNewInvocationClaims_PresenceRules` gives an error and the fake signer was never called.
- `TestIssue_SignerAlgMustBeEdDSA`: a signer reporting `ES256` gives an internal error.
- `TestVerify_Reasons`: bad signature, unknown `kid`, foreign `aud`, foreign `iss`, wrong `run_id` give `errs.CodeInvocationTokenInvalid` with `reason` `invalid` and kind forbidden; an expired, otherwise valid token gives `reason` `expired`; with `IgnoreExpiry` the same token returns its claims; with `IgnoreExpiry` a bad signature is still `invalid`.
- `TestVerify_UnknownClaimIgnored`: a token with an extra member verifies and returns the known claims.
- `TestInvocationToken_Redacted`: `fmt` with `%v`, `%+v`, `%#v`, `slog` with a JSON handler and `json.Marshal` of a struct holding the token show `[redacted]`; `Reveal()` returns the compact string; errors of `Issue` and `Verify` and every captured log record contain neither the token nor its signature segment.

Unit (`internal/runs/adapters/runstest`, a JWKS served by `httptest` on `127.0.0.1` port 0, keys generated in the test):

- `TestReferenceVerifier_AcceptsValidToken`: a token built by the server path of this ticket passes `Check` with all expectations set.
- `TestReferenceVerifier_RejectsAlgAndTyp` (plan acceptance test): every `alg` and `typ` case of `TestParseInvocationToken_HeaderRules`, each with a valid Ed25519 signature over its own header where the algorithm allows it, plus `HS256` keyed with the bytes of the published public key (algorithm confusion) and `none` with an empty signature: each gives `ErrRefInvalid`, never `ErrRefExpired` or success, and leaves the `jti` store unchanged.
- `TestReferenceVerifier_Points`: foreign `iss`, foreign `aud`, changed payload byte, `run_id`, `thread_id` or triggering object different from the expectation give `ErrRefInvalid`; actual billing mode different or `unknown` gives `ErrRefBillingModeMismatch`; actual tool class `host` with claim `scoped` gives `ErrRefToolClassMismatch`, actual `chat` with claim `scoped` passes; an expired token with a billing mismatch gives the mismatch, not `ErrRefExpired`; the same valid token twice gives `ErrRefReplay` the second time.
- `TestReferenceVerifier_JWKSCache`: no fetch before `max-age` has passed; one fetch after; an unknown `kid` causes exactly one forced reload and `ErrRefUnknownKey` if the key is still missing; a second unknown `kid` within `RefreshMinInterval` causes no fetch.
- `TestRunstest_NotInBinary`: `go list -deps ./cmd/bothy` does not list `runstest`.

Integration (`test/integration/invocationtoken`; PostgreSQL 18 through `pgtest`; real `instance.Keys` of M1-32 with a key store in a `mktemp -d` directory of mode 0700, built as M1-32's tests build it; controllable clock; the real `getJwks` handler of M1-32 served on `127.0.0.1` port 0 through `httpservertest` of M1-18 or the way M1-32's `TestGetJwks_HTTP` serves it; the reference verifier gets the loopback client of `egress.NewLoopback` (M1-22) or the test server's client; an agent principal of M1-51, the people Alex and Sam, a queued run of M2-05 in a thread of the space Support):

- `TestInvocationToken_RoundTrip`: `Issue` then `Verify` in the same process and `Check` by the reference verifier succeed; the header `kid` is the `kid` of the `active` row of purpose `invocation` in `server_signing_keys`.
- `TestInvocationToken_ForeignPurposeKey`: a JWS with the invocation header and valid claims signed through `instance.Keys.Signer` with the purpose `access_token` (published in the same JWKS) is `invalid` at the server; a JWS with the access-token header (`typ = at+jwt`) signed with the `access_token` key (built with `identitytest.SignJWS` or the issuer of M1-34) is rejected by `Verify` and by the reference verifier.
- AG-22 (scheduled half, 09 16.2: "no failed runs when scheduled"), `TestAG22_ScheduledRotationFailsNoToken`: two server processes A and B (own `instance.Keys` and `InvocationTokens` each, same database; A runs the function of the job `instance.key_rotation` at every step, not the River runtime); test values for `agent.invocation_token_ttl`, `auth.clock_leeway`, `agent.jwks_max_age`, `signing_key.invocation_rotation_interval`, `signing_key.cache_ttl` and `signing_key.rotation_check_interval` set through the configuration with `signing_key.cache_ttl` and `agent.invocation_token_ttl` plus `auth.clock_leeway` below `agent.jwks_max_age`, which is below `signing_key.invocation_rotation_interval`; three reference verifiers with `Leeway` equal to `auth.clock_leeway`: X fetched the JWKS just before the `next` key was created, Y just after it was published, Z fetches for the first time at the first token after the promotion. Step the clock by `signing_key.rotation_check_interval` through two complete rotations of `invocation`. At every step A issues a token; X, Y and Z check it at issue time and one second before `exp` plus `Leeway`; B's `Verify` checks it at issue time and one second before `exp` plus `auth.clock_leeway`. At the last step before each promotion, the token signed with the old key is also checked at its last valid moment by a verifier that fetched the JWKS at that moment. Asserts: no `invalid`, no unknown `kid`, no `expired` inside validity anywhere; every token carries the `kid` that was `active` at its issue time; `ForcedReloads()` stays zero for X, Y and Z (every reload comes from `max-age`); B never misses a `kid`.
- AG-22 (emergency half at token level), `TestAG22_EmergencyRotationTokenLevel`: after `instance.Keys.Rotate` with `mode = emergency` by an owner actor with fresh step-up (the builders M1-32's `TestEmergencyRotation_*` use), a token signed before is `invalid` in A at once and in B at the latest after `signing_key.cache_ttl`; a reference verifier whose JWKS cache was fetched after the rotation returns `ErrRefUnknownKey` for it after exactly one forced reload and fetches nothing more within `adapter.jwks_refresh_min_interval`; a token issued after the rotation carries the new `kid` and passes A, B and the reference verifier, the latter after at most one forced reload.
- `TestInvocationToken_LogCanary`: issue and verify with the log capture of `httpservertest` (M1-18); the token string, its signature segment and its payload segment appear in no captured log record and no returned error.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/runs/... ./internal/wire/... ./test/integration/invocationtoken/...`
- `make arch`, `make semgrep`, `make docs`
- `make check`

## Acceptance criteria

- [ ] Schema test of the claims: the claim set of the code equals the tables of 05 section 5 and 09 4.5; every issued token has the header `alg = EdDSA`, `kid`, `typ = agent-invocation+jwt` and exactly the claims of its trigger kind, without `null`.
- [ ] Tokens are signed with the `active` key of purpose `invocation`; a new `jti` on every call; nothing is stored; the token appears in no log or error.
- [ ] AG-22 (scheduled rotation without failed runs, at token level): through two complete rotations no token is rejected inside its validity by any of three adapters or by a second server process, and no adapter needs a forced JWKS reload.
- [ ] AG-22 emergency half at token level: the old key is rejected on the server at once in the same process and within `signing_key.cache_ttl` in another; new tokens verify after at most one reload.
- [ ] A token with another `alg` or `typ` is rejected by the reference verifier (and by the server verifier), including `none`, `HS256` with the public key as secret, `at+jwt` and `terminal-session+jwt`.
- [ ] `Verify` returns `403 invocation_token_invalid` with `reason` `invalid` for every non-expiry failure and `expired` only when nothing else fails; a key of another purpose never verifies.
- [ ] `runstest` is not part of the binary; the reference verifier uses only the standard library and builds no HTTP client.
- [ ] README, ADR 0023 "Confirmation", threat model row T-TK-05, `docs/DECISIONS.md` and CHANGELOG updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- `internal/runs/README.md` updated (token section of Scope, the reference verifier and who uses it later).
- Rulings of note 11 recorded in `docs/DECISIONS.md` section 10 (one row each with reason); ADR 0023 "Confirmation" names this ticket's tests and `make adr-index` is run.
- `services/server/CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`) with attention to the M2 red team item "forging or reusing the invocation token": algorithm and type confusion, `kid` of another purpose, header keys, tokens in logs and errors; on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven (for example AG-22 with real deliveries and run starts, which M2-31 proves).
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found; at least the cases of Design note 7 (renewal after a key went `retired`, emergency rotation while an adapter still caches the old key), each with a concrete proposal for the owner.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
