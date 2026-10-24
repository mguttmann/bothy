# Module `terminal`

Status: host records (M2-39), terminal rights (M2-40), pairing (M2-41), control connection of the host services (M2-43), terminal sessions, session token and relay (M2-44), the terminal across several api nodes (M2-47), revocation of every kind, the server's own limits of a session, session limits, rate limits of the terminal channels and the session metrics (M2-46). The normative source is [docs/spec/24-terminal.md](../../../../docs/spec/24-terminal.md); where they differ, the chapter applies.

## Purpose

Hosts and pairing, declaration of the LLM login (I6), session users per person (ruling T12), terminal sessions, terminal session tokens, control connections of the host services, relay of the streams.

## Code

- `internal/terminal` (`hosts.go`): port `Hosts` with `List`, `Get`, `Create`, `Update`, `SetState` and the DTOs (`HostView` field by field like the schema `Host`, `CreateHost`, `UpdateHost`, `SetHostState`, `PairingCodeIssued` with the pairing code as a redacting `identity.Token`); `Register` registers `host.updated`.
- `domain/`: `Transition` is the one place that knows the life cycle of 24, section 3.1 (M2-41 and M2-43 call it for their transitions); `CloseReasons` and `CloseDetails` are the closed lists of 24, section 10.3; `NewHostName`, `NewOSUser` and `CheckReason` check the texts (SEC-IN-4); `PlanAssignments` plans assignment and end of session users after T12 (exactly a reported name, uniqueness per host without case over all rows, at most one valid assignment per person, every `400` before any `409`); `reported.go` reads the reported columns.
- `app/`: the five use cases (`Service`), `EndSessions` (the one way of every end by the server), `HostSource` (implementation of `authz.HostSource`), the view `view.go`, the outgoing ports `Store`, `PairingCodes`, `People`, `Presence` and `Cursors`; `NoConnections` is the presence without control connections.
- `adapters/postgres/`: the sqlc queries in `queries/`, the generated code in `sqlcgen/` and `Repository`.
- `adapters/httpapi/`: the handlers of `listHosts`, `createHost`, `getHost`, `updateHost`, `setHostState`, `setHostLlmCredentials` (M2-40), `issueHostPairingCode`, `pairHost` and `confirmHost` (M2-41).
- Test code under `services/server/test/`: `terminaltest` creates hosts in every state with reported profiles, session users and check results as well as assignments and sessions directly in SQL (in production M2-41, M2-43 and M2-44 write these); `terminalworld` is the HTTP test world of M2-39.

## Port (root package `internal/terminal`)

- Inbound: `Hosts` (M2-39; `SetLlmCredentials` since M2-40) and `Sessions` (M2-44: `Open`, `Attach`, `List`, `Get`, `Close`, implemented by `app.Sessions`). The relay is no port for other modules; it lives in `adapters/wsrelay` (02 4.3).
- Emits in-tx events: none.
- Subscribes to in-tx events: kernel `authorize.AccessChanged`, `identity.DeviceRevoked`, `identity.SessionEnded` (M2-46: they end sessions in the transaction of the revocation; `app.Subscribers`, see "Built in M2-46").
- Implements source ports and cross-cutting ports: `authz.HostSource` (state, assignee, LLM declaration, server host of the host), bound in `internal/wire` for the authorizer, the rights hook and grants.

## Dependencies

May use the ports of these modules: `instance`, `identity`, `authz`, `agents`.

Reason: signer for terminal session tokens (purpose `terminal`); people, devices, owners, consumption of the step-up, pairing codes (`identity.Tokens.NewHostPairingCode`); grants on hosts and I6; readiness of the dev environment (milestone).

Additionally for all modules: `internal/kernel`; `internal/platform` and `api/` only in `adapters/`.
Checked by go-arch-lint (ADR 0008).

## Tables

`hosts`, `host_session_users`, `terminal_sessions`.

Only this module writes and reads these tables via SQL (03-data-model.md, principle 12).

## Retention (M2-46)

| Table | Retention class | Deleted by |
|---|---|---|
| `terminal_sessions` | `security` (`retention.security_after_end` after `closed_at`) | the job `retention.security` of M7-05 (DP-T05), not this module |

`terminal_sessions` holds metadata only: times, state, close reason and detail, exit code, byte counts and IDs; never terminal content, commands, environment variables, paths or profile command lines. There is no content recording (OQ-04 default, D-M2-46-15). The retention of `hosts` and `host_session_users` follows their rows in [20-privacy.md](../../../../docs/spec/20-privacy.md).

