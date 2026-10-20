# Changelog

All notable changes to the repository as a whole (specification, documentation, tool
chain and the Compose installation bundle) are documented in this file. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html). The version of
this level is the root [`VERSION`](VERSION) file; its release tags are `v<version>`.

Independently versioned components keep their own `VERSION` and `CHANGELOG.md` in their
directory and are tagged `<component>/v<version>` (docs/spec/18-release.md):
server (`services/server/`), host service (`services/host/`), client app (`apps/client/`),
admin console (`apps/console/`), Python SDK (`sdk/python/`), TypeScript SDK (`sdk/typescript/`), Hermes
Agent adapter (`plugins/hermes-adapter/`), Claude Code runner
(`plugins/claude-code-runner/`) and the Grafana panel plugin (`plugins/grafana-bothy-live/`). Each of these files is created with the component's
first code.

## [Unreleased]

Planning stage (milestone M0). Nothing is installable yet.

### Added

- Content Security Policy of the web console with a headless test (M6-03): every response below
  `/console/` carries the policy of 19 5 with a nonce per response, COOP, COEP, CORP and the
  reporting endpoint `POST /api/v1/csp-reports` (`submitCspReport`); `make console-e2e` runs the
  console of the server image under that policy in Chromium, Firefox and WebKit against the
  Compose smoke stack (zero violations, no foreign origin, WebSocket, a passkey step-up, a blocked
  and reported inline script), and the CI job `console-e2e` feeds `ci-ok`.
- Compose smoke test (M1-73): `make portability` runs `infra/compose/smoke.sh`, which starts the
  server core from the real `bothy-server` image with the installation Compose file and a test
  overlay (`infra/compose/test/`), proves secrets init, migrations, the setup lock, the first
  owner, enrollment with device keys and DPoP, space, channel, thread and messages, ordered live
  delivery, REST and gateway catch-up, step-up and revocation with `4012` through the black-box
  probe `services/server/test/smoke`, and scans the collector export and the container logs for
  the run's canary, tokens, enrollment code and principal ID. Missing Docker, an old Engine or
  Compose or a missing image exit with code 2. The release bundle leaves the test overlay out.
- Desktop thread view (M4-20): the history column shows a thread or the main history of a text
  channel from the local database with grouping, origin badges, safe Markdown, "Show more",
  tombstones and system lines, loads older pages and fills gaps, focuses the visible threads and
  marks them read; the header names the operator or "No agent answers here", marks automatic
  titles and offers Done, reopen, title, move, pin and the session column; the composer sends
  through the outbox with Enter or Shift+Enter, keeps the draft in sync, suggests mentions, inserts
  templates and shows who is typing; messages offer reply with quote, start thread, reactions, pin,
  edit with the conflict view, versions, delete, mark as unread and copy; every offline state of
  01 6.6 is visible, and a removed thread closes with "No access anymore". The session column has
  its frame with the pinned messages. The thread, thread list and history window streams of the
  local store have their own trigger statements, so a history next to a post list shows new
  messages.

- Audit log of the administration (M4-29): `packages/admin_ui` lists the audit log of 01 5.2 A6
  (virtualized, newest first, cursor paging) with filters for action, person, result, resource
  and time range, shows an entry's details, checks the hash chain (`verifyAuditChain`, intact
  or the first broken entry with the warnings) and exports a time range as the server's NDJSON
  with hashes or as a CSV converted on the device with a guard against formula injection. The
  fake server of `sync_engine` serves the three audit operations over a hash chain that a test
  can break at a given entry.
- Grafana panel plugin `bothy-live-panel` (M7-25, `plugins/grafana-bothy-live`, its own
  `VERSION` and `CHANGELOG.md`): the five live views of docs/spec/25-visualisierung.md 8.2 from
  metrics only, in the colours and fonts of `design/build/ts/tokens.gen.ts` with a lint rule
  against colour literals; `queries.yaml` with the generated `src/queries.gen.ts` and
  `make -C plugins/grafana-bothy-live generate-check`; the seventh dashboard `bothy-live.json`.
  `make signal-check` now also checks every metric and label of `queries.yaml` against the
  signal catalogue; `make bootstrap-ci` installs the plugin's lock, and a change of
  `design/build/ts/`, of `bothy-live.json` or of the Grafana pin selects the gate component
  `plugins`. TV mode is Grafana's kiosk mode (`?kiosk`, verified for Grafana 13.2).
- Gate `sql-visibility` (M1-70, AZ-P12): `tools/lint/sql_visibility.py` rejects every reading
  sqlc query on a content table without `channel_id = ANY(...)` or `space_id = ANY(...)` unless it
  stands with a category and a reason on `tools/lint/sql_visibility.yaml`; stale, duplicate and
  uncategorised exceptions and content tables unknown to chapter 03 fail too.
