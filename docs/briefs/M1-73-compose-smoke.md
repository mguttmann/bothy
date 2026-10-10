# M1-73: Compose smoke test

Status: ready | Depends on: M1-23, M1-37, M1-69 | Etappe: M1

## Goal

`make portability` starts the server core from the real `bothy-server` image with the
installation Compose file plus a test overlay (plan P-06), and proves the path from first setup to
live delivery over WebSocket: secrets, migrations, first owner, enrollment with device keys and
DPoP, space, channel, thread and messages, ordered live delivery, reconnect with catch-up, step-up
and device revocation closing the connection with `4012`. A canary run through the same stack
proves that no content, token or principal ID leaves the collector or the container logs. This is
exit criterion 8 of Etappe M1 and the only end-to-end proof that the image, the Compose contract
and the server modules work together.

## Read first

Plan and quality:
- `docs/PLAN.md`: section "M1 Fundament" (ticket row M1-73, "Nicht-Umfang", "Exit-Kriterien" 8,
  "Red Team" last paragraph "Rauchtest-Stack"), "Arbeitsweise je Ticket", "Teststrategie",
  "Entscheidungen dieses Plans" P-06, P-09, P-10, P-11; ticket row M7-14 (what stays for M7).
- `docs/spec/23-qualitaet.md` "Komponenten-Vertrag" (row "Portabilität": executable
  `infra/compose/smoke.sh`); `docs/quality/gates.md` (release-only `portability`);
  `docs/quality/red-team-gate.md`.
- `Makefile` targets `portability`, `release-build`, `trivy`, `image` (M1-23), `ops-check` (M1-04).

Specification:
- `docs/spec/16-betrieb.md`: "Grundregeln" and "Zwei Listener" (4.1, 4.2), "Konfiguration",
  "Secret-Provider", "Geheimnisse im Compose-Stack" (5.1 to 5.3), "Umgebungen" (6.1, use
  `staging`), "Rollen" (7.1), "Upgrade" point 1 (an empty database needs no recent backup),
  "Portabilitätstest in CI" (11.1, the full test is M7-14), "Konfigurationswerte dieses Kapitels".
- `docs/spec/08-identitaet.md`: "Einladungs-Code" (3.2), "Erstinstallation" (4), "Registrierung"
  (5.1), "Zwei Schlüssel je nativem Gerät" (6.1), "Step-up-Ablauf" (6.2), "Zugriffstoken" (7.2),
  "DPoP" (7.3), "Tests" (ID-T06, ID-T07).
- `docs/spec/04-sync-protokoll.md`: "Umschlag (Envelope)" (2.2), "Rahmen auf dem WebSocket" (2.3),
  "Nachrichten: `client_msg_id`" (5.1), "Alle anderen Schreibvorgänge: `Idempotency-Key`" (5.2),
  "Endpunkt und Version", "Ops", "Lebenszyklus", "Schließ-Codes" (6.1 to 6.4), "Nachholen" (7.1 to
  7.4), "Widerruf von Sitzung, Gerät oder Prinzipal" (8.5), sequence diagrams "Nachricht senden,
  online" (16.1) and "Wiederverbinden mit Nachholen" (16.3).
- `docs/spec/05-aktionen.md`: action rows `instance.discovery.read`, `auth.login.begin`,
  `auth.login.complete`, `auth.step_up`, `self.read`, `space.create`, `channel.create`,
  `thread.create`, `message.create`, `device.revoke` (step-up required), `events.catch_up`,
  `gateway.connect`; section "Dauerhafte Ereignistypen und Empfangs-Aktion" rows
  `space.created`, `channel.created`, `thread.created`, `message.created`, `session.revoked`;
  section "WebSocket-Ops und Aktion".
- `docs/spec/06-api.md`: "DPoP" (3.1, nonce retry exactly once), "Fehler", "Idempotenz".
- `docs/spec/02-architektur.md`: "Start" (15.1), "Health" (15.2), "Shutdown" (15.3).
- `docs/spec/17-observability.md`: "Datenschutz" (4), "Collector und Backends" (5), "Tests, die
  diese Regeln halten" (10), R-OB-02, R-OB-03.
- `docs/spec/19-sicherheit.md`: SEC-TLS-1, SEC-TLS-5, SEC-CORS-3, SEC-LOG-1 to SEC-LOG-6, SEC-T10.
- `docs/spec/20-datenschutz.md`: DP-T09.
- `docs/observability/privacy.md` section 6 "Kanarien-Test (Gate)";
  `docs/observability/collector.md`.
- ADRs 0012 (images, Compose), 0020 (device binding, DPoP, step-up), 0034 (collector),
  0038 (local gates), 0039 (release class), 0053 (no JSON null).

API (`docs/api/openapi.yaml`, server prefix `/api/v1`) by operationId: `getDiscovery`,
`beginEnrollment`, `completeEnrollment` (as amended by M1-37 for enrollment with device keys
without passkey, plan W5), `getMe`, `createSpace`, `createChannel`, `createThread`,
`createMessage`, `listEvents`, `createStepUpChallenge`, `completeStepUpWithDeviceKey`,
`revokeDevice`. Schemas: `DeviceRegistration`, `DeviceKeyRegistration`, `TokenResponse`,
`Channel` (`main_thread_id`), `ThreadCreate` (thread from a main-thread message via
`parent_message_id`), `MessageCreate`, `StepUpChallenge`, `StepUpDeviceKeyRequest`, `ReasonBody`,
`Discovery` (`setup_required`). Gateway (`docs/api/asyncapi.yaml`, address `/gateway`): ops
`hello`, `identify`, `ready`, `heartbeat`, `heartbeat_ack`, `dispatch`, `cursor`,
`catchup.complete`, `resync_required`; schemas `GatewayHello`, `GatewayIdentify`, `GatewayReady`,
`GatewayCursor`, `GatewayCatchupComplete`.