## Actions (05-actions.md)

`host.read`, `host.manage`, `host.pair`, `host.connect`, `terminal.open`, `terminal.attach`, `terminal.stream`, `terminal.read_own`, `terminal.close`, `terminal.terminate`.

## Events and WebSocket ops

- Durable: `host.updated` (M2-39), `terminal_session.changed` (M2-44).
- Ephemeral: `host.presence_changed` (M2-43).
- WebSocket ops (client to server): none on the shared WebSocket; separate connections with their own ops (05, section 7).

## Background jobs

| Job | Trigger | Queue |
|---|---|---|
| `terminal.sweep` | periodic | `maintenance` |

## Built in M2-39

- Reading a host for owners, the assignee and people with an effective grant terminal; administration
  only by owners with step-up, mandatory reason, audit and rights change. Agents, apps and
  personal tokens get `403 forbidden` with `subject_kind_not_allowed`.
- The event of a host goes to everyone who may read it on every change, never with a key or the
  pairing code.
- `EndSessions` writes the end of every session as a system action into the audit. The remaining
  actions stay deferred until M2-40 to M2-44.

## Built in M2-40

- `SetLlmCredentials` (`setHostLlmCredentials`, `host.manage`): declaration `none`, `personal` or
  `organization` with `credential_holder_id` exactly with `personal` (`domain.ValidateDeclaration`:
  `required`, `not_allowed`; an unknown or deleted person `unknown_user`), mandatory reason,
  `If-Match`, `409 conflict` on a `retired` host, without effect for the same declaration
  (`no_change`). The rights change locks the host after `permission_version`, writes the
  declaration through `ApplyLlmDeclaration` (also for `confirmHost` of M2-41) and, on a switch to
  `personal` or of the holder of the login, ends the sessions of everyone else with `i6_violation`
  (`domain.EndsForeignSessions`, `SessionSelector.ExceptUserID`); `authz` checks I6 in phase 3.1, a
  violation rolls everything back (`409 i6_violation`).
- `RecordSessionUserReport` and `ApplySessionUserReport` (for `pairHost` of M2-41 and `report` of
  M2-43): the reported session users are rights state. An equal report (`domain.SameReport`)
  changes nothing; a different one is written with a new version of the host, ends the sessions of
  the people whose assigned name is missing (`domain.VanishedNames`, `access_revoked`) and writes
  `host.updated`.
- `Update` ends the sessions of the previous assignee with `access_revoked` when the assignee
  changes or is cleared, unless they are an active owner (`People.IsActiveOwner`, SR10).
- `HostSource.PersonalHosts` and `SessionUsersOf` (exact comparison with `domain.Reports`) and
  `HostFacts.Name` for the text of the preview.
- Tests: `domain` (`TestValidateDeclaration`, `TestEndsForeignSessions`, `TestSameReport`,
  `TestVanishedNames`) and `app/terminalauthz_test.go` over PostgreSQL 18 and the
  HTTP chain: `TestTMT09_I6`, `TestI6_UserEnableRevivesGrant`, `TestI6_SwitchEndsForeignSessions`,
  `TestI6_HolderChangeEndsForeignSessions`, `TestSR10_AssigneeChangeEndsSessions`,
  `TestSetHostLlmCredentials_HTTP`, `TestSR12_ReportDropsName`, `TestPreviewHostGrant`,
  `TestTerminalAuthzAudit_NoContent`, `TestAZP1_TerminalAuthzRoutes`.

## Built in M2-41

- Pairing: `IssuePairingCode` (`issueHostPairingCode`, re-pairing), `Pair` (`pairHost`, public,
  the code is the proof) and `Confirm` (`confirmHost`) in `app/pairing.go`. One shared issuer
  (`issueCode` through `PairingCodes.Issue`) serves `createHost`, `issueHostPairingCode` and the
  fallback; `PairingCodes.Hash` (bound to `identity.Tokens.HashHostPairingCode`) checks form, kind
  and checksum without database access.
- `domain/fingerprint.go` (`ComputeFingerprint`, `CompactFingerprint`, Crockford base32, vectors
  in `domain/testdata` and `services/host/testdata/noise/fingerprint.json`) and
  `domain/pairing.go` (`ParseHostKeys`, `ValidatePairReport`, `NormalizeFingerprintPrefix`,
  `PrefixMatches`, `NextAttempt`, `RepairEffects`) with the fuzz targets `FuzzParseHostKeys` and
  `FuzzNormalizeFingerprintPrefix`.
