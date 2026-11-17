# FIX-RUNNER-BG-HELD-FOLLOWUP Background sessions: a value of the host user's crossSessionInbound

On 2026-11-17 the production runner on the owner's host answered the same message of the owner's
steering thread in up to four runs. The host user's own `~/.claude/settings.json` has held
`"crossSessionInbound": "hold"` since September, and the settings file of a run set no value
since FIX-RUNNER-BG-INBOX-AUTH ruling 2. So every follow-up the runner wrote into a running
background turn was held by Claude Code 2.1.325 (transcripts `a144156b`, `d43d3be0`, `d80b173e`,
`e3dec38e`: "Held peer message ... not delivered to Claude (n held). Your "crossSessionInbound"
setting is "hold"", the CLI's cause `explicit-setting`), no echo came, the follow-up stayed
`delivered`, and the server deferred it into a run of its own and offered it to each later run of
the thread as a follow-up again (runner log: message `01a1eadd` written in runs `01a1eab5`,
`01a1eade`, `01a1eadf`, `01a1eae0`). No background transcript of that day shows a line of the
runner that a session took. Print mode, which the reference logs of 2026-11-11 and of the
morning of 2026-11-17 show, has no inbox socket and was not affected.

| No | Date | Ruling | Reason |
|---|---|---|---|
| D-FIX-RUNNER-BG-HELD-FOLLOWUP-1 | 2026-11-17 | The settings file of a run sets `crossSessionInbound` `accept` when the host user's own `settings.json` in the turn's configuration folder (`CLAUDE_CONFIG_DIR`, else `$HOME/.claude`) gives that setting a value that is not `null`, also an unknown one; otherwise it sets none, as before. The runner reads the file at every launch. This replaces, for a host whose settings have a value, the sentence of D-FIX-RUNNER-BG-INBOX-AUTH-2 that the settings file sets no value, and the part of D-FIX-RUNNER-BG-INBOX-AUTH-8 that a follow-up then waits for a run of its own. Evidence: level one `test_the_token_of_a_session_s_own_child_is_no_narrower_way_in` with 2.1.326 and 2.1.325 (the runner's line in its envelope is held under the user's `hold` without the run's value and taken with its `uuid` with it; red with the code before this change), the scenarios `test_a_hold_in_the_host_user_s_settings_does_not_keep_the_runner_s_follow_up` (`host` and `scoped`: the follow-up is `absorbed`), the unit tests of `settings_document`. | A value of the user's settings switches the CLI's rule per message off for every session, so there is nothing narrower left to keep: `hold` and `refuse` keep the runner out entirely, an unknown value holds every message, and `accept` takes every message anyway. A value of `--settings` goes before one of the user's settings, and only `accept` lets the runner's lines through. Changing the host user's settings is not the runner's to do (they are the operator's, shared by every session of that user), and an operator rule in the README alone did not hold: the value had been there since before background mode existed. What the run's `accept` widens: a runner session on such a host takes every message of every sender; the runner logs each one it did not write (FIX-RUNNER-BG-FOREIGN-LOG), and the model acts on it with the rights of its tool class. A value of managed settings still goes first and is not overridden. |

Deploy note: none beyond the runner's restart. The next launch of each background turn reads
the host user's settings; a turn that runs at the restart keeps the settings file it was
launched with until it ends.
