### Changed

- `make check` runs the deny list first, and `make gates` runs commitlint and fragments-check before the long test suites.
- `make semgrep` no longer hides what Semgrep skipped (large generated files, rule timeouts).

### Fixed

- actionlint checks the shell code of the workflows with the pinned shellcheck.
- commitlint no longer accepts any message whose header starts with `Merge `; only a merge in progress is exempt.
- A root changelog fragment no longer selects the full CI job plan.
- Generated `*_gen.go` files no longer count in the Go coverage threshold.
- The Semgrep egress rule reports WebSocket dials without an own HTTP client.
- A server release section must have the headings Upgrade notes, Migrations and Compatibility.
