### Added

- Plan rows V3-20 to V3-45 for the parts of the product that the specification and the guides
  promised but that are not built (among them the deprecation headers, versions of shared admin
  objects, telemetry of the SDKs, process metrics, the price table for owners, previews of
  further rights changes, the privacy link and AI notice, the visible state of the virus scan,
  the environment banner and the accessibility of the web console).
- Tests that hold documents to the built artifacts: the unpaginated lists of 06 against the API
  contract, a ratchet on missing descriptions of the API contract, the command table of `bothy`
  against the binary, table ownership of the query files and the tiers of 02 against the module
  register.

### Changed

- `make config-doc-check` refuses a planned configuration key whose milestone is finished; each
  such key now names its real place or a V3 plan row.
- `CONTRIBUTING.md` and the help of `make check` no longer say that `make check` is identical to
  CI; they name the jobs that run only in CI.

### Fixed

- The specification, the operations guides, the architecture diagrams, the roadmap and the
  accessibility checklists describe what is built: autonomous runs (stage AR), deprecation
  headers, time format, versions and pages of the API, `instance.environment`, the off-site
  targets, the upgrade check, the service name of the Tailscale profile, presigned uploads,
  the demo, `bothy files rescan`, the price table, app interactions, avatars, the channel
  export, the backup service in the diagrams and the state of Renovate.
