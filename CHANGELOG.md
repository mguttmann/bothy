# Changelog

All notable changes to the repository as a whole (specification, documentation, tool
chain and the Compose installation bundle) are documented in this file. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html). The version of
this level is the root [`VERSION`](VERSION) file; its release tags are `v<version>`.

Independently versioned components keep their own `VERSION` and `CHANGELOG.md` in their
directory and are tagged `<component>/v<version>` (docs/spec/18-release.md; the list of
components is `RELEASE_COMPONENTS` in `tools/ci/components.py`): server
(`services/server/`), host service for the terminal (`services/host/`), client app
(`apps/client/`), web console (`apps/console/`), Python SDK (`sdk/python/`), TypeScript SDK
(`sdk/typescript/`), Hermes Agent adapter (`plugins/hermes-adapter/`), Claude Code runner
(`plugins/claude-code-runner/`) and Grafana panel plugin (`plugins/grafana-bothy-live/`).
Each of these files is created with the component's first code.

## [Unreleased]

Planning stage (milestone M0). Nothing is installable yet. Bothy is planned as a command
center for AI agents in chat form: chat, live view, grants, terminal, costs and
statistics in one open-source app that you host yourself. This level holds the complete plan for
building it: what the product does, the rules it must never break, how it is built and
operated, and the order of the build.

### Added

- Local database of the apps: `packages/local_store` is a drift database on
  SQLite3MultipleCiphers (cipher `chacha20`, 256-bit key from a key source port), schema
  version 1 with the mirror tables of the client architecture chapter and the local tables
  `outbox`, `sync_state`, `loaded_ranges`, `file_cache`, `share_inbox` and the FTS5
  `search_index`; DAOs with the version rule on every upsert, the cursor stored in the page's
  transaction, removal of targets and wipe, the cache cleanup by `client.cache_max_age` and
  `client.cache_max_bytes`, and a migration survival test for outbox, drafts and share inbox;
  `make codegen-check` compares the drift generation (M4-12).
- Sync engine gateway connection: `GatewayConnection` holds the one WebSocket of a device
  (transport port with a `dart:io` implementation), answers `hello` with `identify` or `resume`
  and a gateway DPoP proof over the `hello` nonce, stores the session fields of `ready`, renews
  the token on `reauth_required`, sends heartbeats with a random first delay and reconnects when
  an acknowledgement is missing, waits by full-jitter backoff that a network change or the
  foreground cuts short, and reacts to every close code of 04 6.4 through one table
  (`4012` signs out with deletion of local data, `4016` waits for the pending device, `4017` for
  the foreground, `4018` for user action, `4008` for `retry_after`); `ConnectionState` exposes
  the phases of 15 7.2 with a reason, `FocusManager` and `TypingThrottle` send `focus` and
  typing within their limits, and `SessionListener` is where catch-up plugs in (M4-13).
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
- Design system core components: message (every variant, collapse with "Show more", origin
  badges, isolated foreign names), a safe Markdown renderer that runs and loads nothing, live box
  with collapsed and mobile lines and a throttled announcer, tool chip, status line, trigger
  state, thread, space and channel rows, components on messages with a header built only from
  server fields, the composer with suggestions, the backend policy badge with cost texts and
  banners; view models with `unknown` members, texts in German and English, a golden matrix in
  light and dark, three densities, text scale 1 and 2, de and en, and guideline tests for contrast,
  tap targets and labels (M4-07).
- Design tokens: `tools/design-tokens` writes `design/build/ts/tokens.gen.ts` for the Grafana
  panel plugin (colour roles of both themes including the chart and state roles, font families,
  font sizes and radii as typed constants) and copies the vendored Atkinson Hyperlegible fonts
  with their SIL Open Font License to `design/build/ts/fonts/`; `make tokens` fails on a stale or
  hand-edited file, a missing font or licence, and compiles the file with `tsc --strict` (M7-24).
- API breaking change check (gate G6 of chapter 06): `make api-breaking`, part of `make check`
  when the server component is selected, compares `docs/api/openapi.yaml` and its frame and event
  schemas with the document of the newest stable `server/v*` tag and fails on every breaking row of
  06 section 13 (removed or renamed parts, newly required request fields and parameters, type and
  format changes, removed enum values, changed status codes, sort order, pagination and
  idempotency changes, a changed `x-authz-action`); a major step of `info.version` accepts a
  break, and without a tag the gate passes with a notice. Own comparison instead of oasdiff, which
  missed rows on this document (M7-11).
