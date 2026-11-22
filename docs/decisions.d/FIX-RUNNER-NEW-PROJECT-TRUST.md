# FIX-RUNNER-NEW-PROJECT-TRUST Background mode: the runner trusts a bound project folder

Error from operation, 2026-11-22: the owner created a new forum through the app
dialog "New project / existing folder" and bound it to a folder below `runner.projects_root`.
Every run there failed: the runner log has `background_unavailable` with
`background_workspace_untrusted` and then `run_finished` `failed` `runtime_error`; the app showed
the cause "does not trust this project folder". The CLI's global configuration file of the
runner's host user had no entry `projects["<folder>"].hasTrustDialogAccepted` for the new
folder (read in the copy taken before the manual repair; the folders of the forums that worked
had the entry from an earlier interactive `claude` there). Since FIX-RUNNER-CLI-PIN-326 such a
run fails instead of using print mode, and D-FEAT-RUNNER-BG-SESSIONS-13 kept the trust entry a
manual step of the host user, so every newly bound folder needed a step on the host before its
first run.

| No | Date | Ruling | Reason |
|---|---|---|---|
| D-FIX-RUNNER-NEW-PROJECT-TRUST-1 | 2026-11-22 | Before every background launch in the folder of the forum's binding the runner records workspace trust for that folder in the CLI's global configuration file (`$CLAUDE_CONFIG_DIR/.claude.json`, else `$HOME/.claude.json` of the launch environment), unless the entry is there already (`workspacetrust.grant_trust`, called by `RunTurn._trust_workspace` while it holds `BackgroundMode.launching`). Limits: only the real path of the folder itself and only when it is a strict descendant of the real path of `runner.projects_root` (never the root, a parent or a path outside it, also not through a link); only for a binding, never for `runner.default_workspace`; only the one member `hasTrustDialogAccepted` of that entry, every other member of the file as read; the write takes the CLI's own lock of the file (the folder `<file>.lock`), reads the file again under it and replaces it atomically with mode `0600`; a lock the CLI holds is waited for at most `runner.cli_probe_timeout` and never taken; a legacy `<config dir>/.config.json`, a missing file or one that is no JSON object is not written. Log: `workspace_trust_granted` when the runner wrote the entry, `workspace_trust_failed` with the code (`locked`, `legacy_config`, `config_missing`, `config_unreadable`, `write_failed`) when it could not, both with the run ids and no path. A folder that stays untrusted makes the launch refuse and the run ends `failed` with `background_workspace_untrusted` as before; its text now says that the runner trusts only bound folders and that the log names why it could not. Supersedes the sentence "The runner never writes the CLI's trust entry" of D-FEAT-RUNNER-BG-SESSIONS-13 and 09 18.9 for the project folder of a binding. Evidence: `tests/scenarios/test_bg_workspace_trust.py` (a bound folder without the entry runs and gets exactly that entry, the next run writes nothing; the default workspace gets none; a held lock leaves the file as it was, logs `workspace_trust_failed` `locked` and ends the run with its cause), `tests/unit/test_workspacetrust.py`; Fake-Claude refuses like the pinned CLI while the entry is missing (`bg_trust_check`). | Owner goal: a folder the owner binds runs without a step on the host. Binding a folder to a forum is the owner's decision that the agent works there, and a binding needs the right `manage` on the forum (03 R-DM-54); the runner already confines every binding to `runner.projects_root` (18.1) and creates folders there itself (18.12). What trust adds is that the CLI loads the folder's project settings (hooks, MCP servers, permissions); a run of tool class `scoped` or `host` in that folder can already write those files and run what the class allows, and in print mode, which ran in these folders without trust until 2026-11-17, nothing asked for trust either. The entry is the manual step the CLI itself names in its refusal ("set projects[...].hasTrustDialogAccepted: true"). |
| D-FIX-RUNNER-NEW-PROJECT-TRUST-2 | 2026-11-22 | Rejected: (a) print mode for an untrusted folder; (b) only a clearer message; (c) trust of `runner.projects_root` itself; (d) trust at the time of binding (readiness, 18.1). | (a) contradicts the owner order of 2026-11-17 (D-FIX-RUNNER-CLI-PIN-326-4, no path from background mode into print mode). (b) leaves a step on the host for every new project, which the owner goal excludes; the message stays for what the runner does not cover. (c) would trust every folder below the root, bound or not, and the CLI does not count a trusted parent for a Git repository below it (design 2.2 item 5). (d) readiness runs for every served forum in every mode and often, while trust is needed only by a background launch; at the launch the runner holds the lock of its launches and knows the run. |
| D-FIX-RUNNER-NEW-PROJECT-TRUST-3 | 2026-11-22 | Not covered: an existing entry with `hasTrustDialogAccepted` `false` is set to `true` (the CLI writes `false` as the default of every entry it creates, so a refusal of the host user cannot be told apart); a folder whose trust the CLI judges by another key than its own real path (the CLI checks only the repository root for some Git layouts) still refuses, and the run ends with `background_workspace_untrusted`; the runner never removes an entry it wrote, also not when the binding goes; the folder of chat files (`background_files_untrusted`, D-FIX-RUNNER-CLI-PIN-326-9) stays a one-time step of the operator. | The binding is the owner's word for that folder; the other cases end with a cause that names the remedy. |

## Open

Minors of the review (2026-11-22), for the next run in this area; each fails safe (the run ends
with `background_workspace_untrusted`):

- A folder name in NFD form never matches: the CLI normalises the key to NFC
  (`workspacetrust.grant_trust` writes the real path as stored). Write the NFC form.
- `global_config_file` ignores the CLI's file name suffix (`.claude-custom-oauth.json` with
  `CLAUDE_CODE_CUSTOM_OAUTH_URL`); no production login uses it.
- A lock a crashed CLI left is never judged stale (proper-lockfile: 10 s), so the outcome stays
  `locked` until a CLI clears it; and the `finally` would remove a lock the CLI took over after
  that time (the runner holds it for milliseconds).
- A `UnicodeEncodeError` of the write (a lone surrogate in the file) is no `OSError` and is not
  mapped to `write_failed`.
- Ruling 3 says the CLI judges some Git layouts by the repository root only; in 2.1.327 that
  branch (`rootOnly`) is not reached, so the sentence is cautious, not wrong.
