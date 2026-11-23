# AUD-P12-docs-b Documents held to what is built: operations, compliance, specification, quality pages

Work package P12-docs-b of the audit of 2026-11-11: findings about documents that promise more
than is built or proven, or describe it wrongly. Each finding was checked against the code of
2026-11-23. The documents now say what is built and mark the rest as not built, not rehearsed or
not proven where the text stands; a missing product part is a plan row of V3 (V3-46 to V3-61),
not built here. Small gates hold the corrected texts (`test_secrets_init_commands.py`,
`test_spec_test_ids.py` and the CI job table in `test_ci_parallel_gates.py`), and the route
generator refuses an open envelope of a gateway client frame.

| No | Date | Ruling | Reason |
|---|---|---|---|
| AUD-P12-DOCS-B/1 | 2026-11-23 | Every `secrets init` command of docs/operations that runs in the container `secrets-init` names the same `--owner` values as `secrets-init` in `infra/compose/compose.yaml`; `tools/docs/tests/test_secrets_init_commands.py` holds it. | The command of case B left out `postgres` and `backup`, whose pgBackRest TLS keys must not be readable by others; a pytest that reads the real Compose file is a small gate. |
| AUD-P12-DOCS-B/2 | 2026-11-23 | Case B of disaster-recovery.md and 5.3 of backup-restore.md say "not rehearsed" with their known gaps instead of being rewritten; corrected commands come from a rehearsal (plan row V3-46). | Without a run the corrected order (backup depends on a healthy postgres, the repo choice of pgBackRest with an empty repo1, restic commands, roles before `pg_restore`) would be a second untested runbook. |
| AUD-P12-DOCS-B/3 | 2026-11-23 | Server keys outside `x-server-env` are set in `compose.override.yaml` for every server service; this is the documented way (install-compose.md 10). No `env_file` and no mounted configuration file are added to compose.yaml. | The file is already ignored by Git (D-A1-25) and kept by wizard and upgrade, and Compose merges it without a flag. A new mechanism would change the stack and its tests for a documentation gap. |
| AUD-P12-DOCS-B/4 | 2026-11-23 | The two font folders are ScanCode exceptions with the reason of licenses.md "Bundled fonts"; OFL-1.1 does not enter allowed.txt. The tree-wide patterns stay until the ScanCode run of V1-04 lists reviewed paths (#1615 deferred). | allowed.txt is the policy for dependencies, and licenses.md already settles the fonts outside it; a full scan takes hours under emulation and belongs to the publishing run. |
| AUD-P12-DOCS-B/5 | 2026-11-23 | The performance targets of 02 17 and the desktop terminal target are recorded as not proven or not met where they stand; no run of `make perf` was made here. | ADR 0063 makes the reference run an owner operation; the recorded runs (M1-72/9, M1-72/10, D-M7-18-4, M4-38) missed targets, and the plan's closing rule 4 item 3 has no evidence for any milestone. |
| AUD-P12-DOCS-B/6 | 2026-11-23 | The compliance templates list the flows built up to M8 and A1 (terminal, search assistant, interview, reference apps, cost reconciler, personal billing mode); a DPIA framework, a breach procedure and procedures for Art. 18 and 21 are plan row V3-48. | The templates had not changed since M0. The missing procedures are new documents with legal content, not a correction of the existing text. |
| AUD-P12-DOCS-B/7 | 2026-11-23 | "The Bothy Authors" are everyone whose contributions are in the history of the repository (GOVERNANCE.md); the signer of a commit certifies the DCO also for tool and AI output (CONTRIBUTING.md, AGENTS.md). | NOTICE named a holder without a definition, and the project is worked on by AI agents under a DCO. |
| AUD-P12-DOCS-B/8 | 2026-11-23 | "View as person" is described as built: both apps show the structure list of `viewAsUser` with level and capabilities as text, no app shows the content of a channel as the person sees it, and a cancelled step-up closes the view (`onExit`). The read-only content view of the desktop app is not planned. | `view_as_person.dart` renders text only by design (07 9.2, nothing in the frame writes), and the console has no content routes (ADR 0028, D-M6-04-3). A content view as another person is a new feature with its own risk (SR6); the check per channel answers what the person can see and trigger. The empty dialog after a cancelled step-up was a reproduced defect with a one-line fix. |
| AUD-P12-DOCS-B/9 | 2026-11-23 | The push chapter names `wake_ended` wherever the code treats it as a kind with a thread: its loc-key, the preview kinds of 6.3 and 6.4 (`domain.PreviewKindAllowed`) and the Android channel `runs`. The badge of 01 10.3 is the definition of 13 8. | The code (`kinds.go` `HasThread`, `notification_channels.dart:41`, `Localizable.strings`) and AR-04 added the kind; the chapter text kept the six kinds of M3. No gate for the list: the enum and the decision column of 13 2 are the source. |
| AUD-P12-DOCS-B/10 | 2026-11-23 | Hosted turns are stopped at every SIGTERM, also with `runner.turn_handover`; 09 18.19.3 and 26 3.2 say so, and the handover and adoption of hosted turns is plan row V3-52. | D-AR-03-9 decided it and the runner README states it; the chapter promised the handover without exception. Adopting hosted records needs the identity checks of 26 3.2 item 5 on the adoption path: a ticket, not a text fix. |
| AUD-P12-DOCS-B/11 | 2026-11-23 | 14 section 8 marks the reassignment job of `search_language` (V3-53) and the search dashboard, panel and alert (V3-54) as not built. | D-M3-13-8 left the job out; no dashboard reads the search indexes or `bothy.search.duration`. Both are server or observability work with gates of their own (signal catalogue, dashboards, alert rules). |
| AUD-P12-DOCS-B/12 | 2026-11-23 | 08 section 11 keeps the target (list of outside steps, "revoke all grants") and marks both as not built (V3-55). | The person view shows only `adminPersonDisabled`; grants are revoked one by one. A bulk revoke is an owner action with step-up and an audit design; the list is texts in two languages. |
| AUD-P12-DOCS-B/13 | 2026-11-23 | 20 section 9 no longer promises an audit entry for the co-determination switches: they are server configuration read at start, and the operator's version control is the record, as 4.1 says for `retention.content`. A system audit entry for a changed effective value at start is plan row V3-56. | No code writes such an entry; the chapter itself chose version control for configuration in 4.1. Whether a works agreement needs the in-product record is the operator's question; the row keeps the option. |
| AUD-P12-DOCS-B/14 | 2026-11-23 | 06 section 2 states as built that 201 answers carry no `Location` and that the cross-route headers are declared once; neither is changed in the contract. | Both are info findings without a consumer that fails: the SDKs and apps read `id` from the body and set the headers themselves. An additive `Location` and a header extension for generated clients are minors. |
| AUD-P12-DOCS-B/15 | 2026-11-23 | Before the first release `info.version`, `features` and `schema_version` are not raised per change and must not be used as compatibility signals (06 13.1); the gate that requires the steps from the first release tag on is plan row V3-57. #1441 is kept, #1142 and #1582 are its duplicates. | A per-change bump gate now would make every parallel pull request that touches openapi.yaml collide on one line, before any client depends on the value; G6 already has no base without a tag. The console version check and `min_api_version` compare equal values today, which the text now says. |
| AUD-P12-DOCS-B/16 | 2026-11-23 | The envelope `{op, d}` of every gateway client frame is closed in asyncapi.yaml, and the route generator refuses an open one. | `decodeEnvelope` already refuses a third member with 4002; the contract must say what the server enforces (W7), and the generator held only the body. A contract change that narrows nothing the server accepted. |
| AUD-P12-DOCS-B/17 | 2026-11-23 | `tools/docs/tests/test_spec_test_ids.py` ties the test IDs of docs/spec and docs/security to the test files by name, with a ratchet list of 28 IDs (V2, open AR tickets, A1-10, tested under another name, lint rules, the terminal umbrella). It checks that a test names the ID, not that it passes. | The audit asked for the catalogue approach or an ID-in-name rule with a gate. The ratchet stops new unnamed IDs now; renaming the existing tests (e.g. adding `DP-T11` to `TestMinGroup`) shrinks the list in later runs. |
| AUD-P12-DOCS-B/18 | 2026-11-23 | The three `gates` rows of the gates.md table "CI jobs" are held equal to the `make` lines of their ci.yml jobs by `test_ci_parallel_gates.py`. | The table drifted twice (`sql-visibility`, `tofu-check`, `signal-check`); the existing test already parses ci.yml, so the check is a few lines. |
| AUD-P12-DOCS-B/19 | 2026-11-23 | The [Reconcile] button of A5 is not built; reconciling by hand is a feature-list row with place API (`reconcileCosts`) and plan row V3-60. ADR 0055 is not edited. | No app calls `reconcileCosts`; the cost reconciler app and the API are the ways. ADRs are records of their time (SPEC 8.1). |

## Open

Minors left on purpose, for the next run in the area:

- An additive `Location` header on `201 Created` answers, first for children that are addressed
  flat (06 section 2).
- An extension for the headers of every route (`x-global-headers`) in the reference and the
  SDKs, and a third security profile of the route generator for the token of an incoming
  webhook (06 section 2).
- No gate holds the loc-keys and the preview kinds of 13 6.1 to 6.4 against `NotificationKind`.
- The log line of the search assistant counts candidates before the rights filter of the agent
  (part of V3-54).
- The tests that carry a spec test ID under another name (`KNOWN_UNNAMED` in
  `tools/docs/tests/test_spec_test_ids.py`) can name the ID; ID-T32 to ID-T34 are unverified.
- The frames of the terminal channels are binary Noise frames and are not checked by the route
  generator.
- `TerminalScreenView` rebuilds its subtree per data frame; belongs to the run that sets a
  target for the web terminal.
- The row `Connection` of 15 7.1 still says "immediate attempt on a network change"; the note
  under the diagram of 7.2 explains the built behavior.
- The import tool does not repeat an idempotent call after a write error; a bounded repeat with
  the `Idempotency-Key` would end the aborts behind a proxy of discord-migration.md.
- No smoke test sets a key through `compose.override.yaml`.
- The `tailscale` profile has no record of a hand test on a real tailnet (an account is needed).
- `quality/licenses/scancode-exceptions.txt` keeps the tree-wide patterns for `docs/`, `tools/`
  and `design/` until the ScanCode run of V1-04 lists reviewed paths.
- The suite of the Hermes adapter against a real server has no mirrored session.
- No gate binds the compliance templates to new tables or recipients.
