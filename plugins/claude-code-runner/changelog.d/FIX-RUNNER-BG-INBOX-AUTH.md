### Security

- Background mode (`runner.launch_mode = "background"`, off by default) no longer takes a
  message of another Claude Code session of the host user for one of the runner's. A follow-up
  reaches a background session through its inbox socket, which every program of the host user
  can write to. The runner now knows its own lines by their `uuid`: when a session takes any
  other input, or a person types into it, the runner stops the session and the run ends
  `failed` with the cause `background_foreign_input`, without an answer to that input. The
  settings file of a run no longer sets `crossSessionInbound` to `accept`, so Claude Code's own
  rule per message applies and a stricter setting of the host is kept; `SendMessage` and
  `ListAgents` are disallowed in a session of the runner; and the runner prompt no longer tells
  the model to follow a message because of its lead-in. What stays open, and the rule for
  operators, is in the README section "Background sessions" under "Before you switch it on".
- The check that a `personal` background turn runs on the stored login now fails closed.
  Nothing a session puts out is accepted unless its newest request record lists the login beta
  of the pinned Claude Code version by its whole name; otherwise the session is stopped and
  the run ends `billing_mode_mismatch`. Before, a session without a request record ran
  unchecked, only the first record was read, and a prefix of the name was enough. A login that
  is gone stays an account error: the run waits for the account as in print mode.
- A background session no longer inherits a value of another run's environment. The launch
  that starts Claude Code's supervisor process now gets a fixed environment without the SSH
  agent and without a run's temporary folder, and every settings file sets `SSH_AUTH_SOCK`, to
  the empty text unless `runner.forward_ssh_agent` is on. Before, a supervisor that a run with
  the forwarded agent had started handed the agent socket to the sessions of later runs.

### Changed

- A follow-up for a background session of tool class `host` is written inside the element
  Claude Code uses for a message of another session, which states the sender's permission
  class. A follow-up whose text holds the name of that element (`cross-session-message`) is
  not written into the running turn; the server defers it into a run of its own.