- Sync engine run documents: `RunStore` keeps the live box document of every focused run in
  memory, applies `run.updated` by the client rule of 04 11.3 item 4 (equal `base_version`
  applies, a stale frame is ignored, a gap, a truncated frame or an invalid patch refetches the
  whole document through a port, a `snapshot` replaces it), queues patches during a refetch,
  publishes at most one state per frame and discards the document at the end of the run;
  `applyRunPatch` applies RFC 6902 `add`, `replace` and `remove` with copy on write, and
  `RunDocument` reads the live box fields of 09 5.4 (M4-16).
- Design system icons: `DsIcon` draws every semantic icon of `icon.tokens.json` from Phosphor
  2.1.1 vectors compiled by `vector_graphics_compiler` (chosen over an icon font after a
  sharpness and size measurement, docs/design/icons.md), at the `icon.size.*` tokens, with a
  required label or `decorative: true` and the spin of `trigger.working`, `run.running` and
  `state.reconnecting` only in full motion. `tokens.py icons` (part of `build`) generates the
  assets, their lookup, the MIT text and a manifest, and `make tokens` fails on a missing glyph
  or a stale asset; a golden shows every icon in its weight and in fill (M4-05).
- Design system package: `packages/design_system` exports the generated token classes (a copy
  that `tokens.py build` writes next to `design/build/dart/` and `make tokens` compares), a
  settings resolver from `appearance.theme`, `appearance.density`, `appearance.reduce_motion` and
  the instance accent to theme, density, motion and accent, a theme builder with the five token
  extensions and the color scheme mapping, `DsThemeScope` (no animated switch under reduced
  motion), the bundled variable fonts Atkinson Hyperlegible Next and Mono with their OFL texts and
  a checked manifest, and the alchemist golden setup with `dsGoldenMatrix` (M4-04).
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
  CI jobs `apps (Android API 26)` and `apps (Android API 36)` run the instrumented tests on
  emulators and feed `ci-ok`, and `make test` runs the JVM tests of the Gradle modules under
  `packages/*/android` (M5-01).
- Device key for Windows: `packages/device_key` gains its Windows part, a C++ Pigeon host and the
  static library `device_key_core` over CNG. The `dpop` key is an ECDSA P-256 key of the user in the
  TPM (Platform Crypto Provider, after `NCryptIsAlgSupported`) or, without a TPM or without P-256,
  in the software key storage provider and reported as not hardware-bound; export policy `0`, the
  provider fixed at creation, a lost key reported as `key_invalidated`. There is no step-up key on
  Windows, because Windows enforces no confirmation per use for a P-256 key (08 6.1, D-M4-11-2);
  `make apps-platform` runs the CTest suite on Windows (M4-11).
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
- Login core of the apps (`packages/auth`, pure Dart): DPoP proofs after RFC 9449 signed through
  a device key port (ES256, `ath`, server nonce, the gateway form), one repeat on
  `use_dpop_nonce`, refresh as a single flight per family that stores the new refresh token
  before the access token is used, step-up with the step-up key and a repeat with the same
  `Idempotency-Key`, the login flows (OIDC with PKCE, password with TOTP, proxy header, passkey,
  enrollment code, pairing, pending device, logout), the signed-out event for a revoked session,
  and Dio interceptors for the slot of `createApiClient`; no secret in any `toString`, error or
  log line (M4-09).
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
- One Dart pub workspace at the repository root (`pubspec.yaml`, one `pubspec.lock`) with the
  fourteen members of docs/spec/15-client-architecture.md 3: the apps `apps/client` and
  `apps/console` and the packages under `packages/` as skeletons, `bothy_noise` taken over;
  the Flutter pin moves to the root `pubspec.yaml` (M4-01).
- Visualization package (M6-11): `packages/bothy_viz` gets view models from the generated
  statistics types, the ports `LiveFrameSource` (with the stale rule on an injected clock) and
  `StatsQuerySource` with fixture implementations, the basic forms lane, run card, area, bar,
  traffic light and metric tile as `CustomPainter`s with semantics and goldens, JSON fixtures per
  schema and state with the check `viz-fixtures` of `make api-checks` (VZ-T11), and the harness
  app `bothy_viz_harness`. `tools/observability/sli_bounds.py` generates the SLI bounds of server
  and apps from the Sloth files, every SLO carries the label `bothy_sli_metric`, and the checks
  `sli-bounds` and `sli-labels` of `make ops-check` hold them (VZ-T03).
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
  through `make apps-platform` and `make perf` (`viz.perf.frame_build_p95` on reference
  hardware, `viz.perf.frame_build_p95_ci` on a shared hosted CI runner).
