# M4-16: Sync engine: run documents

Status: ready | Depends on: M4-08 | Milestone: M4

## Goal

After this ticket the sync engine has a `RunStore` that keeps the live box content of every run in focus in memory (15 7.4, 04 11.3): it takes the whole document when a run comes into focus, applies the coalesced `run.updated` patches (RFC 6902 with only `add`, `replace` and `remove`, JSON Pointer paths into `run_document.v1`) by the client rule of 04 11.3 item 4 (`base_version` equal to the local version: apply; `version` not newer: ignore; otherwise a gap: refetch the document and replace it), replaces the document on a `snapshot` frame, publishes at most one new state per frame for drawing, and discards the document when the run ends, so the collapsed row comes from the run summary in the local database. The store depends on no transport: frames come in as decoded maps and the refetch goes through a port, so M4-17 wires it to the dispatcher of M4-14 and to `GET /api/v1/runs/{id}`. Why: the live box is the core of the agent interface (01 6.2), and a patch applied on the wrong base shows a state that never existed; the version rule and the refetch make that impossible.

## Read first

- `docs/PLAN.md`: section 3; section 10 rows M4-16, M4-17, M4-21, M4-34 (`client.perf.patch_to_frame_p95`).
- `docs/spec/15-client-architecture.md`: 4 (short-lived data in a `Notifier` per run from the store of the `sync_engine`), 5 (run documents never in the local database), 7.1 (row `RunStore`), 7.4 "Live box", 19 (`client.perf.patch_to_frame_p95`).
- `docs/spec/04-sync-protocol.md`: 10 (`run.updated` ordered through `version`), 11 "Live box streaming" (11.1 document, 11.3 items 2 to 6).
- `docs/spec/09-agents.md`: 5.1 (end states), 5.3 "Run document" (paths and allowed operations), 5.4 "Live box content" (what the display needs).
- `docs/api/openapi.yaml`: schemas `Run`, `RunMetrics`, `RunStep`, `RunUpdatedPayload`, `JsonPatchOperation`, `RunState`; operation `getRun`.
- RFC 6902 (sections 4.1 `add`, 4.2 `remove`, 4.3 `replace`), RFC 6901 (JSON Pointer with `~0` and `~1`).
- `docs/DECISIONS.md`: D-W-100, D-W-114; section 10.
- Briefs: `docs/briefs/M4-08-domain.md` (`RunId`, `RunState` with `unknown`, `ResourceVersion`); for the server side `docs/briefs/M2-10-run-document-live-box.md` (coalescing and snapshot).

This ticket touches no API operation, no table of chapter 03, no action, no event and no configuration key.

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
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory. Interface texts come from `packages/l10n`, never as literals in widgets.
9. No new configuration key; the store has no limits of its own (the server limits focus and patch rates).
10. Specification first: 09 5.4 shows the stall notice from `stall_warned_at`, but the `Run` schema of `docs/api/openapi.yaml` has no such field. `RunDocument` reads it from the raw map when present and reports `null` otherwise; the report names the gap with the recommendation to add the optional field to `Run`. Any other field this ticket needs that `RunUpdatedPayload` or `Run` lacks: stop (Boundaries).

Product requirements implemented here:

11. `base_version` equal to the local version: apply the ops; `version` less than or equal to the local version: ignore; otherwise refetch `GET /api/v1/runs/{id}` and replace the document (04 11.3 item 4).
12. A `run.updated` with `snapshot` replaces the document when its `version` is newer (04 11.3 item 3).
13. Only `add`, `replace` and `remove` with JSON Pointer paths; steps are an object keyed by step ID and ordered by `ordinal` (04 11.1, R-SY-06).
14. The interface draws at most once per frame; patches are collected per frame and only the newest state is drawn (15 7.4).
15. After the end of the run the document is discarded; the collapsed row comes from the table `runs` (15 7.4, 09 5.4).
16. Run documents never go into the local database (15 5, 15 6.1 row `runs`).

## Scope

Create in `packages/sync_engine/lib/src/runs/`:

- `run_document.dart` (`RunDocument`: an immutable view over the JSON map with typed getters for the fields of 09 5.4, `stepsInOrder`, `openSubagents`, `currentTool`), `json_patch.dart` (`applyRunPatch`, Design note 2), `run_store.dart` (`RunStore`, Design note 1), `ports.dart` (`RunDocumentFetcher`, `FrameTicker`).
- Tests under `test/runs/` with fixtures under `test/runs/fixtures/` (Tests).

Change:

- `packages/sync_engine/lib/sync_engine.dart` exports, `README.md` (the client rule, ports, how M4-17 wires the store).
- `packages/sync_engine/pubspec.yaml`: the member dependency `domain` if it is not there yet.
- `docs/DECISIONS.md` section 10: rows `D-M4-16-<n>`; root `CHANGELOG.md` under `[Unreleased]`.

## Out of scope

- Receiving `run.updated` from the gateway and the focus handling (M4-13, M4-14; M4-17 connects the dispatcher hook to `RunStore.onRunUpdated`).
- The live box widget and its view model mapping (M4-07 component, M4-21 wiring).
- Following the end state from `run.state_changed` in the database (M4-14 stores the summary; M4-17 calls `RunStore.onRunEnded`).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M4-16-sync-run-documents`; one writer per worktree.
- Conventional Commits with the package or component as scope (for example `feat(client): ...`, `feat(sync-engine): ...`, `build(gates): ...`, `docs(spec): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`. Never publish a Dart package (`publish_to: none` in every `pubspec.yaml`).
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` with port 0 and make no network call; fakes replace the server unless a section below names a container. Temporary directories only via `mktemp -d` (Dart `Directory.systemTemp.createTemp`, pytest `tmp_path`), removed at the end. Stop only processes you started, by PID.
- Never read or print secrets. No network calls except package downloads for pinned tools and pub packages and what the gates of `make check` do themselves.
- The Flutter SDK at the version pinned in the root `pubspec.yaml` must be on `PATH` (and Go and Docker where a section names them); without them the gates fail by design (R-QA-02), and the ticket stops with `Verdict: BLOCKED <tool> missing` instead of skipping.
- Never lower a threshold, disable a gate, delete a test, raise `repo.max_file_kb` or extend `quality/licenses/allowed.txt`; never exclude hand-written code from a lint, format, coverage or doc gate.
- The product name appears only at the places of 02 "Code layout" ("Places that carry the product name"); the app shows `branding.product_name` from the instance, never a literal.
- No permission logic in the client (15 1 principle 1): controls follow server responses, snapshots and events, never an own derivation from grants.
- If the core contract (chapters 00, 03, 04, 05) or `docs/api/` lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M4 by D-W-114). Where the merged code of an earlier ticket names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Store.** `RunStore({required RunDocumentFetcher fetch, required FrameTicker ticker})`; `focus(RunId id)` registers interest and fetches the whole document unless one arrives first; `unfocus(RunId id)` drops it; `onRunUpdated(Map<String, Object?> payload)` applies the rule of 04 11.3 item 4 to the run of `run_id` if it is focused (others are ignored, because the server sends patches only for focused threads and a late frame after unfocus is harmless); `onRunEnded(RunId id)` discards the document; `watch(RunId id) -> Stream<RunDocument?>` emits at most once per tick of the `FrameTicker` with the newest state (the app passes a ticker on `SchedulerBinding`; tests pass a manual ticker). A frame with only `run_id` and `version` (truncated, no `ops` and no `snapshot`) is a gap when its version is newer. A fetched document replaces the local one only when its `version` is newer than the local one (a slow first fetch never overwrites a newer snapshot). A refetch in flight for a run ignores further patches of that run until it returns, then re-applies the rule to patches that arrived later and are newer than the fetched version (keep them in a short per-run queue). A fetch error leaves the last good document visible and retries on the next patch or `focus`. A fetch that returns after `unfocus` or `onRunEnded` of its run is ignored (each fetch carries a generation counter of its run that both calls advance), and a test covers the late response.

