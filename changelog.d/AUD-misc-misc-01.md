### Changed

- The Semgrep rule `bothy-go-sql-string-building` follows a statement built with `fmt.Sprintf`
  or string concatenation through variables, lists and function literals to the call that runs
  it (taint mode); before, it saw only a statement built inside the argument list. The nine
  places that must build their text from sanitized identifiers carry `nosemgrep` with the reason
  (AUD-MISC-MISC-01/7, /8).
- CodeQL analyses the C and C++ of the Windows runner and of the plugins without a build
  (`c-cpp`). The header of `codeql.yml` no longer claims every supported language: Swift and
  Kotlin need a build of the apps and are named exceptions of the guard, which now derives the
  native languages from the tracked sources (AUD-MISC-MISC-01/9, /10).

### Fixed

- `host-platform.yml` gives `/opt` the owner and mode of a stock system before the platform
  suite on Linux. The hosted Ubuntu image opens the folder to every user, and `bothy-host`
  rightly refused to start below it, which failed every test of the Ubuntu job that starts the
  service (AUD-MISC-MISC-01/1).
- 24 section 14 says that no platform has a recorded passing run of `host-platform.yml`; plan
  row V1-13 tracks the open causes (AUD-MISC-MISC-01/4, /5).
