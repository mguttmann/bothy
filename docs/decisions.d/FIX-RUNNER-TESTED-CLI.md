# FIX-RUNNER-TESTED-CLI A proved Claude Code version runs without a change of the pin

Owner order of 2026-11-18: the pin must not block automatic updates of the Claude Code CLI. The
operator's update chain proves a new CLI with the free contract subset of the real-CLI suite and
then runs the same runner code on it; background mode, which ran only on
`PINNED_CLAUDE_CODE_VERSION`, failed every run on such a CLI (`background_version_untested`).

| No | Date | Ruling | Reason |
|---|---|---|---|
| D-FIX-RUNNER-TESTED-CLI-1 | 2026-11-18 | The pin stays the default. Two keys of the runner configuration, `runner.tested_claude_code_version` and `runner.tested_claude_code_pin`, name one further version that counts as tested like the pin (version probe `online`, `check-config` `runtime ok`, background mode runs). Both keys or none, each `major.minor.patch` in digits and at least `MIN_CLAUDE_CODE_VERSION`; any other version stays `runtime_version_untested` and fails background runs. | The configuration file is the operator's own and already names the program (`runner.claude_bin`); a writer of it can point the runner at any binary, so the entry adds no new trust. A file in the state folder would be writable by more of the runner's own processes than the configuration. |
| D-FIX-RUNNER-TESTED-CLI-2 | 2026-11-18 | The entry counts only while `runner.tested_claude_code_pin` equals the code's pin. Otherwise it is ignored and `check-config` prints `runner.tested_claude_code_version pin_changed`, a failure; with a valid entry it prints `... ok`, and without one no line (the output stays as before). | The contract subset compares the new CLI with the recordings of one pin (`stream_fields_<pin>.json`, the error texts, the names of background mode). A deploy that moves the pin invalidates that proof, so the stale entry must be removed in the same step. |
| D-FIX-RUNNER-TESTED-CLI-3 | 2026-11-18 | The supervisor check before a launch compares the supervisor's version with the installed CLI the mode accepts (`BackgroundMode.accepted_version`), not with the pin. | With the pin a supervisor the runner itself started from a tested CLI read as `background_supervisor_version`, so the second of two runs that start together failed. |
| D-FIX-RUNNER-TESTED-CLI-4 | 2026-11-18 | The real-CLI tests that held the installed version equal to the pin hold it equal to `CLAUDE_CODE_RUNNER_TESTED_VERSION` when that is set (`tests/real_cli/record.py` `expected_version`), and the probe and `check-config` tests then pass the tested version or write the entry. Everything else of the subset is unchanged and still compares with the pin's recordings. The subset includes `tests/real_cli/test_send_guard.py`. | The version equality is no proof by itself; the field table, the error texts, the names of background mode, the named pipe and the refused request are. A tested version also runs `SendMessage` behind the hook of D-FIX-RUNNER-SENDMESSAGE-OWN-1, and a CLI that stopped running that hook would fail no run, so the subset proves the hook too. |

## Not covered

- The contract subset proves neither a real turn (level two) nor the mechanics of
  `test_background.py`; a CLI change there shows as failed runs after the switch.
