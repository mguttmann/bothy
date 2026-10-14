# M7-20: Alerts into the instance and to the outside

Status: ready | Depends on: M1-73, M2-27 | Milestone: M7

## Goal

After this ticket alerts reach people (17 6, `alerting.md`): Alertmanager sends every `ticket` and `page` alert to the alert channel of the instance through an incoming webhook in the format `alertmanager` (M2-27), sends `page` alerts in addition to an external channel of the instance operator's choice, and sends the always-firing `Watchdog` to an external heartbeat service, so a dead alert path is noticed outside the instance. The webhook URLs are secrets in files (`alertmanager_bothy_webhook_url`, `alertmanager_external_webhook_url`, `alertmanager_heartbeat_url`), never in the configuration. The owner creates the channel "Operations" (only owners and their deputies read it), the app and its incoming webhook in a session with step-up (console or desktop, `alerting.md` 2), and a guided setup writes the URL into the secret file. `promtool test rules` and `amtool check-config` with `amtool config routes test` are green for every route, and an integration test proves the path end to end: a test alert fired in Prometheus appears as a message in the alert channel, a `page` alert also reaches the external stub, and the Watchdog reaches the heartbeat stub. Why: exit criterion 6 of M7; a server failure must not swallow its own alert, and without a tested path an alert is only a hope.

## Read first

- `docs/observability/alerting.md` (all: levels and routes, the alert channel, the rules, runbook anchors, check), `docs/spec/17-observability.md` 6 (SLOs and alerts), 10 (tests that hold these rules), `docs/observability/slos.md`.
- `docs/spec/11-extensions.md` 8 (incoming webhooks, format `alertmanager`, R-EX-08), `docs/spec/05-actions.md` (`webhook_in.create`, `channel.create`), `docs/spec/16-operations.md` 5.3 (supplied secrets), 12 (limit: the external alert path when the server is down).
- `infra/observability/alertmanager/alertmanager.yaml`, `infra/observability/prometheus/rules/`, `infra/observability/prometheus/tests/`, `infra/compose/compose.yaml` (profile `observability`), `docs/operations/configuration.md` (secret rows of Alertmanager).
- `docs/DECISIONS.md`: D-W-100 and its extension to M7 (D-W-125); section 10 (format of `D-M7-20-<n>` rows).
- Briefs (merged names win, D-W-100): `M1-04-ops-config-checks.md` (`make ops-check`, pinned `promtool` and `amtool`), `M2-27-webhooks-incoming.md` (format `alertmanager`, `origin = app`), `M1-35-identity-step-up.md` (`createStepUpChallenge`, `completeStepUpWithDeviceKey`; API tokens cannot step up, 08 12), `M1-73-compose-smoke.md` (stack and overlays under `infra/compose/test`).
- Code on the base branch: `infra/observability`, `infra/compose`, `services/server/internal/apps` (incoming webhooks), `tools/`.

This ticket touches no table, no action and no event.

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
8. White label: the channel is named "Operations"; the external examples are `https://oncall.example.com/hook` and `https://heartbeat.example.com/ping`; no company, host, person or real service names in examples.
9. Alert thresholds stay where they are (rules files and named values); this ticket changes routes and delivery, not thresholds.
10. Specification first: a change to routes or the setup updates `docs/observability/alerting.md` in the same change. The core contract is not edited by this ticket. Everything this ticket adds is English.
11. No cost (D-W-125): the external channel and the heartbeat service are the instance operator's choice; tests use local stubs; no account is required.

Product requirements implemented here:

12. Routes as in `alerting.md` 1: `page` to `external-oncall` and `bothy-ops`, `ticket` to `bothy-ops`, `Watchdog` to `external-heartbeat`; `amtool config routes test` proves each.
13. Alertmanager reaches the server only through the internal listener (`server.internal_listen_addr`), as `alerting.md` 2 item 3 says; no public path.
14. A message from the alert webhook never triggers an agent (`origin = app`, 11 8).
15. The setup never prints the webhook URL; it writes it only into the secret file with mode `0400` (or the mode the secrets layout of 16 5.3 needs for a third-party service) and reports the fingerprint.
16. Missing secret files keep Alertmanager from starting with a clear message instead of silently dropping alerts (`url_file` behaviour verified for the pinned version).