- Hosts of the administration (M4-37): `packages/admin_ui` has screen A8 of 01 5.2 for owners:
  the hosts list, the host page (state, assignee, platform and version, connection, fingerprint
  and key generation, LLM login with I6 explained, server host, check results whose named session
  users show the person of their assignment, also an ended one from the audit of the host,
  profiles, grants with the preview state, session users), the pairing wizard in three steps
  (code with the command and its remaining time, the fingerprint prefix with the attempts the
  server names, the mandatory questions, session users only from the reported names), disable,
  enable, re-pair and retire with reason and step-up, and the host target in the grant wizard
  (capability `terminal`, no level, "Not effective" badge); the fake server answers the host
  operations.
- Permission screens of the administration (M4-27): `packages/admin_ui` has the grant wizard of
  01 5.2 A2 in three steps with the server's plain-language preview (text, effects, I1 and SR8
  explained, hints such as H1 and a missing expiry, delegation denials with their reason) before
  the step-up, the grants of a target with revoke (immediate, required reason), "Who has
  access?" with the source chains of 07 9.1 and "View as person" as a read-only snapshot; the
  fake server answers the grant operations and the example of 07 9.3, read at run time.
- Administration of people, groups and devices (M4-26): `packages/admin_ui` has the shell of
  01 5.2 A1 (People, Groups, Devices with the count of waiting devices) and its screens: the
  people list with search and filters, "Add person" with an enrollment code shown once, a
  person's detail (owner flag, credentials, devices, sessions; disable, enable, new code),
  owners (add and revoke; the last owner stays), groups (create, rename, delete, members with
  an expiry) and the queue of waiting devices with the origin of the login (approve with the
  code, reject). Every change asks once and then for the step-up where the server demands it;
  the desktop app hosts the shell at `/admin`. The fake server answers these operations and
  takes the step-up of each from `docs/spec/actions.yaml`.
- Desktop settings, devices and sessions (M4-25): Settings has the sections of 01 5.1 D8
  (Appearance, Language, Keyboard, Devices and sessions, Privacy, About and the Notifications
  entry M4-33 fills) under `/settings/<section>`. The personal settings of 01 11 apply their
  defaults on the client, a change goes through the outbox as `putMySetting` and a reset as
  `deleteMySetting`, and changes from another device apply per key. Language and time zone are
  profile fields, and the app language follows the profile. `packages/admin_ui` gains the own
  devices and sessions views (confirm a waiting device by its verification code, add a device by
  its pairing code, revoke a device, end a session) and the step-up dialog that explains an
  action before the system asks for the step-up key; revoking this device or ending the current
  session wipes the local data. The fake server gains the own device, session, profile, settings
  and step-up operations and a second simulated device of Alex.
- Sidebar and post list (M4-19): the desktop sidebar shows the instance's product name and
  logo, the "Jump to" field and "Running now" as placeholders, the spaces in the person's own
  order (collapse and drag synchronized across devices through `putMySidebarSpace` and
  `sidebar.updated`, keys from the shared fractional index in `domain`) and their channels with
  read, unread, mention, run, muted and selected states and counters kept by the sync engine.
  A forum opens its post list with "New post" (`createThread` through the outbox), the filters
  Open, Waiting for me, Tags and Sort, paging through `listChannelThreads` and the "Done (n)"
  section, whose collapse state is kept per device in the new local table `forum_view_states`.
- Desktop app shell (M4-18): `apps/client` builds for macOS and Windows with platform folders,
  the build branding of `branding.example.yaml` (URL scheme, app name, bundle identifier, default
  server) and deep links of that scheme on both platforms. One composition root per server
  address wires the device keys, the secrets in secure storage, the encrypted local database,
  the auth session and the sync engine; Riverpod providers over the database and the engine feed
  the widgets. The onboarding screens E1 to E7 run the login flows (OIDC in the system browser
  with a loopback redirect, proxy, password with one-time code, enrollment code, waiting device
  with its verification code, pairing), and every end of an account (logout, cancel, rejection,
  expiry, revocation, `4012`) deletes the local database, the file cache, the keys and the
  tokens. The router has one path per target with "Not found or no access", and the main view
  has four resizable column frames whose widths are stored as `layout.column_widths`. Widget,
  golden and flow tests run against the fake server of M4-17; the integration tests run on the
  desktop runners of CI.
