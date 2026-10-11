# M1-46: Instance settings, branding and discovery

Status: ready | Depends on: M1-36 | Etappe: M1

## Goal

After this ticket the `instance` module serves the public discovery document, the instance view
for every signed-in principal, the documentation of every problem type, and lets owners change
instance settings and branding with step-up and audit. Settings are typed and validated in one
code registry; the accent colour is checked against keys generated from the design tokens, so no
free colour value or emoji can enter the product (ADR 0044, R-PR-08), and the same generated keys
serve space colours, space icons and forum tag colours in later tickets. Every change writes
`instance.updated` with exactly the changed keys.

## Read first

- `docs/PLAN.md`: section 3 "Arbeitsweise je Ticket"; section 7 rows M1-15, M1-17, M1-36, M1-37,
  M1-46, M1-47, M1-52, M3-10; "Voraussetzungen aus dem Kern-Vertrag" (row `instance.settings.read`).
- `docs/spec/01-produkt.md`: "Sprachen" (9), "Branding je Instanz" (12).
- `docs/spec/03-datenmodell.md`: 4.1 tables `instance`, `instance_settings`; "Grundsätze" items
  10 (optimistic lock) and 13 (no `null`).
- `docs/spec/05-aktionen.md`: 4.1 row `instance.discovery.read`; 4.2 rows `instance.read`,
  `instance.settings.read`, `instance.settings.update`, `instance.branding.update`; 2.6 (no scope
  for `instance.read`); 6 row `instance.updated`.
- `docs/spec/06-api.md`: "Grundregeln" (2), "Authentifizierung je Prinzipal-Art" (3, anonymous
  operations), "Fehler" (5), "Nebenläufigkeit" (8, items 1 to 3: branding ETag derived from
  several setting versions; settings with optional `If-Match`), "Versionierung, Deprecation,
  Kompatibilität" (13, item 1: discovery reports `api_versions`, `spec_version`, `features`),
  16 "Endpunkte" groups Discovery and Admin.
- `docs/spec/08-identitaet.md`: "Erstinstallation" (4: discovery answers with `setup_required`
  before setup).
- `docs/spec/16-betrieb.md`: "Konfiguration" (5.1 item 6: settings are not process
  configuration).
- `docs/spec/18-release.md`: "Kompatibilität zwischen App und Server" (9, item 2: discovery
  delivers `client.min_version.<platform>` and `client.recommended_version.<platform>`).
- `docs/spec/19-sicherheit.md`: SEC-IN-4 (control and bidi characters in display texts).
- `docs/operations/configuration.md`: 6 "Instanz-Einstellungen"; sections "03 Datenmodell",
  "12 Dateien", "18 Release" of section 4 for the limit and version keys.
- `docs/spec/02-architektur.md`: 4.2, 4.3 (row `instance`), 4.4 (`instance` depends on nothing),
  4.5 "Querschnitts-Ports", 5 "Schichten im Modul", 6 "Code-Layout" (paragraph on name-bound
  places), 7.2 "go-arch-lint", 7.3.
- ADRs: `docs/adr/0044-arbeitsname-und-branding-als-konfiguration.md`,
  `docs/adr/0030-design-tokens-als-eine-quelle-mit-generator.md`,
  `docs/adr/0013-spec-first-openapi-31-und-asyncapi-30-normativ.md`,
  `docs/adr/0015-fehlerformat-rfc-9457-idempotenz-paginierung.md`.
- `design/build/tokens.resolved.json` (`contexts.theme.light` and `.dark` keys
  `color.category.<key>.*`; `icons.space-icons`), `design/build/README.md`.
- `docs/api/openapi.yaml`: operations `getDiscovery`, `getInstance`, `listInstanceSettings`,
  `putInstanceSetting`, `updateBranding`, `getProblemType`; schemas `Discovery`, `GatewayInfo`,
  `ClientVersionRequirement`, `AuthProviderInfo`, `PublicBranding`, `Branding`,
  `BrandingUpdate`, `InstanceInfo`, `InstanceLimits`, `EvaluatedFlag`, `InstanceSetting`,
  `InstanceSettingPut`, `InstanceSettingList`, `ProblemType`, `Problem`, `FieldError`,
  `InstanceUpdatedPayload`; parameters `KeyPath`, `CodePath`, `IfMatchRequired`,
  `IfMatchOptional`. `docs/api/asyncapi.yaml` `info`.
