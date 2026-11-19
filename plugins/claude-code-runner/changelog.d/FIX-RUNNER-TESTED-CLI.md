### Added

- `runner.tested_claude_code_version` and `runner.tested_claude_code_pin`: a Claude Code version
  besides the pin that the operator proved with the contract subset of the real-CLI suite
  counts as tested while the pin entry names the code's pin. The version probe reports it
  `online`, `check-config` prints `runtime ok`, and background mode runs on it; any other
  version stays untested. `check-config` reports an entry proved against another pin as
  `pin_changed` (a failure). The real-CLI tests take the expected version from
  `CLAUDE_CODE_RUNNER_TESTED_VERSION` when it is set.

### Fixed

- Background mode compares the runner's own supervisor with the version of the installed CLI
  it runs on, so a supervisor started by a tested CLI no longer fails the next launch with
  `background_supervisor_version`.