- Confirmation: the prefix first; a failed attempt counts without a new version and stays also
  under an `Idempotency-Key` (`idempotency.KeepEffectOnFinal`); after
  `terminal.fingerprint_max_attempts` the host falls back to `pairing` without keys. Then
  assignments (`PlanAssignmentsAt`), declaration (`ApplyLlmDeclaration`, I6) and
  `declared_server_host` in one transaction.
- Re-pairing: `RepairEffects` per state, end of the sessions with `host_rekeyed`, `key_changed`
  exactly when stored keys are discarded; the fingerprint of discarded keys stays stored and
  refuses the same keys with `host_key_reused`.
- Tests: `domain` (fingerprint, rules), `app` (`TestTMT20_WrongChecksumNoDatabase`) and
  `adapters/httpapi/pairing_test.go` (TM-T19, TM-T20, TM-T28 (b) and (e), re-pairing,
  concurrency, no secrets).

## Built in M2-43

- Control connection `GET /api/v1/hosts/connect` (asyncapi `hostControl`, 24 section 3.5) in
  `adapters/wsrelay`: `Handler` (upgrade with `github.com/coder/websocket`, reader, bounded writer
  queue of `sync.outbound_queue_max` with `gateway.write_timeout`, the deadlines
  `terminal.host_auth_timeout` and `terminal.host_heartbeat_interval` times
  (`terminal.host_heartbeat_miss_limit` + 1)), the strict codec on the generated `Host*` types of
  `api/rest` (`codec.go`, `FuzzDecodeControlFrame`) and `Links`, the per-process registry of
  links. `Links` implements `app.ControlLinks` (`OpenSession`, `CloseSession`, `CloseHost`,
  `Connected`, `SendReady`) and publishes the ephemeral `host.presence_changed {host_id,
  connected}` through `kernel/eventlog.EphemeralPublisher` (the gateway hub, bound in
  `internal/wire`). "Connected" means usable for sessions: from the first applied `report` after
  `ready` until the connection ends; `HostView.Connected` reads it once `Service.BindControl` ran.
- `domain/control.go`: `CheckHostProof` (compact JWS, `alg` EdDSA, `typ` `host-auth+jwt`, `kid` the
  RFC 7638 thumbprint, the closed claims `challenge`, `host_id`, `iat` within `auth.clock_leeway`;
  `FuzzCheckHostProof`), `IdentifyOutcome` (the order of the close codes), `ReconcileLiveSessions`,
  `ValidateReport`, `DiffReport`, `SessionsEndedByReport`, `ValidateHostSessionEnd`,
  `HeartbeatDeadline`, `SupportedProtocolVersions`. `Transition` draws `confirming --> pairing` for
  a second live control connection of an unconfirmed host.
- `app/control.go`: `Authenticate` (proof and `authorize` of `host.connect` with the right
  `host-self`; denials are audited by authz), `LoginWrite` (`live_sessions`: every other session in
  `active` or `detached` ends with `host_restarted`; `reported_host_version`, `last_seen_at`),
  `Ready`, `Recheck` (every op authorizes again), `Heartbeat`, `ApplyReport` (the reported
  columns in one unit; a changed list of session users is the rights change `host.connect` through
  `ApplySessionUserReport`, SR12; the server-host flag turning on ends the sessions of non-owners,
  SR10; one allow entry `host.connect` on a change of the flag, the checks or the session users),
  `SessionStarted`, `SessionEnded`, `ConnectionLost`, `StaleReplaced`, `ResolveDuplicate` (24 3.4:
  disable an active host, an unconfirmed one falls back to `pairing`; deny entry
  `host_duplicate_connection`). `setHostState`, `issueHostPairingCode`, the fallback of
  `confirmHost` close the connection after their commit (`4111`, `4110`); `confirmHost` and
  `setHostLlmCredentials` send `ready` again.
- `EndSessions` and every other end go through `endSession`: audit `terminal.terminate` (with
  `close_detail` when the host reported one) and the personal event `terminal_session.changed`.
- `adapters/hosttest`: the Go test client of the control connection (`Dial`, `Identify`, `Send`,
  `Next`, `ExpectClose`, `Pause`, `StartHeartbeat`), validating every frame with
  `api/specschema.ValidateChannelFrame` (G9).