- Code of earlier tickets: `internal/instance` (M1-15 instance row and `Counters`, M1-32 keys),
  `internal/identity` (M1-26 display-name check, M1-36 identity providers), M1-17 generator of
  `api/routes` and `api/operations.yaml`, M1-18 problem catalogue in `internal/platform/httpserver`,
  M1-07 configuration schema, M1-05 `architecture/modules.yaml` and `tools/archgen`.
- Tables: `instance`, `instance_settings`. Action ids: `instance.discovery.read`,
  `instance.read`, `instance.settings.read`, `instance.settings.update`,
  `instance.branding.update`.

## Fixed requirements that apply

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
6. Test first: every behaviour gets a test that is red before the code exists (plan section 3).
7. No en dash or em dash in any text you write; white label (people Alex, Sam, Robin, domains
   under example.com); configuration only through named keys from
   `docs/operations/configuration.md`.
8. 01 12 and ADR 0044: `branding.accent_color` is a key of the category colours `spruce`,
   `loch`, `heather`, `rowan`, `amber`, `gorse`, `moss`; a hex value, a CSS colour or an emoji
   is rejected. The server checks against keys from `design/build/tokens.resolved.json`.
9. 03 `instance_settings`: typed key-value pairs, value checked against the key's schema,
   `version` as optimistic lock; keys and defaults as in configuration.md section 6.
10. 05 4.2: reading all settings is owner only; changing settings and branding is owner only
    with step-up and audit (allow and deny).
11. 06 8: branding changes need `If-Match` (`428` without, `412 version_conflict` with
    `current` on mismatch); the branding ETag is derived from the versions of its setting keys;
    settings take an optional `If-Match`.
12. 05 6 and 16 6.2: `instance.updated` names only the changed keys, never values.
13. 08 4: discovery answers before setup with `setup_required: true`.
14. 06 13 item 1 and 18 9 item 2: discovery reports `api_versions`, `spec_version`, `features`
    and per platform the minimum and recommended app version.
15. 06 5: `GET /api/v1/problems/{code}` documents every known problem code.
16. 19 SEC-IN-4: `branding.product_name` with control or bidi control characters is rejected.

## Scope

Paths are relative to `services/server/` unless they start with `docs/`, `design/` or `Makefile`.

1. **Specification** (in place from planning, spec first; change it only where the
   implementation proves it wrong): `docs/api/openapi.yaml` (`Discovery.setup_required` required,
   `ClientVersionRequirement.recommended_version`, `InstanceSetting` with `updated_at` optional and
   version `0` for defaults, `FieldError.pointer` with `#path/<name>`, `x-problem-codes` with
   `setup_required`); `docs/api/asyncapi.yaml` `info.x-protocol-versions: [1]` (04 6.1); 06 5
   item 3; 03 `instance_settings` note (version `0`) and `branding.product_name_max_chars`; 02 4.3
   (`instance` declares `LoginOptionsSource`, implemented by `identity`), 4.5 and 7.2
   (`kernel/designkeys` in kernel-core), 6 (the list of places with the product name, by kind);
   ADR 0044 (accent list). To do here: `docs/DECISIONS.md` the rulings under Design notes, in the
   file's existing format.
2. **Generated design keys** `internal/kernel/designkeys`:
    - generator `tools/designkeysgen` (Go `main`, beside M1-05's `tools/archgen`; follow its
      location convention), reading `design/build/tokens.resolved.json` and writing
      `internal/kernel/designkeys/designkeys_gen.go`;
    - `internal/kernel/designkeys/doc.go` with the `//go:generate` line, so `make codegen-check`
      regenerates and diffs it (verify that the target runs `go generate` for this package; if it
      uses an explicit list, add the package);
    - `internal/kernel/designkeys/accent.go` (hand written, see Design notes);
    - register the package as kernel-core in the module register (where M1-05 lists
      `kernel/ids`, `kernel/errs`, `kernel/clock`), so domains of all modules may use it, and
      regenerate `.go-arch-lint.yml`.
3. **Instance domain** `internal/instance/domain`: `settings.go` (registry), `branding.go`,
   `discovery.go`; pure functions only.
4. **Display-text check.** `branding.product_name` uses `internal/kernel/textcheck` (M1-26,
   kernel-core, 02 4.5) and the length `branding.product_name_max_chars` (03 9). Do not write a
   second copy.