Data tables: none are written directly; state is checked through the API only.

Existing files and outputs of the dependencies:
- `infra/compose/compose.yaml`, `infra/compose/README.md`, `infra/compose/.env.example`.
- `infra/otel/collector.yaml` (forward connectors, `tail_sampling`, `batch`, redaction
  allowlist and `blocked_values`) and `infra/otel/exporters/*.yaml`.
- `docs/operations/configuration.md` sections 4 and 5 (key catalogue, secrets catalogue).
- `services/server/Dockerfile`, `.dockerignore` and the `image` target (M1-23).
- `services/server/internal/identity/README.md` and `bothy setup --help` (M1-37: output of
  `setup bootstrap-owner`, the database role it uses, whether a running server needs a restart
  after setup; M1-35: step-up JWS format; M1-33: DPoP rules and the JOSE library in `go.mod`).
- `services/server/internal/platform/otel/README.md` (M1-10: `service.name`, log exporter).
- `services/server/internal/platform/pg/pgtest` (M1-12: the PostgreSQL 18 image reference).
- `services/server/test/canary` (M1-69) and `architecture/modules.yaml` (M1-05: how test
  harness packages are registered).
- `tools/opscheck` (M1-04: list of collector configurations it validates).

## Fixed requirements that apply

Owner build rules (verbatim):

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

Rules for every brief:

6. Test first: every behaviour has a test that was red before the code (record the red output).
7. No en or em dashes in any text (`grep -nP "[\x{2013}\x{2014}]" <file>` is empty). White label:
   people are "Alex", domains `example.com`; no company, host or personal names, no paths of a
   personal machine. SPDX header `Apache-2.0` in every new source, script and YAML file.
8. Server configuration only through named keys from `docs/operations/configuration.md` (naming
   rule `a.b_c` to `BOTHY_A__B_C`, 16 "Konfiguration"). Variables of the smoke harness are not
   server configuration; they start with `SMOKE_` and are documented in
   `infra/compose/test/README.md`.

Product requirements implemented here:

9. The smoke test uses the real server image, the installation Compose file unchanged and a test
   overlay with the official PostgreSQL 18 image pinned by digest and without the `backup`
   service (P-06).
