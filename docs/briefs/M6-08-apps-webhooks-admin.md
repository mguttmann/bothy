# M6-08: Manage apps, webhooks and incoming webhooks

Status: ready | Depends on: M2-26, M2-27, M6-04 | Milestone: M6

## Goal

After this ticket owners manage extensions in the desktop administration and in the console (01 5.5 "Apps, webhooks"; 11 4 to 8): installing an app by uploading its manifest file (never a URL), with the consent dialog of 11 4.1 that shows in plain text the scopes, events and target hosts, commands, components, grants and data outflow, lets the owner choose optional scopes and the concrete grant targets with the "Who has access?" preview, and confirms with step-up; the app token and the webhook key ID are shown exactly once; updating an app with a new manifest shows the diff and asks for new consent (`app.scopes.update`); suspending, resuming and uninstalling with the effects of 11 4.3 explained before confirming; outgoing webhook subscriptions per installation (event types within the granted events, target URL within `webhook.allowed_hosts`), their state, the deliveries with result and next attempt, and redelivery; incoming webhooks per channel (create, shown once, revoke), with the "incoming app" option of 11 8.1 for owners. Every change requires step-up. Widget tests and `patrol` web tests in `make console-e2e` cover the flows, and a test proves that the token of an incoming webhook appears exactly once. Why: apps and webhooks move data out of the instance, so the owner must see and confirm every outflow (11 4.1), and a token shown twice would weaken "shown once" (08 12).

## Read first

- `docs/spec/11-extensions.md`: 3 (manifest format and validation), 4 (installation, update, suspension and uninstallation with the table of 4.3), 5 (scopes and grants), 6 (limits; `webhook.max_pending_per_subscription`, automatic `disabled`), 7 (outgoing webhooks: subscription rules, signature, `webhook-keys`), 8 (incoming webhooks: creation, "incoming app", call, token once), 14, 16.
- `docs/spec/05-actions.md` (core contract, read only): rows `app.*`, `webhook_out.*`, `webhook_in.*`, `token.create`, `token.revoke`, `grant.create`, `access.preview`; 2.6 (scopes table).
- `docs/spec/01-product.md` 5.2 (administration rules), 5.5 (navigation).
- `docs/api/openapi.yaml`: `listApps`, `validateManifest`, `installApp`, `upgradeAppManifest`, `createGrant`, `previewGrant`, `createToken`, `revokeToken`, `getApp`, `setAppScopes`, `suspendApp`, `resumeApp`, `uninstallApp`, `listWebhookSubscriptions`, `createWebhookSubscription`, `updateWebhookSubscription`, `deleteWebhookSubscription`, `listWebhookDeliveries`, `redeliverWebhook`, `listIncomingWebhooks`, `createIncomingWebhook`, `revokeIncomingWebhook`, `previewGrant`, `getWebhookKeys`; schemas `AppManifest`, `AppScope` and the install request and response.
- `docs/DECISIONS.md`: D-W-100 and its extension to M5 and M6 (D-W-120); section 10 (format of `D-M6-08-<n>` rows) and the rows of M2-24 to M2-27.
- Briefs (merged names win, D-W-100): `M2-24-apps-install.md`, `M2-26-webhooks-outgoing.md`, `M2-27-webhooks-incoming.md` (server behaviour, errors, the once-only token), `M6-04-console-admin.md` (console hosting, step-up in the browser, `patrol` web suites), the brief of M4-27 (grant wizard and preview for the grant targets).
- Code on the base branch: `packages/admin_ui`, `apps/console`, `apps/client` (administration), `tools/console-e2e`.

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
8. White label: examples, manifests and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names. Interface texts come from `packages/l10n`.
9. No configuration key is added; limits shown (`webhook.allowed_hosts`, `webhook.max_pending_per_subscription`) come from server responses.
10. Specification first: a change to behaviour updates `docs/spec/` in the same change. The core contract is not edited by this ticket.

Product requirements implemented here:

11. Installation from a manifest file only, with the consent dialog of 11 4.1 in plain text; grants with concrete targets chosen by the owner; step-up; token and key ID shown exactly once.
12. Update with a diff and new consent for new scopes, events, hosts or grants (11 4.2).
13. Suspend, resume and uninstall with the effects of 11 4.3 stated before confirming.
14. Outgoing subscriptions within the granted events and allowed hosts; deliveries with state and redelivery; a `disabled` subscription shows its reason ("gap", failures).
15. Incoming webhooks per channel with the token shown once and the "incoming app" option for owners (11 8.1).
16. Every change requires step-up; the screens show what the server returns and decide nothing about permissions themselves.
17. Tokens shown once are never written to browser storage, logs, the local database or crash reports; they leave the screen when the person closes the dialog.

## Scope

Create:

- `packages/admin_ui/lib/src/apps/`: apps list and detail, install flow (file picker, manifest validation errors from the server, consent dialog, scope and grant target selection with the preview, confirmation), update diff, suspend and uninstall dialogs with the 11 4.3 table, outgoing subscriptions and deliveries, redelivery (Design notes 1 to 3).
- `packages/admin_ui/lib/src/incoming_webhooks/`: list per channel, create (with the "incoming app" option for owners as a client-side composition, Design note 5), the once-only token dialog with copy, revoke (Design note 4).
- `packages/admin_ui/lib/src/secrets/once_only_secret.dart`: the shared once-only dialog used for app tokens, incoming webhook tokens and, where the screens of M4-28 show agent tokens, refactored to use it (Design note 4).
- Tests (Tests).

Change:

- Desktop administration navigation (`apps/client`) and console routes (`apps`): host the screens.
- `tools/console-e2e`: the `patrol` web suites of this ticket.
- `packages/l10n` ARB files (consent texts, 11 4.3 effects); `packages/admin_ui/README.md`; root `CHANGELOG.md` under `[Unreleased]`; `docs/DECISIONS.md` section 10 rows `D-M6-08-1` to `D-M6-08-4`.

## Out of scope