5. **Port** `internal/instance`: interface `Settings` (`Branding(ctx) (Branding, error)`,
   `LocaleDefault(ctx) (string, error)`), the DTOs `Branding`, `LoginOptions`, `ProviderInfo`,
   the exported func `SupportedLocales() []string`, and the declared source port
   `LoginOptionsSource` (Design notes).
6. **Application** `internal/instance/app`: `GetDiscovery`, `GetInstanceInfo`, `ListSettings`,
   `PutSetting`, `UpdateBranding`, `GetProblemType`.
7. **Adapter postgres** `internal/instance/adapters/postgres`: migration
   `migrations/<next>_instance_settings.sql` (table exactly as 03 4.1; renumber at merge if a
   parallel ticket took the number), `queries/settings.sql`, sqlc output.
8. **Adapter httpapi** `internal/instance/adapters/httpapi`: strict-server methods
   `GetDiscovery`, `GetInstance`, `ListInstanceSettings`, `PutInstanceSetting`,
   `UpdateBranding`, `GetProblemType`.
9. **Identity side of the source port**: `internal/identity/adapters/loginoptions` implementing
   `instance.LoginOptionsSource` over the identity provider store of M1-36 and the identity
   configuration value `auth.local_password_enabled`; wired in `internal/wire`.
10. **Generated API constants**: extend the M1-17 generator of `api/routes` with
    `SpecVersion string` (from `info.version` of `docs/api/openapi.yaml`) and
    `GatewayProtocolVersions []int` (from `info.x-protocol-versions` of
    `docs/api/asyncapi.yaml`). If M1-17 already exposes the spec version, reuse it.
11. **Configuration schema** (M1-07): make sure these keys exist with the defaults of their
    chapter tables, and hand them to `instance` as one typed value from `internal/wire`:
    `message.max_content_bytes`, `message.max_attachments`, `file.max_upload_bytes`,
    `thread.title_max_chars`, `voice.max_duration`, `client.min_version.ios`,
    `client.min_version.android`, `client.min_version.macos`, `client.min_version.windows`,
    `client.recommended_version.ios`, `client.recommended_version.android`,
    `client.recommended_version.macos`, `client.recommended_version.windows`. Validate the
    version keys as SemVer without `v` when set. Regenerate `docs/operations/configuration.md`
    (`make config-reference`).
12. **Freischalt-Liste** `api/operations.yaml` (P-04): the six operations.
12a. **Language of new people** (PLAN row M1-46): `createUser` (M1-31) stores `locale.default` as
    `users.locale` when the request has no `locale`; M1-31 used the 03 column default until now.
    Read the setting through the `Settings` port (identity may use `instance`, 02 "Erlaubte
    Abhängigkeiten"); test `TestCreateUser_UsesLocaleDefault` (setting `en`, request without
    `locale`, stored `en`).
13. **Documentation**: `internal/instance/README.md` (settings registry, branding version,
    discovery sources, source port), `internal/kernel` README or package docs,
    `services/server/README.md` ("Namensgebundene Stellen": the default product name constant),
    `services/server/CHANGELOG.md` under `Unreleased`.
14. **Places with the product name** (02 "6. Code-Layout", ADR 0044 Confirmation): a test
    `tools/ci/tests/test_product_name_places.py` searches tracked code and configuration (not
    `docs/`) for the working name, case-insensitively, and allows it only at the kinds 02 6 lists
    (module path, binary and `cmd/bothy`, `BOTHY_` variables, image names and container paths,
    database roles, telemetry, alert and SLO prefixes, the token prefix with its gitleaks rule,
    `DefaultProductName`); every other hit fails with file and line. Add it to the existing
    `tools/ci` test run.

## Out of scope

- Feature flags, their evaluation and the `features` list of `getInstance`: M1-47 (this ticket
  returns `features: []`).
- Logo upload, download and deletion (`uploadBrandingLogo`, `getBrandingLogo`,
  `deleteBrandingLogo`): M3-10. This ticket never writes `branding.logo_file_id`.
- JWKS and webhook keys (`getJwks` M1-32, `getWebhookKeys` M2).
- The setup lock (`503 setup_required` for every other route): M1-37.
- Space colours and icons (M1-52) and forum tag colours (M1-53) use the generated keys but are
  not built here.
