# Changelog

All notable changes to the Hermes Agent reference adapter (`plugins/hermes-adapter`) are
documented in this file. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this component adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html). Its version is the
[`VERSION`](VERSION) file next to this one (docs/spec/18-release.md).

## [Unreleased]

### Added

- Python project `hermes-adapter` (no runtime dependencies yet) with ruff (including
  docstring rules), strict mypy over the harness and pytest with a coverage threshold under
  `quality.coverage.python_min_percent`.
- Recording harness `tests/hermes_harness`: the pin of the Hermes Agent version
  (`tests/hermes_pin.toml`), provisioning into a temporary directory from a `git archive`
  export with a read-only checkout, integrity snapshots of the checkout and the installed
  files, throwaway homes with an allowlisted environment, refusing credential store and
  service shims, an egress recorder, a sandbox of the Hermes process tree on macOS, a fake
  model endpoint with fake credentials, the control channel, the gateway process group, the
  normalization of calls and the observations of the six points of
  `docs/spec/09-agents.md` 17.10.
- Probe platform plugin `recording_probe` and the recording scenarios; the live recording is
  part of the default test run and compares the call patterns with the checked-in fixtures.
- Target `make -C plugins/hermes-adapter record` that writes the fixtures again.