10. Missing image, Docker runtime, too old Docker Engine or too old Compose fail with exit code 2
    and a message that names the fix; nothing is skipped (P-09; plan ticket row "ohne Image klarer
    Fehler").
11. No host port is published and nothing binds outside `127.0.0.1` (P-10; 16 4.1).
12. Plaintext HTTP reaches the server only from loopback (SEC-TLS-1); WebSocket clients send no
    foreign `Origin` (SEC-CORS-3).
13. Setup path: `503 setup_required` before setup (ID-T06), first owner only via
    `bothy setup bootstrap-owner`, a second run aborts without change (ID-T07), the enrollment code
    confirms the first device (08 3.2, 4).
14. DPoP on every request with the server nonce and one retry on `use_dpop_nonce` (06 3.1, 08 7.3);
    on the gateway the proof sits in `identify` with the nonce from `hello` (04 6.3).
15. Live delivery in `seq` order with `client_msg_id` echo; reconnect with `identify`, `cursor` and
    `sync_epoch` delivers the missed events in order, then `catchup.complete` (04 7.2, 16.3).
16. `device.revoke` needs step-up and closes the device's connection with `4012` (05, 04 6.4, 8.5).
17. Canary: one random secret word per run in every content field; neither it, nor any token, nor
    the enrollment code, nor a principal ID appears in collector exports or container logs
    (privacy.md section 6; SEC-T10; DP-T09; R-OB-03).

## Scope

All paths are repository-relative. Go paths below `services/server/` follow the module layout of
M1-05.

1. `infra/compose/smoke.sh` (new, executable, bash, `set -euo pipefail`, SPDX header). Each step
   is announced as `smoke: <step>` on stderr; on failure it prints the step, `compose ps --all` and
   the last `SMOKE_LOG_TAIL` lines of `api`, `worker`, `migrate`, `otel-collector`, then exits 1:
   - **Inputs.** `SMOKE_SERVER_IMAGE` (default: the local tag that `make image` writes, taken from
     the `Makefile` variable M1-23 defines), `SMOKE_KEEP` (`1` keeps the stack and the temp dir and
     prints how to remove them), `SMOKE_WAIT_TIMEOUT_S`, `SMOKE_TELEMETRY_SETTLE_S`,
     `SMOKE_LOG_TAIL`. Defaults live only in `smoke.sh`.
   - **Sanitise.** After reading the inputs, unset every exported variable whose name starts with
     `BOTHY_`, `COMPOSE_` or `OTEL_`, so a developer's shell cannot change the stack; always pass
     `--env-file infra/compose/test/smoke.env`, so `infra/compose/.env` of a real installation is
     never read.
   - **Preflight** (exit 2 each): `docker` on `PATH`; `docker info` reachable; Docker Engine at
     least 26.0.0 (`docker version --format '{{.Server.Version}}'`, needed for volume `subpath`);
     Compose at least 2.26.0 (`docker compose version --short`, strip a leading `v`, compare
     numerically; needed for `!reset`, `!override`, `configs`); the image exists locally
     (`docker image inspect`), else
     `smoke: server image '<ref>' not found locally; build it with 'make image' or pull it, then set SMOKE_SERVER_IMAGE`.
     No implicit pull or build of the server image.
   - **Workspace.** `tmp=$(mktemp -d)`; subdirectories `secrets-input` (empty, mode 0700: no
     owner-supplied secrets), `out`, `otel`, `logs`; run ID of 8 random hex characters; project name
     `bothy-smoke-<run ID>`; run canary `smk` plus 24 random lowercase hex characters from
     `/dev/urandom`; `trap cleanup EXIT`. Export `SMOKE_RUN_ID`, `SMOKE_SECRETS_INPUT`,
     `SMOKE_OUT` (`$tmp/out`), `SMOKE_TMP`, `SMOKE_CANARY` for the overlay. All compose calls go
     through one function: `docker compose --project-name "$project" --project-directory
     infra/compose --file infra/compose/compose.yaml --file infra/compose/test/compose.smoke.yaml
     --env-file infra/compose/test/smoke.env "$@"` (paths resolved from the script's own
     directory).
   - **Validate.** `compose config --quiet`.
   - **Build and pull.** `compose build smoke-probe` (the image is shared with `smoke-scan`);
     `compose pull --quiet postgres otel-collector otel-out-init`.
   - **Start.** `compose up --detach --wait --wait-timeout "$SMOKE_WAIT_TIMEOUT_S" api worker
     otel-collector`; dependencies run `secrets-init` (`bothy secrets init`), `postgres`,
     `migrate` (`bothy migrate up --bootstrap`) and `otel-out-init`. Then assert exit code 0 of
     the one-shot containers `secrets-init` and `migrate` (`docker inspect --format
     '{{.State.ExitCode}}'` on `compose ps --all --quiet <service>`).
   - **Idempotence.** `compose run --rm --no-deps secrets-init` and `compose run --rm --no-deps
     migrate` exit 0 a second time.
   - **Precheck.** `compose run --rm --no-deps --user "$(id -u):$(id -g)" smoke-probe precheck`.
   - **Bootstrap.** `compose run --rm --no-deps -T bothy-cli setup bootstrap-owner --handle alex
     --display-name "Alex <canary>"`, stdout to `$tmp/out/setup.out` (mode 0600, never printed,
     never scanned), stderr to `$tmp/logs/setup.err` (scanned). A second identical call must exit
     non-zero; its stdout goes to `$tmp/out/setup-second.out`, its stderr is appended to
     `setup.err`. If the M1-37 documentation says a running server picks up setup only after a
     restart, `compose restart api worker` and wait until healthy; otherwise nothing.
   - **Probe.** `compose run --rm --no-deps --user "$(id -u):$(id -g)" smoke-probe run` (see 5).
     Its stdout and stderr are shown; it never prints values.
   - **Flush.** `compose stop api worker` (graceful shutdown flushes telemetry, 02 15.3), wait
     `SMOKE_TELEMETRY_SETTLE_S`, then `compose stop otel-collector`.
   - **Collect.** `compose logs --no-color --timestamps secrets-init postgres migrate otel-out-init
     api worker otel-collector > $tmp/logs/compose.log`;
     `compose cp otel-collector:/var/lib/otelcol-smoke/. "$tmp/otel/"`.
   - **Scan.** `compose run --rm --no-deps --user "$(id -u):$(id -g)" smoke-scan scan` (see 5).
   - **Cleanup** (trap): unless `SMOKE_KEEP=1`, `compose down --volumes --remove-orphans --rmi
     local`, `docker image rm` of the custom-tagged probe image, then remove `$tmp` (guard: the
     variable is non-empty and names the directory `mktemp -d` created). Afterwards no container,
     volume or network with the project label remains (`docker ps -a --filter
     label=com.docker.compose.project=<project>` is empty; assert it).
   - Exit codes: 0 pass, 1 smoke failure, 2 precondition missing.
2. `infra/compose/test/compose.smoke.yaml` (new overlay; header comment: test only, never for an
   installation, plan P-06). Relative paths resolve against `infra/compose` (the project
   directory). Contents:
   - An anchor `x-smoke-env` with the installation's `x-server-env` minus the keys the M1 server
     rejects (expected: every `BOTHY_FILE__*` key; files arrive in M3). Determine the set by
     running `docker run --rm <image> config check` with the installation values; list every
     dropped key in a comment with the etappe that introduces it.
   - `secrets-init`, `migrate`, `api`, `worker`: `image: ${SMOKE_SERVER_IMAGE:?set by smoke.sh}`,
     `pull_policy: never`; `environment: !override` built from `x-smoke-env` plus the service's own
     installation overrides (`migrate`: `BOTHY_DATABASE__USER`, `BOTHY_DATABASE__PASSWORD_REF`);
     `api` additionally `BOTHY_SERVER__LISTEN_ADDR: "127.0.0.1:8080"` and `ports: !reset []`.
   - `secrets-init`: `volumes: !override` with `${SMOKE_SECRETS_INPUT:?set by smoke.sh}:/input:ro`
     and the `secrets` and `signing-keys` volumes as in the installation. `api` and `worker` keep
     the installation's `signing-keys` volume (ADR 0066: the server writes its signing keys there).
   - `postgres`: `image:` exactly the reference that `internal/platform/pg/pgtest` uses (M1-12,
     official `postgres:18.x` multi-arch index digest); everything else from the installation.
   - `backup`: `profiles: [smoke-disabled]` (never activated).
   - `otel-collector`: `command: !override` with `--config=/etc/otelcol/collector.yaml` and
     `--config=/etc/otelcol-smoke/file.yaml`; additional volumes `./test/otel:/etc/otelcol-smoke:ro`
     and `otel-smoke-out:/var/lib/otelcol-smoke`; `ports: !reset []`; additional `depends_on`
     `otel-out-init: {condition: service_completed_successfully}`. The image's own user stays.
   - `otel-out-init`: the same pinned PostgreSQL image (already pulled, has a shell),
     `user: "0:0"`, `network_mode: none`, `restart: "no"`, `entrypoint: ["/bin/sh", "-c"]`,
     `command: ["chmod 1777 /out"]`, volume `otel-smoke-out:/out`.
   - `bothy-cli`: profile `smoke`, `image: ${SMOKE_SERVER_IMAGE}`, `pull_policy: never`, hardening
     as `migrate` (`read_only`, `cap_drop: [ALL]`, `no-new-privileges`), network `internal`, and
     the database role, environment and secrets subpath that M1-37 documents for `bothy setup`.
   - `smoke-probe`: profile `smoke`, `build: {context: ../../services/server, dockerfile:
     Dockerfile, target: smoke}`, `image: bothy-smoke:${SMOKE_RUN_ID:?set by smoke.sh}`,
     `network_mode: service:api`, `read_only: true`, `cap_drop: [ALL]`,
     `security_opt: ["no-new-privileges:true"]`, environment `SMOKE_PUBLIC_URL: ${BOTHY_PUBLIC_URL}`
     and `SMOKE_CANARY` (passed through, no value in the file), volume `${SMOKE_OUT:?}:/out`.
   - `smoke-scan`: profile `smoke`, the same `image`, `pull_policy: never`, `network_mode: none`,
     the same hardening, volume `${SMOKE_TMP:?}:/in:ro`.
   - `volumes: {otel-smoke-out: {}}`.
3. `infra/compose/test/smoke.env` (new; no secrets): `COMPOSE_PROFILES=direct,smoke`,
   `BOTHY_REGISTRY=registry.invalid` and `BOTHY_VERSION=smoke` (only to satisfy the `:?` checks of
   the installation file; every server-image service is overridden),
   `BOTHY_PUBLIC_URL=https://chat.example.com`, `BOTHY_ENVIRONMENT=staging`,
   `BOTHY_OTEL_EXPORTER=none`, `BOTHY_OTEL_EXPORTER_2=none`, `BOTHY_TRACE_SAMPLE_PERCENT=100`,
   `BOTHY_OTEL_METRIC_EXPORT_INTERVAL_MS=1000`. These are Compose interpolation variables of the
   installation file, not server keys.
4. `infra/compose/test/otel/file.yaml` (new collector overlay "file", test only): exporters
   `file/traces`, `file/metrics`, `file/logs` (`format: json`, `flush_interval: 1s`, paths
   `/var/lib/otelcol-smoke/{traces,metrics,logs}.jsonl`, no rotation, no compression); pipelines
   `traces/smoke`, `metrics/smoke`, `logs/smoke` fed from `forward/traces`, `forward/metrics`,
   `forward/logs`; processor overrides `tail_sampling.decision_wait: 1s` and `batch.timeout: 1s`
   (map keys merge into the base; the policies stay). Add this file to the configurations that
   `tools/opscheck` validates with `otelcol-contrib validate` together with the base, plus a
   negative fixture in the style of M1-04.
5. Go package `services/server/test/smoke` (new, package `smoke`, `doc.go` with the package doc)
   and command `services/server/test/smoke/cmd/bothy-smoke` (`main.go`). The command has three
   subcommands; flags with defaults: `--base-url http://127.0.0.1:8080`,
   `--public-url $SMOKE_PUBLIC_URL`, `--out /out`, `--in /in`.
   - `precheck`: `getDiscovery` returns 200 with `setup_required: true`; `GET /api/v1/me` without
     token returns `503` with problem type `setup_required`.
   - `run`, in this order (each step logs `ok <step>` to stderr, never a value):
     1. `getDiscovery`: `setup_required` is not `true`.
     2. Read the enrollment code from `/out/setup.out` (see Design notes "Enrollment code").
     3. Generate two ECDSA P-256 keys in memory: `dpop` and `step_up`; `device_id` a fresh UUIDv7;
        device `name` `"smoke <canary>"`, `platform` `cli`, `alg` `ES256`. `beginEnrollment` and
        `completeEnrollment` with the device registration and a DPoP proof without `ath`; expect
        `201`, `token_type: DPoP`, `device.state: active`. Keep access token, refresh token,
        `session.id`, `session.user_id`, `device.id`.
     4. `getMe`: handle `alex`.
     5. Gateway connection A: dial `ws://127.0.0.1:8080/api/v1/gateway` without `Origin`; expect
        `hello`; send `identify` (`protocol_version` from `hello.protocol_versions`,
        `access_token`, `dpop`, `device_id`, `client`); expect `ready`; keep `sync_epoch`; run the
        heartbeat loop per `heartbeat_interval_ms` with a random first delay (04 6.3 step 5).
     6. `createSpace` (`name` `"Research <canary>"`), `createChannel` (`kind: text`, `name`
        `"smoke-<canary>"`), `createMessage` into the channel's `main_thread_id`, `createThread`
        with `parent_message_id` of that message and `title` `"<canary> thread"`, then
        `liveMessages` messages in the new thread (`content_md` `"<canary> message <i>"`), each
        with a fresh UUIDv7 `client_msg_id`. Every other POST carries an `Idempotency-Key`
        (UUIDv7).
     7. On connection A expect, as an ordered subsequence of `dispatch` frames:
        `space.created`, `channel.created`, `thread.created`, then the `message.created` events of
        the new thread in send order with matching `client_msg_id` and `resource.id`; every
        dispatched `seq` is strictly greater than the previous one.
     8. Close A with `1000`. Cursor = highest `seq` seen in `dispatch` or `cursor` frames.
     9. Send `awayMessages` messages over REST while disconnected.
     10. `listEvents?after=<cursor>`, following `next_cursor` until `head_seq`: contains exactly
         these `message.created` events of the thread in send order (REST variant, 04 7.4).
     11. Gateway connection B: `hello`, `identify` with `cursor` and `sync_epoch`, `ready`; expect
         the missed `message.created` events in send order, none with `seq <= cursor`, then
         `catchup.complete` with `head_seq` at least the last seen `seq`; then send one live
         message over REST and expect its dispatch after `catchup.complete`.
     12. `createStepUpChallenge`; sign the step-up JWS over `{challenge, sid, iat}` with the
         `step_up` key in the format M1-35 documents; `completeStepUpWithDeviceKey` returns 200.
     13. `revokeDevice` for the own device (`reason` `"smoke <canary>"`, `Idempotency-Key`):
         200; the server closes connection B with code `4012` within `revokeCloseWait`; no
         `dispatch` arrives after the revocation response.
     14. `getMe` with the old access token and a fresh proof returns `401`.
     15. Write `/out/canaries.txt` (mode 0600), one `label<TAB>value` per line: `canary`,
         `access_token`, `refresh_token`, `enrollment_code`, `principal_id` (the user ID).
   - `scan`: reads `/in/out/canaries.txt` and every regular file below `/in/otel` and `/in/logs`
     (nothing else below `/in/out`). Fails (exit 1) when:
     - the canary list is empty, lacks the label `canary`, or a value is shorter than
       `minCanaryLen`;
     - the positive control fails: before scanning, the scanner must find the first canary in a
       built-in control text, so a broken scanner cannot pass;
     - any value occurs in any scanned file; each hit is reported as `<file>:<line>: <label>`,
       never with the value;
     - the OTLP JSON in `/in/otel` lacks, for the server's `service.name` (M1-10), at least one
       span whose `http.route` is the route pattern of `createMessage`, at least one log record
       and at least one metric data point (an empty export cannot pass).
   - Package layout (every exported symbol with a doc comment):
     - `dpop.go`: `type Key` (ES256 private key with `PublicJWK()` and `Thumbprint()` per
       RFC 7638), `func NewProof(k Key, method, htu, nonce, accessToken string, now time.Time)
       (string, error)` (header `typ: dpop+jwt`, `alg: ES256`, public `jwk` only; claims `jti`
       random, `htm`, `htu` without query and fragment, `iat`, `nonce` when set, `ath` =
       base64url SHA-256 of the token when set). Use the JOSE library M1-33 already pinned in
       `go.mod`; add no new JOSE dependency.
     - `htu.go`: `func GatewayHTU(publicURL string) (string, error)`: the HTTPS form of
       `<public URL>/api/v1/gateway` (04 6.3); `func RESTHTU(publicURL, path string) (string,
       error)`. The server checks `htu` against its public URL (08 7.3), not against the loopback
       address the probe dials; follow the rule M1-33 documents.
     - `rest.go`: `type REST` interface with one method per operation used above, request and
       response types from the generated package `api/rest` (M1-17), no hand-written JSON
       structs; `type HTTPClient` implements it with `net/http` (dials only `--base-url`), DPoP on
       every request, `DPoP-Nonce` tracking, exactly one retry on `401 use_dpop_nonce`.
     - `gateway.go`: `type Gateway` interface (`Dial`, `Identify`, `Next(ctx) (Frame, error)`,
       `Close(code)`, `CloseCode()`), implementation with `github.com/coder/websocket`.
     - `scenario.go`: `func Precheck(ctx, REST) error`, `func Run(ctx, Config, REST, func()
       Gateway) (Canaries, error)` with the steps above; constants `liveMessages`,
       `awayMessages`, `revokeCloseWait`, `frameWait` with doc comments that give the reason.
     - `scan.go`: `func Scan(canaries Canaries, roots []string) ([]Hit, error)`,
       `func CheckTelemetry(otelDir, serviceName, route string) error`.
     - `enroll.go`: `func ReadEnrollmentCode(stdout []byte) (string, error)`.
   - Register the package in `architecture/modules.yaml` as a test harness the way M1-69
     registered `test/canary`, allowing exactly `api/rest`, `github.com/coder/websocket` and the
     JOSE library; regenerate `.go-arch-lint.yml` with `tools/archgen`; no import of any
     `internal/...` package.
6. `services/server/Dockerfile` (change): two stages before the production stage, which stays
   the last stage and unchanged: `smoke-build` (from the existing builder stage, builds
   `./test/smoke/cmd/bothy-smoke` with `CGO_ENABLED=0` and `-trimpath`) and `smoke` (from the same
   runtime base reference as production, same non-root user,
   `ENTRYPOINT ["/usr/local/bin/bothy-smoke"]`). If `.dockerignore` excludes `test/`, re-include
   only `test/smoke/`.
7. `tools/ci/tests/test_compose_smoke.py` (new) with the tests in "Tests" below.
8. `Makefile`: `release-build` excludes `infra/compose/test` from the bundle (`git archive`
   pathspec `:(exclude)infra/compose/test`); the help text of `portability` names
   `SMOKE_SERVER_IMAGE`.
9. `renovate.json`: put `infra/compose/test/compose.smoke.yaml` into the same group as the
   `pgtest` image pin (create the group "PostgreSQL test image" if M1-12 did not), so both pins move
   in one pull request; `make lint` validates the file.
10. Documentation:
    - `infra/compose/test/README.md` (new, German like `infra/compose/README.md`): purpose and
      P-06, prerequisites (Docker Engine 26, Compose 2.26, the image from `make image`), the
      `SMOKE_*` variables, the steps, what is proven and what is not (see "Out of scope"), how to
      debug with `SMOKE_KEEP=1`, why the probe shares the network namespace of `api`.
    - `infra/compose/README.md`: table "Dateien" gets `test/` and `smoke.sh`; section "Geprüft"
      states what the smoke test now proves (server part of the stack) and what still waits for M7.
    - `docs/observability/collector.md`: the test overlay `file` under `infra/compose/test/otel/`
      exists only for the smoke test; `docs/observability/privacy.md` section 6: name the
      implementation (`infra/compose/smoke.sh`, overlay `file`) and that the probe's own output is
      not scanned.
    - `docs/security/threat-model.md` section 5.14 "Betrieb und Sicherungen": one row for the smoke
      stack (threat: the test overlay or its settings end up in an installation; countermeasures:
      overlay only under `infra/compose/test`, excluded from `release-build`, own project name, no
      published ports, `staging` environment), and a register entry in 6.5 with a new ID following
      the register's scheme and the test `test_release_bundle_excludes_smoke_overlay`.
    - `docs/DECISIONS.md`: rulings for design notes D1, D2, D4, D6 and D7 (next free IDs, existing
      format).
    - `CHANGELOG.md` (root, `[Unreleased]`, `### Added`): the smoke test of the Compose stack.

## Out of scope

- The full portability test (profiles `scan` and `s3`, files and EICAR, search, status page,
  backup and restore test, amd64 and arm64 runners, `portability.yml` wiring with released images
  and its failure-log step): M7-14.
- Making the installation stack startable (`bothy-postgres`, `backup`, `bothy-backupd`) and the
  bootstrap instructions in `docs/operations/install-compose.md`: M7.
- The in-process canary harness for every server path, error bodies and foreign events: M1-69.
- Agents, runs, adapters, uploads (M2, M3). Load and chaos tests (M1-72).
- Changing `infra/compose/compose.yaml`. Exception: a value or command there contradicts a contract
  already merged in M1 (renamed key, renamed subcommand, secrets layout); then fix it in
  `compose.yaml`, chapter 16 and `infra/compose/README.md` in the same change (specification
  first) instead of masking it in the overlay.
- Server behaviour. Exception: the scan finds a real leak; fix it at the source with the smallest
  change, add the case to `test/canary` (M1-69), and name it in the report. If the fix needs a
  specification decision, stop with `BLOCKED`.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-73-compose-smoke`.
- Conventional Commits with DCO sign-off (`git commit -s`), scopes `compose`, `server`, `ci` as
  fitting; squash to meaningful commits before the merge.
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope list except the documentation and changelog entries named
  there.
- Follow the host rules supplied by the launcher. Stop only processes and Compose projects you
  started (by PID, or by your own `bothy-smoke-*` project name); never touch other containers,
  volumes, networks or Compose projects on the host. Temporary directories only via `mktemp -d`,
  removed at the end.
- Never read or print secrets: the `secrets` volume, `setup.out` and `canaries.txt` are never
  printed; the report names labels, never values.
- No network calls except package and image downloads of pinned versions (Go modules, the pinned
  PostgreSQL, collector and builder images).

## Design notes

- **D1 Probe in the network namespace of `api`.** Inside the container the server sees a
  published port's traffic from the bridge gateway, not from loopback; SEC-TLS-1 would reject it,
  and widening `server.trusted_proxies` for a test would weaken the rule under test. The probe
  shares the namespace (`network_mode: service:api`) and dials `127.0.0.1:8080`, like the
  `tailscale` sidecar pattern. No host port is published (P-10).
- **D2 Overlay changes are removals or the listed overrides only.** Keys of later etappen are
  dropped; a value the M1 server rejects (for example `database.sslmode=disable` under SEC-TLS-5)
  is not changed in the overlay: stop with `BLOCKED` and quote the `config check` message, because
  it needs a specification decision.
- **D3 Isolation of each run.** Own project name, explicit `--env-file`, sanitised environment,
  empty owner-secrets input directory: the smoke test can neither read nor change a real
  installation on the same host, and parallel worktrees do not collide.
- **D4 Collector overlay `file`.** It lives under `infra/compose/test/otel/` because it needs the
  writable volume of the overlay and has no use in an installation. `tail_sampling` and `batch`
  are shortened there and `BOTHY_TRACE_SAMPLE_PERCENT=100`, so every trace of the run is exported
  and the wait after the flush stays short. The collector keeps the image's own user; the one-shot
  `otel-out-init` prepares the output directory, so a permission defect of the collector's secrets
  is not hidden by running it as root.
- **D5 Canary format.** `smk` plus lowercase hex: JSON-safe (no escaping can hide it) and matching
  none of the collector's `blocked_values` patterns, so a leak shows up as plain text instead of
  being replaced by an HMAC. Principal IDs are canaries too (R-OB-03, privacy.md section 6 item 5).
- **D6 A passing scan must prove something.** Positive control in the scanner and the telemetry
  presence check (span of the `createMessage` route, a log record, a metric point).
  `SMOKE_TELEMETRY_SETTLE_S` must be at least `decision_wait` plus `batch.timeout` plus
  `flush_interval` from `file.yaml`; a test holds this.
- **D7 Probe image as a Dockerfile target.** Same builder and runtime base as the server image, so
  no second pin can drift; the production stage stays last, so default builds and releases are
  unchanged. Only Docker is needed on the host (16 11.1).
- **D8 Telemetry flush.** Graceful stop of `api` and `worker` flushes the SDK (02 15.3); then the
  settle wait; then a graceful stop of the collector; then `compose cp` (works on stopped
  containers with named volumes, unlike tmpfs).
- **Readiness before setup.** 08 4 says health checks answer before setup. If M1-11 or M1-37
  document that `/readyz` stays not ready until setup, start with `--wait` only on `migrate`
  completion and `api` running, and require `healthy` after the bootstrap step; do not change the
  server for the test.
- **Enrollment code.** M1-37 writes the code once to the console (08 4 step 4). Read it as M1-37
  documents (expected: the only stdout line, logs on stderr). If no stable machine-readable form
  exists, add `--output json` (`{"enrollment_code": "...", "expires_at": "..."}`) to
  `bothy setup bootstrap-owner` and `bothy setup issue-enrollment-code`, specification first
  (08 "Erstinstallation", 02 command table), with unit tests, in this ticket.
- **Setup lock.** Before setup every operation except Discovery answers `503 setup_required`
  (08 4); the probe checks this through the public listener, so ID-T06 is proven end to end.
- **IDs and keys.** `device_id`, `client_msg_id` and `Idempotency-Key` are UUIDv7 (RFC 9562).
  The probe never sends JSON `null` (ADR 0053).
- **Timeouts.** Every wait in the probe has a context deadline from a named constant; every wait in
  `smoke.sh` uses `SMOKE_WAIT_TIMEOUT_S`; nothing waits unbounded.
- **Language.** Code, comments, `smoke:` messages and the Go package docs are English;
  `infra/compose/test/README.md` and the German docs stay German.

## Tests (write them first)

Go unit tests in `services/server/test/smoke` (run with `go test -race ./test/smoke/...`; only
`127.0.0.1` and port 0 through `httptest`):
- `TestNewProofClaims`: header and claims as specified; `jti` differs between two proofs; `ath`
  present only with a token; no private JWK member.
- `TestNewProofVerifiesWithPublicKey`: the proof verifies with the public JWK.
- `TestThumbprintRFC7638`: the thumbprint equals SHA-256 over the canonical JSON
  `{"crv","kty","x","y"}` computed independently in the test.
- `TestGatewayHTU` (table: `https://chat.example.com` gives
  `https://chat.example.com/api/v1/gateway`; trailing slash; a path prefix; a query is rejected).
