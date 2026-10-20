# Modul `terminal`

Status: Datensätze der Rechner (M2-39), Rechte des Terminals (M2-40), Kopplung (M2-41), control connection of the host services (M2-43), terminal sessions, session token and relay (M2-44), the terminal across several api nodes (M2-47). Revocation of every kind, idle and maximum duration, session limits and the remaining metrics come with M2-46. Normativ ist [docs/spec/24-terminal.md](../../../../docs/spec/24-terminal.md); bei Abweichung gilt das Kapitel.

## Zweck

Rechner und Kopplung, Erklärung der LLM-Anmeldung (I6), Sitzungsbenutzer je Person (Ruling T12), Terminal-Sitzungen, Sitzungs-Marken, Steuerverbindungen der Host-Dienste, Relais der Ströme.

## Code

- `internal/terminal` (`hosts.go`): Port `Hosts` mit `List`, `Get`, `Create`, `Update`, `SetState` und den DTOs (`HostView` Feld für Feld wie das Schema `Host`, `CreateHost`, `UpdateHost`, `SetHostState`, `PairingCodeIssued` mit dem Kopplungs-Code als redigierendem `identity.Token`); `Register` meldet `host.updated` an.
- `domain/`: `Transition` ist die einzige Stelle, die den Lebenszyklus aus 24, Abschnitt 3.1 kennt (M2-41 und M2-43 rufen sie für ihre Übergänge); `CloseReasons` und `CloseDetails` sind die geschlossenen Listen aus 24, Abschnitt 10.3; `NewHostName`, `NewOSUser` und `CheckReason` prüfen die Texte (SEC-IN-4); `PlanAssignments` plant Zuordnung und Ende der Sitzungsbenutzer nach T12 (genau gemeldeter Name, Eindeutigkeit je Rechner ohne Groß- und Kleinschreibung über alle Zeilen, höchstens eine gültige Zuordnung je Person, alle `400` vor jedem `409`); `reported.go` liest die gemeldeten Spalten.
- `app/`: die fünf Anwendungsfälle (`Service`), `EndSessions` (einziger Weg jedes Endes durch den Server), `HostSource` (Implementierung von `authz.HostSource`), die Sicht `view.go`, die ausgehenden Ports `Store`, `PairingCodes`, `People`, `Presence` und `Cursors`; `NoConnections` ist die Anwesenheit ohne Steuerverbindungen.
- `adapters/postgres/`: die sqlc-Abfragen in `queries/`, der erzeugte Code in `sqlcgen/` und `Repository`.
- `adapters/httpapi/`: die Handler von `listHosts`, `createHost`, `getHost`, `updateHost`, `setHostState`, `setHostLlmCredentials` (M2-40), `issueHostPairingCode`, `pairHost` und `confirmHost` (M2-41).
- Testcode unter `services/server/test/`: `terminaltest` legt Rechner in jedem Zustand mit gemeldeten Profilen, Sitzungsbenutzern und Prüfergebnissen sowie Zuordnungen und Sitzungen direkt in SQL an (in Produktion schreiben das M2-41, M2-43 und M2-44); `terminalworld` ist die HTTP-Testwelt von M2-39.

## Port (Wurzelpaket `internal/terminal`)

- Eingehend: `Hosts` (M2-39; `SetLlmCredentials` seit M2-40) and `Sessions` (M2-44: `Open`, `Attach`, `List`, `Get`, `Close`, implemented by `app.Sessions`). The relay is no port for other modules; it lives in `adapters/wsrelay` (02 4.3).
- Gibt In-Tx-Ereignisse aus: keine.
- Abonniert In-Tx-Ereignisse: ab M2-46 kernel `authorize.AccessChanged`, `identity.DeviceRevoked`, `identity.SessionEnded` (Sitzungen beenden).
- Implementiert Quellen-Ports und Querschnitts-Ports: `authz.HostSource` (Zustand, Inhaber, LLM-Erklärung, Server-Host des Rechners), gebunden in `internal/wire` für Authorizer, Rechte-Hook und Freigaben.

## Abhängigkeiten

Darf die Ports dieser Module nutzen: `instance`, `identity`, `authz`, `agents`.

Grund: Signer für Sitzungs-Marken (Zweck `terminal`); Menschen, Geräte, Owner, Verbrauch des Step-up, Kopplungs-Codes (`identity.Tokens.NewHostPairingCode`); Freigaben auf Rechner und I6; Bereitschaft der Dev-Umgebung (Etappe).

Zusätzlich für alle Module: `internal/kernel`; `internal/platform` und `api/` nur in `adapters/`.
Geprüft durch go-arch-lint (ADR 0008).

## Tabellen

`hosts`, `host_session_users`, `terminal_sessions`.

Nur dieses Modul schreibt und liest diese Tabellen per SQL (03-datenmodell.md, Grundsatz 12).

## Aktionen (05-aktionen.md)