- Sync engine composition root and fake server (M4-17): `SyncEngine.create` builds the gateway
  connection, catch-up and dispatcher, outbox and run store and wires their hooks (echo to the
  outbox, outbox start after `catchup.complete` and pause on every disconnect, `run.updated` and
  the run end to the run store, focused runs into the gateway focus); only test builds accept a
  plain `http` server. The separate library `package:sync_engine/testing.dart` is the one Dart
  fake of the server for every integration test: REST subset and gateway on `127.0.0.1`, an
  in-memory world with event log and sync epoch, DPoP checks, contract validation of every
  request, response and frame against `docs/api/`, deterministic fault injection (any close
  code, duplicate delivery, delay, `resync_required`, `4012`, `409`, `410`, `412`, `429`,
  `5xx`, a lost response after the commit), a scripted agent and the scenarios of 04 16 as
  drivers. The headless scenarios 16.1 to 16.5 run against `SyncEngine` with its local
  database. `make arch` gains rule 10: a `testing.dart` library is imported only from `test/`
  and `integration_test/` folders.
- Sync engine outbox: `Outbox` stores every write with its own UUIDv7 before sending, sends
  strictly FIFO per thread or per lane with lanes in parallel, retries network errors, timeouts,
  `5xx`, `429` and `409 request_in_progress` with full jitter between `client.outbox_retry_base`
  and `client.outbox_retry_cap` (or `Retry-After`) and the same ID, keeps final errors and
  `412 version_conflict` as failed entries with their reason, confirms entries by the echo,
  sends a message only after its uploads, follows 04 13.3 for edits and deletes, survives a
  restart, sends only while started, and reports "Sending delayed" after
  `client.outbox_stuck_after`; executors for the message and personal writes are registered per
  operation (M4-15).
- Web console app and serving (M6-02, ADR 0028): `apps/console` is a Flutter Web app with the
  navigation of 01 5.5 (placeholder pages), the login E2 against the console form of the login
  operations (OIDC with the callback `/console/login/callback` and the verifier in
  `sessionStorage`, passkey through WebAuthn, password with the one-time code, proxy login),
  the step-up directly after the login (passkey, new OIDC login or password with TOTP), E7 with
  the reason, and the version check against `Discovery.spec_version`. Its HTTP layer uses the
  generated `api_client` with the session cookie and `X-Console-Request: 1`, never a token or
  DPoP. Semantics are on at start. The server serves the build under `/console/` from
  `console.assets_dir` with SPA fallback, cache rules and the headers of SEC-HDR-1 and part of
  SEC-HDR-3; the server image builds and carries it.
- Sync engine catch-up and reconciliation: `CatchUpListener` applies the snapshot of `ready`
  (the whole snapshot without a cursor, otherwise counters, presence, flags and run summaries),
  catch-up pages up to each `cursor` frame or `catchup.complete` in one transaction with their
  cursor and epoch, live events one transaction each, and `resync_required` on the same
  connection (mirror emptied, outbox, drafts and share inbox kept, snapshot, focused threads
  reloaded); the `Dispatcher` discards duplicates by the cursor, skips unknown types and types
  without a local table, applies the version rule or the `seq` rule per the catalogue and writes
  through one applier per resource group; `truncated` events reload through REST; gaps in
  `thread_seq` load through the `Fetcher` before the message shows; `access.changed` removes or
  loads targets; `4012` and `session.revoked` of this session wipe the local database; typing
  and agent presence live in the `EphemeralStore`; personal data follow the echo, typing and
  monotonic read rules of 04 14.
- Sync engine gateway connection: `GatewayConnection` holds the one WebSocket of a device
  (transport port with a `dart:io` implementation), answers `hello` with `identify` or `resume`
  and a gateway DPoP proof over the `hello` nonce, stores the session fields of `ready`, renews
  the token on `reauth_required`, sends heartbeats with a random first delay and reconnects when
  an acknowledgement is missing, waits by full-jitter backoff that a network change or the
  foreground cuts short, and reacts to every close code of 04 6.4 through one table
  (`4012` signs out with deletion of local data, `4016` waits for the pending device, `4017` for
  the foreground, `4018` for user action, `4008` for `retry_after`); `ConnectionState` exposes
  the phases of 15 7.2 with a reason, `FocusManager` and `TypingThrottle` send `focus` and
  typing within their limits, and `SessionListener` is where catch-up plugs in. The gateway
  close code `4018 forbidden` of D-W-24 is ported to 04 6.4 and the AsyncAPI contract (M4-13).
- Contract tests (M1-71): the gate `contract-test` (G7, Schemathesis 4.30.1 from `tools/contract/`
  with ES256 DPoP proofs built with `cryptography`), G8 in `make test` through the build tag
  `contract_validate`, and the named values `quality.contract.max_examples` and
  `quality.contract.timeout`.
- Sync engine run documents: `RunStore` keeps the live box document of every focused run in
  memory, applies `run.updated` by the client rule of 04 11.3 item 4 (equal `base_version`
  applies, a stale frame is ignored, a gap, a truncated frame or an invalid patch refetches the
  whole document through a port, a `snapshot` replaces it), queues patches during a refetch,
  publishes at most one state per frame and discards the document at the end of the run;
  `applyRunPatch` applies RFC 6902 `add`, `replace` and `remove` with copy on write, and
  `RunDocument` reads the live box fields of 09 5.4 (M4-16).