## Scope

- `infra/observability/alertmanager/`: configuration as built (templates of the message if the format needs them), route tests.
- The guided setup of the alert channel (Design note 1) with tests.
- `infra/compose/`: Alertmanager's secrets from the volume `secrets` (subpath `alertmanager`); the catalogue rows of the three supplied secrets.
- An integration test overlay under `infra/compose/test/` (never in the bundle) with stub receivers for the external channel and the heartbeat, and a test runner (Design note 2).
- Docs: `docs/observability/alerting.md` 1, 2 and 5 as built, `docs/operations/configuration.md` (secrets), `docs/operations/` guide for setting up alerts, `CHANGELOG.md` under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M7-20-<n>`.

## Out of scope

- New alert rules or thresholds; the Grafana dashboards; choosing or paying for an external service.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M7-20-alert-routing`; one writer per worktree.
- Conventional Commits with the component as scope (for example `feat(observability): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag, never publish an image or a release, never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules). The stack and the stubs publish only on `127.0.0.1`.
- Stop only processes and containers you started, by PID or container ID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets; the test generates its own token and webhook URL in its temporary directory.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M7 by D-W-125). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Guided setup** (`alerting.md` 2; Ruling for docs/DECISIONS.md). `webhook_in.create` and `app.install` need a step-up, which only a person's session gives (07 SR5; an API token cannot step up, 08 12), so the tool never creates them. The owner creates the channel with the rights of `alerting.md` 2 item 1, installs the app with `write` on it and creates the incoming webhook with format `alertmanager` (M2-27) in the console or the desktop app; the guide in `alerting.md` lists these steps. A small tool (for example `tools/ops/setup_alert_channel.py` or a subcommand the build records) reads the URL from standard input, writes it into the secret file with mode `0600`, never prints it, and prints only its fingerprint; a second run with the same URL changes nothing. No new server command is needed (02 3.3 stays unchanged).

2. **End-to-end test** (17 10). Start the M1-73 stack with the profile `observability` and the test overlay; create the channel, the app with `write` and the incoming webhook through the API as a test owner session with a device key and step-up (`createStepUpChallenge`, `completeStepUpWithDeviceKey` of M1-35), then run the tool with the returned URL; load a test rule that fires immediately (in the overlay only); assert the message in the channel through the API (alert name, level, summary, state), the `page` copy at the external stub, the Watchdog at the heartbeat stub, and the resolved message after the rule stops firing.

3. **Rulings for `docs/DECISIONS.md`** (`D-M7-20-<n>`): the setup tool (note 1), the test rule overlay (note 2), the secret file modes.

## Tests (write them first)

- `make ops-check` (existing `promtool test rules` and `amtool check-config`) plus route tests for the three routes with `amtool config routes test`.
- Unit tests of the setup tool (idempotency, file mode `0600`, no URL in the output, only the fingerprint).
- The end-to-end test of Design note 2.

Commands that must pass, from the repository root:

- `make ops-check`
- the setup tool tests and the end-to-end test (the target the build adds, outside `check` if it needs the stack; record it in `docs/quality/gates.md`)
- `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] `promtool test rules` and `amtool` are green; every route is tested.
- [ ] Integration test: a test alert appears as a message in the alert channel; `page` also reaches the external stub; the Watchdog reaches the heartbeat stub.
- [ ] The setup creates channel and webhook idempotently and never prints the URL.
- [ ] `alerting.md`, the guide, the configuration reference, CHANGELOG and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- Rulings of Design note 3 recorded in `docs/DECISIONS.md` section 10.
- `CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check` and of the end-to-end test.
3. Review and red team verdicts and the findings fixed.
4. A rendered example message of a firing and a resolved alert (no secrets).
5. What is not proven (a real external on-call or heartbeat service).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