- Close codes on the connection: `4002` decoding and order, `4009` heartbeat, `4013` shutdown (stop
  step `host_control_connections`), `4014` slow consumer, `4000` internal error, `4110` to `4114`
  as in 24 section 13.
- Configuration keys: `terminal.host_heartbeat_interval`, `terminal.host_heartbeat_miss_limit`,
  `terminal.host_auth_timeout`, `terminal.host_duplicate_probe_timeout` (`configkeys_terminal.go`),
  besides `sync.ws_max_frame_bytes`, `sync.outbound_queue_max`, `gateway.write_timeout`,
  `http.max_json_depth`, `auth.clock_leeway` and `identity.last_active_granularity`.
- Recordings in both directions: `services/host/testdata/control/server_frames.json` (written here
  with `go test -update`, `TestServerFramesRecordingCurrent`) and `host_frames.json` (written by the host service, checked
  here by `TestHostClientFramesValid`).
- Tests: `domain/control_test.go` and the integration tests of `adapters/wsrelay` over PostgreSQL 18
  and the real use cases (login, close codes `4110` to `4114`, TM-T23, TM-T16, reports, SR10, SR12,
  heartbeat, JWKS refresh, loss, shutdown, no secrets).

## Extension points

- M2-44: calls `identity.StepUp.ConsumeFresh` in the transactions of `openTerminalSession` and `attachTerminalSession`, sets `Resource.HostID` and `Resource.Terminal` and adds `host_not_active` and `terminal_profile_unknown` after SR12.
- M2-44: produces `session.open` and `session.close` through `app.ControlLinks` (`ErrHostNotConnected` and `ErrHostNotReady` map to `409 host_unavailable`) and reuses the login of `Authenticate` for `hostData`.
- M2-47 (built, see "Built in M2-47"): the duplicate probe and the presence across nodes behind `app.ControlLinks`, the signals through `app.Signals` and `app.SignalHandler`.
- M7-17 (built): the rest of the operation of several nodes on the same `cluster.mode`; the terminal's listener became a route of the one `LISTEN ephemeral` connection of the process (`platform/pg.EphemeralListener`, docs/decisions.d/M7-17.md D-M7-17-2). A signal kind for the re-sent `ready` (D-M2-47-17) was not needed.
- M2-44: sessions and relay; adds `terminal_session.changed`, the close code `4100` and `session.close` to the host service to `EndSessions`.
- M2-46 (built, see "Built in M2-46"): revocation through the three subscriptions, the limits in the relay and in `terminal.sweep`, the session limits, the rate limits of the channels and the session metrics.
- M2-48: runs TM-T06, TM-T10 and TM-T11 against the real host service in a container; M7-05: deletes `terminal_sessions` rows with `retention.security`.

## Notes for the build

- Terminal content is never on the server, in telemetry, in the audit, in search, in push or in
  the context of an agent; the content runs end-to-end encrypted between device and host, the
  server is relay and authorization point (24-terminal.md, sections 1 and 5.1).
- The streams run over separate connections with their own ops, not over the gateway (05-actions.md,
  section 7); relay and host connections live only in the adapter `wsrelay`.
- The role `bothy_app` reads, inserts and updates the three tables and never deletes; an assignment
  stays after its end, so its name stays taken on the host.
- The name of a host is never in the audit, a log or a problem response (24, section 10.4; `TestHostCanary_SECT10`).

## Chapters

[02-architecture.md](../../../../docs/spec/02-architecture.md), [24-terminal.md](../../../../docs/spec/24-terminal.md), [07-permissions.md](../../../../docs/spec/07-permissions.md), [ADR 0068](../../../../docs/adr/0068-terminal-through-a-host-service-with-noise-stream-and-invariant-i6.md)

## Built in M2-44

- Port `Sessions` (`sessions.go`) with its DTOs and the redacting `SessionToken` (only `Reveal`
  hands out the compact JWS, once, in the HTTP adapter).
- `domain/`: `token.go` (`SessionClaims`, `NewSessionClaims`, `TokenAlg`, `TokenTyp`; the claims of
  24 5.3, schema test `TestSessionClaims_MatchSpec`), `profile.go` (`ProfileKnown`, D-M2-44-1),
  `closecodes.go` (the codes of `terminalStream`, `StreamCodeForEnd`, `HostRefusal`), `relay.go`
  (the pure pairing table `Decide`), `session.go` (`AttachKind`, `RecheckEnd`, the Noise limit and
  the protocol labels as protocol constants).