- Architecture test `tools/arch-check` in `make arch`: the dependency rules of 15 3.1 for every
  member, with the reach of the diagram as an allow list (M4-01).
- Sync engine outbox: `Outbox` stores every write with its own UUIDv7 before sending, sends
  strictly FIFO per thread or per lane with lanes in parallel, retries network errors, timeouts,
  `5xx`, `429` and `409 request_in_progress` with full jitter between `client.outbox_retry_base`
  and `client.outbox_retry_cap` (or `Retry-After`) and the same ID, keeps final errors and
  `412 version_conflict` as failed entries with their reason, confirms entries by the echo,
  sends a message only after its uploads, follows 04 13.3 for edits and deletes, survives a
  restart, sends only while started, and reports "Sending delayed" after
  `client.outbox_stuck_after`; executors for the message and personal writes are registered per
  operation (M4-15).
- Documentation in code as a gate for Dart: `public_member_api_docs` as an error in every
  member, and `make docs` puts the `dart doc` pages of every package into the site
  (`docs/reference/dart-api.md`) (M4-01).
- `make apps-platform` and the CI jobs `apps (macOS)` and `apps (Windows)` for the platform tests
  of the apps (M4-01).
- Design tokens: `make tokens` checks the chart series `color.viz.series.1` to `.8` (adjacent
  pairs and series 8 with series 1) and the traffic light states `success.fg`, `warning.fg` and
  `danger.fg` under simulated protanopia, deuteranopia and tritanopia (Machado et al. 2009,
  severity 1.0) against the minimum OKLab distance `cvd_min_delta_e_ok`; failures name theme,
  deficiency, pair and distance, and `docs/design/contrast.md` lists every row (M6-10).
- The docs site has a REST reference, a schema reference and a gateway reference, rendered in
  every `make docs` from `docs/api/openapi.yaml` and `docs/api/asyncapi.yaml` by the MkDocs
  hook `tools/docs/mkdocs_api_reference.py`; `make docs` also fails when a built page loads a
  script, stylesheet or font from another origin (`tools/docs/external_assets_check.py`)
  (M7-19).
- Design tokens: accent variants for every instance accent of `branding.accent_color` in light and
  dark. The generator swaps the accent roles for the same step of the chosen scale, moves a
  failing step, checks every pairing with a swapped role and writes `DsAccent` (Dart),
  `data-accent` blocks (`tokens.css`), the member `accents` of `tokens.resolved.json` and one
  section per accent and theme in `docs/design/contrast.md`; `make tokens` fails below AA (M4-03).
- `plugins/hermes-adapter`: recording harness, probe plugin and fixtures of the live recording
  against the pinned Hermes Agent version (`plugins/hermes-adapter/tests/hermes_pin.toml`); the
  answers of docs/spec/09-agents.md 17.10 come from it (M2-32).
- The named value `quality.coverage.python_min_percent` also holds every Python project under
  `plugins/` (M2-32).
- CI path filter: a change of docs/spec/09-agents.md selects the gate component `plugins`.
- Root Makefile: `make typecheck` runs mypy inside each Python project, and `make lint` excludes
  `<project>/.venv` and `<project>/generated` from bandit.
- Host service module `services/host` (own `VERSION` and `CHANGELOG.md`) and the Dart package
  `packages/bothy_noise` with the Noise core of the terminal (M2-38); the gates, CI and CodeQL
  cover both.
- Rules for AI coding agents (`AGENTS.md`, loaded by `CLAUDE.md`): public-repository
  hygiene, English only, one pull request per ticket, review findings fixed before merge.
- Specification entry point (`docs/SPEC.md`) with goals, non-goals, hard invariants,
  system overview, chapter index and reading paths per role.
- Specification chapters 00 to 25 (`docs/spec/`): core contract (glossary, data model,
  sync protocol, action catalogue), product with screens as wireframes, architecture,
  API conventions, permissions, identity and devices, agents and reference adapters,
  backend policy with invariants I1 to I6, extensions and SDKs, files, push and
  notifications, search, client architecture, operations, observability, release,
  security, data protection, isolated development environments, Discord import, quality,
  terminal and visualization.
- Terminal as a core feature: host service `bothy-host` on connected hosts, an
  end-to-end encrypted stream (Noise) relayed by the server, a `terminal` capability per
  host, fresh confirmation on every open, audit of every session and invariant I6
  for hosts with a personal AI subscription (`docs/spec/24-terminal.md`, ADR 0068,
  ADR 0070).
