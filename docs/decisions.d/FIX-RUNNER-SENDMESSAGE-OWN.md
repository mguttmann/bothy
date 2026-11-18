# FIX-RUNNER-SENDMESSAGE-OWN Background sessions: SendMessage to the session's own subagents

Since FIX-RUNNER-BG-INBOX-AUTH ruling 3 a background session of the Claude Code runner had no
`SendMessage` at all, so a session could not give one of its own running background agents (the
`Agent` tool with `run_in_background`) more to work with; it had to wait for the agent's result
and start a rework round. The owner asked on 2026-11-18 that a session may message its own
background subagents while it still must not message any other Claude Code session of the host.

Everything about the CLI below was read in the file of the pinned Claude Code 2.1.326 and run
with that CLI in print mode against the local stand-in for the API with a configuration folder of
its own (no login, no model call, no background session and no supervisor on the host). The
runs are `tests/real_cli/test_send_guard.py`.

What the pinned CLI does with `to`:

- It resolves `main`, then a teammate, then a recipient of the form of an `agentId`
  (`^a(?:[\w-]{1,63}-)?[0-9a-f]{16}$`, also after lowercasing) among the agents of the session,
  then a name among the agents of the session, and only then a name among the sessions of this
  machine, of Remote Control and of the cloud. `uds:<socket>` and `bridge:<id>` address another
  session directly. A recipient of the form of an `agentId` never reaches another session.
- Run: a name of another live session of the same configuration folder delivered the message
  into that session (`another Claude session on this machine; in that session's inbox`); a
  `uds:` address was sent to that socket; the `agentId` of the session's own agent resumed that
  agent with the message.
- The CLI has a mode that keeps `SendMessage` to the agents of the session
  (`messagingBeyondOwnAgentsDisabled`), but nothing of the command line or of the settings sets
  it: only an embedding host does. `CLAUDE_CODE_HARBOR_KITE=0` switches cross-session messaging
  off, and with it the inbox socket the runner writes its follow-ups to, so it is no way either.
  `SendMessage` has no rule content of its own (`SendMessage(...)` matches no recipient).
- A `PreToolUse` hook runs in `bypassPermissions` too, and exit status 2 refuses the call. Any
  other failure of a hook, and a hook that runs out of time, lets the call through.
  `disableAllHooks` `true` in the user's or the project's settings switches every hook off,
  also one of `--settings`; a `false` of `--settings` goes before a `true` of either.

| No | Date | Ruling | Reason |
|---|---|---|---|
| D-FIX-RUNNER-SENDMESSAGE-OWN-1 | 2026-11-18 | A background session of the runner's own agent (not a hosted turn) under `scoped` and `host` keeps `SendMessage`: the deny list is `AskUserQuestion ListAgents SendFile`. The settings file of the run carries a `PreToolUse` hook for `SendMessage` that runs `plugins/claude-code-runner/src/claude_code_runner/sendguard.py` by its path with the runner's interpreter and `-I`, with `\|\| exit 2` and a time limit of 60 seconds. The hook lets a call through only when `message` is plain text without `notify_when_idle` and `to` is `main` or has the form of an `agentId` and the session's transcript folder holds `subagents/agent-<to>.jsonl` or `.meta.json` and no `subagents/workflows/*/agent-<to>.jsonl`. Everything else, and every error, ends with exit status 2 | the form of an `agentId` is the one recipient the CLI never resolves among other sessions, so the hook does not have to know the other sessions; a name is refused because the CLI looks it up among other sessions when no agent of the session has it; a workflow agent is refused because a message there starts a second copy of it in the same folder; `ListAgents` stays denied because the `agentId` is in the result of the `Agent` tool |
| D-FIX-RUNNER-SENDMESSAGE-OWN-2 | 2026-11-18 | The settings file with the hook also sets `disableAllHooks` `false`. When the host user's own `settings.json` (the folder of `crossSessionInbound`) sets `disableAllHooks` `true`, or a managed settings file (`managed-settings.json` or `managed-settings.d/*.json` in `/Library/Application Support/ClaudeCode`, on Linux `/etc/claude-code`) sets `disableAllHooks` or `allowManagedHooksOnly` `true` or cannot be read, the launch keeps `SendMessage` denied and the file has no hook and no `disableAllHooks`, as before. The runner decides once per launch for both | the project's settings are content of the folder the run works in and would switch the guard off; the host user's choice is never overridden (as `refuse` in D-FIX-RUNNER-BG-HELD-FOLLOWUP-2), and a guard that cannot run must not leave the tool open |
| D-FIX-RUNNER-SENDMESSAGE-OWN-3 | 2026-11-18 | A hosted turn (26 5.3) keeps `SendMessage` denied | its settings switch hooks off (26 5.4) |

What stays open, and is accepted:

- Managed settings of a device management profile or of a server are not read by the runner;
  a host that has them (`/Library/Managed Preferences` or a server's settings) is checked
  before the runner runs there.
- A hook that runs out of its time limit lets the call through. The hook checks a few files and
  starts an interpreter without the package; 60 seconds is far above that.
- A session of class `host` can reach other sessions through `Bash` anyway (the `claude` command,
  or writing to a socket). The hook closes the tool path, as the deny rule did, not that one.
- Print mode was never covered by the deny list of 18.2 and is not changed here.
- Under `scoped` the permission mode `dontAsk` runs only the tools of
  `runner.scoped_allowed_tools`, and the runner does not add `SendMessage` to them: a
  `scoped` session messages its subagents only when the operator names `SendMessage` there,
  as it starts them only when `Agent` is named. The hook refuses every other recipient
  either way. A `chat` turn gets neither the hook nor `disableAllHooks` (it has no tool).

Open minors (for the next run in this area of the runner):

- A subagent that sends `SendMessage` to `main` passes the hook; the main transcript then holds
  a `queued_command` with origin `peer`, and the runner logs `background_foreign_input` `peer`
  for the session's own agent (only a warning in the log). Either the hook refuses `main` or
  the mapper knows such a sender by `senderTaskId`.
- The pinned CLI knows `onFailure` `"block"` for a command hook. With it a hook that runs out
  of its time limit would refuse the call instead of letting it through (accepted above);
  adding it needs a real-CLI test that a timed out hook blocks `SendMessage`.
- `hooks_switched_off` treats a `managed-settings.d` folder that exists but cannot be listed
  like an empty one (`Path.glob` swallows the error); it should keep `SendMessage` denied.
  The runner host has no managed settings folder.
- `sendguard.decide` lets a payload without `tool_name` through; the CLI calls the hook only
  for `SendMessage` (the matcher), so only a missing name would reach it.
- Tests: the real-CLI guard test leaves its `mkdtemp` folder behind and waits for the peer
  session with a fixed sleep instead of a readiness check (a slow peer would let the final
  assertion pass without proof); the scenario of the background launch does not isolate
  `MANAGED_SETTINGS_DIR`; the unit test of the hook command compares the quoted text instead
  of the parsed arguments.

Supersedes for the runner's own agent: D-FIX-RUNNER-BG-INBOX-AUTH-3 as far as it denies
`SendMessage`; `ListAgents` and `SendFile` stay denied.

Deploy note: none beyond the runner's restart. A turn that runs at the restart keeps the flags
and the settings file it was launched with until it ends.