- `app/`: `sessions_open.go`, `sessions_attach.go`, `sessions_read.go`, `sessions_close.go`
  (the five use cases; open and attach consume the step-up as the first row lock, sign the token
  inside the unit of work, audit `terminal.open` and `terminal.attach` with `pin_changed` and
  `foreign_assignee`, recheck after the commit under `authorize.Recheck`, then tell the relay and
  send `session.open`), `sessions_stream.go` (the decisions of both stream endpoints),
  `sessions_relay.go` (the callbacks of the relay), `reconcile.go` (the start step), `token.go`
  (the issuer) and the outgoing ports `TokenSigner`, `StepUps`, `RelayControl`, `Timers` and
  `Metrics`. The one end path `finish` adds the live counters, writes `closed_by` and
  `terminal.close` for an end by a person, else `terminal.terminate`.
- `adapters/wsrelay`: `stream.go` (device stream, Origin check before the upgrade, `hello` with
  the DPoP nonce, `identify` within `terminal.open_timeout`; the web console path of M6-18: an
  upgrade with the console cookie needs an `Origin` equal to `server.public_url`, else `403`
  before the upgrade, and its `identify` carries no token: it passes only for the console session
  of that cookie and its device, then authorize `terminal.stream` admits only the device of the
  session's token; an `identify` without token on any other connection and a token on a console
  connection close with `4004`, a revoked console session with `4012`; tests
  `stream_console_internal_test.go`), `hostdata.go` (data connection on the
  login function of the control connection), `relay.go` (slots, bounded queues per direction
  `terminal.relay_buffer_max_bytes`, deadlines, counters, the log tail observer, the stop step),
  `frames.go` (one binary message is one Noise message of at most 65535 bytes, `FuzzReadFrame`).
- `adapters/otelmetrics`: `bothy.terminal.relay.unpaired` with `bothy.terminal.side`.
- Close codes of the device stream: `4002` decoding, text frame or message above the Noise limit;
  `4004` authentication; `4012` revoked; `4013` shutdown; `4014` slow consumer; `4100` the session
  ended (reason `close_reason`); `4101` and `4104` the host's refusal; `4102` no counterpart;
  `4103` another device attached. Data connection: `1000` no expectation or a paired host side,
  `4100` unknown, foreign or closed session, `4002`, `4013`, `4014`, and the login codes of M2-43.
- Effects after the commit come from the event log tail: a `terminal_session.changed` with state
  `closed` closes both sides and sends `session.close` (D-M2-44-7). Start: sessions of the previous
  process end (`opening`) or detach (`active`); stop: every relay connection closes with `4013`.
- Actions: `terminal.open`, `terminal.attach`, `terminal.stream`, `terminal.read_own`,
  `terminal.close`, `terminal.terminate`. Events: `terminal_session.changed` to the person of the
  session, with the person as actor for open, attach and close.
- Tests: `domain` (claims, profile, close codes, the relay table), `app` (rollback, recheck,
  quick reattach race, redaction, reconcile), `adapters/wsrelay` (`FuzzReadFrame`, shutdown) and
  `test/integration/terminalsessions` over the full-server harness with the Fake-Host of
  `test/terminal/fakehost`.

## Built in M2-47

Several api nodes serve the terminal as one ([24-terminal.md](../../../../docs/spec/24-terminal.md)
sections 3.4 and 9; the reasons are the rows `D-M2-47-<n>` of
[DECISIONS.md](../../../../docs/DECISIONS.md)).

- Cluster mode: `cluster.mode` (`single` or `multi`, `internal/wire/configkeys_cluster.go`). With
  `single` the use cases bind `app.LocalSignals`, no listener runs and no `NOTIFY` is sent; the
  behavior of M2-43 and M2-44 stays. With `multi` they bind `adapters/postgres.NotifySignals`; the
  roles `api` and `all` run the listener, the role `worker` only sends (it holds no connection).
  `internal/wire` (`terminal_cluster.go`) chooses by the mode; `app.New` binds `LocalSignals` over
  the `Service` itself, so a process without wiring behaves like one node.