- The setup lock itself and `bothy setup`: M1-37 (the problem code `setup_required` is in the
  specification already).

## Boundaries

- A new migration needs its line in `migrations.sha256`: run `make migrations-lock` after creating
  the file and before `make check`; never edit a merged migration (M1-01, ADR 0007).
- Work in a dedicated git worktree on the branch `feat/M1-46-instance-settings`.
- Conventional Commits with module scope (for example `feat(instance): discovery and branding`)
  and DCO sign-off (`git commit -s`).
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope above except `services/server/CHANGELOG.md` and
  `docs/DECISIONS.md`.
- Follow the host rules supplied by the launcher.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`; remove
  them by their literal path at the end. PostgreSQL runs only as the throwaway container of
  `pgtest` on a random port bound to `127.0.0.1`.
- Never read or print secrets. No network calls except package downloads for pinned tools.

## Design notes

**Design keys.** The generator reads `contexts.theme.light` and `contexts.theme.dark`, collects
the second path segment of every key `color.category.<key>.<role>`, and fails if the two themes
give different sets. It reads `icons.space-icons` as the icon list. Output (sorted, no
duplicates):

```go
func CategoryColors() []string // copy of the generated list
func IsCategoryColor(key string) bool
func SpaceIcons() []string
func IsSpaceIcon(key string) bool
func AccentColors() []string // hand written in accent.go
func IsAccentColor(key string) bool
```

`accent.go` holds the accent list as an explicit constant of the seven keys of ADR 0044 and 01
12 (`spruce`, `loch`, `heather`, `rowan`, `amber`, `gorse`, `moss`), not derived from the tokens:
`granite` is the neutral palette of all surfaces, and a new category colour becomes an accent only
after the contrast gate (ADR 0044). Tests assert that every accent is a generated category colour
and that the constant equals the list in 01 12. Keys are case sensitive;
`Spruce`, ` spruce` and `#5865f2` are not keys.

**Settings registry** (`domain/settings.go`). One entry per key of configuration.md section 6:

| Key | JSON type | Default | Route | Validation |
|---|---|---|---|---|
| `branding.product_name` | string | `DefaultProductName` | branding | trimmed length 1 to `branding.product_name_max_chars`, display-text check (SEC-IN-4) |
| `branding.accent_color` | string | `spruce` | branding | `designkeys.IsAccentColor` |
| `branding.logo_file_id` | string (UUID) | absent | branding, written only by M3-10 | UUIDv7 |
| `locale.default` | string | `de` | settings | one of `SupportedLocales()` = `de`, `en` (01 9) |

`DefaultProductName = "Bothy"` is the one constant with the working name in this module (ADR
0044 counts `branding.*` among the name-bound places). `SupportedLocales()` lives in the port so
that `identity` can validate `users.locale` against the same list; if M1-31 holds its own list,
point it to this function in the same change. A test parses the table in configuration.md
section 6 and fails if keys or defaults differ from the registry.

**Reading a setting.** The stored row wins; a key without a row has its registry default and
`version = 0`. `listInstanceSettings` returns every registered key in key order: rows with
`version`, `updated_at`, `updated_by`; defaults with `version = 0` and without `updated_at` and
`updated_by`. Rows whose key is not registered are not listed (they cannot exist: the writer
only accepts registered keys).

**`putInstanceSetting`** (owner, step-up, audit, `IfMatchOptional`): unknown key: `404
not_found`. Key with route `branding`: `400 validation_failed` with `errors[0] = {pointer:
"#path/key", code: "use_branding_endpoint"}`. Invalid value: `400 validation_failed` with
pointer `/value`. `If-Match` present and different from the current version (`0` for a default):
`412 version_conflict` with `current`. Same value as stored: no write, no event, `200` with the
current setting. Otherwise insert with `version = 1` or update with `version + 1`,
`updated_by = actor`, `updated_at = t`; append `instance.updated` with `changed_keys = [key]`;
audit allow with `details = {key}` and no value, so that one rule covers every future key,
including keys whose values are personal. Response `200` with `ETag: "<version>"`.

**Branding version.** `Branding.version` and its ETag are the sum of the versions of
`branding.product_name`, `branding.accent_color` and `branding.logo_file_id` (absent row counts
`0`). Every write raises at least one of them by one, so the sum rises strictly and works as an
optimistic lock (06 8 item 1).

