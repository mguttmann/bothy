# M6-09: Manage instance, branding, login providers and feature flags

Status: ready | Depends on: M1-47, M3-10, M6-04 | Milestone: M6

## Goal

After this ticket owners manage the instance in the desktop administration and in the console (01 5.5 "Instance, branding"): the instance settings of the configuration reference section 6 (each key with its type and allowed values from the configuration reference section 6, its current value, version and who changed it, and a form that sends `putInstanceSetting` with `If-Match` on the version), branding (product name with the length rule `branding.product_name_max_chars` and without control or bidi characters, the logo upload through the file upload of M3-10 with the preview from the server's thumbnail, removing the logo, and the accent chosen only from the palette keys `spruce`, `loch`, `heather`, `rowan`, `amber`, `gorse`, `moss` with a live preview of the accent roles of M4-03 in light and dark), the login providers (list, create, edit, disable and delete OIDC and proxy header providers with the configuration of 08 2.6, the redirect URI to register at the provider, and a warning that disabling a driver ends the sessions that came from it, R-ID-12), and the feature flags (list with their evaluated state and a switch per flag with a reason). Every change requires step-up, shows what it changes before the confirmation, and survives a concurrent change by another owner through the version check (`412` shows the newer value). Widget tests and `patrol` web tests in `make console-e2e` cover the flows. Why: the owner configures the instance from the console right after the installation (B5), and branding and login providers change what every person sees (ADR 0044, 08 2).

## Read first

- `docs/operations/configuration.md`: 1 (sources: instance settings and feature flags take effect immediately with `instance.updated`), 6 (instance settings), 7 (feature flags).
- `docs/spec/01-product.md`: 5.2 (step-up rule), 5.5, 12 "Branding per instance" (runtime layer, accent only from the palette, logo served only through the server, `branding.product_name_max_chars`, no control or bidi characters).
- `docs/spec/08-identity.md`: 2 (providers: port, OIDC, Entra example, trusted proxy header with the loopback rule, local accounts), 2.6 (configuration per driver), 16 (R-ID-12), 18.
- `docs/spec/05-actions.md` (core contract, read only): rows `instance.settings.read`, `instance.settings.update`, `instance.branding.update`, `feature_flag.read`, `feature_flag.update`, `identity_provider.read`, `identity_provider.manage`.
- `docs/spec/03-data-model.md` (core contract, read only): `instance_settings`, `feature_flags`, `identity_providers` (as the configuration reference names them).
- `docs/spec/19-security.md` SEC-IN-4 (display names without bidi and control characters).
- `docs/api/openapi.yaml`: `getInstance`, `listInstanceSettings`, `putInstanceSetting`, `updateBranding`, `uploadBrandingLogo`, `deleteBrandingLogo`, `getBrandingLogo`, `listFeatureFlags`, `putFeatureFlag`, `listIdentityProviders`, `createIdentityProvider`, `getIdentityProvider`, `updateIdentityProvider`, `deleteIdentityProvider`.
- ADR 0044 (branding), ADR 0030 (tokens).
- `docs/DECISIONS.md`: D-W-100 and its extension to M5 and M6 (D-W-120); section 10 (format of `D-M6-09-<n>` rows) and the rows of M1-36, M1-46, M1-47, M3-10, M4-03.
- Briefs (merged names win, D-W-100): `M1-46-instance-settings.md`, `M1-47-feature-flags.md`, `M1-36-identity-providers.md`, `M1-41-identity-oidc.md` (provider configuration validation), `M1-43-identity-trusted-header.md` (loopback rule), `M3-10-avatar-logo.md` (logo upload and thumbnails), `M4-03-accent-variants.md` (accent roles), `M6-04-console-admin.md` (console hosting, step-up, `patrol` web suites).
- Code on the base branch: `packages/admin_ui`, `packages/design_system` (accent variants), `apps/console`, `apps/client` (administration), `tools/console-e2e`.

This ticket touches no API operation, no table, no action, no event and no configuration key.

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
7. No en dash and no em dash in any file you write; the `text-rules` pre-commit hook rejects them, and the ARB test of M4-06 rejects them in interface texts. Use a hyphen or rephrase.
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names; provider examples use `https://login.example.com`. Interface texts come from `packages/l10n`.
9. No configuration key is added; kinds and allowed values come from the configuration reference section 6, and the server's `400` decides.
10. Specification first: a change to behaviour updates `docs/spec/` in the same change. The core contract is not edited by this ticket.

Product requirements implemented here:

11. Instance settings, branding, login providers and feature flags in desktop and console; every change requires step-up and states what changes before the confirmation.
12. The accent is selectable only from the palette keys; a free color value cannot be entered (R-PR-08); the preview uses the generated accent roles.
13. The product name follows `branding.product_name_max_chars` and rejects control and bidi characters (the server checks; the form shows the server's reason).
14. Concurrent changes: writes carry the version (`If-Match` or the API's version member); a `412` shows the newer value and asks again.
15. Disabling or deleting a login provider warns that sessions from it end (R-ID-12); the proxy header driver shows the loopback rule (08 2.4) and the server's refusal when the listener is not loopback.
16. Secrets of providers (client secrets, private keys) are entered as references or write-only fields as the API defines them and are never displayed back.

## Scope

Create:

- `packages/admin_ui/lib/src/instance/`: settings list and editor (typed inputs per key from the configuration reference section 6, Design note 1), branding screen (name, logo, accent picker with preview), providers list and editor (OIDC and proxy header forms per 08 2.6, write-only secrets, the redirect URI with a copy action), feature flags list (Design notes 1 to 4).
- Tests (Tests).

Change:

- Desktop administration navigation (`apps/client`) and console route `instance`: host the screens.
- `packages/design_system`: only if the accent preview needs a public builder for a theme with a given accent that M4-04 did not expose; then add it with a test.
- `tools/console-e2e`: the `patrol` web suites of this ticket.
- `packages/l10n` ARB files; `packages/admin_ui/README.md`; root `CHANGELOG.md` under `[Unreleased]`; `docs/DECISIONS.md` section 10 rows `D-M6-09-1` to `D-M6-09-4`.

## Out of scope

- Server behaviour of settings, flags, providers and logo (M1-36, M1-46, M1-47, M3-10).
- The build layer of branding (`branding.yaml`, M5-02, M5-03, M7).
- Process configuration (environment and files) is not editable in the console; settings with place `Server` are shown read-only only if the API exposes them (it does not today).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M6-09-instance-admin`; one writer per worktree.
- Conventional Commits with the package or component as scope (for example `feat(admin-ui): ...`, `test(console): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules); the stack publishes its ports only on `127.0.0.1`; the OIDC provider in the E2E is the fake IdP of M1-41 inside the Compose network.
- Stop only processes and containers you started, by PID or container ID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets; provider secrets in tests are example values marked `EXAMPLE`.
- Docker, the pinned Flutter and the pinned browsers must be available for `make console-e2e`; the ticket must run it once on the build host or stop with `Verdict: BLOCKED <tool> missing`.
- Never lower a threshold, disable a gate, delete a test, raise `repo.max_file_kb` or extend `quality/licenses/allowed.txt`.
- No permission logic in the client (15 1 principle 1).
- If the core contract or `docs/api/` lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M5 and M6 by D-W-120). Where the merged code of an earlier ticket names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Settings editor** (configuration reference 6; Ruling for docs/DECISIONS.md). `InstanceSetting` carries only key, value, version and who changed it, so the editor holds a typed editor per key of the configuration reference section 6, generated or written from that table (kind, allowed values, bounds) and held equal to it by a test, as `AppConfig` is held equal (D-W-115); a key that the server lists but the table does not know shows a read-only value with the note to use the API; the server's `400` with its field error is shown at the input. Saving sends the value with the version; `412` reloads and shows the newer value next to the person's input. Every save needs step-up through the adapter of M6-04 or the desktop step-up of M4-25.

2. **Branding** (01 12; ADR 0044; R-PR-08). Product name: the length limit `branding.product_name_max_chars` is a server key that the API does not deliver, so the field shows no counter of its own and shows the server's `400` (too long, or the SEC-IN-4 reason for control and bidi characters) at the field. Logo: pick an image, upload through the logo operation (M3-10), show the server's thumbnail when ready, remove. Accent: a picker of exactly the seven palette keys, each with a swatch and a preview card rendered with the theme builder of `design_system` for that accent in light and dark; no text field for colors.

3. **Login providers** (08 2, 2.6; R-ID-12). Per driver a form with the fields of 08 2.6; secrets as write-only inputs (empty means unchanged); the redirect URI the instance expects shown with a copy action; "Test configuration" only if the API offers a check (otherwise not built; record it). Disable and delete dialogs say that sessions from this provider end at once. The proxy header driver shows that it works only on a loopback listener and shows the server's `409` explanation otherwise.

4. **Feature flags** (configuration reference 7). The list shows key, description, evaluated state for the caller and the stored value; a switch with a required reason; step-up; `instance.updated` refreshes the list on the desktop (the console reloads after its own action, as it has no live updates).

5. **Rulings for `docs/DECISIONS.md`** (`D-M6-09-<n>`): typed setting inputs and the version conflict (note 1); the accent picker without free values and its preview (note 2); write-only provider secrets (note 3); the reload rule in the console (note 4).

## Tests (write them first)

- Widget and golden (`packages/admin_ui/test/instance/`, fake server): each settings kind of the configuration reference section 6 and an unknown key shown read-only; a `412` conflict; branding with the server's `400` for a too long name shown at the field, a rejected bidi name, logo upload and removal, the accent picker with exactly seven options and the preview in light and dark; provider forms for OIDC and proxy header with write-only secrets; flags with reason; all in light and dark, text scaling 1 and 2, de and en; every save path triggers the step-up (fake returns `401 step_up_required` first).
- `accent_picker_test.dart` (PLAN row): no input accepts a color value; the selection sends only a palette key.
- `patrol` web suites in `make console-e2e`: change a setting with step-up and see it after reload; set the accent and see the console theme change; add an OIDC provider pointing to the fake IdP and see its button on E2 after logout; switch a flag and see `getInstance` report it.

Commands that must pass, from the repository root:

- `cd packages/admin_ui && flutter analyze --fatal-infos && flutter test --coverage`
- `cd apps/console && flutter test --coverage` and `cd apps/client && flutter test --coverage`
- `make console-e2e` on the build host
- `make arch`, `make lint`, `make fmt-check`, `make coverage`
- `make check`

## Acceptance criteria

- [ ] Widget and `patrol` web tests pass in desktop and console.
- [ ] The accent is selectable only from the palette; every change requires step-up.
- [ ] Concurrent changes are detected; provider secrets are write-only; disabling a provider states its effect.
- [ ] README, ARB files, CHANGELOG and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comments on every public Dart symbol; comments explain why.
- `packages/admin_ui/README.md` updated in English.
- Rulings of Design note 5 recorded in `docs/DECISIONS.md` section 10.
- Root `CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check` and of `make console-e2e`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven.
5. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