- `TestHTTPClientRetriesOnceOnUseDPoPNonce`, `TestHTTPClientStopsAfterSecondNonceError`,
  `TestHTTPClientTracksLatestNonce`.
- `TestPrecheck` (setup required `true` passes; `false` fails; `/me` not 503 fails).
- `TestRunHappyPath` with fake `REST` and `Gateway`.
- `TestRunFailsOnOutOfOrderDispatch`, `TestRunFailsOnMissingClientMsgIDEcho`,
  `TestRunFailsOnEventAtOrBelowCursor`, `TestRunFailsWithoutCatchupComplete`,
  `TestRunFailsWhenLiveEventArrivesBeforeCatchupComplete`, `TestRunFailsWithoutClose4012`,
  `TestRunFailsOnDispatchAfterRevocation`, `TestRunFailsWhenOldTokenStillWorks`.
- `TestScanFindsCanaryAndReportsLabelNotValue`, `TestScanPositiveControl`,
  `TestScanRejectsShortOrMissingCanaries`, `TestScanIgnoresProbeOutputDir`.
- `TestCheckTelemetry` (fixtures of OTLP JSON lines: complete passes; no span of the route, no log
  record, no metric point, another `service.name` each fail).
- `TestReadEnrollmentCode` (documented format passes; empty output, several candidates, log noise
  on stdout fail).