- Signals (`domain/cluster.go`, `app/signals.go`): kinds `terminal.session_open` (host, session),
  `terminal.host_close` (host), `terminal.host_present` and `terminal.host_gone` (host, connection)
  and `terminal.session_paired` (host, session); compact JSON with exactly these IDs, strict
  `DecodeSignal`, foreign kinds ignored, the payload below `NotifyPayloadMax`. Port `app.Signals`:
  `Stage` in the unit of work (with `multi` a `NOTIFY ephemeral` in the transaction, delivered only
  after the commit), `Committed` after it (with `single` the local handler), `Send` outside any unit
  (presence and pairing). The end of a session has no signal: it reaches every node through the log
  tail of M2-44.
- Handler: the hub of `adapters/wsrelay` (`Hub`, `cluster.go`) is the `app.SignalHandler`. It hands
  `terminal.session_open` and `terminal.host_close` to `app.Service.HandleSignal`, which reads the
  rows and calls `RelayControl.Expect` (every node) and `ControlLinks.OpenSession` (only the node of
  an eligible control connection of an `active` host, for every session that is not `closed`), or
  closes the control connections with the code of the host's state (`4110`, `4111`, `4112`); data
  connections and device streams close with the end of their sessions through the log tail.
- Listener: since M7-17 the terminal is a route (`adapters/postgres.SignalRoute`) of the one
  `LISTEN ephemeral` connection of the process, `platform/pg.EphemeralListener`, the component
  `ephemeral_listener` started after the HTTP listeners (application name `bothy-ephemeral`,
  dialer `pg.Pool.DialDedicated`), which it shares with the gateway; each route ignores the kinds
  of the other. On a loss the listener logs once, retries at once and then every
  `database.listen_reconnect_backoff_cap`, and after `LISTEN` succeeded again the hub reconciles
  (`Reconcile`: host states of all its connections, closes what may no longer be connected,
  announces its eligible connections). Soft dependency: no readiness check; stop step in
  `lifecycle.PhaseEventTail`; an unexpected end of its goroutine fails the process. The terminal
  route is added only with `cluster.mode = multi`.
- Presence view: `domain.PresenceView` of the control connections of other nodes, renewed by
  `terminal.host_present` after the first `report` and after each `heartbeat`; an entry counts for
  `terminal.host_heartbeat_interval` times `terminal.host_heartbeat_miss_limit`. "Host connected"
  (`ControlLinks.Connected`) is a local eligible connection or a counting remote entry.
- Probe and probation: every control connection gets a connection ID (UUIDv7 on the node's clock,
  `domain.OlderConnection`); the node of the older connection probes it when a newer one is
  announced elsewhere (`terminal.host_duplicate_probe_timeout`, `4009` or the duplicate verdict with
  `4112`); a new connection gets no `session.open` before `domain.ProbationEnd` (twice the probe
  timeout after its first announcement), then the session is checked again in the database.
- Lone side (`domain.LoneSideOutcome`, `LoneDeadline`): each side has its own timer of
  `terminal.open_timeout` from its accepted `identify`; a lone side closes with `4102` and counts
  `bothy.terminal.relay.unpaired` with `bothy.terminal.side`; an `opening` ends `open_failed` with
  `open_timeout` through the compare-and-swap `RelayOpeningTimedOut`; a `detached` session stays
  `detached`; a host data connection of a started session waits without a timer. The slot deadline
  waits while an unpaired side is present, and a slot that never saw `session.started` reads the
  session state at its deadline. With `multi` a side that arrives first builds the expectation of
  the current grant from the row (`RelayExpectation`; an expectation of the grant the slot holds
  changes nothing), and the log tail marks a slot started (`terminal_session.changed` with `active`
  or `detached`).
- Log records (constant messages, no ID): `terminal relay side unpaired`, `terminal signal
  dropped`, `terminal duplicate control connection`; the shared listener logs `ephemeral
  listener lost` and `ephemeral listener restored` (M7-17). No new instrument and no new attribute.
- Edge requirement: both stream paths of a session must reach one node (consistent hashing over
  `{session_id}`, 16 section 4.4); the example is `infra/compose/compose.multi-node.yaml` with
  `infra/compose/edge/nginx.conf`. Without it a side fails visibly with `4102`.
- Limits: the order of two control connections on two nodes rests on synchronized clocks; a dead
  node's presence counts for at most the window above; after a start or a reconnect the view is
  complete within one `terminal.host_heartbeat_interval`; while a node's `LISTEN ephemeral` is down
  it misses every signal (its control connections of a host disabled elsewhere stay open until the
  reconciliation, openings through them end `open_failed`). The re-sent `ready` of
  `setHostLlmCredentials` and `confirmHost` stays local to the node that served the call
  (D-M2-47-17).
