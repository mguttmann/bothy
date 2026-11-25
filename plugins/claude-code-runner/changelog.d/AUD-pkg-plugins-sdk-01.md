### Fixed

- The runner log is a JSON-line log again: every line carries its time, level and logger, and
  the HTTP client no longer writes a line with the full address of every request.
- SIGHUP stops the runner like SIGTERM. A stop that fails is logged as `stop_failed` with the
  kind of the error only and the runner exits 1 instead of hanging until the service kills it.
- A `stop-turns` marker left by a stop command that was killed no longer turns the next
  restart into a stop of every turn; the marker counts only while its command runs.
- `runner.poll_interval` and `runner.cli_probe_timeout` of zero are refused at start instead of
  making every wait loop spin or every probe fail.
- A trigger attachment whose scan found malware or failed is reported as `file_blocked`, no
  longer as `file_not_ready`.
- One turn record whose adoption fails no longer stops the start of the runner: it is logged
  with `turn_adoption_failed`, kept for the next start, and the other records are adopted.
- After a restart, live turns the runner adopts count against `runner.max_parallel_runs`, so the
  runner no longer starts up to that many further turns beside them.
