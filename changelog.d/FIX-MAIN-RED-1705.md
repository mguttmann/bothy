### Fixed

- CI: the accessibility audit of the phone screens on the iOS simulator starts a screen again
  and audits it once more when XCTest ended the audit at its own time limit ("Audit failed to
  complete in time"). Such an audit has no verdict; the first start of the freshly installed app
  on a loaded runner took that long in a main run and failed `apps (iOS)`. A second time limit
  fails as before, and so does every issue an audit reports.
- CI: the Fake-Hermes tests of the Hermes adapter start the gateway again when Hermes reported at
  its start that it could not import one of its tool modules. Hermes imports them on a thread
  while it builds the platform adapters, and every test of a delegation that failed
  `test (SDKs and plugins)` in two main runs had that line in its gateway log. How the failed
  import leads to the failed test is not shown yet (docs/decisions.d/FIX-MAIN-RED-1705.md).