- Local database of the apps: `packages/local_store` is a drift database on
  SQLite3MultipleCiphers (cipher `chacha20`, 256-bit key from a key source port), schema
  version 1 with the mirror tables of the client architecture chapter and the local tables
  `outbox`, `sync_state`, `loaded_ranges`, `file_cache`, `share_inbox` and the FTS5
  `search_index`; DAOs with the version rule on every upsert, the cursor stored in the page's
  transaction, removal of targets and wipe, the cache cleanup by `client.cache_max_age` and
  `client.cache_max_bytes`, and a migration survival test for outbox, drafts and share inbox;
  `make codegen-check` compares the drift generation (M4-12).
- Login core of the apps (`packages/auth`, pure Dart): DPoP proofs after RFC 9449 signed through
  a device key port (ES256, `ath`, server nonce, the gateway form), one repeat on
  `use_dpop_nonce`, refresh as a single flight per family that stores the new refresh token
  before the access token is used, step-up with the step-up key and a repeat with the same
  `Idempotency-Key`, the login flows (OIDC with PKCE, password with TOTP, proxy header, passkey,
  enrollment code, pairing, pending device, logout), the signed-out event for a revoked session,
  and Dio interceptors for the slot of `createApiClient`; no secret in any `toString`, error or
  log line (M4-09).
- Visualization views (M6-12): `packages/bothy_viz` gets the six views of 25 6, built only on
  the models, ports and basic forms of M6-11: live topology (lanes, run cards, rates of the last
  minute, stale and "More runs than shown"), backend view (configured and reported mode with the
  mismatch marker, supported modes, switches, I2 and I3 denials, costs per mode, every
  rate-limit state), delivery and sync live (instance scope only), executive page (usage, runs
  and error rate, costs per mode, SLO traffic light "own measurement"), the replayable daily
  timeline with play, pause, jump and the speeds of `viz.timeline_speeds`, and TV mode (dark,
  one font step larger, rotation every `viz.tv_rotation_interval`, costs only with
  `viz.tv_show_costs`, ended by a key or a click). Every view has the switch "As table" with
  sortable tables, follows reduced motion and is pinned by goldens in the full matrix; the
  registry `vizViewRegistry` lists the views for the hosts, and the harness shows them all.
  A performance test of the live topology with the `load` frame runs in profile mode on macOS
  through `make apps-platform` and `make perf` (`viz.perf.frame_build_p95`).
- Visualization package (M6-11): `packages/bothy_viz` gets view models from the generated
  statistics types, the ports `LiveFrameSource` (with the stale rule on an injected clock) and
  `StatsQuerySource` with fixture implementations, the basic forms lane, run card, area, bar,
  traffic light and metric tile as `CustomPainter`s with semantics and goldens, JSON fixtures per
  schema and state with the check `viz-fixtures` of `make api-checks` (VZ-T11), and the harness
  app `bothy_viz_harness`. `tools/observability/sli_bounds.py` generates the SLI bounds of server
  and apps from the Sloth files, every SLO carries the label `bothy_sli_metric`, and the checks
  `sli-bounds` and `sli-labels` of `make ops-check` hold them (VZ-T03).
- Design system core components: message (every variant, collapse with "Show more", origin
  badges, isolated foreign names), a safe Markdown renderer that runs and loads nothing, live box
  with collapsed and mobile lines and a throttled announcer, tool chip, status line, trigger
  state, thread, space and channel rows, components on messages with a header built only from
  server fields, the composer with suggestions, the backend policy badge with cost texts and
  banners; view models with `unknown` members, texts in German and English, a golden matrix in
  light and dark, three densities, text scale 1 and 2, de and en, and guideline tests for contrast,
  tap targets and labels (M4-07).
- API breaking change check (gate G6 of chapter 06): `make api-breaking`, part of `make check`
  when the server component is selected, compares `docs/api/openapi.yaml` and its frame and event
  schemas with the document of the newest stable `server/v*` tag and fails on every breaking row of
  06 section 13 (removed or renamed parts, newly required request fields and parameters, type and
  format changes, removed enum values, changed status codes, sort order, pagination and
  idempotency changes, a changed `x-authz-action`); a major step of `info.version` accepts a
  break, and without a tag the gate passes with a notice. Own comparison instead of oasdiff, which
  missed rows on this document (M7-11).
