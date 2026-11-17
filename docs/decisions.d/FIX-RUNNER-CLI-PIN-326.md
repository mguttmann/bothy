# FIX-RUNNER-CLI-PIN-326 Claude Code pin 2.1.326, background mode without print fallback

Owner order of 2026-11-17: only the cheap path. With `runner.launch_mode` `background` the
runner never falls back to `claude -p`; where a background session cannot run, the run fails
with a visible cause; turns with channel skills run in background mode too, hosted agents cannot
(ruling 10).
Everything below ran with the CLI 2.1.326 against the local API stand-in (no login, no model
call, `DISABLE_AUTOUPDATER=1`).

| No | Date | Ruling | Reason |
|---|---|---|---|
| D-FIX-RUNNER-CLI-PIN-326-1 | 2026-11-17 | `PINNED_CLAUDE_CODE_VERSION` is `2.1.326`; every other version, 2.1.325 included, reports `runtime_version_untested`. | Same rule as D-FIX-RUNNER-CLI-PIN-325-1. |
| D-FIX-RUNNER-CLI-PIN-326-2 | 2026-11-17 | The recordings are written again by `tests/real_cli/record.py` (`*_2.1.326*`, `captured/api_errors_2.1.326.jsonl`), the files of 2.1.325 removed. The field table gains only members the runner does not read (`memory_check_detail`, `summary_is_report`, `keeps_request_id`). | A change of the pin is one change with its recordings. |
| D-FIX-RUNNER-CLI-PIN-326-3 | 2026-11-17 | 2.1.326 writes no `system` `turn_duration` after a turn of a background session that ended with an API error (run: a refused request, the refused notification turn, a session without a credential). `TranscriptMapper` ends the turn at the API error record (`result` with `is_error`); a `turn_duration` right after it ends nothing more. Fake-Claude and the real-CLI harness (`turn_ends`) count turn ends the same way. | Without it every refused request left the background turn open: the account-error wait and the retry line never started. This is the only change 2.1.326 forced. |
| D-FIX-RUNNER-CLI-PIN-326-4 | 2026-11-17 | In background mode every former fallback cause ends the run `failed` with `runtime_error` and the code as the cause (`causes.CAUSES`); the log line is `background_unavailable`, a cause of the host still sets `background_unavailable` in the heartbeat. No path from background mode into print mode remains (`RunTurn._fall_back`). | Owner order: print mode costs about twice on the subscription window. |
| D-FIX-RUNNER-CLI-PIN-326-5 | 2026-11-17 | Hosted turns and channel skill folders run as background sessions (supersedes D-BG-2 for them): the launch carries the hosted tool flags, `--restricted` or `--setting-sources project,local`, `--plugin-dir`, `--mcp-config` with `--strict-mcp-config`; the hosted settings are merged into the one settings file of the launch. Run with 2.1.326: the plugin skill is listed, the MCP server starts, the hosted settings apply. The launch files and the supervisor record lie under the runner's own `state_dir` also for a hosted agent. | One settings file avoids relying on how two `--settings` merge. |
| D-FIX-RUNNER-CLI-PIN-326-6 | 2026-11-17 | `--max-budget-usd` is left out of a background launch (it does not act there, design 2.2 item 4); the runner stops the session once the run's cost reaches the cap and ends it `runtime_error` with `cost_cap_reached`. | The cost cap of 26 5.8 must hold in both modes. |
| D-FIX-RUNNER-CLI-PIN-326-7 | 2026-11-17 | Cannot run in background mode, so the run fails with a cause: `organization` (`background_api_key`: a background session takes no API key), the compaction of a full context (`background_compaction`), a hosted turn below `host` (`background_restricted`: run with 2.1.326, `--restricted` binds no inbox socket, so no follow-up and no retry line could reach it). Found, not built: a `--resume --bg` launch with the prompt `/compact` does compact the session in 2.1.326 (a `compact_boundary`, no `turn_duration`); following that end needs a change of the mapper. | Owner order: fail visibly instead of print mode. |
| D-FIX-RUNNER-CLI-PIN-326-8 | 2026-11-17 | The example configuration, the README and the tests name `claude-opus-5-5[1m]` as the model of a run (was `claude-fable-5-2[1m]`). | Owner order of 2026-11-17: no run uses Fable any more. The price tables keep their rows (they price what a run reports, they choose nothing). |
| D-FIX-RUNNER-CLI-PIN-326-9 | 2026-11-17 | 2.1.326 refuses a `--bg` launch whose `--add-dir` folder has no workspace trust ("Added folder not trusted: <folder>"; 2.1.325 launched it; run against the stand-in, trust of a folder above it counts). The runner reads that refusal as `files_untrusted` and ends the run `failed` with `background_files_untrusted`, a cause of the host. The operator trusts `<runner.state_dir>/files` once; the runner writes no trust entry. | Without it every `scoped` or `host` turn with files failed as `background_launch_failed`, which names no remedy. |
| D-FIX-RUNNER-CLI-PIN-326-10 | 2026-11-17 | No hosted turn runs in background mode; a hosted `host` turn ends the run `failed` with `background_hosted_bypass` (supersedes ruling 5 for hosted turns and ruling 6; the runner's own check of the cost cap in a background session is removed again, it had no other caller). Turns of the runner's own agent with channel skill folders stay in background mode with `--plugin-dir`. | Run with 2.1.326 against the stand-in: a `--bg` session launched with `--permission-mode bypassPermissions` and `--setting-sources project,local` (26 5.3) records the permission mode `default`, holds the runner's inbox line ("The sending session's permission mode class doesn't match") and would wait for permission prompts; the same launch without `--setting-sources`, and the own `host` turn with a plugin folder, keep `bypassPermissions`. `skipDangerousModePermissionPrompt` or `permissions.defaultMode` in the launch's settings file, or `skipDangerousModePermissionPrompt` in the folder's local settings, did not change it. Loading the host user's settings into a hosted agent would undo the isolation of 26 5.3. |

## Open

- SonarQube Cloud on pull request 1748 (quality gate passed, nine code smells, none of them a
  bug): cognitive complexity (python:S3776) of `app.py` `_adopt_background`, of
  `runturn.py` `_start_session`, `_abort_launch` and one more function of the background part,
  and of two functions of `bgtranscript.py`; the `timeout` parameter (python:S7483) of
  `bgsession.launch` and `BgSession.wait`; and python:S7632 at the comment in
  `bgsession.launch` that says "NOSONAR below", which the analysis reads as a malformed
  suppression. A split of `runturn.py` is a ticket of its own.
- Not repeated at this pin change: the search of the CLI file for tools that reach other
  sessions (the deny list of FIX-RUNNER-BG-INBOX-AUTH is a list of findings) and an error
  `result` in the schema table of `tests/real_cli/record.py`.
- Hosted agents in background mode (ruling 10): they need either a way of the CLI to keep
  `bypassPermissions` with `--setting-sources project,local`, or an owner decision to load
  the host user's settings into hosted agents. Until then a runner that serves hosted agents
  keeps `runner.launch_mode` `print`.
- A full context in background mode ends the run (`background_compaction`); a `--resume --bg`
  launch with the prompt `/compact` compacts the session in 2.1.326 (ruling 7) and would let
  the run go on once the mapper follows a `compact_boundary` without a `turn_duration`.
