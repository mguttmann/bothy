### Fixed

- The adapter stays installed when Hermes replaces the Python environment of the profile, which
  Hermes `v0.22` does on almost every CLI command: before, the gateway started after a new
  environment without the adapter's `.pth` file and `attrs`, and the agent was silently offline.
  `install` now writes a self-contained plugin directory (`packages.json`, `attrs` copied into
  `vendor/`), and nothing goes into Hermes' environment any more.

### Added

- `check` is the doctor of an install: it prints `adapter: loadable` or `adapter: not loadable:`
  with the reasons (plugin not enabled, a file of the plugin entry or a package missing, a
  dependency Hermes ships gone) and exits with `1`; a missing package also stops the plugin at
  the gateway's start with an error in the gateway log that names it.