- Design tokens: `tools/design-tokens` writes `design/build/ts/tokens.gen.ts` for the Grafana
  panel plugin (colour roles of both themes including the chart and state roles, font families,
  font sizes and radii as typed constants) and copies the vendored Atkinson Hyperlegible fonts
  with their SIL Open Font License to `design/build/ts/fonts/`; `make tokens` fails on a stale or
  hand-edited file, a missing font or licence, and compiles the file with `tsc --strict` (M7-24).
- Design system icons: `DsIcon` draws every semantic icon of `icon.tokens.json` from Phosphor
  2.1.1 vectors compiled by `vector_graphics_compiler` (chosen over an icon font after a
  sharpness and size measurement, docs/design/icons.md), at the `icon.size.*` tokens, with a
  required label or `decorative: true` and the spin of `trigger.working`, `run.running` and
  `state.reconnecting` only in full motion. `tokens.py icons` (part of `build`) generates the
  assets, their lookup, the MIT text and a manifest, and `make tokens` fails on a missing glyph
  or a stale asset; a golden shows every icon in its weight and in fill (M4-05).
- Device key for Windows: `packages/device_key` gains its Windows part, a C++ Pigeon host and the
  static library `device_key_core` over CNG. The `dpop` key is an ECDSA P-256 key of the user in the
  TPM (Platform Crypto Provider, after `NCryptIsAlgSupported`) or, without a TPM or without P-256,
  in the software key storage provider and reported as not hardware-bound; export policy `0`, the
  provider fixed at creation, a lost key reported as `key_invalidated`. There is no step-up key on
  Windows, because Windows enforces no confirmation per use for a P-256 key (08 6.1, D-M4-11-2);
  `make apps-platform` runs the CTest suite on Windows (M4-11).
- Design tokens: `make tokens` checks the chart series `color.viz.series.1` to `.8` (adjacent
  pairs and series 8 with series 1) and the traffic light states `success.fg`, `warning.fg` and
  `danger.fg` under simulated protanopia, deuteranopia and tritanopia (Machado et al. 2009,
  severity 1.0) against the minimum OKLab distance `cvd_min_delta_e_ok`; failures name theme,
  deficiency, pair and distance, and `docs/design/contrast.md` lists every row (M6-10). The chart
  roles `color.viz.*`, their contrast pairing and `tokens.py previews --view` come with it (plan
  ruling D-W-13).
- Design system package: `packages/design_system` exports the generated token classes (a copy
  that `tokens.py build` writes next to `design/build/dart/` and `make tokens` compares), a
  settings resolver from `appearance.theme`, `appearance.density`, `appearance.reduce_motion` and
  the instance accent to theme, density, motion and accent, a theme builder with the five token
  extensions and the color scheme mapping, `DsThemeScope` (no animated switch under reduced
  motion), the bundled variable fonts Atkinson Hyperlegible Next and Mono with their OFL texts and
  a checked manifest, and the alchemist golden setup with `dsGoldenMatrix` (M4-04).
- Localization in `packages/l10n`: the ARB files `app_en.arb` (template) and `app_de.arb`
  (default locale) with the first keys, the generated `AppLocalizations`, `problemText` for every
  problem code with a fallback that shows the code, `L10nFormat` for money, numbers, durations
  and times per locale, `resolveAppLocale` and the term table generated from 01 section 9. Tests
  reject a missing translation, a placeholder mismatch, a missing description, en and em dashes,
  emoji and a German text without the term of the table; `make codegen-check` regenerates in a
  temporary copy and fails on untranslated messages or stale generated code (M4-06).
- Dart REST client `packages/api_client`: generated from `docs/api/openapi.yaml` with the pinned
  openapi-generator `dart-dio` image and checked in, plus a hand layer with `createApiClient`,
  `traceparent` and `Idempotency-Key` headers, retry for reads only, `ApiProblem` and tolerant
  event decoding; `make api-clients` regenerates it in a temporary copy, compares and analyses it
  (G4 for Dart), and a G5 test proves that unknown fields, enum values and event types are
  skipped (M4-02).
- Domain package (`packages/domain`): typed IDs and a monotonic UUIDv7 generator, the entities
  of the client schema mirror with closed enums that decode unknown values to `unknown`, the
  sync rules of chapter 04 (version rule, cursor rule, monotonic read state, loaded ranges of
  `thread_seq`, unread and mention counting), the display rules of chapter 01 (message grouping,
  forum order with the Done section, cost labels), an event catalogue generated from
  `docs/spec/actions.yaml` and `AppConfig` generated from the configuration reference;
  `make codegen-check` fails when either generated table is stale (M4-08).
