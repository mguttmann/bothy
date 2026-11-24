### Fixed

- The contrast gate of the design tokens compares the unrounded contrast ratio with the WCAG
  threshold, so a pair at 4.496:1 no longer passes 4.5:1; the accent step search uses the same
  rule (AUD-PKG-P10-repo-ops-host).
- `make bootstrap` asks for Node.js 24, the highest `engines.node` of the npm packages it
  installs, instead of 22.13; a test holds the check to every tracked `package.json` and to the
  `node-version` of the workflows (AUD-PKG-P10-repo-ops-host).
- The root `osv-scanner.toml` drops the license overrides of `jni` 1.1.0 and `jni_flutter`
  1.0.4+1, which the lock no longer holds, and a test fails on such an orphaned Pub override
  (AUD-PKG-P10-repo-ops-host).