Python tests in `tools/ci/tests/test_compose_smoke.py` (`cd tools && uv run --locked pytest
ci/tests/test_compose_smoke.py -q`):
- `test_missing_image_fails_with_hint`: a fake `docker` on `PATH` (in `tmp_path`) answers `info`,
  `version` and `compose version` and fails `image inspect`; `smoke.sh` exits 2, stderr names the
  image and `make image`, and the fake saw no `compose up`.
- `test_missing_docker_fails`, `test_old_compose_fails`, `test_old_engine_fails` (exit 2 each).
- `test_overlay_server_services_use_smoke_image` (`image` and `pull_policy: never`).
- `test_overlay_postgres_pin_equals_pgtest` (same `postgres:<tag>@sha256:<64 hex>` as
  `internal/platform/pg/pgtest`).
- `test_overlay_publishes_no_ports` (`api` and `otel-collector` reset `ports`; no other service
  adds ports).
- `test_overlay_env_only_removes_keys` (each overlay environment equals the installation's
  `x-server-env` plus the service's installation overrides, minus the documented dropped keys,
  plus only `BOTHY_SERVER__LISTEN_ADDR` for `api`).
- `test_backup_is_never_started` (its profile is not in `COMPOSE_PROFILES` of `smoke.env`).
- `test_smoke_env_has_no_secrets_and_staging`.
- `test_settle_covers_collector_waits` (D6).
- `test_dockerfile_production_stage_last_and_smoke_target_exists`.
- `test_release_bundle_excludes_smoke_overlay`: `make release-build VERSION=<content of VERSION>
  OUT=<tmp_path>`; the archive has no member below `infra/compose/test/`; the test first asserts
  that `infra/compose/test/compose.smoke.yaml` is tracked, so it cannot pass vacuously.