- Release chain of the server (M7-10): `make release-prep COMPONENT=<name> [CHANNEL=beta]`
  computes the next version per component from its Conventional Commits (git-cliff, `cliff.toml`),
  renames the changelog section and writes `VERSION`; `make release-trial COMPONENT=server`
  proves the chain on the build host against a local registry with a throwaway key pair
  (build, SBOMs, signatures, `SHA256SUMS`, `cosign verify`, `sha256sum --check`, version tags);
  `make scancode` scans the tracked tree with ScanCode and a license gate. `release.yml` builds
  `bothy-postgres` per architecture, joins it, scans, runs ScanCode, writes SBOMs, signs every
  digest, binary and `SHA256SUMS` (key pair in the private phase, keyless in the public phase),
  verifies and only then tags. cosign, syft and git-cliff are pinned; `check_runners.py` rejects
  signing steps outside the environment `release`; a tag without a changelog section stops in
  `resolve`.
- Design tokens: accent variants for every instance accent of `branding.accent_color` in light and
  dark. The generator swaps the accent roles for the same step of the chosen scale, moves a
  failing step, checks every pairing with a swapped role and writes `DsAccent` (Dart),
  `data-accent` blocks (`tokens.css`), the member `accents` of `tokens.resolved.json` and one
  section per accent and theme in `docs/design/contrast.md`; `make tokens` fails below AA (M4-03).
- The docs site has a REST reference, a schema reference and a gateway reference, rendered in
  every `make docs` from `docs/api/openapi.yaml` and `docs/api/asyncapi.yaml` by the MkDocs
  hook `tools/docs/mkdocs_api_reference.py`; `make docs` also fails when a built page loads a
  script, stylesheet or font from another origin (`tools/docs/external_assets_check.py`)
  (M7-19).
- One Dart pub workspace at the repository root (`pubspec.yaml`, one `pubspec.lock`) with the
  fourteen members of docs/spec/15-client-architektur.md 3: the apps `apps/client` and
  `apps/console` and the packages under `packages/` as skeletons, `bothy_noise` taken over;
  the Flutter pin moves to the root `pubspec.yaml` (M4-01).
- Architecture test `tools/arch-check` in `make arch`: the dependency rules of 15 3.1 for every
  member, with the reach of the diagram as an allow list (M4-01).
- Documentation in code as a gate for Dart: `public_member_api_docs` as an error in every
  member, and `make docs` puts the `dart doc` pages of every package into the site
  (`docs/reference/dart-api.md`) (M4-01).
- `make apps-platform` and the CI jobs `apps-macos` and `apps-windows` for the platform tests
  of the apps (M4-01).
- Host service module `services/host` (own `VERSION` and `CHANGELOG.md`) and the Dart package
  `packages/bothy_noise` with the Noise core of the terminal (M2-38); the gates, CI and CodeQL
  cover both.
- `plugins/hermes-adapter`: recording harness, probe plugin and fixtures of the live recording
  against the pinned Hermes Agent version (`plugins/hermes-adapter/tests/hermes_pin.toml`); the
  answers of docs/spec/09-agenten.md 17.10 come from it (M2-32).
- The named value `quality.coverage.python_min_percent` also holds every Python project under
  `plugins/` (M2-32).
- CI path filter: a change of docs/spec/09-agenten.md selects the gate component `plugins`.
- Core contract of the specification: glossary, data model, sync protocol and action
  catalogue (`docs/spec/00`, `03`, `04`, `05`).
- Quality and repository tool chain: `make check` with every gate identical to CI,
  pre-commit and pre-push hooks, pinned tools (`tools/`), workflow policy check, deny-list
  scanner for white-label checks, SonarQube profile and gate shared by the local
  Community Build and SonarQube Cloud, Semgrep project rules, gitleaks rules, Renovate.
- Workflows for pull requests, releases, documentation, CodeQL, OpenSSF Scorecard and the
  portability test, with one runner switch for the private and the public phase.
- Community files: license (Apache-2.0), contributing guide with DCO, security policy,
  code of conduct, support, governance, issue and pull request templates.
- Architecture decision records 0000 to 0003, 0038 to 0042 and 0045; quality chapter
  `docs/spec/23-qualitaet.md` and the guides under `docs/quality/`.
- Server gates in the make contract: `codegen-check`, `arch` and `migrations-lint` in
  `make gates`, `make perf` beside them. The gates and `make perf` skip until the first Go
  module is tracked and then fail without Go, a Docker-compatible runtime or the pinned
  go-arch-lint and sqlc. The migration rules of ADR 0007 are checked by
  `tools/ci/migrations_lint.py`; `make migrations-lock` records the checksums of new
  migrations.
- Machine-readable action catalogue `docs/spec/actions.yaml` as the single source of
  actions, rate-limit classes, event types and ops (ADR 0049): `make catalog` generates the
  tables of chapter 05 and the lists in `docs/api/redocly.yaml`, and the gate
  `catalog-check` (`tools/catalog/`) fails on drift or on a contradiction with the
  hand-written parts of chapter 05.