- Visualization on two levels: the server's own measurement with a live stream
  `stats.live`, native views in the apps and the web console (live topology of agent
  runs, backend view, delivery, executive page, timeline, TV mode), and the Grafana panel
  plugin "Bothy Live" in the same design language (`docs/spec/25-visualization.md`,
  ADR 0069).
- Normative API contracts: OpenAPI 3.1, AsyncAPI 3.0 and webhooks (`docs/api/`).
- Architecture decision records 0000 to 0070 with a generated index (`docs/adr/`).
- Architecture diagrams as Mermaid sources with rendered SVGs: C4 levels 1 to 3, request
  and event paths, sequences and entity-relationship diagrams (`docs/architecture/`).
- Build plan with milestones, exit criteria and build tickets (`docs/PLAN.md`), a public
  roadmap and one build brief per ticket of milestone M1 (`docs/briefs/`).
- Ruling log, open questions for the owner and product risks (`docs/DECISIONS.md`,
  `docs/OPEN-QUESTIONS.md`, `docs/RISKS.md`).
- Design system: design tokens as the single source with a generator, palette with
  contrast checks, typography, icons, components and preview images in light and dark
  (`design/`, `docs/design/`), including the terminal view with its own color roles
  (16 ANSI colors, cursor, selection) and the chart roles for the statistics views.
- Planned one-command install: a setup wizard (`install.sh`) that checks the host, fetches
  the bundle of its version with a checksum, asks for address, edge and first owner,
  starts the stack and prints the enrollment code (`docs/spec/16-operations.md`, section 3.1).
- Entry screens for every app: server address, login, enrollment code, device waiting
  for approval, pairing, app lock and signed out (`docs/spec/01-product.md`, section 5.6).
- Planned reference integrations: GitHub and Jira apps, monitoring alerts and cron
  messages through incoming webhooks (`docs/spec/11-extensions.md`, section 10).
- Operations: Compose installation bundle, OpenTelemetry Collector configuration,
  monitoring stack with dashboards, SLOs and alerts, OpenTofu and Helm skeletons, and
  guides for configuration, backup and restore, disaster recovery, upgrade and moving to
  a cloud (`infra/`, `docs/operations/`, `docs/observability/`).
- Threat model, and templates for the record of processing activities and the list of
  processors (`docs/security/`, `docs/compliance/`).
- Quality and repository tool chain: `make check` with every gate identical to CI,
  pre-commit and pre-push hooks, pinned tools (`tools/`), workflow policy check, deny-list
  scanner for white-label checks, SonarQube profile and gate shared by the local
  Community Build and SonarQube Cloud, Semgrep project rules, gitleaks rules, Renovate.
  `make docs` also checks that every status with a problem code named in the documentation
  is registered in `x-problem-codes` (`tools/docs/problem_code_refs.py`). The
  `secret-formats` hook rejects values in the Stripe webhook secret format (`whsec_`) that
  GitHub secret scanning reports, the EXAMPLE marker exempts only this product's own token
  formats and generic key-like values from gitleaks, and the tool tests run without a git
  identity, like a fresh CI runner.
- English as the language of all repository content (D-W-94): the term list `docs/glossary.md`
  maps the German terms of earlier planning material, and `make docs` fails on German text in
  tracked files (`tools/docs/language_check.py`).
- Workflows for pull requests, releases, documentation, CodeQL, OpenSSF Scorecard and the
  portability test, all on GitHub-hosted runners with fixed labels.
- Documentation site (MkDocs) whose navigation lists every page; a page missing from the
  navigation fails the strict build. Its start page shows the product with a preview, the
  six pillars, the planned quick start and a reading path per role; `make linkcheck`
  rejects links to the placeholder repository.
- Community files: license (Apache-2.0), contributing guide with DCO, security policy,
  code of conduct, support, governance, issue and pull request templates.
- Folder structure with READMEs for apps, packages, plugins, SDKs, services and each
  server module.
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
- Gate `release-pins` (`tools/ci/components.py pins`): the server version that the product
  bundle pins in `infra/compose/.env.example` is SemVer or the placeholder, and only a product
  beta pins a server pre-release (D-W-98).
- `runners-check` finds release jobs by every spelling of the environment name, rejects an
  environment name with an expression (`environment-not-literal`, D-W-110) and rejects
  `actions: write` and `write-all` on top level and on jobs (`actions-write`, D-W-111).