- YAML with `!reset` and `!override` is loaded with a loader that registers both tags.

Commands that must pass:
- `cd services/server && go test -race ./test/smoke/...`
- `cd tools && uv run --locked pytest ci/tests/test_compose_smoke.py -q`
- `make ops-check` (collector overlay `file` validated), `make trivy` (Dockerfile), `make lint`,
  `make arch`, `make codegen-check`
- `docker compose --project-directory infra/compose -f infra/compose/compose.yaml -f
  infra/compose/test/compose.smoke.yaml --env-file infra/compose/test/smoke.env config --quiet`
  (with every `SMOKE_*` variable the overlay interpolates set, paths inside a `mktemp -d`
  directory)
- `make image`, then `make portability` (green; record its full `smoke:` step log in the report),
  and once `SMOKE_SERVER_IMAGE=bothy-smoke-missing:none make portability` (exit 2 with the hint)
- `make check`

## Acceptance criteria

- [ ] `make portability` is green with the image from `make image` and runs, in order: secrets
      init, `migrate up --bootstrap`, idempotent second runs, precheck `503 setup_required`,
      `setup bootstrap-owner` (second run refused), enrollment with device keys and DPoP, space,
      channel, thread and messages, ordered live delivery, REST catch-up, reconnect with catch-up
      and `catchup.complete`, step-up, `revokeDevice` closing the connection with `4012`, old token
      `401`, canary scan.
