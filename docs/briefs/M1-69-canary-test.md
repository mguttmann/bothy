# M1-69: Canary test across all outputs (SEC-T10)

Status: ready | Depends on: M1-50, M1-68 | Etappe: M1

## Goal

After this ticket a Go test package `services/server/test/canary` runs a scenario through every
enabled operation and every client gateway op of the server core with run-unique canary values
(content, names, e-mail addresses, IP addresses, tokens and server secrets). It proves that none of
them appears in logs, traces, metrics, audit rows, error responses or events delivered to
principals without read rights, and that principal and device IDs appear in telemetry only as
pseudonyms (DP-T09). This is the test behind threat T-LG-01 and the regression guard for every
later ticket: a new operation that is not in the scenario makes the test fail.

## Read first

- `docs/spec/19-sicherheit.md`: sections "Logs ohne Geheimnisse und ohne Inhalte" (SEC-LOG-1 to
  SEC-LOG-7), "Geheimnisse" (SEC-SEC-2, SEC-SEC-3), "Eingaben und Ausgaben" (SEC-IN-2, SEC-IN-4,
  SEC-OUT-5), "Tests" (SEC-T10).
- `docs/spec/20-datenschutz.md`: "Audit-Log und IP-Adressen", "Telemetrie", "Tests" (DP-T09).
- `docs/spec/17-observability.md`: "Grundregeln", "Datenschutz", "Tests, die diese Regeln halten".
- `docs/observability/privacy.md` (all sections, especially "Was nie in Telemetrie steht",
  "Pseudonyme", "Kanarien-Test (Gate)"), `docs/observability/signals.md` ("Kardinalität",
  "Attribute in Spans und Logs", "Logs"), `docs/observability/audit-vs-telemetry.md`.
- `docs/spec/03-datenmodell.md`: tables `audit_events`, `event_log`, `auth_sessions` (`last_ip`),
  `owners` (`reason`), `grants` (`reason`), `messages`, `message_revisions`.
- `docs/spec/04-sync-protokoll.md`: "Umschlag (Envelope)" (payload rules 1 to 3), "Nachholen
  (Catch-up)", "Sichtbarkeit bei Rechte-Änderungen".
- `docs/spec/06-api.md`: "Grundregeln" (row Korrelation), "Fehler", "Idempotenz",
  "Authentifizierung je Prinzipal-Art".
- `docs/spec/08-identitaet.md`: "Lokale Konten", "Einladungs-Code", "Erstinstallation",
  "Geräte", "Token-Modell", "API-Tokens für Agenten, Apps und Werkzeuge".
- ADRs: 0011 (secret provider), 0021 (audit separate from telemetry), 0034 (OpenTelemetry via
  collector), 0061 (token format, if present).
- `docs/security/threat-model.md`: T-LG-01.
- `docs/PLAN.md`: section "M1 Fundament", rows M1-69 and M1-73 (the collector part of the canary
  belongs to M1-73).
- `services/server/api/operations.yaml` (enabled operations, P-04), `docs/api/openapi.yaml`,
  `docs/api/asyncapi.yaml` (client ops `identify`, `resume`, `heartbeat`, `reauth`, `focus`,
  `typing.start`, `typing.stop`).
- Code from earlier tickets: `internal/platform/otel` (M1-10), `internal/platform/secrets`
  (redacted secret type, M1-09), metric instruments (M1-68), `internal/audit` (M1-24, M1-50),
  `internal/platform/httpserver` (M1-18).

## Fixed requirements that apply

1. No MVP. Built from scratch directly in its final state; etappen are only the build order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined
   interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface has a docstring or
   doc comment (purpose, parameters, return value, errors, example where useful); comments explain
   why, not what. API documentation is generated from code (OpenAPI from the routes, dartdoc,
   Python API docs in the docs site).
4. Documentation in the repo: README per module/service, an ADR for every architecture decision, a
   CHANGELOG entry per release, docs site kept current. A change without matching documentation is
   not done.
5. Quality gates (SonarQube, linters, type checks, security scanners, test coverage) block the
   merge.
6. Test first: every test below is written and seen failing (red) before the code that makes it
   pass; the red output goes into the report.
7. No en dash and no em dash in any file; white label (example.com, Alex, Sam, Robin; no company,
   host or person names, no paths under a home directory).
8. Configuration only through named keys from `docs/operations/configuration.md`; no new number in
   prose.
9. SEC-T10: canary values through all ways of the server core; none appears in logs, traces,
   metrics, audit details, error responses or foreign events (19 "Tests").