`host.read`, `host.manage`, `host.pair`, `host.connect`, `terminal.open`, `terminal.attach`, `terminal.stream`, `terminal.read_own`, `terminal.close`, `terminal.terminate`.

## Ereignisse und WebSocket-Ops

- Dauerhaft: `host.updated` (M2-39), `terminal_session.changed` (M2-44).
- Kurzlebig: `host.presence_changed` (M2-43).
- WebSocket-Ops (Client an Server): keine im Gateway; eigene Kanäle mit eigenen Ops (05, Abschnitt 7).

## Gebaut in M2-39

- Lesen eines Rechners für Owner, Inhaber und Menschen mit wirksamer Freigabe Terminal; Verwaltung
  nur durch Owner mit Step-up, Pflichtgrund, Audit und Rechteänderung. Agenten, Apps und
  persönliche Tokens bekommen `403 forbidden` mit `subject_kind_not_allowed`.
- Das Ereignis zu einem Rechner geht bei jeder Änderung an alle, die ihn lesen dürfen, nie mit
  einem Schlüssel oder dem Kopplungs-Code.
- `EndSessions` schreibt das Ende jeder Sitzung als Systemaktion ins Audit. Die übrigen Aktionen
  bleiben bis M2-40 bis M2-44 zurückgestellt.

## Gebaut in M2-40

- `SetLlmCredentials` (`setHostLlmCredentials`, `host.manage`): Erklärung `none`, `personal` oder
  `organization` mit `credential_holder_id` genau bei `personal` (`domain.ValidateDeclaration`:
  `required`, `not_allowed`; ein unbekannter oder gelöschter Mensch `unknown_user`), Pflichtgrund,
  `If-Match`, `409 conflict` auf einem `retired` Rechner, ohne Wirkung bei derselben Erklärung
  (`no_change`). Die Rechte-Änderung sperrt den Rechner nach `permission_version`, schreibt die
  Erklärung über `ApplyLlmDeclaration` (auch für `confirmHost` von M2-41) und beendet beim Wechsel
  auf `personal` oder des Inhabers der Anmeldung die Sitzungen aller anderen mit `i6_violation`
  (`domain.EndsForeignSessions`, `SessionSelector.ExceptUserID`); I6 prüft `authz` in Phase 3.1, eine
  Verletzung rollt alles zurück (`409 i6_violation`).
- `RecordSessionUserReport` und `ApplySessionUserReport` (für `pairHost` von M2-41 und `report` von
  M2-43): die gemeldeten Sitzungsbenutzer sind Rechte-Stand. Eine gleiche Meldung
  (`domain.SameReport`) ändert nichts; eine andere schreibt sich mit neuer Version des Rechners,
  beendet die Sitzungen der Personen, deren zugeordneter Name fehlt (`domain.VanishedNames`,
  `access_revoked`), und schreibt `host.updated`.
- `Update` beendet beim Wechsel oder Löschen des Inhabers die Sitzungen des bisherigen Inhabers mit
  `access_revoked`, außer er ist aktiver Owner (`People.IsActiveOwner`, SR10).
- `HostSource.PersonalHosts` und `SessionUsersOf` (genauer Vergleich mit `domain.Reports`) und
  `HostFacts.Name` für den Text der Vorschau.
- Tests: `domain` (`TestValidateDeclaration`, `TestEndsForeignSessions`, `TestSameReport`,
  `TestVanishedNames`) und `adapters/httpapi/terminalauthz_test.go` über PostgreSQL 18 und die
  HTTP-Kette: `TestTMT09_I6`, `TestI6_UserEnableRevivesGrant`, `TestI6_SwitchEndsForeignSessions`,
  `TestI6_HolderChangeEndsForeignSessions`, `TestSR10_AssigneeChangeEndsSessions`,
  `TestSetHostLlmCredentials_HTTP`, `TestSR12_ReportDropsName`, `TestPreviewHostGrant`,
  `TestTerminalAuthzAudit_NoContent`, `TestAZP1_TerminalAuthzRoutes`.

## Gebaut in M2-41

- Kopplung: `IssuePairingCode` (`issueHostPairingCode`, Neukopplung), `Pair` (`pairHost`,
  öffentlich, der Code ist der Nachweis) und `Confirm` (`confirmHost`) in `app/pairing.go`. Ein
  gemeinsamer Ausgeber (`issueCode` über `PairingCodes.Issue`) dient `createHost`,
  `issueHostPairingCode` und dem Rückfall; `PairingCodes.Hash` (gebunden an
  `identity.Tokens.HashHostPairingCode`) prüft Form, Art und Prüfsumme ohne Datenbankzugriff.
- `domain/fingerprint.go` (`ComputeFingerprint`, `CompactFingerprint`, Crockford-Base32, Vektoren
  in `domain/testdata` und `services/host/testdata/noise/fingerprint.json`) und
  `domain/pairing.go` (`ParseHostKeys`, `ValidatePairReport`, `NormalizeFingerprintPrefix`,
  `PrefixMatches`, `NextAttempt`, `RepairEffects`) mit den Fuzz-Zielen `FuzzParseHostKeys` und
  `FuzzNormalizeFingerprintPrefix`.
