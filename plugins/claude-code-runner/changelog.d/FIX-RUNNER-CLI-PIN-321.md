### Changed

- The pinned Claude Code version is 2.1.321 (was 2.1.288). A host with 2.1.321 reports `online`
  again and `check-config` prints `runtime ok`; 2.1.288 now reports `runtime_version_untested`.
  The recordings of the CLI (stream field table, agent tree, resumed session, captured API
  errors) come from 2.1.321 and are written by `tests/real_cli/record.py` against a local
  stand-in for the API, without a login or a model call.
- `config.example.toml` names a real model for `runner.default_model`. A placeholder such as
  `model-large` reaches the API unchanged and is refused there; with a fallback model in the
  Claude Code settings a turn loads, Claude Code then answers that turn with the fallback
  model without a sign in the thread.

### Fixed

- A turn the API refuses with "Claude Code <version> or later is required for your
  organization" (a text Claude Code knows since 2.1.321) ends like the other answers to an
  outdated CLI: `runtime_error` with a box note that names the required version, and the
  runner reports `degraded` with `runtime_version_unsupported` until the CLI is updated.
- A workflow whose agents start side by side no longer shows the waiting agents twice in the
  run's tree. Claude Code lists such an agent first without its ID; the runner now keeps the
  agent's step when the ID appears instead of adding a second one that stayed `running` until
  the workflow ended.