**`updateBranding`** (owner, step-up, audit, `IfMatchRequired`): body `BrandingUpdate`. PUT
replaces both fields: an absent `accent_color` means the default `spruce`, written as a value
(not by deleting the row, which would lower the version sum). Missing `If-Match`: `428
precondition_required`; mismatch: `412 version_conflict` with `current` = the `Branding`
object. Only keys whose value changes are written and listed in `changed_keys`; no change at
all: no write, no event, `200`. Audit allow with `details = {changed_keys}`. Errors: pointer
`/product_name` code `invalid_text` or `required`; pointer `/accent_color` code
`unknown_color_key`.

**`getInstance`** (`instance.read`, any active principal including agent and app tokens,
`ETag`): `InstanceInfo` with `instance_id`, `branding` (from settings), `locale_default`,
`spec_version` (`routes.SpecVersion`), `features: []` (M1-47 fills it), `limits` from the typed
configuration value (`voice.max_duration` in milliseconds). The ETag is a strong hash of the
canonical response body (SHA-256, base64url, first 22 characters), because the body differs per
caller once flags exist.

**`getDiscovery`** (`instance.discovery.read`, public, works before setup):

| Field | Source |
|---|---|
| `instance_id` | `instance.instance_id` |
| `setup_required` | `instance.setup_completed_at IS NULL` |
| `api_versions` | `["v1"]` |
| `spec_version` | `routes.SpecVersion` |
| `features` | `[]` until a server capability beyond the base contract exists; the list is a constant in `domain/discovery.go` with a comment on how to add one |
| `gateway` | `{url: "/api/v1/gateway", protocol_versions: routes.GatewayProtocolVersions}` |
| `branding` | `PublicBranding{product_name, accent_color, logo_url}`; `logo_url = "/api/v1/instance/branding/logo"` only when `branding.logo_file_id` is set |
| `auth_providers` | `LoginOptionsSource`: enabled providers, ordered by name |
| `local_password_enabled` | `LoginOptionsSource`: `auth.local_password_enabled` and an enabled provider of kind `local` |
| `jwks_uri`, `webhook_keys_uri` | `/api/v1/instance/jwks`, `/api/v1/instance/webhook-keys` |
| `min_client_versions` | one entry per platform with `client.min_version.<p>` or `client.recommended_version.<p>` set, in the order ios, android, macos, windows; array always present |

Arrays are always sent (possibly empty); optional scalars are omitted, never `null`.

**Source port** (declared in `internal/instance`, implemented in `identity`):

```go
// LoginOptionsSource tells discovery which sign-in ways exist. Declared here because
// instance may not import identity (02 4.4); identity implements it (02 4.1 item 4).
type LoginOptionsSource interface {
    LoginOptions(ctx context.Context) (LoginOptions, error)
}
type LoginOptions struct { Providers []ProviderInfo; LocalPasswordEnabled bool }
type ProviderInfo struct { ID ids.ID; Kind string; Name string } // kind: oidc, trusted_header, local
```

**`getProblemType`** (`instance.discovery.read`, public): look up `code` in the problem
catalogue that `internal/platform/httpserver` uses for problem bodies (M1-18). Its source is the
machine-readable problem list `x-problem-codes` in `docs/api/openapi.yaml` that also generates
the table in 06 5 (R-API-23, checked by M1-03). Map to `ProblemType{code, status, title,
description, client_action}`; unknown code: `404 not_found`. If the catalogue in code lacks
`description` or `client_action`, extend its generator; never add a second hand-written list.

**Events.** `instance.updated`: `resource = {kind: instance, id: instance_id}` (no version),
empty `scope`, payload `{changed_keys}` sorted.

**Rulings for `docs/DECISIONS.md`:**

1. Branding keys change only through `updateBranding`; the settings route rejects them. Reason:
   one action per write path and one audit action per change.
2. Branding version is the sum of the three branding row versions.
3. A setting with its default has `version = 0` and no `updated_at`.
4. Accent keys are the explicit list of ADR 0044, held in `kernel/designkeys`.
5. `kernel/designkeys` is a kernel-core package (02 4.5). Reason: `chat` (M1-52, M1-53) needs the
   same keys and may not import `instance` (02 4.4).
6. `local_password_enabled` in discovery means configuration allows it and a `local` provider
   is enabled.