- What M7-17 adds: `NOTIFY ephemeral` for gateway, `runs` and `agents`, possibly one shared
  listener per process, rate limits and the rest of the operation of several nodes.
- Tests: `domain/cluster_test.go` (codec, fuzz, payload bound, order, presence, probation, lone
  side), `app` (`LocalSignals`, staged signals), `adapters/wsrelay` (the handler), `internal/wire`
  (key and wiring), `test/terminal/multinode` (TM-T15 and the cross-node tests over two in-process
  nodes) and `test/edge` (the example edge with the real nginx image and the overlay).

## Built in M2-45

- `hello` of the control and the data connection (`adapters/wsrelay`, `helloFrame`) carries the
  server values the host service applies to a session: `clock_leeway_ms` from `auth.clock_leeway`
  (the tolerance on `exp` of the session token and its `jti` memory), `idle_timeout_ms` from
  `terminal.idle_timeout` and `detach_grace_ms` from `terminal.detach_grace` (its timers); the
  host service takes them from the `hello` of the data connection that carries the handshake
  (M2-45 Design note 2a). `internal/wire` binds the three keys into `wsrelay.Config`; the two
  terminal keys are registered here, and the server's own checks with them follow in M2-46. The
  Fake-Host's script `EndAfterHandshake` plays the close order of the host service after message 2
  (`session.ended`, data connection kept open), proven by `TestRelay_HostEndCloseOrder`.

## Built in M2-46

Every revocation and every limit ends terminal sessions without the help of the host service
([24-terminal.md](../../../../docs/spec/24-terminal.md) sections 6.3, 8.3 and 10.3; the reasons
are the rows `D-M2-46-<n>` of [docs/decisions.d/M2-46.md](../../../../docs/decisions.d/M2-46.md)).