- Bestätigung: der Präfix zuerst; ein Fehlversuch zählt ohne neue Version und bleibt auch unter
  `Idempotency-Key` (`idempotency.KeepEffectOnFinal`); nach `terminal.fingerprint_max_attempts`
  Rückfall auf `pairing` ohne Schlüssel. Danach Zuordnungen (`PlanAssignmentsAt`), Erklärung
  (`ApplyLlmDeclaration`, I6) und `declared_server_host` in einer Transaktion.
- Neukopplung: `RepairEffects` je Zustand, Ende der Sitzungen mit `host_rekeyed`, `key_changed`
  genau beim Verwerfen gespeicherter Schlüssel; der Fingerabdruck verworfener Schlüssel bleibt
  gespeichert und weist dieselben Schlüssel mit `host_key_reused` ab.
- Tests: `domain` (Fingerabdruck, Regeln), `app` (`TestTMT20_WrongChecksumNoDatabase`) und
  `adapters/httpapi/pairing_test.go` (TM-T19, TM-T20, TM-T28 (b) und (e), Neukopplung,
  Nebenläufigkeit, keine Geheimnisse).

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

## Erweiterungspunkte

- M2-44: ruft `identity.StepUp.ConsumeFresh` in den Transaktionen von `openTerminalSession` und `attachTerminalSession`, setzt `Resource.HostID` und `Resource.Terminal` und ergänzt nach SR12 `host_not_active` und `terminal_profile_unknown`.
- M2-44: produces `session.open` and `session.close` through `app.ControlLinks` (`ErrHostNotConnected` and `ErrHostNotReady` map to `409 host_unavailable`) and reuses the login of `Authenticate` for `hostData`.
- M2-47 (built, see "Built in M2-47"): the duplicate probe and the presence across nodes behind `app.ControlLinks`, the signals through `app.Signals` and `app.SignalHandler`.
- M7-17: the rest of the operation of several nodes on the same `cluster.mode`; it may merge the listener `terminal_ephemeral` with its own `LISTEN ephemeral` connection, which the namespaced kinds (prefix `terminal.`) allow, and may add a signal kind for the re-sent `ready` (D-M2-47-17).
- M2-44: Sitzungen und Relais; ergänzt `EndSessions` um `terminal_session.changed`, den Schließ-Code `4100` und `session.close` an den Host-Dienst.
- M2-46: Entzug, Grenzen und Metriken (`bothy.terminal.sessions.closed`), Job `terminal.sweep`.

## Hinweise für den Bau

- Terminal-Inhalte stehen nie im Server, in der Telemetrie, im Audit, in der Suche, im Push oder im
  Kontext eines Agenten; der Inhalt läuft Ende-zu-Ende verschlüsselt zwischen Gerät und Rechner,
  der Server ist Relais und Autorisierungspunkt (24-terminal.md, Abschnitte 1 und 5.1).
- Die Ströme laufen über eigene Kanäle mit eigenen Ops, nicht über das Gateway (05-aktionen.md,
  Abschnitt 7); Relais und Host-Verbindungen liegen nur im Adapter `wsrelay`.
- Die Rolle `bothy_app` liest, legt an und ändert die drei Tabellen, löscht nie; eine Zuordnung
  bleibt nach ihrem Ende stehen, damit ihr Name auf dem Rechner belegt bleibt.
- Der Name eines Rechners steht nie im Audit, in einem Log oder einer Problem-Antwort (24, Abschnitt 10.4; `TestHostCanary_SECT10`).

## Kapitel

[02-architektur.md](../../../../docs/spec/02-architektur.md), [24-terminal.md](../../../../docs/spec/24-terminal.md), [07-rechte.md](../../../../docs/spec/07-rechte.md), [ADR 0068](../../../../docs/adr/0068-terminal-ueber-host-dienst-mit-noise-strom-und-invariante-i6.md)

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
  the DPoP nonce, `identify` within `terminal.open_timeout`), `hostdata.go` (data connection on the
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
  behaviour of M2-43 and M2-44 stays. With `multi` they bind `adapters/postgres.NotifySignals`; the
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
- Listener: `adapters/postgres.EphemeralListener`, the component `terminal_ephemeral` started after
  the HTTP listeners (application name `bothy-terminal-ephemeral`, dialer `pg.Pool.DialDedicated`);
  on a loss it logs once, retries at once and then every `database.listen_reconnect_backoff_cap`,
  and after `LISTEN` succeeded again the hub reconciles (`Reconcile`: host states of all its
  connections, closes what may no longer be connected, announces its eligible connections). Soft
  dependency: no readiness check; stop step in `lifecycle.PhaseEventTail`; an unexpected end of its
  goroutine fails the process. One listener per process.
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
  dropped`, `terminal ephemeral listener lost`, `terminal ephemeral listener restored`, `terminal
  duplicate control connection`. No new instrument and no new attribute.
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