- Server behaviour of apps and webhooks (M2-24 to M2-27); the ready-made integrations (M2-49, M2-50).
- Agents' webhook subscriptions (`listAgentWebhookSubscriptions`, managed with the agents screen of M4-28).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M6-08-apps-webhooks-admin`; one writer per worktree.
- Conventional Commits with the package or component as scope (for example `feat(admin-ui): ...`, `test(console): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules); the stack publishes its ports only on `127.0.0.1`; webhook targets in the E2E are a receiver inside the Compose network listed in `webhook.allowed_hosts` of the E2E configuration.
- Stop only processes and containers you started, by PID or container ID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets; tokens in tests are compared and discarded, never logged.
- Docker, the pinned Flutter and the pinned browsers must be available for `make console-e2e`; the ticket must run it once on the build host or stop with `Verdict: BLOCKED <tool> missing`.
- Never lower a threshold, disable a gate, delete a test, raise `repo.max_file_kb` or extend `quality/licenses/allowed.txt`.
- No permission logic in the client (15 1 principle 1).
- If the core contract or `docs/api/` lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M5 and M6 by D-W-120). Where the merged code of an earlier ticket names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Install flow** (11 4.1; M2-24; Ruling for docs/DECISIONS.md). `installApp` answers `201 App` without a token and without grants (M2-24, `AppInstall`, `App`); grants come from `createGrant` per target and the token from `createToken`. Step 1: pick a manifest file (`.json` or `.yaml` as 11 3.1 allows) and validate it with `validateManifest`, which returns the parsed manifest or the rule that failed. Step 2: the consent dialog lists, from the server's answer or the parsed manifest: required and optional scopes with their meaning from 05 2.6 in plain text, events, target hosts of outgoing data, commands, components, requested grants with a target picker per grant, `data_egress`; each grant target choice runs `previewGrant` and shows the plain text. Step 3: one confirmation with step-up runs, in order, `installApp` with the chosen scopes, `createGrant` for each chosen target (each already previewed), and `createToken` for the app; the token appears in the once-only dialog, the key ID for receivers comes from `getWebhookKeys`. If a later call fails, the screen shows which steps succeeded and offers to retry the failed one or to uninstall the half-installed app (`uninstallApp`); nothing is retried silently. An error of 3.2 validation shows the field and rule.

2. **Update and lifecycle** (11 4.2, 4.3). Update: upload a new manifest with the same `slug` through `upgradeAppManifest`; show added and removed scopes, events, hosts and grants; removed items take effect at once (say so); added ones need `setAppScopes` with consent. Suspend, resume, uninstall: the dialog shows the row of 11 4.3 for the chosen action before the step-up.

3. **Outgoing webhooks** (11 7). A subscription form limits event types to the granted events (from `getApp`) and checks the URL scheme `https`; the server enforces `webhook.allowed_hosts` and answers with the reason; deliveries show state, attempts, next attempt and the last status code, never the response body (not stored, 11 7); "Redeliver" calls `redeliverWebhook`. The key for receivers is shown from `getWebhookKeys` with a copy action.

4. **Once-only secrets** (11 4.1, 8.1; 08 12; Ruling for docs/DECISIONS.md). One dialog widget shows a secret (app tokens, incoming webhook tokens, and the `hmac_secret` that `createWebhookSubscription` returns once where the subscription uses it) with a copy action and the warning that it will not be shown again; the secret lives only in the widget state, is cleared on close, is excluded from semantics announcements except on explicit focus, and is never passed to a logger, a crash report, the local database or browser storage. A test asserts that after closing the dialog no widget, provider or log capture still holds the canary token, and that reopening the detail screen shows only the token's prefix or nothing.

5. **"Incoming app" without an API of its own** (11 8.1; M2-27). `IncomingWebhookCreate` requires an `app_id`, and the server offers no combined call (M2-27 reported the gap). For owners the screen composes it: `installApp` with a built-in minimal manifest (`messages:write` only, no events, no hosts), `createGrant` with `write` on exactly the chosen channel, then `createIncomingWebhook` with that `app_id`, under one confirmation; a failure after the first step offers retry or `uninstallApp`. Managers choose an existing app (11 8.1). Record the composition as a ruling and name the API gap in the report for a later spec change.

6. **Rulings for `docs/DECISIONS.md`** (`D-M6-08-<n>`): the install flow and where validation happens (note 1); update and lifecycle dialogs (note 2); the deliveries view without response bodies (note 3); the shared once-only dialog (note 4); the composed "incoming app" (note 5).

## Tests (write them first)

- Widget and golden (`packages/admin_ui/test/apps/`, `test/incoming_webhooks/`, fake server): install with consent, optional scope deselected, grant targets with preview, confirmation; update diff; suspend and uninstall dialogs with the 11 4.3 effects; subscriptions and deliveries; incoming webhook create and revoke; light and dark, text scaling 1 and 2, de and en.
- `once_only_secret_test.dart` (PLAN row): the incoming webhook token appears exactly once; after close, no trace of the canary in widgets, providers, the log capture or the fake storage; the detail screen never shows it again.
- `patrol` web suites in `make console-e2e`: install an app from a manifest file with step-up and see the token once; create an outgoing subscription to the receiver in the Compose network and see a delivery succeed; create an incoming webhook, post to it with the shown token from the test, see the message in the channel through the API, revoke it and see the next post answered `404`.

Commands that must pass, from the repository root:

- `cd packages/admin_ui && flutter analyze --fatal-infos && flutter test --coverage`
- `cd apps/console && flutter test --coverage` and `cd apps/client && flutter test --coverage`
- `make console-e2e` on the build host
- `make arch`, `make lint`, `make fmt-check`, `make coverage`
- `make check`

## Acceptance criteria

- [ ] Widget and `patrol` web tests of the flows pass in desktop and console.
- [ ] The token of an incoming webhook appears exactly once; app tokens likewise.
- [ ] Installation shows the consent of 11 4.1 with concrete grant targets; update, suspend, resume and uninstall state their effects; every change requires step-up.
- [ ] README, ARB files, CHANGELOG and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comments on every public Dart symbol; comments explain why.
- `packages/admin_ui/README.md` updated in English.
- Rulings of Design note 6 recorded in `docs/DECISIONS.md` section 10.
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