2. **Patches.** `applyRunPatch(Map<String, Object?> doc, List<Map<String, Object?>> ops) -> Map<String, Object?>` returns a new map (copy on write of the touched branches only, not a deep copy of the whole document): `add` creates or replaces an object member (and appends to an array only with `-` as RFC 6902 says, although the documented paths address objects), `replace` requires the member to exist, `remove` requires it to exist; pointers decode `~1` and `~0` in that order. Any other `op`, a missing target for `replace` or `remove`, or a path through a non-container throws `RunPatchError`, which the store treats like a gap (refetch). Ruling for docs/DECISIONS.md: an invalid patch is a gap, never a partial application.

3. **Document view.** `RunDocument` reads `state` (`RunState` with `unknown`), `version`, `status_line`, `last_text_excerpt`, `model`, `metrics` (`context_pct`, token counts, `last_api_retry`), `cost` (`amount_micros`, `currency`, `is_estimate`), `billing_mode_used` and `billing_mode_expected`, `plan.items`, `steps` (ordered by `ordinal`, children by `parent_step_id`), `followups`, `stall_warned_at`, times. Unknown fields are kept in the map and ignored by the getters (additive changes, 04 6.1).

4. **Rulings for docs/DECISIONS.md** (`D-M4-16-<n>`): ports instead of transport dependencies (Goal); invalid patch as a gap (note 2); coalescing per frame tick (note 1); the per-run queue during a refetch (note 1).

## Tests (write them first)

`packages/sync_engine/test/runs/` (fixtures of a document at versions 1 to 4 and the patches between them; fake fetcher that counts calls; manual frame ticker):

- `client_rule_test.dart`: equal `base_version` applies; a stale frame (`version` not newer) is ignored; a gapped frame (`base_version` ahead) refetches once and replaces; patches that arrive during the refetch and are newer than the fetched version are applied afterwards, older ones dropped.
- `snapshot_test.dart`: a `snapshot` with a newer version replaces the document; an older one is ignored; a truncated frame with a newer version refetches; a slow initial fetch that returns an older version than a snapshot received meanwhile does not replace it.
- `json_patch_test.dart`: `add`, `replace`, `remove` on the documented paths (`/status_line`, `/metrics/context_pct`, `/steps/<id>`, `/steps/<id>/status`, `/followups/<id>/state`, `/plan`); pointer escapes `~0` and `~1`; `move`, `copy`, `test` and a missing target throw `RunPatchError` and trigger a refetch; the input map is never changed.
- `frame_coalescing_test.dart`: ten patches between two ticks produce one emission with the newest state.
- `lifecycle_test.dart`: `focus` fetches the document; `unfocus` and `onRunEnded` drop it; patches for unfocused runs are ignored.
- `document_view_test.dart`: steps ordered by `ordinal` with children; the current tool; unknown fields kept.

Commands that must pass, from the repository root:

- `cd packages/sync_engine && dart test --coverage=coverage` (or the command M4-01 uses for pure Dart members)
- `make arch`, `make lint`, `make fmt-check`, `make coverage`
- `make check`

## Acceptance criteria

- [ ] Tests for an equal, a stale and a gapped `base_version` with refetching, and for `snapshot`, are green.
- [ ] Patches support exactly `add`, `replace` and `remove`; an invalid patch leads to a refetch, never to a partial state.
- [ ] At most one emission per frame; documents are discarded at the end of a run and never stored in the database.
- [ ] The store depends on no transport and no `api_client` type (ports only); rulings recorded.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why and cite 04 11.3 or RFC 6902.
- `packages/sync_engine/README.md` (English) as in Scope.
- Rulings of Design note 4 in `docs/DECISIONS.md` section 10.
- Root `CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. The number of refetches in each test scenario, and the document fields of 09 5.4 that the `Run` schema on the base branch lacks or names differently (at least `stall_warned_at`).
5. What is not proven (for example a platform test that ran only in CI, or a check done by hand).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100, D-W-114), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