10. DP-T09: no content and no clear IDs in telemetry (20 "Tests"); principal and device IDs only
    as HMAC pseudonyms (20 "Telemetrie", privacy.md "Pseudonyme").
11. SEC-LOG-2, SEC-LOG-3, SEC-LOG-4, SEC-LOG-5, SEC-LOG-6: header allow list, route patterns,
    no content, IP addresses only pseudonymised in the audit log, telemetry attributes only from
    the catalogue.
12. SEC-SEC-3: secrets never in events, audit details, telemetry or error messages.
13. SEC-OUT-5 and 06 "Fehler": problem bodies never contain content, secrets or internal paths.

## Scope

1. `services/server/test/canary/` (Go, package `canary`), files:
   - `values.go`: canary registry. `type Class string` with the values `content`, `admin_reason`,
     `secret`, `network`, `identity`. `type Canary struct { Value string; Class Class; Scope string;
     Label string }` where `Scope` is the channel ID the value belongs to or empty for
     instance-wide values. `func (r *Registry) New(class Class, scope, label string) string`
     returns `cnry` plus 20 lowercase hex characters from `crypto/rand`. `Register(value, class,
     scope, label)` adds server-issued values (tokens, codes) and secret file contents.
   - `sinks.go`: capture of all outputs (see Design notes, "Sinks").
   - `scan.go`: `func Scan(reg *Registry, sinks *Sinks, rules AllowRules) []Finding` with
     `type Finding struct { Sink, Location, Class, Label string; Prefix string }` (`Prefix` is the
     first eight characters of the matched value; the full value is never printed).
   - `scenario.go`: the scenario that drives every enabled operation and gateway op (Design notes,
     "Scenario").
   - `canary_test.go`, `scan_test.go`: the tests below.
   - `README.md`: purpose, sinks, allow matrix, how to add a new operation to the scenario, what
     the test does not cover.