- Core contract for the briefs of M2 and M3: check without a request (R-AZ-29), the action
  `cost.reconcile.read` with `listCostEstimates`, the system action `webhook_out.transition`,
  the durable event `device.pending`, hints without a thread (R-AZ-25 to R-AZ-31), and the
  data model rules R-DM-42 to R-DM-48; rulings D-W-100 to D-W-108 and OQ-24.
- `brief_deps.py` fails when a start check of a brief names a ticket outside the brief's
  dependency closure, and when a brief calls a ticket of its own closure missing or optional
  (D-W-113).
- Build briefs of the milestones M2 and M3 (`docs/briefs/M2-*.md`, `docs/briefs/M3-*.md`) in the
  brief index with waves and graph, and in the site navigation (D-W-100).
- Build briefs of milestone M4 (`docs/briefs/M4-*.md`, desktop clients) in the brief index with
  waves and graph and in the site navigation, written before M1 closes (D-W-114, which also
  corrects the column "Depends on" of nine M4 tickets in the plan, and D-W-116 for nine more); a
  docs test holds the shape of every brief (title, status line, sections, report placeholders);
  the apps read the server-place keys that steer their own behaviour through `AppConfig` (D-W-115).
- Build briefs M7-01 to M7-09 and M7-22 of milestone M7 (image `bothy-postgres`, backup service,
  restore test, `bothy restore finalize` with a start lock, retention per class, privacy requests,
  deletion journal, maintenance mode, status page, audit checkpoints in object storage) in the brief
  index with waves and graph and in the site navigation, written before M1 closes (D-W-125); core
  contract: the database identity in `instance` (R-DM-50), the numbering of the deletion journal
  (R-DM-52) and the personal event `gdpr.request_due` (R-AZ-35).
- Build briefs M7-10 to M7-16, M7-23 and M7-29 of milestone M7 (server release chain with a trial
  release, API breaking-change gate, desktop and mobile distribution with a test publisher,
  portability test, OpenTofu modules, host service release chain, setup wizard) in the brief index
  with waves and graph and in the site navigation (D-W-127).
- Build briefs M7-17 to M7-21 and M7-24 to M7-28 of milestone M7 (multi-node operation,
  capacity load test, API reference in the docs site, alert routing, status views, token output for
  TypeScript, Grafana panel plugin and its release, demo instance, showcase) in the brief index
  with waves and graph and in the site navigation (D-W-128); core contract: the shared presence
  table `gateway_presence` (R-DM-51).
- Build briefs of milestone M6 for the console session, the statistics in the server and the
  visualization track (`docs/briefs/M6-01`, `M6-05`, `M6-10` to `M6-15`) in the brief index with
  waves and graph and in the site navigation, written before M1 closes (D-W-120); core contract
  for the console session: the stored "View as person" on `auth_sessions` (R-DM-49) and the action
  `access.view_as.end` (R-AZ-32).
- Build briefs M5-01 to M5-07 of milestone M5 (Android device key, phone layouts for iPhone and
  Android, `ClientKit` with the Notification Service Extension, the Android push service, sharing,
  voice and camera) in the brief index with waves and graph and in the site navigation (D-W-120,
  D-W-121).
- Build briefs M5-08 to M5-13 of milestone M5 (app lock, emergency actions, accessibility,
  device E2E and performance, terminal with extra keys bar and its E2E) in the brief index with
  waves and graph and in the site navigation; make target `e2e-mobile` in the table of targets
  created by tickets (D-W-120, D-W-122).
- Build briefs of milestone M6 for the web console (app, CSP, administration, statistics view,
  permission hints, apps and webhooks, instance settings, visualizations in all apps, optional
  Grafana embedding, terminal) in the brief index with waves and graph and in the site
  navigation; make target `console-e2e` in the table of targets created by tickets; core contract
  actions `console.csp_report` (R-AZ-33) and `access.hints.read` (R-AZ-34) (D-W-120, D-W-123).

- Gate `register-check` (`tools/doccheck/register_check.py`): the module register
  `services/server/architecture/modules.yaml` must equal the tables and their modules in
  chapter 03 and the actions, event types and ops of `docs/spec/actions.yaml`, and the module
  tables of chapter 02, the server component diagram and the module READMEs must equal the
  register (ADR 0050); every difference is one finding with file and item.
- `make config-doc-check` in `make gates` (`tools/configdoc/configdoc.py`): the configuration
  reference against the server schema, the chapter tables, the milestone of planned keys
  (`tools/configdoc/planned.yaml`), the names retired by W1 (`tools/configdoc/retired.yaml`) and
  the Compose `stop_grace_period` of every service that runs `serve` (rules D1 to D10).