- [ ] Without the image, Docker, Engine 26 or Compose 2.26, `make portability` exits 2 with a
      message that names the fix; nothing is pulled or built for the server image.
- [ ] The scan finds no canary, token, enrollment code or principal ID in the collector export or
      the container logs; its positive control and the telemetry presence check pass.
- [ ] No host port is published; the server's public listener is `127.0.0.1:8080` inside its
      namespace; the probe sends no `Origin`.
- [ ] After the run no container, volume, network or image of the project remains (unless
      `SMOKE_KEEP=1`), and the temp directory is gone.
- [ ] `infra/compose/compose.yaml` is unchanged, or changed only under the stated exception with
      chapter 16 updated in the same commit.
- [ ] `make release-build` output contains no file below `infra/compose/test/`.
- [ ] The production stage of `services/server/Dockerfile` is unchanged and last; `make trivy`
      is green.
- [ ] All Go and Python tests listed above exist, were red first, and pass; `make check` is green.
- [ ] Documentation, threat model row and register entry, rulings and changelog entry are in place.

## Definition of done

Code, tests and docs: `doc.go` explains the probe and every exported symbol has a doc comment
(purpose, parameters, return value, errors); comments explain why; the harness is documented in
`infra/compose/test/README.md` (add `services/server/test/smoke/README.md` as well if M1-69 set
that convention for `test/canary`); `infra/compose/README.md`, `collector.md`, `privacy.md`, the
threat model and `DECISIONS.md` updated; no ADR (no new architecture decision; P-06 and ADR 0012
hold); `CHANGELOG.md` entry under Unreleased. `make check` green and `make portability` green.
Independent review (`make review` plus a code review over the diff) and the red team gate
(`docs/quality/red-team-gate.md`) over the diff with the verdict line recorded; merge locally into
the base branch only if the review is clean, the verdict is `JA` and both commands are green; then
`git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then the
commits (SHA and subject), files with line counts, test results with counts (red runs and green
runs), the step log of `make portability` (labels only, no values), the Docker Engine and Compose
versions used, the keys dropped by the overlay, any leak fixed at the source, what is not proven
(the architecture that was not run, profiles and services left to M7-14), and follow-ups that
could not be done in scope with the reason. Then write `{{DONE_PATH}}` with
`exit=0 sha=<merge sha>` or `exit=1 <reason>`.