## Tests (write them first)

1. `tools/designkeysgen`: golden test on a small fixture token file; a fixture where light and
   dark differ fails; a hand edit of `designkeys_gen.go` makes `make codegen-check` fail (run it
   in the test of the Makefile target or document the manual proof in the report).
2. `internal/kernel/designkeys`: `TestAccentColorsMatchADR0044`, `TestIsAccentColorRejects`
   (`#5865f2`, `rgb(0,0,0)`, `Spruce`, `granite`, an emoji such as U+1F535, empty string),
   `TestSpaceIconsFromTokens`.
3. `internal/instance/domain`: `TestSettingsRegistryMatchesConfigurationDoc` (parses
   configuration.md section 6), `TestValidateProductName` (empty, spaces only, U+202E, U+0007,
   valid), `TestValidateLocale`, `TestBrandingVersionSum`, `TestDiscoveryMinClientVersions`.
4. Integration against PostgreSQL 18 with the real HTTP chain, real authentication and step-up
   (identity helpers of M1-34 and M1-35), real audit and event log:
    - `TestDiscovery_BeforeAndAfterSetup` (`setup_required` true, then false after setting
      `setup_completed_at`), `TestDiscovery_Anonymous`, `TestDiscovery_SpecVersionMatchesOpenAPI`,
      `TestDiscovery_ProvidersFromIdentity` (disabled provider absent), `TestDiscovery_NoLogoUrl`.
    - `TestGetInstance_AnyPrincipal` (person, agent token), limits equal configuration,
      `features` is an empty array, ETag stable for the same body.
    - `TestListSettings_OwnerOnly` (Sam gets `403 forbidden` with `reason` `owner_only`), defaults with `version = 0`.
    - `TestPutSetting_LocaleDefault` (event `instance.updated` with `["locale.default"]`, audit,
      ETag), `TestPutSetting_UnknownKey404`, `TestPutSetting_BrandingKeyRejected`,
      `TestPutSetting_IfMatchConflict412`, `TestPutSetting_SameValueNoEvent`,
      `TestPutSetting_StepUpRequired`.
    - `TestUpdateBranding_HexAndEmojiRejected`, `TestUpdateBranding_PaletteKeyAccepted`,
      `TestUpdateBranding_OnlyChangedKeysInEvent`, `TestUpdateBranding_IfMatchRequired428`,
      `TestUpdateBranding_VersionConflict412`, `TestUpdateBranding_AbsentAccentResetsDefault`,
      `TestUpdateBranding_NonOwner403`, `TestUpdateBranding_AuditOnAllowAndDeny`.
    - `TestGetProblemType_KnownAndUnknown`.
5. Every response passes the response validation of the test build.

Commands that must pass:

```sh
cd services/server && go test -race ./internal/instance/... ./internal/kernel/... ./internal/identity/... ./tools/...
make codegen-check
make config-doc-check
make api-lint
make arch
make check
```

## Acceptance criteria

- [ ] Hex values, CSS colours and emoji are rejected as accent; the seven palette keys are
      accepted; the generated key list is current (`make codegen-check`).
- [ ] Discovery reports `setup_required`, `api_versions`, `spec_version`, `features`, gateway
      protocol versions and minimum and recommended versions per platform; it works without
      sign-in and before setup.
- [ ] `instance.updated` names only changed keys; a no-op writes no event.
- [ ] Settings and branding changes need an owner with fresh step-up and are audited on allow
      and deny; `listInstanceSettings` is owner only.
- [ ] Branding needs `If-Match`; settings accept it optionally; conflicts answer `412` with
      `current`.
- [ ] Every known problem code is served by `getProblemType` from the one catalogue.
- [ ] The spec changes of Scope item 1 are in the same change, and `make check` is green.

## Definition of done

Code, tests and documentation are complete: module README updated, doc comments on every public
symbol, an ADR if you took an architecture decision beyond this brief, CHANGELOG entry under
`Unreleased` in `services/server/CHANGELOG.md`. `make check` is green. An independent review and
the red team (`docs/quality/red-team-gate.md`) have run over the diff with their verdicts
recorded in the report. Merge locally into the base branch only if both are green; then remove
the worktree with `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then the
commits, the files with line counts, the test results with counts (including the red run of each
new test before the code), what is not proven, and follow-ups that could not be done in scope
with the reason. Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
