### Fixed

- One turn record that the runner cannot handle no longer stops its start. A record with a
  process ID outside the system's range, with an ID that is no text, or in a state folder with a
  fault ended the start of the runner, and with it the service for every built-in agent it
  hosts. Such a record is now logged (`turn_record_unreadable`) and left, and the start goes on.
- An orderly stop of a runner that hosts several built-in agents stops every agent's turns also
  when stopping one connection fails. Before, the first failure ended the stop for all others.
  The failure is logged with the agent and the kind of the error (`hosted_stop_failed`).

### Security

- The runner no longer acts on a file in the place of a turn record that it did not write. A
  record must be a regular file named after its run, with UUIDs for the thread and the session
  (with `runner.turn_handover` the record of a run that has not started yet carries no session);
  before, a session reference with path characters made the runner create a lock file and
  folders outside its lock folder at the next start.
- At its start the runner stops a leftover turn of a built-in agent only when the command line
  of the process names the record's session as the argument of `--session-id` or `--resume` and
  the control folder of that agent. Before, a record could name any process of the runner's
  user whose command line contained a chosen text, and the runner stopped it.
- The log line for a failed follow or takeover of a turn after a restart names only the kind of
  the error (`adopted_follow_failed`, `turn_takeover_failed`). Before, it carried the message of
  the error and its traceback, which can hold a path or a value.
