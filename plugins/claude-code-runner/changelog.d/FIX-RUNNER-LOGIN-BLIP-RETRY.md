### Fixed

- Background mode: a run no longer fails with `billing_mode_mismatch` when Claude Code sent a
  single request without the stored login, which happens in the moment that login is
  rewritten on the host. When the counter-check fails for that reason alone
  (`request_without_login`) and the stopped attempt cannot have acted (no output of it was
  accepted, its process is gone, and its transcript holds no tool call), the runner waits 10
  seconds, checks the login again with `claude auth status` as at the start of a run, and
  resumes the session once with the same inputs. A second request without the login, a tool
  call of the stopped attempt, an adopted turn, a stop during the wait and a login check that
  fails still end the run as before. New log lines: `login_retry_wait` (with `seconds`),
  `login_retry_started`, and the warning `login_retry_refused` with the code of the refusal.