- Gate `register-check` (`tools/doccheck/register_check.py`): the module register
  `services/server/architecture/modules.yaml` must equal the tables and their modules in
  chapter 03 and the actions, event types and ops of `docs/spec/actions.yaml`, and the module
  tables of chapter 02, the server component diagram and the module READMEs must equal the
  register (ADR 0050); every difference is one finding with file and item.
- API contract checks in the gate (06 G2 and G3, `tools/api-checks/`): `make api-checks`
  compares `openapi.yaml` and `asyncapi.yaml` with the action catalogue (actions, classes,
  ops, events, app scopes, the union `Event`) and the close codes of chapter 04, holds
  `x-idempotency-exempt`, the deprecation dates against
  `api.deprecation_min_notice`, `x-problem-codes` and `x-status`, and fails on a stale
  generated table in chapter 06 or `docs/api/webhooks.md`; `make api-tables` regenerates
  these tables. An operation message without `$ref` is a finding, and no input ends in a
  traceback.
- Operations configuration gate `make ops-check` in `make gates` (`tools/opscheck/`): the
  OpenTelemetry Collector base alone, with each overlay and with every ordered pair of
  overlays, the Prometheus configuration, rules and rule unit tests, the Alertmanager
  configuration and routes, Loki, Tempo and the OpenTofu modules are validated with the
  pinned upstream tools; generated SLO rules, the SLO table, the collector allowlist, alert
  thresholds and validator versions are checked against their sources. Seven validators are
  pinned with sha256 (`otelcol-contrib`, `promtool`, `amtool`, `loki`, `tempo`, `sloth`,
  `tofu`); `tools/bin/fetch_tools.py` reads zip release assets. Every hand-written alert has
  `promtool` unit tests. Renovate updates the helper images of the Compose stack in one group
  "Compose-Images" and leaves their validators to `make tools-lock`.
- `make config-doc-check` in `make gates` (`tools/configdoc/configdoc.py`): the configuration
  reference against the server schema, the chapter tables, the etappe of planned keys
  (`tools/configdoc/planned.yaml`), the names retired by W1 (`tools/configdoc/retired.yaml`) and
  the Compose `stop_grace_period` of every service that runs `serve` (rules D1 to D10).
- `packages/device_key` for iOS and macOS: two ES256 device keys per server (`dpop` without a
  prompt, `step_up` with biometrics or the device password) in the Secure Enclave where there is
  one, otherwise a software key in the Data Protection keychain reported as not hardware-bound;
  never synchronizable, always `ThisDeviceOnly`. A Pigeon channel, a Dart API with the public JWK,
  the RFC 7638 thumbprint and `R || S` signatures, and the Swift package `DeviceKeyCore` that the
  app extensions reuse. `make codegen-check` regenerates the Pigeon code, `make apps-platform`
  runs the Swift tests on macOS and in the iOS simulator (M4-10).
- `packages/device_key` for Android: the same Dart API and Pigeon channel, backed by the Kotlin
  module `android/devicekeycore` (Android Keystore, P-256, StrongBox where the device has it with
  a fallback to the trusted execution environment, a software key reported as not
  hardware-bound elsewhere). The `step_up` key needs authentication for every signature through
  `BiometricPrompt` (strong biometrics or the device credential from API 30, strong biometrics
  only on API 26 to 29) and is invalidated by new biometrics; the `dpop` key signs without a
  prompt. Signatures leave as `R || S`; the attestation is the key's certificate chain. `sign`
  takes optional `StepUpPrompt` texts. The core module is meant for reuse by the push service and
  the share activity (M5-01).
- `make apps-platform` knows the system `android` (exactly one attached device or emulator), the
  CI job `apps-android` runs the instrumented tests on emulators with API 26 and 36 and feeds
  `ci-ok`, and `make test` runs the JVM tests of the Gradle modules under `packages/*/android`
  (M5-01).

### Changed

- Design tokens: series and state roles moved along their scales so that they stay apart under
  the simulation: light `color.viz.series.3` `spruce.600` to `spruce.700`, `.6` `moss.600` to
  `moss.900`, `color.warning.fg` `amber.700` to `amber.800`, `color.danger.fg` `rowan.700` to
  `rowan.900`; dark `color.viz.series.3` `spruce.400` to `spruce.300`, `.7` `rowan.400` to
  `rowan.500`, `color.danger.fg` `rowan.300` to `rowan.400`. Every contrast pairing stays green;
  the previews are regenerated (M6-10).

- Gates for a second Go module and the first Dart package (M2-38): `make perf` runs only for
  the Go modules with performance targets (`PERF_MODULES`), a Dart package without `test/` fails
  `make test` and one without `coverage/lcov.info` fails `make coverage`; CI installs Go for every
  tracked `services/*/go.mod` and Flutter for every tracked Dart package, CodeQL builds the host
  module, a change under `services/host/testdata/noise/` selects the apps gates as well, and
  SonarQube reads the coverage of both new components.