- Workflow `sonar-config.yml` (manual, `main` only): applies the shared quality gate and
  quality profiles of `quality/sonar/` to SonarQube Cloud with `make sonar-config` and checks
  them with `make sonar-config-check` (public-switch.md, step 12).

### Fixed

- `quality_sync.py` addresses the quality gate by its id on SonarQube Cloud, whose
  `api/qualitygates/create_condition` and `set_as_default` take `gateId` and `id` instead of
  the gate name (https://sonarcloud.io/api/webservices/list, retrieved 2026-10-13), and lets the profiles inherit there from `cloud_parent` of
  `quality/sonar/profiles.json` ("Sonar way comprehensive"), because the cloud has no built-in
  profile "Sonar way" (ADR 0040); before, `make sonar-config` could not apply the setup to the cloud.
- `quality_sync.py` on SonarQube Cloud: when the plan refuses a custom default quality gate
  (HTTP 403 on `api/qualitygates/set_as_default`), `make sonar-config` still applies the
  quality profiles of every language and `make sonar-config-check` reports the gap
  "gate not default: plan does not allow it" instead of failing; every other difference
  still fails the check. On a SonarQube Server the check now also fails when the gate is not
  the default. Before, the first refusal stopped the run before any profile was applied.
- `quality_sync.py` prints the reason the SonarQube Web API gives for a refused call (the
  `errors[].msg` of the response, shortened) after the HTTP status; before, a refusal such as
  HTTP 400 on `api/qualityprofiles/set_default` showed only the status and could not be
  diagnosed from the run log.
- `quality_sync.py` on SonarQube Cloud no longer makes the quality profiles the organization
  default, which plans without that feature refuse (HTTP 400 on
  `api/qualityprofiles/set_default` stopped the run at the first language). It assigns each
  profile and the quality gate to the project instead (`api/qualityprofiles/add_project`,
  `api/qualitygates/select`, project key from `SONAR_PROJECT_KEY`). A refusal by the plan
  becomes a named gap with the server's reason and the next language is still applied;
  `make sonar-config-check` reads the project's profiles and gate and reports a built-in one
  as a gap, while any other profile or gate fails the check. SonarQube Server is unchanged.
- `quality_sync.py` also counts the refusal "Organization ... is not allowed to modify Quality
  gates" (HTTP 403 on `api/qualitygates/select`) as a plan gap; before, it stopped
  `make sonar-config` because only the wording "is not allowed to perform this action" was
  recognized.
- `docs/api/openapi.yaml` 0.2.0: the response `Conflict` names the problem code
  `file_blocked` in the order of the codes; `createHost` and `issueHostPairingCode` take
  `Idempotency-Key` and list `422 idempotency_key_reuse` instead of the exemption `auth_flow`,
  as 06 section 7 asks for a POST that returns a one-time secret (the host and terminal
  operations `pairHost`, `openTerminalSession` and `attachTerminalSession` keep `auth_flow`,
  ruling M1-03/6); by 06 section 13 this changes the idempotency rule of two operations, a
  breaking change made while the API is still `0.x`.
- `docs/api/asyncapi.yaml` 0.1.1: `dispatch` lists the event types in the order of the
  catalogue; the event table of chapter 06, generated from it, now follows that order too.
- Chapter 06, section 7: the table of operations without `Idempotency-Key` lists the host
  pairing and terminal session operations and `renewInvocationToken`, which `openapi.yaml`
  exempts.
- The alert `BothyMalwareScanFailing` now fires when no malware scan has ever produced a
  result; before, it stayed silent exactly when the scanner never worked since the start.
- `make ops-check`: `slo-docs` follows `ticket_alert.disable` (a row may say `Page only` or
  `none`) and reads the `le` bound in every PromQL quoting; `tofu` also checks modules made
  only of `.tofu` or JSON files; `alertmanager` parses the templates named by their
  `/etc/alertmanager/` path and fails on an entry that matches no file or lies outside the
  mount; `collector` fails on a reference to a config provider other than `env`, `file` and
  `yaml` (`${https:...}`, `${s3:...}`) before any `validate`; SIGTERM stops a run like
  SIGINT, killing the running validators and removing the work directory. The alert unit
  tests now also catch an added `for` on an alert without one.
- `tools/bin/fetch_tools.py` reports an empty, cut off or corrupt release archive as an error
  instead of a traceback.

### Changed

- CI: the job `sonar` no longer runs every test a second time. The job `gates` uploads the
  coverage reports of its `make test` as the artifact `sonar-coverage`; `sonar` needs `gates`,
  unpacks them and runs the new target `make sonar-scan` (the scan alone, same quality gate).
  `make sonar` and `make check` still test first. ci.yml also runs on pushes to `main`, so
  SonarQube keeps an analysis of `main` (docs/quality/gates.md). A run on `main` in progress is
  not cancelled by a later push; the newest push waits as the one pending run.
- CodeRabbit: the knowledge base stores learnings for this repository (no Jira, Linear or MCP
  sources), and the review instructions name settled facts that are no findings (`go.sum`
  excluded on purpose, dark default theme, git variables cleared by the pre-push hook, test
  names and loopback or documentation addresses in tests); "SQL only through sqlc" covers data
  access queries, not LISTEN/NOTIFY or advisory locks (CRT-1/3 in docs/DECISIONS.md).
- Design tokens: series and state roles moved along their scales so that they stay apart under
  the simulation: light `color.viz.series.3` `spruce.600` to `spruce.700`, `.6` `moss.600` to
  `moss.900`, `color.warning.fg` `amber.700` to `amber.800`, `color.danger.fg` `rowan.700` to
  `rowan.900`; dark `color.viz.series.3` `spruce.400` to `spruce.300`, `.7` `rowan.400` to
  `rowan.500`, `color.danger.fg` `rowan.300` to `rowan.400`. Every contrast pairing stays green;
  the previews are regenerated (M6-10).
- `make test` measures Go coverage with `-coverpkg=./...`: a statement counts as covered when
  any test of its module runs it, also one in another package, for the gate `coverage` and for
  the new-code coverage in SonarQube (PL/1 in [DECISIONS.md](docs/DECISIONS.md)).
- `make test` gives each Go test binary the time limit `quality.test.go_timeout` instead of the
  default ten, because the module-wide coverage instrumentation slows the larger packages down
  (PL/2 in [DECISIONS.md](docs/DECISIONS.md)).
- The pre-push hook runs `make check` for the parts the push touches: the gates of the touched
  components (the component map of CI), the Python tools gates unless only component source
  files changed, the file hygiene hooks on the changed files and every repository-wide gate; a
  shared input (Makefile, `tools/`, `.github/`, root files) runs the full `make check`, and so
  does `PUSH_FULL=1` (`tools/hooks/push_plan.py`, PL/10 in [DECISIONS.md](docs/DECISIONS.md)).
- Golden tests of the design system compare with `DsGoldenFileComparator`
  (`package:design_system/testing.dart`): a pixel counts as different only when one color
  channel differs by more than `dsCiGoldenChannelTolerance`, so the anti-aliasing of another
  host no longer fails a CI golden recorded on macOS (PL/5 in [DECISIONS.md](docs/DECISIONS.md)).
- CodeRabbit reviews with the `chill` profile and reports bugs, security issues, broken
  contracts and problems that break CI, without style, wording or formatting nits; it asks
  for a source URL and retrieval date only in `docs/spec/**`, `docs/security/**` and
  `docs/adr/**` (CRT-1 in [DECISIONS.md](docs/DECISIONS.md)).
- Chapter 02 "Configuration loading" and the configuration reference describe the loader as built
  by the server: built-in defaults live in the schema, `bothy config print` shows secret
  references instead of `<redacted>`, and `docs/operations/configuration.md` sections 1 to 3 give
  the exact grammar of every kind (now with `list(cidr)` and `url`), the file rules and the output
  of `bothy config check` and `bothy config print`.
- `make coverage` leaves generated Go files (`*.gen.go`) out of the Go total, and the license
  gate requires the Go generated-code marker in every file named as generated Go code; a
  directory named `gen` no longer counts as generated for the license gate, Sonar and the lint
  rules gocognit and goconst of the server module.
- The CI path filter selects the server component for a change to `docs/spec/actions.yaml`,
  from which the server generates its route table and action catalogue.
- CodeQL analyses the Go server module (manual build of `services/server`) in the public
  phase; a test keeps the CodeQL languages in step with the tracked components.
- Configuration reference: section 4 of `docs/operations/configuration.md` is generated from the
  configuration schema of the server plus the chapter tables, with the new columns type and
  status (`active` for keys this server version reads, `planned (<milestone>)` for keys of a later
  ticket); the generator moved from `tools/docs/config_reference.py` to `tools/configdoc/`, and
  `make docs` no longer checks the reference.
- `docs/PLAN.md`: the W1 rows for trusted proxies and database timeouts and the acceptance test of
  M3-06 follow the chapters.
- Pull requests of the dependency bots Dependabot and Renovate pass CI like fork pull
  requests: a run the bot starts gets the generic deny rules and skips SonarQube with a
  "Reduced checks" notice, because GitHub gives Dependabot runs no secrets, and the bot's own
  commits in its own pull request need no DCO sign-off; Conventional Commits still apply, and a
  maintainer's edit of the pull request runs every check (D-DEPS-3).
- Chapter 12 "4.1 Port" and "4.2 File system driver (default)" describe the port contract
  (key grammar, byte-range reads, error contract, idempotent `Delete` and `AbortMultipart`) and the
  file system rules (start check of the root, no symbolic links, temporary files in `.tmp`,
  directory removal on delete, multipart layout and ETag) as built by M3-01; chapter 02
  "13. Configuration loading" item 4 lists the cross-check `file_fs_root_absolute`; ADR 0010
  "Confirmation" names the conformance suite and the property test.
- `make api-clients` (gate G4) runs `generate-check` of `sdk/python` and `sdk/typescript`, fails
  for a client directory with a tracked manifest but without that target and respects
  `COMPONENTS`; `make bootstrap-ci` installs the locked SDK dependencies (`npm ci
  --ignore-scripts`, `uv sync --locked`); `make typecheck` runs mypy inside each Python project and
  `make lint` keeps bandit out of its `.venv` and generated code. New named values
  `quality.coverage.python_min_percent` and `quality.coverage.typescript_min_percent`.
- The CI path filter selects both SDK components for a change under `sdk/testdata/` (shared G5
  fixtures); CodeQL analyses `javascript-typescript` (the TypeScript SDK); Renovate groups the SDK
  code generators.
- Gates for a second Go module and the first Dart package (M2-38): `make perf` runs only for
  the Go modules with performance targets (`PERF_MODULES`), a Dart package without `test/` fails
  `make test` and one without `coverage/lcov.info` fails `make coverage`; CI installs Go for every
  tracked `services/*/go.mod` and Flutter for every tracked Dart package, CodeQL builds the host
  module, a change under `services/host/testdata/noise/` selects the apps gates as well, and
  SonarQube reads the coverage of both new components.

### Security

- Development tools: `katex` (pulled in by mermaid-cli) is pinned through `overrides` to
  0.18.10, which has the fix for
  [GHSA-238p-pmpm-9mq7](https://github.com/advisories/GHSA-238p-pmpm-9mq7); mermaid still
  asks for 0.16, so the Dependabot update could not resolve it. mermaid's own bundle still
  carries katex 0.16 code, which the diagrams do not reach because they contain no math
  (D-DEPS-1 in [DECISIONS.md](docs/DECISIONS.md), whose sources were retrieved on
  2026-10-11).
- Development tools:
  [GHSA-86w9-cpqp-85rv](https://github.com/advisories/GHSA-86w9-cpqp-85rv) in node-forge
  (pulled in by `@sonar/scan`) has no fixed release and is not reachable from the
  scanner's use of node-forge; the alert is dismissed and the OSV exception keeps its
  expiry date (D-DEPS-2 in [DECISIONS.md](docs/DECISIONS.md), whose sources were
  retrieved on 2026-10-11).
- Pull request title and body are deny-list scanned: they become the squash commit on
  `main`. Both deny-list jobs of `ci.yml` read them from the event payload through the
  environment (`PR_TITLE`, `PR_BODY`) and fail `ci-ok` on a finding, `ci.yml` runs again
  when title or body is edited, and `make denylist` checks the planned squash message
  locally (`PR_TITLE`, `PR_BODY` or `PR_BODY_FILE`). The review report stays local; pull
  request bodies are written fresh, without raw reviewer output or local paths
  (`AGENTS.md`, [push discipline](docs/quality/push-discipline.md)). CodeRabbit writes its
  summary into the walkthrough comment instead of the body (`high_level_summary_in_walkthrough`
  in `.coderabbit.yaml`, documented in <https://coderabbit.ai/integrations/schema.v2.json>,
  retrieved on 2026-10-12). The `ci-ok` job starts with
  every run and waits for the other jobs (`tools/ci/ci_ok.py`), so an older green result is
  no longer the latest one while the run after an edit is going (the remaining gaps are
  listed in the push discipline), and identity trailers count only as
  `Name <local@domain>`, with everything but the local part of the address checked by the
  generic rules (LG/1 to LG/5 in [DECISIONS.md](docs/DECISIONS.md)).