- One end function: `app.Service.EndSessions` (inside the caller's unit of work) and `EndSession`
  (its own unit, as the system) with the selector members `SessionID`, `DeviceID` and
  `AuthSessionIDs` besides `HostID`, `UserID` and `ExceptUserID`. The end value is `domain.End`
  (`Reason`, `Detail`, `ExitCode *int`, `ClosedBy`, `Source`, `Progress`) with `domain.EndSource`
  `server`, `host` or `close_request`; `domain.ServerEnd` and `domain.HostEnd` build it, and
  `End.Normalized` keeps an exit code only with `process_exited` from the host and a closer only
  with `close_request`. `domain.ValidateEnd` applies the pair rule of 24 10.3 to every writer.
  Every end goes through `finish`: the one closing statement `EndTerminalSession`
  (`queries/ending.sql`, the only statement that sets `close_reason`; it replaces `CloseSession`,
  `CloseSessionReported` and `CloseTerminalSessionByPerson`) with `WHERE state <> 'closed'`, the
  audit entry `terminal.terminate` as the system with exactly `session_id`, `close_reason`,
  `close_detail` (only when set), `bytes_in`, `bytes_out` and `duration_ms` (or `terminal.close`
  for an end by a person), the personal event `terminal_session.changed`, and
  `bothy.terminal.sessions.closed` after the commit. A session that is already closed is skipped:
  the first committed end wins.
- Close path: no connection is touched in a transaction. The log tail of every api process hands
  the committed `terminal_session.changed` with `closed` to the relay, which closes the host data
  connection it holds with `4100` and the device stream with the code of `domain.StreamCodeForEnd`
  (`4100` for every revocation and limit; `4101` or `4104` for an `open_failed` with a token or
  handshake detail, M2-44) and the `close_reason` as reason, and the process
  holding the host's control connection sends `session.close`, after which the PTY helper ends the
  whole process tree. A host service that was offline gets `session.close` at its next login
  (`live_sessions`, M2-43).
- Subscriptions (`app/subscribers.go`, `app/revocation.go`): `Subscribers` with
  `OnAccessChanged` (removed host targets end that person's sessions on the host; a lost owner
  status ends what `domain.SR10Lost` names; reasons from `domain.ReasonForLostHost`),
  `OnDeviceRevoked` (sessions whose current `device_id` is the device,
  `domain.ReasonForDeviceRevoke`) and `OnSessionEnded` (sessions of the ended login sessions,
  `domain.ReasonForSessionEnd`). `internal/wire` builds them with the txn registry (golden file
  `txn_registrations.golden`) and binds the use cases in the terminal step (`Subscribers.Bind`).
  Unbound, a subscriber does nothing without open sessions and fails the unit when it finds a
  session it would have to end, so a revocation never commits around an open session.
- Job `terminal.sweep` (`adapters/jobs`, `app/sweep.go`): queue `maintenance`, every
  `terminal.sweep_interval` and once at the start, roles `worker` and `all`, no actor. It reads the
  sessions that are not closed with the state of their host (`SweepCandidates`), asks identity
  which login sessions ended (`app.AuthSessions`, bound to `identity.Auth.EndedSessions`), and ends
  each session in its own unit of work (host row, then session row) with the first match of
  `domain.SweepVerdict`: host not `active`, login session ended, `opening` past the last grant
  plus `terminal.open_timeout` (not on a host reporting `require_local_confirm`), `max_duration`,
  `detach_expired`, `idle_timeout` with the stored `last_input_at`. The last grant is the UUIDv7
  time of `token_jti`, which the use cases now generate on their clock.
- Deadlines of the relay (`adapters/wsrelay/relay.go`, `domain/deadlines.go`): per held session one
  timer on the existing scheduler port `app.Timers` at `domain.RelayDeadline` (idle from the
  latest of the last device message this process saw, `last_input_at`, `started_at` and
  `opened_at` plus `terminal.idle_timeout` plus `terminal.idle_grace`, in every open state;
  `opened_at` plus `terminal.max_session_duration` across reattaching; `detached_at` plus
  `terminal.detach_grace`). The instant itself ends (`domain.Reached`); the relay ends through
  `RelayEnd` with its unwritten counts. Progress: the relay writes its counts since the last write
  as additions at most once per `terminal.idle_grace` (`domain.PersistDue`) and when a pairing is
  released, `last_input_at` only forward, without a new version and without an event; a failed
  periodic write gives the counts back to the slot.
- Session limits (`app/sessions_open.go`): the unit of work of `terminal.open` takes the person
  lock (`LockPersonForOpen`, an advisory lock), consumes the step-up (the first row lock), locks
  the host row and compares the counts of `CountOpenSessions` with those of the decision; when they
  moved, the opening starts again with a new decision, at most `database.tx_retry_max` times, then
  `503 unavailable`. The guard `terminal_session_limit` lives in `authz` (`403 terminal_denied`,
  `reason` `session_limit`); a denial keeps the step-up. The recheck after the commit does not
  repeat the limit.
- Rate limits (`adapters/wsrelay/limits.go`, bound in `internal/wire` to `platform/ratelimit`):
  `rl.terminal` of the person at `identify` of the device stream after the credential check,
  `rl.host` of the host after the proof check on control and data connections, and one token of
  `rl.public_auth` of the client address for each failed check of presented credentials; an empty
  bucket closes with `4008` and `retry_after=<seconds>`. REST opening and attaching count against
  `rl.terminal` in the REST chain.
- Metrics (`app/metrics.go`, `adapters/otelmetrics`): `bothy.terminal.sessions.closed`
  (`bothy.reason`, after the commit in the process that committed the end, api and worker),
  `bothy.terminal.sessions.active` (`bothy.terminal.state`, the sessions the relay of this api
  process holds with at least one connection) and `bothy.terminal.hosts.connected` (the control
  connections of this process that are usable for sessions), next to M2-44's
  `bothy.terminal.relay.unpaired`; closed attribute sets, no ID of a person, no host name, no span
  per frame.
- Configuration keys consumed first here (`internal/wire/configkeys_terminal.go`):
  `terminal.idle_grace`, `terminal.max_session_duration`, `terminal.max_sessions_per_person`,
  `terminal.max_sessions_per_host`, `terminal.sweep_interval`; besides `terminal.idle_timeout`,
  `terminal.detach_grace`, `terminal.open_timeout` and `database.tx_retry_max`.
- Tests: `domain` (`ending_test.go`, `deadlines_test.go`), `adapters/wsrelay`
  (`relay_limits_internal_test.go`), `adapters/otelmetrics` (`TestTerminalMetrics_ClosedAttributes`)
  and the integration tests of `test/terminal` over the full-server harness with the Fake-Hosts on a
  fake clock with the deadline scheduler `fakehost.ClockTimers`.