- Chapter 02 "Konfigurationsladen" and the configuration reference describe the loader as built
  by the server: built-in defaults live in the schema, `bothy config print` shows secret
  references instead of `<redacted>`, and `docs/operations/configuration.md` sections 1 to 3 give
  the exact grammar of every kind (now with `list(cidr)` and `url`), the file rules and the output
  of `bothy config check` and `bothy config print`.
- CodeQL analyses the Go server module (manual build of `services/server`) in the public
  phase; a test keeps the CodeQL languages in step with the tracked components.
- `make coverage` leaves generated Go files (`*.gen.go`) out of the Go total, and the license
  gate requires the Go generated-code marker in every file named as generated Go code; a
  directory named `gen` no longer counts as generated for the license gate, Sonar and the lint
  rules gocognit and goconst of the server module.
- The CI path filter selects the server component for a change to `docs/spec/actions.yaml`,
  from which the server generates its route table and action catalogue.
- Configuration reference: section 4 of `docs/operations/configuration.md` is generated from the
  configuration schema of the server plus the chapter tables, with the new columns type and state
  (`aktiv` for keys this server version reads, `geplant (<etappe>)` for keys of a later ticket);
  the generator moved from `tools/docs/config_reference.py` to `tools/configdoc/`, and `make docs`
  no longer checks the reference.
- `docs/PLAN.md`: the W1 rows for trusted proxies and database timeouts and the acceptance test of
  M3-06 follow the chapters.
- `make api-clients` (gate G4) runs `generate-check` of `sdk/python` and `sdk/typescript`, fails
  for a client directory with a tracked manifest but without that target and respects
  `COMPONENTS`; `make bootstrap-ci` installs the locked SDK dependencies (`npm ci
  --ignore-scripts`, `uv sync --locked`); `make typecheck` runs mypy inside each Python project and
  `make lint` keeps bandit out of its `.venv` and generated code. New named values
  `quality.coverage.python_min_percent` and `quality.coverage.typescript_min_percent`.
- The CI path filter selects both SDK components for a change under `sdk/testdata/` (shared G5
  fixtures); CodeQL analyses `javascript-typescript` (the TypeScript SDK); Renovate groups the SDK
  code generators.

- Chapter 12 "4.1 Port" and "4.2 Dateisystem-Treiber (Standard)" describe the port contract
  (key grammar, one byte range, error contract, idempotent `Delete` and `AbortMultipart`) and the
  file system rules (start check of the root, no symbolic links, temporary files in `.tmp`,
  directory removal on delete, multipart layout and ETag) as built by M3-01; chapter 02
  "13. Konfigurationsladen" item 4 lists the cross-check `file_fs_root_absolute`; ADR 0010
  "Confirmation" names the conformance suite and the property test.

### Fixed

- `docs/api/openapi.yaml` 0.2.0: the response `Conflict` names the problem code
  `file_blocked`, which no response listed; `createHost` and `issueHostPairingCode` take
  `Idempotency-Key` and list `422 idempotency_key_reuse` instead of the exemption `auth_flow`,
  as 06 section 7 asks for a POST that returns a one-time secret (`openTerminalSession` and
  `createExtensionSession` keep `auth_flow`); by 06 section 13 this changes the idempotency
  rule of two operations, a breaking change made while the API is still `0.x`; the enum
  `AppScope` contains `costs:reconcile`, the scope of `cost.reconcile.submit` (05 section
  2.6).
- `docs/api/asyncapi.yaml` 0.1.1: `dispatch` lists the event types in the order of the
  catalogue; the event table of chapter 06, generated from it, now follows that order too.
- Chapter 06, section 7: the table of operations without `Idempotency-Key` lists the host
  pairing and terminal session operations that `openapi.yaml` exempts.
- The alert `BothyMalwareScanFailing` now fires when no malware scan has ever produced a
  result; before, it stayed silent exactly when the scanner never worked since the start.
- `make ops-check`: `slo-docs` follows `ticket_alert.disable` (a row may say `nur Page` or
  `keine`) and reads the `le` bound in every PromQL quoting; `tofu` also checks modules made
  only of `.tofu` or JSON files; `alertmanager` parses the templates named by their
  `/etc/alertmanager/` path and fails on an entry that matches no file or lies outside the
  mount; `collector` fails on a reference to a config provider other than `env`, `file` and
  `yaml` (`${https:...}`, `${s3:...}`) before any `validate`; SIGTERM stops a run like
  SIGINT, killing the running validators and removing the work directory. The alert unit
  tests now also catch an added `for` on an alert without one.
- `tools/bin/fetch_tools.py` reports an empty, cut off or corrupt release archive as an error
  instead of a traceback.