2. Shared test harness `services/server/test/harness` (create or extend, see "Shared test
   harness" below). This ticket adds `Options.LogWriter` and `Options.Telemetry`.
3. Production seams, only if missing: `internal/platform/otel` must accept injected exporters for
   tests (`SpanExporter`, `MetricReader`, `LogExporter` and a log `io.Writer` in its options
   struct; nil means production behaviour). If M1-10 already offers this, use it. Each new exported
   field gets a doc comment that says it exists for in-process tests.
4. Fixes of leaks the test finds, in the owning module, each with its own regression unit test in
   that module (see Boundaries).
5. Documentation: `services/server/README.md` (list of test packages), `services/server/CHANGELOG.md`
   under "Unreleased" (Added), `docs/observability/privacy.md` section "Kanarien-Test (Gate)": one
   sentence naming `services/server/test/canary` as the in-process part and M1-73 as the collector
   part.

### Shared test harness (identical text in the briefs M1-69 to M1-72)

If `services/server/test/harness` exists on the base branch, use it and extend it only additively.
If it does not exist, first search for an existing full-server test helper
(`grep -rn "pgtest" services/server/internal/wire services/server/test`); wrap it if it builds the
real `internal/wire` graph, otherwise create the package with exactly this interface. Parallel
tickets build the same interface, so merges converge.

- Package `harness`, non-test Go files, architecture component `testsupport` (Design notes).
- `func Start(tb testing.TB, opts Options) *Server`: the real `internal/wire` graph with role `all`
  in this process; PostgreSQL 18 from `internal/platform/pg/pgtest`; migrations as
  `bothy migrate up --bootstrap` runs them; secrets created as `bothy secrets init` creates them, in
  `tb.TempDir()`; public and internal listener on `127.0.0.1:0`; the first owner created through the
  same application service that `bothy setup bootstrap-owner` calls (08 "Erstinstallation"), then
  enrolled with its enrollment code and fresh ES256 device keys (08 "Einladungs-Code",
  "Registrierung"); everything stopped in `tb.Cleanup` in reverse order.
- `type Options struct { Config map[string]string; LogWriter io.Writer; Telemetry *TelemetrySinks;
  EgressDenyAll bool; Clock clock.Clock }`. `Config` takes dotted keys from
  `docs/operations/configuration.md` and is applied through `platform/config` with flag precedence,
  so an unknown key still fails.
- `type Server struct { PublicURL, APIBase, GatewayURL, InternalURL, SecretsDir string; Owner *Client }`
  with `APIBase = PublicURL + "/api/v1"` and `func (s *Server) AdminDB() *pgxpool.Pool` (superuser
  of the disposable container; assertions only).
- `func (s *Server) NewUser(tb, UserSpec{Handle, DisplayName, Email string}) *Client` and
  `func (s *Server) NewAgent(tb, AgentSpec{Handle, DisplayName, BillingMode, CredentialHolder string}) *Client`,
  only through the public API (`createUser`, `createEnrollmentCode`, `completeEnrollment`;
  `createAgent`, `createToken`), owner step-up via `createStepUpChallenge` and
  `completeStepUpWithDeviceKey`.
- `type Client struct { PrincipalID, DeviceID string; ... }` with `Do(ctx, Request) Response`
  (DPoP proof per RFC 9449 with the latest `DPoP-Nonce`, one retry on `use_dpop_nonce`, refresh on
  `token_expired`, `Idempotency-Key` UUIDv7 on POST and PATCH except the list in 06 "Idempotenz",
  optional `Observer func(Request, Response)`), `StepUp(ctx)`, `Gateway(ctx) *GatewayConn`
  (`hello`, `identify`, `ready`, `Next`, `Send`, `Close`, automatic `heartbeat` and `reauth`).
- `type Response struct { Status int; Header http.Header; Body []byte; Elapsed time.Duration }`.
- `func BuildMatrixWorld(tb, s *Server) *World`: the M1 subset of 07 "Beispiele" and
  "Testmatrix": spaces Support, Marketing, Engineering, Research; channels Support/Allgemein,
  Support/Eskalation, Marketing/Kampagnen, Engineering/Projekt-A (forum, no project binding),
  Engineering/Werkzeuge, Research/Notizen; people Alex, Sam, Robin; agents Lotse (organization,
  operator of Support), Texter (organization, operator of Marketing), Runner (organization, operator
  of Engineering/Werkzeuge), Assistent (personal, credential holder the owner, operator of Research);
  group Leser with member Sam (membership with expiry); grants G-1 (Alex, `trigger`, `upload` and
  `approve_actions`, space Support), G-2 (Sam, `trigger`, `upload`, space Marketing), G-3 (Robin,
  `trigger`, `upload`, channel Engineering/Projekt-A, with expiry), G-4 (group Leser, `read`,
  Support/Allgemein), G-5 (Lotse, `write`, `upload`, space Support); one thread with messages per
  text channel and one forum post in Projekt-A. Left out until their etappe: app Tracker and G-6
  (M2), the DM Alex-Sam (V3), project binding and tool class `host` (M2). Expiry times are relative
  to the harness clock, never calendar dates.
- `func StartProcess(tb, ProcessOptions) *Process` (subprocess server for M1-72): built once per
  test binary from `./cmd/bothy`; not needed in this ticket, do not create it here if absent.
- Unit tests for every part you add (at least: DPoP proof claims and the nonce retry).

## Out of scope

- The canary through the real OTel Collector with the `file` exporter and container stdout (M1-73).
- File names, transcripts, search terms, channel prompts, push tokens, component form values and
  tool arguments (their features arrive in M2 and M3; those tickets extend this scenario).
- App crash reports (M4, M5), console cookies (M6).
- Output of the CLI commands `bothy setup` and `bothy secrets` (M1-37, M1-09, M1-73).
- Logs of the PostgreSQL server itself (operator configuration).
- New metrics or spans (M1-68, M1-10); this ticket only checks them.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-69-canary-test`.
- Conventional Commits with module scope (for example `test(canary): ...`, `fix(identity): ...`)
  and DCO sign-off (`git commit -s`); description language as in the existing `git log`.
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside this scope, except: the docs and changelog entries named here, the
  shared harness, the otel seam, and minimal fixes in the owning module for leaks this test finds,
  each with a regression test in that module and listed in the report.
- Follow the host rules supplied by the launcher. Stop only processes you started, by PID.
  Temporary directories only via `mktemp` or `t.TempDir()`, removed at the end.
- Never read or print real secrets; test secrets are generated per run in temp directories. No
  network calls except package downloads for pinned tools; the harness binds only `127.0.0.1` and
  port 0 (P-10); PostgreSQL runs as a disposable container on a random port (P-09).
- Do not lower a threshold, disable a gate or delete a test.

## Design notes

Sinks (all captured during the scenario, scanned afterwards):

1. Logs: the server's structured log stream through `Options.LogWriter` with
   `telemetry.logs.exporter=both` and `telemetry.logs.level=debug` (debug lines must be clean too),
   plus an in-memory OTLP log exporter. During the scenario also redirect `log.SetOutput` and
   `slog.SetDefault` to the capture, so libraries that log through the standard logger are caught.
2. Traces: in-memory span exporter with `telemetry.traces.sampler_ratio=1.0`. Scan span names,
   attributes, events (including exception events and their messages), status descriptions, link
   attributes and resource attributes.
3. Metrics: a manual reader collected at the end; scan metric names, attribute sets and exemplars.
4. Audit: every row of `audit_events` read through `AdminDB()`, all columns, `details` as text.
5. Error responses: every response with status 400 or higher seen by any harness client (body and
   all headers), recorded through `Client.Observer`.
6. Foreign events: observer connections that must not see a scope: Sam (no right on Support,
   Research, Engineering) and Robin (only Engineering/Projekt-A) keep a gateway connection open for
   the whole scenario and also page `GET /api/v1/events` (`listEvents`) from cursor 0 and load
   `getSnapshot` at the end. Scan frames, pages and snapshots.

Allow matrix (a value found in a sink not allowed for its class is a finding):

| Class | Examples | Allowed only in |
|---|---|---|
| `content` | message text and edits, thread titles, space and channel names, forum tag names, template names and content, draft content, group names and descriptions, human display names, human handles, e-mail addresses, OIDC claim values from the Fake-IdP, agent display names | success responses and events to principals with read rights on its scope; never in sinks 1 to 4, never in 5, never in 6 for a scoped value |
| `admin_reason` | `reason` of grants, owners, user disable, revocations | additionally `audit_events.details` (03 `owners.reason` "steht im Audit", I4) |
| `secret` | access and refresh tokens, API tokens, enrollment and pairing codes, `login_code`, step-up challenges, DPoP proofs (signature segment), passwords, TOTP codes and secrets, Fake-IdP codes and ID tokens, every file in the secrets directory (token pepper, private signing keys, telemetry pseudonym key, database passwords) | only the one response that issues it and the request that carries it; never in any sink |
| `network` | one IPv4 from 198.51.100.0/24 and one IPv6 from 2001:db8::/32 per run, random host part, sent as `X-Forwarded-For` from the loopback trusted proxy | `auth_sessions.last_ip` and API fields the schema documents for it (session lists); never in sinks 1 to 4 (audit holds only `ip_hmac` with `ip_key_id`), never in 5 or 6 |
| `identity` | every principal ID and device ID created in the run | anywhere except sinks 1 to 3 (logs, traces, metrics), where only `bothy.principal.pseudo_id` may appear |

Agent handles are allowed in clear in telemetry (privacy.md "Pseudonyme" item 4); they are not
canaries. Object IDs of threads, channels, messages are allowed in spans and logs (same section).

Matching rules in `scan.go`:

- Canary words are alphanumeric, so JSON and URL escaping do not change them; match
  case-insensitively.
- Also match base64 (standard and URL alphabet, with and without padding) at the three byte
  alignments, and lowercase hex of the raw bytes.
- JWS values: match the payload and signature segments separately; skip the header segment, it is
  not secret and repeats across tokens.
- Secret files: match the raw content, its base64 and hex forms, and for PEM or JWK keys the
  private key material.
- IPv4 dotted form; IPv6 in RFC 5952 compressed form and fully expanded form.
- Principal and device IDs: lowercase and uppercase UUID text and the 32 hex digits without
  hyphens.
- A finding names sink, location (record index plus attribute key, JSON pointer or header name),
  class and label; the test fails with all findings listed.

Scenario (in `scenario.go`, on one harness server with `BuildMatrixWorld` plus canary-bearing
inputs):

1. Every operation ID in the enabled list of `services/server/api/operations.yaml` is called at
   least once, with canary values in every free-text input it accepts. A recorded set of called
   operation IDs is compared with the enabled list; any missing operation fails
   `TestSEC_T10_ScenarioCoversEveryEnabledOperation` and is printed. An exemption needs an entry
   with a reason in `scenario.go`; the expected exemption list is empty.
2. Every client gateway op from asyncapi.yaml that M1 registers is sent at least once.
3. Login and session ways of M1, each with canary inputs: enrollment code (also a wrong code with a
   canary value), local password with TOTP (unknown name and wrong password with canary values),
   OIDC through the Fake-IdP of M1-41 (claims with canary e-mail, name and group values), trusted
   header login of M1-43 (enable it as M1-43's tests do), passkey through the virtual authenticator
   of M1-40, device pairing, refresh rotation including a reuse, step-up with the device key, API
   token creation and use, `Authorization: Bearer bothy_<canary>` and `Authorization: DPoP <canary>`
   with invalid values.
4. Error paths, each with a canary in the request: `400` from SEC-IN-4 (display name with a bidi
   control plus canary), `400` for duplicate JSON keys, `400` for an unknown field whose name is a
   canary (W7), `401`, `403`, `404` for a foreign resource, `409` from a unique conflict (space name
   already taken, so the PostgreSQL detail text would contain the name), `412` on a message edit,
   `413` for a message over `message.max_content_bytes`, `422` for an `Idempotency-Key` reuse with a
   canary body, `428`.
5. Rate limits: the main scenario raises every `ratelimit.<class>.per_minute` and `.burst` through
   `Options.Config` so the run is deterministic. A second, small harness server with default
   configuration triggers `429` on `rl.public_auth` with canary-bearing login bodies
   (`TestSEC_T10_RateLimitedResponse`).
6. Message delete must leave a redacted event; the observers still see nothing of Support.
7. At least one audit row for a request sent with the network canary must carry `ip_hmac` and
   `ip_key_id` (proves the IP path was exercised).

## Tests (write them first)

All Go tests run inside `make test` (no build tag; they need the Docker runtime like every
integration test, P-09). Test names carry the test ID with `_` instead of `-` because Go
identifiers cannot contain `-`; follow an existing convention if `grep -rn "AZ_M0\|SEC_T"
services/server` shows one.

1. `scan_test.go` (unit, no server): `TestSEC_T10_ScanFindsPlainCaseInsensitive`,
   `TestSEC_T10_ScanFindsBase64AllAlignments`, `TestSEC_T10_ScanFindsHex`,
   `TestSEC_T10_ScanSkipsJWSHeaderSegment`, `TestSEC_T10_ScanFindsIPv6Forms`,
   `TestSEC_T10_ScanAllowMatrix` (one table case per class and sink), `TestSEC_T10_FindingNeverPrintsValue`.
2. Detector meta tests on a harness server: `TestSEC_T10_DetectorFindsInjectedLog` (log one canary
   through the server's logger as an attribute; exactly one finding in the log sink with that
   attribute key), `TestSEC_T10_DetectorFindsInjectedSpanAttribute`,
   `TestSEC_T10_DetectorFindsClearPrincipalIDInLog`.
3. `TestSEC_T10_CanaryAcrossAllOutputs`: the full scenario, then `Scan`; zero findings.
4. `TestSEC_T10_ScenarioCoversEveryEnabledOperation`.
5. `TestSEC_T10_RateLimitedResponse`.
6. `TestDP_T09_NoClearPrincipalOrDeviceIDsInTelemetry`: the identity class over sinks 1 to 3,
   plus a positive check that `bothy.principal.pseudo_id` appears for at least one request span
   (proves pseudonyms are emitted, not just IDs missing).
7. For every leak fixed in a module: a unit test in that module that fails before the fix.

Commands that must pass:

```sh
cd services/server && go test -race -count=1 ./test/canary/... ./test/harness/...
make test
make check
```

## Acceptance criteria

- [ ] `go test -race -count=1 ./test/canary/...` passes on the clean state; the red output of the
      meta tests before `scan.go` existed is in the report.
- [ ] An injected log line, span attribute and clear principal ID each make the test fail with a
      finding that names sink and location.
- [ ] Every enabled operation of `operations.yaml` and every M1 client gateway op is exercised;
      the coverage test fails when one is missing.
- [ ] All five classes are covered by the scenario, including the network canary through
      `X-Forwarded-For` and every file of the secrets directory.
- [ ] No canary value in logs (at level debug), traces, metrics, audit rows (except
      `admin_reason` in `details`), error responses or observer frames, pages and snapshots.
- [ ] DP-T09 passes and pseudonyms are present.
- [ ] Every leak found is fixed at its source with a regression test and listed in the report.
- [ ] `make check` is green.

## Definition of done

- Code and tests as above; README in `services/server/test/canary/` and in
  `services/server/test/harness/` (if created); doc comment on every exported symbol (purpose,
  parameters, return value, errors).
- No ADR needed unless you change a production interface beyond the otel seam; if you do, write
  the ADR (MADR 4, `docs/adr/0000-vorlage-madr-4.md`).
- CHANGELOG entry under "Unreleased" in `services/server/CHANGELOG.md`.
- `make check` green.
- Independent review and red team over the diff (`docs/quality/red-team-gate.md`) with verdict
  recorded in the report; merge locally into the base branch only if both are green, then
  `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then commits,
files with line counts, test results with counts (including the red runs), leaks found and fixed
(module, sink, class), what is not proven (at least: collector path, PostgreSQL server logs,
libraries that write to the process file descriptors directly), follow-ups that could not be done
in scope with reason. Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
