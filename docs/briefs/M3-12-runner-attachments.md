# M3-12: Claude Code runner: answer attachments

Status: ready | Depends on: M2-35, M3-07 | Milestone: M3

## Goal

After this ticket the Claude Code runner of M2-34 and M2-35 sends files with its answers, following 09 18.10 "File rules for reply attachments". Claude announces a file with a line `ATTACH: <path>` in its answer (the runner prompt asks for it). For every answer the runner collects these lines outside code blocks, removes them from the text, resolves every path with `realpath` and accepts it only when the real path lies under one of the roots of `runner.attach_allowed_roots` for this run (the run's project directory, the inbox of the run's thread, the run's own temporary directory), is a regular file without a second hard link, matches no pattern of `runner.attach_denied_patterns` (secret files such as `.env*`, `*.pem`, `*.key`, `id_*`, `.netrc` and the directories `.ssh`, `.claude`, `.config`, `.aws`, `.azure`, `.gnupg`), is not empty and stays within the limits the server publishes (`InstanceLimits.message_max_attachments`, `InstanceLimits.file_max_upload_bytes`). Only when every announced file passes does it upload them, each through the upload session of M3-03 (`createUpload`, `uploadPart`, `completeUpload`) from a private copy taken right after the check, inside the run's binding (07 "SR9"), and attach them to the answer with `MessageCreate.attachment_file_ids` (M3-07). If one file fails a check, an upload fails or the server refuses the attachments, no file is attached and the answer is sent without attachments, with a notice that names each announced file by its base name and the reason: all or none. The heartbeat reports `capabilities.files_out = true`. Why: a coding agent runs with the rights of the runner's operating system account, so a single symbolic link or an injected instruction in a repository must never turn an answer into a channel for SSH keys, cloud credentials or the runner's own state (T-DEV-01, T-AI-03, SEC-LOG-4); the server only checks rights, limits and content (12 11), so the path rules can only live in the runner. This is the plan row M3-12: a symbolic link escape and the secret patterns are rejected, and an answer carries all of its attachments or none.

## Read first

- `docs/PLAN.md`: section 3 "Way of working per ticket" (point 5: real model calls only behind an explicit switch); section 5 "Test strategy" (row "Test doubles": Fake-Claude, FakeServer); section 9 "M3 Platform services" (Goal, Scope "reply attachments of the runner", Exit criteria, Red team "a file across space boundaries in the context of another agent (SEC-UP-9)", Dependencies "from M2 for M3-07, M3-09, M3-12 and M3-19") and the rows M3-03, M3-04 and M3-07 (what exists on the server), M3-12 (this ticket, the contract), M3-08 (cleanup of never attached files, beside it); section 8 rows M2-29, M2-34 and M2-35 (what exists in the runner and the SDK), M2-14 (run binding), M2-20 (heartbeat capabilities); section 18 row "Dev forum with project binding and runner agent"; section 19 (P-09, P-10, P-11).
- Dependencies: the header names M2-35 and M3-07. Implied through them and therefore not in the header: M2-34 (the runner core: configuration, environment, turn process, stream reader, answer path, turn record, Fake-Claude), M2-29 (SDK hand layer, `Client.bind_run`, outbox, FakeServer), M2-23 (generated core `bothy_sdk`), M2-14 (header `Invocation-Token`, the mode table of SR9), M2-12, M2-10, M2-03, M3-04 (scan pass), M3-03 (upload sessions, `getFile`), M3-01, M1-46 (`getInstance` with `InstanceLimits`), M1-56, M1-58 and the rest of their closure. Not in the closure: M3-08 (cleanup and `deleteFile`), M3-05 (download), M2-20 (server side of the heartbeat capabilities); each place below says how that is handled.
- `docs/spec/09-agents.md`: 1 (points 4 and 5); 2.2 "Features" (row "Sending files": `supports_files_out`, `files_out`); 2.3 "Grants of an agent" (`upload` needs a grant); 4.6 "Run context" (files are always data); 4.7 rows "Send reply" and "Send file"; 4.8 (`capabilities`); 5.3 (`/status_line`); 15 "Reliability"; 18 completely, above all 18.2 "Process contract" (rows "Environment", "Input", "Recording"), 18.10 "File rules for reply attachments" and 18.11 "Flow"; 21 rows `runner.state_dir`, `runner.projects_root`, `runner.stream_log_retention`, `runner.attach_allowed_roots`, `runner.attach_denied_patterns` and the list of existing values (`message.max_attachments`, `file.max_upload_bytes`).
- `docs/spec/12-files.md`: 1 "Goals and limits"; 2 "States of a file"; 3.1 "Creating an upload session"; 3.2 "Transferring parts (mode `direct`, default)"; 3.3 (why `presigned` stays off); 3.4 "Completion and verification pass" step 3 (a completed upload may be attached at once); 3.6 "Error cases"; 6.2 "File names"; 10 "Attaching and agent invocation" points 1 to 3; 11 "Files from agents and from imports" (paragraph "Reply attachments from agents"); 12 row "Never attached"; 14.
- `docs/spec/07-permissions.md`: "SR9 Acting-for binding (confused deputy)" (every action that writes into a channel is decided on the intersection; exceptions only for answers in the run's thread and the other listed actions); 4.1 (reasons `capability_missing`, `acting_for_insufficient`). `docs/spec/05-actions.md` (core contract, read only): 2.3 point 6; 4.2 row `instance.read`; 4.7 row `message.create` ("with attachments additionally `+upload`"); 4.8 row `file.upload`.
- `docs/spec/19-security.md`: SEC-UP-9, SEC-LOG-2, SEC-LOG-4, SEC-AI-1, SEC-AI-3, SEC-T10. `docs/spec/06-api.md` 5 "Errors" and 7 "Idempotency". `docs/spec/23-quality.md` 3 (row "Python project") and "Documentation in code".
- `docs/api/openapi.yaml`: operations `getInstance`, `createUpload`, `uploadPart`, `getUpload`, `completeUpload`, `abortUpload`, `createMessage`, `patchRun`, `finishRun`, `sendHeartbeat`; schemas `InstanceInfo`, `InstanceLimits`, `UploadCreate`, `UploadSession`, `UploadPart`, `HeaderPair`, `UploadState`, `UploadPartResult`, `UploadComplete`, `File`, `MessageCreate`, `Heartbeat`, `Problem` (`x-problem-codes`), `FieldError`; parameters `IdempotencyKey`, `ChannelIdPath`, `FileIdPath`, `PartNumberPath`; `components.parameters.InvocationToken` (M2-14).
- ADRs 0022, 0023, 0026, 0053, 0054.
- `docs/DECISIONS.md`: D-R-RUNNER, D-AG-22, D-FI-07, D-W-100, D-W-104 (R-DM-43: the copies on `message_attachments` that M3-07 builds), section 10 (format of the rows `D-M3-12-<n>`, and the rows `D-M2-34-<n>`, `D-M2-35-<n>`, `D-M3-03-<n>` and `D-M3-07-<n>` that this ticket builds on).
- `docs/security/threat-model.md`: T-DEV-01, T-AI-01, T-AI-02, T-AI-03, T-AI-04, T-UP-01. `docs/RISKS.md`: RK-14. No item of `docs/OPEN-QUESTIONS.md` concerns this ticket.
- `docs/operations/configuration.md`: `runner.attach_allowed_roots`, `runner.attach_denied_patterns`, `runner.state_dir`, `runner.projects_root`, `runner.stream_log_retention`, `sdk.retry_max_attempts`, `sdk.http_timeout`, `agent.run_update_min_interval`; server keys read only for documentation: `message.max_attachments`, `file.max_upload_bytes`, `file.upload_part_bytes`, `file.upload_session_ttl`, `file.unattached_ttl`, `sync.idempotency_key_ttl`, `message.max_content_bytes`. `docs/observability/signals.md` names no runner metric; this ticket adds none.
- Briefs (merged names win, D-W-100): `docs/briefs/M2-34-claude-code-runner-core.md` (Design notes 2, 5, 7, 9 to 11, 13 to 17: configuration loader, run environment, `--add-dir` with the thread inbox, prompt and `prompts/runner_prompt.md`, `attachments.py` for incoming files, turn process, the answer path with the outbox's `client_msg_id`, the heartbeat with `files_out: false`, the run binding of every request, Fake-Claude, the FakeServer routes), `M2-35-claude-code-runner-paths.md` (Design notes 5, 6 and 12: one answer per `result`, the stop wins, adoption after a crash with the turn record), `M2-29-python-sdk.md` (Design notes 8 to 15, and "Out of scope": file upload goes through `Client.call` with the generated operations), `M2-14-acting-for.md` (Design note 5: `file.upload` is not an exception, so it is bound; "Out of scope": attachments of an agent reply), `M3-03-upload-sessions.md` (Design notes 5, 9, 11 to 13 and 19: check order of `createUpload`, `sha256` required as lowercase hex, `Content-Digest`, idempotent parts, answers per session state, `abortUpload` of a completed session is `409`), `M3-07-attachments-invocation.md` (Design notes 4, 5 and 15: attach rules, one `409 conflict` with `reason` `file_not_attachable`, empty content with attachments, the switch `supports_files_out` with `403 feature_disabled`), `M1-46-instance-settings.md` (`getInstance` for any active principal including agent tokens).
- Code on the base branch: `plugins/claude-code-runner` (M2-34 and M2-35: package `claude_code_runner`, configuration, `environment.py`, `attachments.py`, `prompt.py`, `prompts/runner_prompt.md`, `turn.py`, the answer path, `heartbeat.py`, the turn record, Fake-Claude and fixtures, tests, README, CHANGELOG), `sdk/python` (hand layer, generated core, `testing` FakeServer). Read the merged state with `git log` before you start.

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
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory.
9. Limits, windows and thresholds only through named configuration keys listed in `docs/operations/configuration.md`; code comments and docs name the key, never the number.
10. Specification first: a change to behaviour, API, event, right or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`) is not edited by this ticket.

Note on rule 3: as in M2-34 (`D-M2-34-1`), the runner is a program; its public Python symbols carry docstrings checked by ruff `D` with convention `google`, and its operator interface (the two configuration keys of this ticket) is documented in its README and in 09 21, from which `make config-reference` generates the reference.

Product requirements implemented here:

11. Claude announces an answer attachment with a line `ATTACH: <path>` (runner prompt); the runner removes every such line from the answer before it is sent (09 18.10 introduction and point 5).
12. Every path is resolved with `realpath` (symbolic links followed, then checked) and accepted only under a root of `runner.attach_allowed_roots`: the run's project directory, the inbox of the run's thread, the run's temporary directory; never elsewhere, also not through a symbolic link or a hard link (09 18.10 points 1 and 2, 18.1).
13. A path that matches a pattern of `runner.attach_denied_patterns` is rejected; the patterns of 09 18.10 point 3 always apply (09 18.10 point 3).
14. Count (`message.max_attachments`) and size (`file.max_upload_bytes`) are checked for the whole set before the first upload; then the runner uploads all files or none, and the answer carries all of them or none (09 18.10 point 4, plan row M3-12).
15. Rejected files are named in the answer with their reason (09 18.10 point 5); the notice never shows a directory or a real path.
16. Uploads use the upload session of M3-03 in mode `direct` (12 3.1 to 3.4, R-FI-01, R-FI-02) and attach with `MessageCreate.attachment_file_ids` to the runner's own answer in the run's thread (12 10, 12 11, 09 4.7 row "Send file"); `uploaded_by` is the agent, with the same scan, limits and quota as people (12 11).
17. Every request of an answer goes through the SDK client inside the run binding (header `Invocation-Token`, M2-34 Design note 13, 07 "SR9"), so a file reaches a channel only when the agent and the triggering person may both upload there (SEC-AI-3, T-AI-02).
18. The heartbeat reports `capabilities.files_out = true` (09 2.2, 4.8); the owner's switch `supports_files_out` is enforced by the server (`403 feature_disabled`, M3-07) and answered by the runner without attachments.
19. File contents are never read into a prompt, a log line, a patch or a request other than the upload parts; logs carry no file name, path or content (SEC-LOG-4, SEC-T10, 12 14).
20. No answer, upload or process starts after a stop request; a crash of the runner during an upload neither loses the answer nor uploads a file twice (09 1 points 4 and 5, 15, R-AG-17).

## Scope

All paths under `plugins/claude-code-runner/` unless they start with `docs/` or `sdk/`. `<pkg>` is the runner's import package that M2-34 created (expected `claude_code_runner`). Names of M2-34, M2-35 and M2-29 below are the expected ones; where the merged code names them differently, follow the code (Boundaries).

Create:

- `src/<pkg>/reply_attachments.py`, pure functions without network access: `split_attach_lines(text) -> tuple[str, list[str]]` (Design note 2), `AllowedRoots` and `allowed_roots(config, project_dir, thread_inbox, run_temp)` (note 3), `DeniedPatterns` and `denied_match(...)` (note 4), `Candidate`, `Rejection` with the closed set of reason codes of note 8, `check_candidate(...)` (notes 3 to 5), `check_set(candidates, limits) -> SetVerdict` (note 6) and `build_notice(outcomes, limit) -> str` (note 8).
- `src/<pkg>/reply_upload.py`: `stage_files(...)` (note 5 c), `AttachmentSetRecord` (the persisted record of note 10), the async `upload_set(...)` (note 7) and the async `send_answer_with_attachments(...)` used by the answer path (note 9).
- Fake-Claude fixtures in M2-34's fixture directory (expected `tests/fake_claude/fixtures/`) for the scenarios of the Tests section, each with M2-34's `fixture_meta` line (`origin` `constructed`).
- Tests: `tests/unit/test_reply_attachments.py`, `tests/unit/test_reply_upload.py`, `tests/scenarios/test_reply_attachments.py`. Follow M2-34's test layout if it differs.

Change:

- M2-34's configuration loader: the keys `runner.attach_allowed_roots` and `runner.attach_denied_patterns` with the forms and load checks of Design notes 3 and 4 (only add them if missing; adjust them to these forms if present); `config.example.toml` with both keys at their defaults.
- M2-34's run environment (`environment.py`): `TMPDIR` of a turn is the run's temporary directory of Design note 3 c instead of the runner's own `TMPDIR`; the key set of the allow list does not change.
- M2-34's turn runner: create the run's temporary directory before the turn starts and remove it when the run ends (note 3 c); its retention sweep also covers `outgoing/` and `tmp/` under `runner.state_dir` (note 11).
- M2-34's answer path (and M2-35's one answer per `result`): after the `TITLE:` line is removed, every answer goes through Design note 9; an answer whose text is empty but that carries attachments is sent (M3-07 note 5 a); the notice of note 8 is appended where note 9 says.
- M2-34's prompt (`prompts/runner_prompt.md`, English): one paragraph on `ATTACH:` lines (note 12).
- M2-34's heartbeat: `capabilities.files_out` becomes `true` (note 13).
- M2-35's adoption after a crash and the stop: an answer with a pending attachment set is resumed or ended by note 10; the stop of M2-35 note 6 cancels an upload in progress (note 10 d).
- `sdk/python`, only where this brief needs it, each with a test in `sdk/python/tests/` and a line in `sdk/python/CHANGELOG.md`: (a) if the generated core cannot send a binary body with headers for `uploadPart` through `Client.call`, a hand-layer function `upload_part(client, file_id, part_number, data, headers)` in the module that holds the other call helpers; (b) if the outbox's `send_message` cannot carry `attachment_file_ids`, the member; (c) FakeServer routes `getInstance`, `createUpload`, `uploadPart`, `getUpload`, `completeUpload`, `abortUpload` and `createMessage` with `attachment_file_ids`, a capture of every body and header, and injectable answers for the problems of Tests (Design note 14).
- `docs/spec/09-agents.md`: 18.2 row "Environment" (`TMPDIR`), 18.10 and 21 rows `runner.attach_allowed_roots` and `runner.attach_denied_patterns` (Design note 15); then `make config-reference` regenerates `docs/operations/configuration.md`. `docs/spec/12-files.md` 11 paragraph "Reply attachments from agents": one sentence (note 15).
- `README.md` of the runner: status, answer attachments, the two keys, the roots, the patterns, the notice codes, the residuals of note 16. `CHANGELOG.md` under "Unreleased". `plugins/README.md` only if it lists the runner's status or capabilities.
- `docs/DECISIONS.md` section 10: rows `D-M3-12-1` and following (Design note 17).

This ticket adds no configuration key (both keys exist in 09 21), no operation, no event, no metric, no migration, no server code and no new top-level make target.

## Out of scope

- The server side of uploads, scan and attaching (M3-03, M3-04, M3-07), the owner's switch `supports_files_out` (M3-07), the evaluation of the heartbeat capability `files_out` (M2-20), downloads (M3-05).
- Deleting uploaded files that ended up unattached: `deleteFile` and the job `files.cleanup_pending` are M3-08, which is not in this ticket's closure; such files stay readable only by the agent (M3-03: uploader only before attaching) and are removed after `file.unattached_ttl` once M3-08 is merged (plan gap reported).
- Incoming attachments, their download into the thread inbox and their place in the prompt (M2-34 Design note 10); voice transcription (`file.transcript.submit`); components and cards.
- The Hermes adapter (files of the Hermes adapter are a plan gap of M2-33); the TypeScript SDK; the conformance suite (M2-31).
- Uploads in mode `presigned` (12 3.3 keeps it off).
- Dev environments and the claim `dev_environment_id` (V2-03).
- Real logins, real sessions and model calls of any kind.
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M3-12-runner-attachments`; one writer per worktree.
- Conventional Commits with the component as scope (for example `feat(claude-code-runner): ...`, `docs(spec): ...`, `test(sdk-python): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`. Never publish a package.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0 (P-10); the FakeServer of M2-29 runs in the test process. This ticket needs no database and no container.
- Stop only processes you started, by PID or by the process group of a turn the test started. Temporary directories only via pytest `tmp_path` or `tempfile.mkdtemp`, removed at the end; symbolic links, hard links and FIFOs of the tests are created only inside them.
- Never read or print secrets. The "secret" files of the tests hold canary values written by the test; no test reads a real file under a home directory, `/etc` or any path outside its temporary directories. No network calls except package downloads for pinned tools and Python packages.
- Never start a `claude` CLI installed on this machine: every test uses Fake-Claude through `runner.claude_bin` (M2-34).
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100). Where the merged code of an M1 or M2 ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.
- The same holds for the merged code of M2-34, M2-35, M2-29, M3-03 and M3-07: their names win over the expected names here (modules, the answer path, the turn record, the outbox, `Client.call`, `Client.bind_run`, the FakeServer helpers); list each difference. Never rebuild in the runner what the SDK provides (retries, idempotency of the outbox, the run binding), and never define types the generated core already has (ADR 0026).

## Design notes

0. **Start check.** Run `make check` on the fresh worktree; red gives `Verdict: BLOCKED base branch red`. Confirm: M3-03 is merged (`createUpload`, `uploadPart`, `completeUpload` and `getUpload` are enabled in `services/server/api/operations.yaml`, and `UploadCreate.sha256` is required in `docs/api/openapi.yaml`); M3-07 is merged (`createMessage` no longer answers `not_enabled` for `attachment_file_ids`); M2-35 is merged (the runner has M2-35's stop and adoption). Otherwise `Verdict: BLOCKED dependency <ticket> missing`.

1. **Where this goes.** M2-34 built the answer path (after `result`: strip `TITLE:`, one `createMessage` with `client_msg_id` from the outbox, `run_id`, then the title and `finishRun`), the run environment, the thread inbox `<runner.state_dir>/files/<thread_id>/` for incoming files (`attachments.py`) and the turn record; M2-35 made every `result` without error post one answer, added the stop and the adoption after a crash. This ticket inserts one step into that path for every answer: collect and remove `ATTACH:` lines, check the set, upload it, attach it or fall back. Every decision is a pure function in `reply_attachments.py` (table tests); the file system and network I/O lives in `reply_upload.py`, which the answer path calls. Where the merged code already does one of these things, extend it instead of building a second path, and list it.

2. **`ATTACH:` lines** (09 18.10 point 5; Ruling for docs/DECISIONS.md). `split_attach_lines(text)` runs on the answer text after M2-34 removed the `TITLE:` line.

    a. An `ATTACH:` line is a line that starts at column 0 with exactly `ATTACH:` (case sensitive) followed by at least one space; the path is the rest of the line with leading and trailing white space removed. Lines inside a fenced code block (a line starting with three or more backticks or tildes opens it, the next fence of the same character and at least the same length closes it; an unclosed block runs to the end) are text, never `ATTACH:` lines, so an answer can show the syntax.

    b. Every `ATTACH:` line is removed from the text, together with the line break that ends it; then trailing blank lines are trimmed. The order of the remaining lines is unchanged. An `ATTACH:` line with an empty path is removed and listed as rejected with `not_found` and the name `(empty)`.

    c. The paths keep the order of their lines; that order becomes the order of `attachment_file_ids` and so the `position` of each attachment (M3-07 note 4 d).

3. **Allowed roots** (09 18.10 point 2, 21 row `runner.attach_allowed_roots`; Ruling for docs/DECISIONS.md).

    a. Form: a TOML list of strings, each one of the names `project`, `thread_inbox`, `runner_temp`, without duplicates; default `["project", "thread_inbox", "runner_temp"]`. Any other value, an absolute path included, aborts the start with a message that names the key and the file, never the value (M2-34 note 2). Reason: each name stands for a directory that belongs to exactly this run's forum, thread or run, while an absolute path would be shared by every forum the runner serves, so one forum's answer could carry another forum's files (the reasoning of SEC-UP-9 for the runner's side). An operator can narrow the list, never widen it.

    b. Per run the names resolve to real paths: `project` is the resolved working directory of the turn (M2-34 note 6, never `runner.projects_root` itself, so a project never reaches another project); `thread_inbox` is `realpath(<runner.state_dir>/files/<thread_id>)` of the run's thread only (M2-34 note 9; another thread's inbox is never a root); `runner_temp` is the run's temporary directory of c. A root that does not exist is left out.

    c. The run's temporary directory is `<runner.state_dir>/tmp/<run_id>/`, created with mode `0700` before the turn starts and passed as `TMPDIR` of the turn (09 18.2 row "Environment" names `TMPDIR` on the allow list; this fixes its value), so files a turn writes as scratch output can be attached. It is removed when the run ends (after its last answer); leftovers after a crash fall to the retention sweep (note 11). The command line of 18.2 does not change.

    d. Never a root, whatever the configuration says: `runner.state_dir` itself and every directory under it other than the run's `files/<thread_id>` and `tmp/<run_id>` (the inbox database holds invocation tokens, `sessions/` holds prompts and transcripts), and `runner.projects_root` itself. `check_candidate` enforces this after the root test, so a misbuilt root cannot widen it.

4. **Denied patterns** (09 18.10 point 3, 21 row `runner.attach_denied_patterns`; Ruling for docs/DECISIONS.md).

    a. Form: a TOML list of strings, each a shell-style pattern (`fnmatch` syntax) without `/`, not empty, without control or bidi characters. The default is the list of 18.10 point 3: `.env*`, `*.pem`, `*.key`, `id_*`, `.netrc`, `.ssh`, `.claude`, `.config`, `.aws`, `.azure`, `.gnupg`. A configured list must contain every default pattern; a list without one of them aborts the start, naming the key and the missing pattern (the patterns are not secret). Reason: the key exists to add secret file names of a project, and a list that drops a default would reopen exactly the files 18.10 names.

    b. Matching: a candidate is denied when any pattern matches, compared case-insensitively (both sides `casefold`; case-insensitive file systems would otherwise let `.ENV` through), (1) any component of its real path below the root it lies in (so a pattern for a directory such as `.ssh` denies every file inside it, at any depth), or (2) the last component of the path as announced (so a symbolic link named `id_rsa` is denied even when it points to a harmless file). The components of the root itself are not tested, so a projects root under a directory named `.config` does not deny every file.

5. **Checks of one candidate** (`check_candidate`; 09 18.10 points 1 to 3; Ruling for docs/DECISIONS.md). In this order, the first failure gives the reason:

    a. Resolve: a relative path is joined with the turn's working directory (the resolved project), `~` is not expanded; `os.path.realpath(path, strict=True)`; a missing path, a loop or any `OSError` gives `not_found`. The real path must be a strict descendant of a root of note 3 (`os.path.commonpath`), else `outside_allowed_roots`; a path inside a forbidden place of note 3 d gives `outside_allowed_roots` too. Denied patterns (note 4) give `denied_pattern`.

    b. Type: `os.stat` of the real path; not a regular file (directory, FIFO, socket, device) gives `not_a_regular_file`; `st_nlink` above 1 gives `hard_link`, because a hard link to a file outside the roots passes every path test; size 0 gives `empty_file` (`UploadCreate.size_bytes` has `minimum: 1`).

    c. Open and stage (in `stage_files`, after the whole set passed note 6): `os.open(real_path, O_RDONLY | O_NOFOLLOW | O_NONBLOCK | O_CLOEXEC)`, `fstat` of the descriptor; device, inode and type must equal the `stat` of b, else `changed_during_check` (a file swapped for a link or another file between check and open). Copy from that descriptor into `<runner.state_dir>/outgoing/<run_id>/<answer>/<position>` (mode `0600`, directories `0700`), computing SHA-256 and the byte count while copying; more bytes than `InstanceLimits.file_max_upload_bytes`, or a count different from the size of b, gives `changed_during_check`. Every upload reads only the staged copy, so a turn or another process that changes the file during the upload cannot change what is sent, and the SHA-256 of `createUpload` describes the bytes sent.

    d. The same real path announced twice counts once, at the position and with the announced name of its first line.

6. **The set: all or none** (09 18.10 point 4, plan row M3-12; Ruling for docs/DECISIONS.md). `check_set(candidates, limits)` decides on the whole answer before any upload:

    a. Limits come from `getInstance` (`InstanceInfo.limits`: `message_max_attachments`, `file_max_upload_bytes`; action `instance.read`, allowed for agent tokens, M1-46), read once per answer that has at least one `ATTACH:` line, inside the run binding. A failed read gives every file `limits_unavailable`. Reason: these are server values (03 9, `message.max_attachments`, `file.max_upload_bytes`) that the runner cannot configure, and `InstanceLimits` is the published place for "limits that clients enforce before sending".

    b. A candidate above `file_max_upload_bytes` gives `too_large`; more distinct candidates than `message_max_attachments` gives every candidate `too_many`.

    c. If any candidate is rejected (notes 2 b, 5 and 6 b), the set is rejected: no file is staged or uploaded, every rejected candidate keeps its reason and every other candidate gets `set_not_sent`. Only a set without any rejection goes to staging (note 5 c), where a `changed_during_check` again rejects the whole set before the first `createUpload`.

7. **Upload** (`upload_set`; 12 3.1 to 3.4, M3-03 notes 5, 9, 11 and 12; Ruling for docs/DECISIONS.md). Files one after the other in position order, parts one after the other; every request through the SDK client inside `Client.bind_run` of the run (header `Invocation-Token`); the SDK's retry rules (`sdk.retry_max_attempts`, `sdk.http_timeout`, `Retry-After` of `429 rate_limited`) apply. While a set uploads, the run document carries `replace /status_line` with `uploading_attachments` (09 5.3), removed afterwards, so the box shows why the run still works. Per file:

    a. `createUpload` on the run's channel (`channel_id` of the invocation) with a fresh UUIDv7 as `Idempotency-Key`, stored in the record of note 10 before the request; body `name` = the announced last path component (the server cleans it, 12 6.2), `size_bytes` and `sha256` (lowercase hex, M3-03 `D-M3-03-1`) of the staged copy; no `declared_mime_type`, because the server detects the type itself (12 6.1). The `file_id` of the `201` goes into the record.

    b. `UploadSession.mode` other than `direct`: the file fails with `upload_failed` (log code `upload_mode_unsupported`), because presigned URLs point outside the server and outside the SDK client (12 3.3).

    c. For each entry of `UploadSession.parts`: `uploadPart` with `file_id` and `part_number`, the headers of `UploadPart.headers`, and `Content-Digest: sha-256=:<base64 of the part's SHA-256>:` (RFC 9530, 12 3.2); the part is the slice of the staged copy at `(part_number - 1) * part_size_bytes` with the length of `part_size_bytes` (the last part the rest). The `etag` of each `200` goes into the record.

    d. `completeUpload` with `UploadComplete.parts` (every `part_number` with its `etag`) and a second fresh UUIDv7 as `Idempotency-Key`, stored before the request. The file is attachable from the `200` on, before the scan (12 3.4 step 3); the scan never delays the answer.

    e. Failure: a final problem on any of these requests (`403 forbidden`, for example with `reason` `capability_missing` or `acting_for_insufficient`; `403 channel_read_only`; `404 not_found`; `409 conflict` with `reason` `quota_exceeded`, `part_conflict` or `upload_incomplete`; `410 gone`; `413 payload_too_large`; `415 unsupported_media_type` with `reason` `file_type_denied`; `400 validation_failed`; `503 unavailable` with `reason` `storage_full`) gives this file `upload_refused` with the problem code and `reason`; retries exhausted or a transport error give `upload_failed`. Either way the set fails: no further file is uploaded, every completed file gets `set_not_sent`, the failing file keeps its reason, files not yet started get `set_not_sent`. The open session of the failing file (and only an open one) is aborted with `abortUpload`; an error of the abort is logged as a code and changes nothing. Completed uploads stay unattached (`abortUpload` answers `409 invalid_state_transition` for them, M3-03 note 13 c); M3-08's cleanup removes them (Out of scope).

    f. `401 unauthenticated`, or `403 invocation_token_invalid` with `reason` `run_inactive` (M2-14 note 4 c), means the run ended on the server: stop the turn as M2-35 note 6 e does, send no answer and do not finish.

8. **Notice and reason codes** (09 18.10 point 5; Ruling for docs/DECISIONS.md). The closed set of codes: `not_found`, `outside_allowed_roots`, `denied_pattern`, `not_a_regular_file`, `hard_link`, `empty_file`, `changed_during_check`, `too_large`, `too_many`, `limits_unavailable`, `upload_refused`, `upload_failed`, `not_attached`, `set_not_sent`. `build_notice` appends to the answer text, after one blank line, the fixed English line `Attachments not sent:` and one line per announced file in position order: `` - `<name>`: <code> `` and, for `upload_refused` and `not_attached`, ` (<problem code>, <reason>)` when the problem had a `reason`. `<name>` is the announced last path component with control and bidi characters removed, backticks removed and cut to 255 characters; never a directory, a real path or a home directory (the runner's file system layout is not for the readers of the channel). At most `message_max_attachments` lines name files; further files are summed up in one line `- and <n> more`. The notice stands only in answers whose set failed; a successful set adds nothing. If the notice makes the text exceed `message.max_content_bytes`, M2-34's `400 validation_failed` rule ends the run `adapter_error` as for any long answer.

9. **Sending the answer** (`send_answer_with_attachments`; M2-34 note 14, M2-35 note 5, M3-07 notes 4, 5 and 15; Ruling for docs/DECISIONS.md).

    a. No `ATTACH:` line: M2-34's path unchanged (no `getInstance`, no record).

    b. Set uploaded: one `createMessage` in the run's thread through the outbox with `client_msg_id` stored before sending, `content_md` the text of note 2 (may be empty, M3-07 note 5 a), `run_id` and `attachment_file_ids` in position order. Answers in the run's thread are an exception of SR9, so `message.create` uses the agent's rights; its `+upload` is the agent's own (05 4.7, M2-14 note 5).

    c. The server refuses the attachments: `403 feature_disabled` (the owner's switch `supports_files_out`, M3-07 note 15), `403 forbidden` (for example `capability_missing`), `409 conflict` with `reason` `file_not_attachable`, or `400 validation_failed` whose every `errors[].pointer` starts with `/attachment_file_ids`: send the answer again without `attachment_file_ids`, with the notice (every file `not_attached` with the problem code and reason) and a new `client_msg_id` stored before sending, because the same id with another body answers `422 idempotency_key_reuse` (06 7). These refusals store nothing, so the answer appears once. Every other answer of `createMessage` follows M2-34 (retry by the outbox, `400 validation_failed` ends `adapter_error`).

    d. Set rejected or failed (notes 6 and 7): send the answer without `attachment_file_ids` with the notice.

    e. Empty text and no attachment left: M2-34's rule "an empty reply sends nothing" applies only when no `ATTACH:` line was present; with a failed set the notice is the text and is sent.

    f. Then M2-34's title and finish steps as before. A failed set never fails the run: the run finishes as its turn did.

10. **Persistence, crash and stop** (09 15, 18.2 "Detachment", M2-35 notes 6 and 12; Ruling for docs/DECISIONS.md).

    a. `AttachmentSetRecord`, written atomically (temporary file and rename, mode `0600`) to `<runner.state_dir>/sessions/<session ref>/<run_id>.attach-<answer>.json` (answer = the index of the `result` in the run, from 1): the outcome of the checks, the staged paths with size and SHA-256, per file the two `Idempotency-Key` values, `file_id`, the confirmed parts with etags and the state (`staged`, `created`, `completed`, `failed`), the `client_msg_id` of the answer and of a fallback, and the end state of the set. Persist before the request that depends on it: a key before its request, a `file_id` right after its `201`. The record never holds a token, file contents or the answer text (the outbox of M2-29 holds the message body as it already does).

    b. Adoption (M2-35 note 12 d): an adopted run whose stream file has a `result` after the stored offset goes through the answer path as usual; when a record of that answer exists, the new runner resumes it: a file without `file_id` repeats `createUpload` with the stored key (the server answers the stored `201` within `sync.idempotency_key_ttl`, 06 7), a file with `file_id` reads `getUpload` and sends only the parts not confirmed, `completeUpload` repeats with its stored key; an answer whose `client_msg_id` is stored is resent by the outbox with that id. So no file is uploaded twice and the answer appears once. A staged copy that is missing gives the set `upload_failed`.

    c. Turn records of M2-35 gain the index of the last answer sent, so an answer is never handled twice.

    d. Stop (M2-35 note 6): a stop request cancels the set: the request in flight is cancelled, no further request starts, open sessions of the set are aborted with `abortUpload` (best effort, codes only in the log), no `createMessage` is sent (M2-35: no answer after a stop), and the run finishes `cancelled` with the reason of the request once the turn's process group is gone. The stop wins also between the last upload and `createMessage`.

11. **Files and retention.** Staged copies and records are deleted after their answer was sent (attached or fallback) or the run was stopped; the run's temporary directory when the run ends. M2-34's retention sweep (at start and after every turn) also deletes entries under `outgoing/` and `tmp/` older than `runner.stream_log_retention`. Every file this ticket writes has mode `0600`, every directory `0700`; the staging directory lies under `runner.state_dir`, never in the project or the run's temporary directory, so a turn cannot read or change it through its working directory.

12. **Runner prompt** (`prompts/runner_prompt.md`, English, appended to M2-34's text): "To send files with your answer, add one line per file at the very start of a line, outside code blocks: `ATTACH: <path>`. Only files inside the project directory, the thread's file folder or `$TMPDIR` can be sent. Files that may hold secrets (for example `.env` files, keys, certificates, `.ssh`, `.aws` or `.config` folders) are never sent. The ATTACH lines are removed from your answer; if any file cannot be sent, no file is sent and the answer lists why." The prompt is guidance only; every rule is enforced by the checks above, never by the model.

13. **Heartbeat and capability** (09 2.2, 4.8; M2-34 note 15). `capabilities.files_out` is `true` from this ticket on, in every heartbeat. The owner's switch is enforced by the server (M3-07 note 15) and handled by note 9 c; the runner does not read `operator_config`. Reason: the switch can change between a read and the send, and 09 2.2 makes a capability effective only when both halves allow it, so the send is the one place that decides.

14. **FakeServer and Fake-Claude** (09 16.1). Extend the FakeServer of M2-29 where it lacks a part (Scope): `getInstance` with configurable `InstanceLimits`; `createUpload`, `uploadPart`, `getUpload`, `completeUpload` and `abortUpload` with the semantics of M3-03 notes 5, 9 and 11 to 13 (a small `part_size_bytes` per test, digest and length check of each part, idempotent parts, replay of a stored answer for a repeated `Idempotency-Key`, `abortUpload` of a completed session answers `409 invalid_state_transition`), storing the received bytes per file so a test compares them; `createMessage` with `attachment_file_ids` that checks each id is a completed upload of the agent for that channel and not attached yet (else `409 conflict` with `reason` `file_not_attachable`); injectable problems per operation (`403 forbidden` with `reason` `acting_for_insufficient` or `capability_missing`, `403 feature_disabled`, `409 conflict` with `reason` `quota_exceeded`, `413 payload_too_large`, `415 unsupported_media_type`, a transport error, "stored, then 502"); a capture of every request with headers. Each route gets a test that validates its answers against `docs/api/openapi.yaml`. Fake-Claude needs no new capability: its fixtures carry the answers with `ATTACH:` lines, and the scenario writes the files, links and FIFOs into the test's project, inbox and run temporary directory before the turn (or through Fake-Claude's pause on a marker file, M2-35 note 13, for the swap and crash cases).

15. **Specification text** (rule 10; new text in English like the chapter, as the owner rules for all repository content (D-W-94); nothing in 00, 03, 04 or 05 changes):

    - `docs/spec/09-agents.md` 18.10: after the five points, English paragraphs with the line syntax and the code block rule (note 2), the three root names and what they resolve to, the run's temporary directory as `TMPDIR` and the places that are never roots (note 3), the pattern form, the mandatory defaults and the matching rule (note 4), the regular file, hard link, empty file and staging rules (note 5), the limits from `getInstance` and the all-or-none rule (note 6), uploads inside the run binding with SR9 and the fallback without attachments (notes 7 and 9), the closed set of notice codes (note 8), persistence and the stop (note 10).
    - 18.2 row "Environment": an English sentence: "`TMPDIR` is the run's temporary directory `<runner.state_dir>/tmp/<run_id>` (18.10)."
    - 21: row `runner.attach_allowed_roots` with Default `["project", "thread_inbox", "runner_temp"]` and the meaning in English ("roots for answer attachments, by name only (18.10)"); row `runner.attach_denied_patterns` keeps Default "list in 18.10" and gets the English meaning "denied patterns for answer attachments; must contain the list of 18.10". Then `make config-reference` and `make config-doc-check`.
    - `docs/spec/12-files.md` 11 paragraph "Reply attachments from agents": one English sentence: "Inside a run the upload is bound to the run (07 SR9): an agent uploads only where the triggering person may upload too; the answer in the run's thread is an exception of SR9."
    - No change to `docs/api/openapi.yaml`.

16. **Residuals to state in the README and in 18.10** (no code): the checks protect the paths the runner sends, not what a turn with `host` can read or copy on its own (T-DEV-01, SR2); a secret file under a name no pattern knows is sent if it lies under a root, so operators add their project's names to `runner.attach_denied_patterns`; content checks are the server's (scan, 12 5).

17. **Rulings for `docs/DECISIONS.md`** (section 10, English, one row each with date, decision, reason and place): `D-M3-12-1` syntax of `ATTACH:` lines and the code block rule (note 2); `D-M3-12-2` roots by name only, the run's temporary directory as `TMPDIR`, the places never allowed (note 3); `D-M3-12-3` pattern form, mandatory defaults, case-insensitive matching of real components and the announced name (note 4); `D-M3-12-4` the check order, `O_NOFOLLOW` and the inode comparison, hard links rejected, the staged copy (note 5); `D-M3-12-5` limits from `getInstance` and the all-or-none set (note 6); `D-M3-12-6` sequential upload inside the run binding, the `status_line` value, `presigned` refused, failure handling and aborted open sessions (note 7); `D-M3-12-7` the notice and its closed codes, base names only (note 8); `D-M3-12-8` sending, the fallback with a new `client_msg_id`, a failed set never fails the run (note 9); `D-M3-12-9` the persisted record, resumption after a crash and the stop (note 10); `D-M3-12-10` `files_out = true` without reading `operator_config` (note 13).

## Tests (write them first)

Test names follow the plan row and the reason codes, so `pytest -k symlink` and `pytest -k all_or_none` find them. Configuration values in tests come from a test configuration file; production code never hard-codes a window or a limit.

Unit (`tests/unit/`):

- `test_reply_attachments.py`:
    - `test_split_attach_lines`: a line `ATTACH: report.md` is removed and returns `report.md`; leading spaces before `ATTACH:`, `attach:` and `ATTACH:report.md` (no space) stay text; trailing white space is trimmed from the path; lines inside a fence of three backticks, of four backticks closed by four, of tildes and inside an unclosed fence stay text; an empty path is removed and reported as `not_found` with the name `(empty)`; order of paths equals order of lines; the text after removal keeps all other lines in order and loses trailing blank lines; run after the `TITLE:` removal, both lines are gone.
    - `test_allowed_roots_config`: the default list; a subset; an unknown name, a duplicate and an absolute path abort with a message that names the key and the file and not the value.
    - `test_check_candidate_roots`: in a temporary layout with `projects_root/website` (the run's project), `projects_root/other`, `state_dir/files/<thread>`, `state_dir/files/<other thread>`, `state_dir/tmp/<run>`, `state_dir/tmp/<other run>` and `state_dir/sessions/...`: files in the project, the run's inbox and the run's temporary directory pass; files in `other`, the other thread's inbox, the other run's temporary directory, `sessions/`, the inbox database and `projects_root` itself give `outside_allowed_roots`; `website/../other/x` gives `outside_allowed_roots`; a missing file gives `not_found`; a path with `~` is not expanded.
    - `test_symlink_escape_rejected` (plan row): a link in the project to a canary file in a temporary "home" directory outside every root; a link with an absolute target in the test's temporary tree outside every root; a directory link `docs -> <outside>` with the announced `docs/a.txt`; a link to the other project; a link to `state_dir/sessions/<ref>/<run>.input.jsonl`; a chain of two links ending outside; a link loop: each gives `outside_allowed_roots` (the loop `not_found`); a link inside the project to another file inside the project passes with the real path of its target.
    - `test_secret_patterns_rejected` (plan row): every default pattern with an example (`.env`, `.env.local`, `server.pem`, `tls.key`, `id_ed25519`, `.netrc`, `.ssh/config`, `.claude/settings.json`, `.config/tool/hosts.yml`, `.aws/credentials`, `.azure/profile.json`, `.gnupg/pubring.kbx`), case variants `.ENV` and `Server.PEM`, a file two levels below `.ssh`, a link `notes.txt -> .env` (real name denied), a link `id_rsa -> README.md` (announced name denied): each gives `denied_pattern`; a projects root inside a directory named `.config` still allows `README.md`.
    - `test_denied_patterns_config`: a configured list missing `.ssh` aborts the start naming the key and `.ssh`; an added pattern `*.sqlite` denies `data.sqlite`; a pattern with `/` or a control character aborts.
    - `test_regular_file_and_hard_link`: a directory, a FIFO and a Unix socket give `not_a_regular_file` without blocking; a hard link (second name of a file outside the roots) gives `hard_link`; a 0-byte file gives `empty_file`.
    - `test_check_set_all_or_none` (plan row): three candidates with one `denied_pattern` give that one its reason and the two others `set_not_sent`; more distinct candidates than `message_max_attachments` give all `too_many`; one above `file_max_upload_bytes` gives `too_large` and the rest `set_not_sent`; the same real path announced twice counts once at its first position; limits unavailable give all `limits_unavailable`; a set without rejection passes.
    - `test_build_notice`: base name only for an announced `/srv/projects/website/out/report.md` and for `../x/y.txt`; control, bidi and backtick characters removed; problem code and reason for `upload_refused` and `not_attached`; at most `message_max_attachments` named lines and one `- and <n> more` line; no notice for a successful set.
- `test_reply_upload.py`:
    - `test_stage_files_detects_swap`: the file is replaced by a link to an outside file after `check_candidate` and before staging: `changed_during_check`, nothing staged; the file grows beyond its checked size: `changed_during_check`.
    - `test_stage_files_copy`: the staged copy has mode `0600` under `outgoing/<run_id>/<answer>/`, the same bytes, the SHA-256 and size that `createUpload` will carry; changing the original afterwards does not change the copy.
    - `test_part_slices`: part boundaries for sizes equal to, one below and one above a multiple of `part_size_bytes`; `Content-Digest` of each part equals RFC 9530 `sha-256=:<base64>:`.
    - `test_record_roundtrip`: the record is written atomically with mode `0600`, holds keys and file ids, and never a token, file content or answer text.

Scenarios (`tests/scenarios/test_reply_attachments.py`): the real runner code with real Fake-Claude processes and the FakeServer of M2-29 on `127.0.0.1` and port 0; world of M2-34: space Engineering with the forum Projekt-A (binding `website`, a Git working tree under the test's projects root) served by the agent Runner in mode `organization` with tool class `scoped`; Alex triggers in a thread of Projekt-A; Robin is the owner; Sam has no `trigger`. The FakeServer answers `getInstance` with `message_max_attachments` 3 and a small `file_max_upload_bytes`, and gives `UploadSession.part_size_bytes` small enough that one file needs three parts. Every scenario asserts that no request body, log record, patch or file outside `outgoing/` and the FakeServer's byte store contains a canary file content.

- `test_reply_with_attachments_uploaded_and_attached`: the answer text "Done." plus `ATTACH: dist/report.md` (project) and `ATTACH: <TMPDIR>/summary.txt` (the run's temporary directory, written by the scenario as the turn's `TMPDIR`): one `getInstance`; per file one `createUpload` (`name`, `size_bytes`, lowercase hex `sha256`, an `Idempotency-Key`, no `declared_mime_type`), three `uploadPart` in order with valid `Content-Digest`, one `completeUpload` with all etags and its own `Idempotency-Key`; the FakeServer's stored bytes equal the files; one `createMessage` with `content_md` "Done." (no `ATTACH:` line), `run_id` and `attachment_file_ids` in announced order; every request of the answer carries `Invocation-Token` with the run's token; `/status_line` was `uploading_attachments` during the upload and removed after; the turn's environment had `TMPDIR` equal to `<runner.state_dir>/tmp/<run_id>`, which is gone after the run; `finishRun` `completed`.
- `test_symlink_escape_rejected_nothing_uploaded` (plan row): the answer announces `notes.md` (fine) and `link.txt -> <outside canary file>`: no `createUpload`; one `createMessage` without `attachment_file_ids` whose text ends with the notice listing `notes.md` with `set_not_sent` and `link.txt` with `outside_allowed_roots`; the canary content and the outside path appear in no request; the run finishes `completed`.
- `test_secret_patterns_rejected_nothing_uploaded` (plan row): the answer announces `README.md` and `.env`, and in a second run `config/../.ssh/id_ed25519`: no `createUpload`; the notices name `denied_pattern`; nothing of the canary secret appears in any request.
- `test_all_or_none_upload_failure_attaches_none` (plan row): three files; the FakeServer answers the second `createUpload` with `409 conflict` and `reason` `quota_exceeded`: no upload of the third file, no `createMessage` with `attachment_file_ids`; the answer without attachments names the first and third file `set_not_sent` and the second `upload_refused (conflict, quota_exceeded)`. Variant: the third part of the second file fails with a transport error after the retries: its open session is aborted with `abortUpload`, the first (completed) file is not aborted, the notice says `upload_failed`.
- `test_all_or_none_limits_from_instance`: four announced files against `message_max_attachments` 3 give `too_many` for all and no upload; one file above `file_max_upload_bytes` gives `too_large`; `getInstance` answering `503` gives `limits_unavailable`.
- `test_acting_for_insufficient_refuses_upload` (SR9): the FakeServer answers `createUpload` with `403 forbidden` and `reason` `acting_for_insufficient` (Alex lacks `upload`): no file attached, notice `upload_refused (forbidden, acting_for_insufficient)`, the run finishes `completed`.
- `test_message_refused_falls_back_without_attachments`: after a complete upload, `createMessage` answers `403 feature_disabled`, in a second case `409 conflict` with `reason` `file_not_attachable`: exactly one more `createMessage` with a new `client_msg_id`, no `attachment_file_ids`, the notice with `not_attached` and the problem; the thread shows one answer; the run finishes `completed`. A `502` after storing on the first `createMessage` is retried by the outbox with the same id and gives one message with the attachments.
- `test_attach_only_answer_sends_empty_text`: the answer is only `ATTACH: dist/report.md`: one `createMessage` with empty `content_md` and one attachment. Without any `ATTACH:` line an empty answer still sends nothing (M2-34).
- `test_attach_lines_in_code_block_stay_text`: an answer that shows `ATTACH: example.txt` inside a fenced block: no `getInstance`, no upload, the block is unchanged in the message.
- `test_thread_inbox_file_reattached`: a file of the triggering message, stored by M2-34 in the thread inbox, announced with its inbox path: uploaded and attached as a new file; the inbox path of another thread is `outside_allowed_roots`.
- `test_followup_results_each_answer_own_attachments` (M2-35 note 5): two `result`s in one turn, the first with one `ATTACH:` line, the second with another: two messages, each with its own attachment; records `attach-1` and `attach-2`.
- `test_stop_during_upload_sends_nothing`: Fake-Claude has finished; the FakeServer holds the second `uploadPart` until the test sends `run.cancel_requested`: no further request of the set, the open session is aborted, no `createMessage`, `finishRun` `cancelled` with the request's reason.
- `test_crash_during_upload_resumes_once` (M2-35 adoption): the runner is killed with SIGKILL after the first file's `createUpload` answered and one part was confirmed; a new runner on the same `runner.state_dir` adopts the turn, repeats `createUpload` with the stored key (the FakeServer replays the stored answer), reads `getUpload`, sends only the missing parts, completes and sends one `createMessage`: the FakeServer has exactly one upload session per announced file and one message.
- `test_run_ended_on_server_during_upload`: `uploadPart` answers `403 invocation_token_invalid` with `reason` `run_inactive`: the turn's group is stopped, no `createMessage`, no `finishRun` (M2-35 note 6 e).
- `test_heartbeat_reports_files_out`: every heartbeat body has `capabilities.files_out = true`.
- `test_no_names_paths_or_content_in_logs` (SEC-LOG-4, SEC-T10 for this path): canary file name, canary directory name and canary content of an announced file appear in no log record of the runner, no patch and no finish body; the name appears only in the `createUpload` body and the notice.
- `test_state_files_private_and_retention`: staging copies and records have mode `0600`, directories `0700`; they are gone after the answer; entries under `outgoing/` and `tmp/` older than `runner.stream_log_retention` are deleted by the next sweep.

SDK (`sdk/python/tests/`): one test per FakeServer route added (answers valid against `docs/api/openapi.yaml`, digest and length checks of parts, replay of a stored `Idempotency-Key`, `409 invalid_state_transition` for aborting a completed session, `409 conflict` with `reason` `file_not_attachable` for a foreign or attached file), and a test for `upload_part` and the outbox member if they were added.

Red team focus of the milestone (PLAN 9 "Red team": T-UP-01, T-AI-03; SEC-UP-9): `test_symlink_escape_rejected`, `test_secret_patterns_rejected`, `test_regular_file_and_hard_link`, `test_stage_files_detects_swap`, `test_all_or_none_upload_failure_attaches_none` and `test_acting_for_insufficient_refuses_upload`; name them in the red team request, together with: escaping the roots through links, hard links, case, `..` or a swap between check and read; reading the runner's own state; another thread's or project's files; a notice that leaks a path.

Commands that must pass, from the repository root:

- `cd plugins/claude-code-runner && uv run --locked pytest`
- `make fmt-check`, `make lint`, `make typecheck`, `make test`, `make coverage`, `make licenses`, `make osv`
- `make api-lint`, `make api-checks`, `make codegen-check`, `make api-clients` (when `sdk/python` changed)
- `make config-reference` (no diff afterwards), `make config-doc-check`, `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] An answer with `ATTACH:` lines is sent without them; the announced files are uploaded with `createUpload`, `uploadPart` and `completeUpload` from private copies inside the run binding and attached with `attachment_file_ids` in announced order.
- [ ] A symbolic link escape is rejected: no path outside the run's project, the run's thread inbox and the run's temporary directory is ever uploaded, also not through a link, a linked directory, `..`, a hard link or a swap between check and read (`test_symlink_escape_rejected`, `test_symlink_escape_rejected_nothing_uploaded`).
- [ ] The secret patterns of 09 18.10 are rejected, case-insensitively, also as directory components and as the announced name of a link; a configuration cannot drop them (`test_secret_patterns_rejected`, `test_secret_patterns_rejected_nothing_uploaded`).
- [ ] All or none: one rejected file, a count or size above the server's limits, a failed upload or a refused attachment leaves the answer without any attachment and with a notice naming each file by base name and reason (`test_check_set_all_or_none`, `test_all_or_none_upload_failure_attaches_none`, `test_message_refused_falls_back_without_attachments`).
- [ ] A refusal under SR9 (`403 forbidden` with `reason` `acting_for_insufficient`) and the owner's switch (`403 feature_disabled`) are handled by the fallback; the run still finishes as its turn did.
- [ ] A stop sends nothing more; a crash during an upload is resumed by the new runner without a second upload session or a second message.
- [ ] The heartbeat reports `files_out: true`; no file name, path or content appears in a log line, patch or finish body.
- [ ] 09 18.2, 18.10 and 21, 12 11, the configuration reference, the runner README and CHANGELOG and the rows `D-M3-12-1` to `D-M3-12-10` are written.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- README of `plugins/claude-code-runner` updated in English (status, answer attachments, the two keys and their forms, the roots, the patterns, the notice codes, the residuals of note 16); README of `sdk/python` if its FakeServer or hand layer changed.
- Rulings of Design note 17 recorded in `docs/DECISIONS.md` section 10 (one row each with reason); the changes to 09 and 12 of Design note 15 and the regenerated configuration reference made in the same change.
- `plugins/claude-code-runner/CHANGELOG.md` entry under 'Unreleased' (and `sdk/python/CHANGELOG.md` if changed).
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven.
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
