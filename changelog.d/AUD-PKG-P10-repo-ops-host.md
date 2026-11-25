### Changed

- A failed job of `host-platform.yml` prints the owner, mode and ACL of every folder on the way to
  the program folders of the host service and of its platform suite, and the free disk space on
  Windows, so a red run names the folder behind a refused start.
- `make test` runs the tests of the setup wizard `install.sh` only with the area `tools`, so a full
  CI run starts their container once; the workflow policy check refuses a write permission on top
  level and the `workflow_run` trigger (AUD-PKG-P10-repo-ops-host).
- `make check` runs the deny list first, and `make gates` runs commitlint and fragments-check before the long test suites.
- `make semgrep` no longer hides what Semgrep skipped (large generated files, rule timeouts).
- The collector container of the Compose stack has a memory limit, set with the new variable
  `BOTHY_OTELCOL_MEMORY_LIMIT` (default `1g`); its `memory_limiter` percentage now counts against
  that limit instead of the memory of the whole host (AUD-PKG-P10-repo-ops-host).
- The example edge of the multi-node overlay is sized for `perf.ws_connections_per_node` on both
  nodes (`worker_connections`, `worker_rlimit_nofile`, the container's `nofile` limit) and keeps
  idle connections to the nodes for REST requests (AUD-PKG-P10-repo-ops-host).

### Fixed

- CI jobs that run Gradle restore a cache of their downloaded dependencies per job, and both
  Gradle builds retry a repository request that fails with 408, 429 or a 5xx six times with a
  growing pause, because Maven Central refused downloads with HTTP 429; in those jobs every
  Gradle build gets Maven Central from Google's public mirror of it (AUD-PKG-P10-repo-ops-host).
- The Dart API pages of the documentation site link a video of Flutter's doc comments instead of
  embedding it from another origin (AUD-PKG-P10-repo-ops-host).
- The Android emulator jobs fail without a device instead of passing without a test, and `ci-ok`
  requires the SonarQube scan in a pull request that has the secrets for it
  (AUD-PKG-P10-repo-ops-host).
- actionlint checks the shell code of the workflows with the pinned shellcheck.
- commitlint no longer accepts any message whose header starts with `Merge `; only a merge in progress is exempt.
- A root changelog fragment no longer selects the full CI job plan.
- Generated `*_gen.go` files no longer count in the Go coverage threshold.
- The Semgrep egress rule reports WebSocket dials without an own HTTP client.
- A server release section must have the headings Upgrade notes, Migrations and Compatibility.
- A promoted macOS build starts its phased rollout at the promotion: `appdist.py promote-appcast`
  sets the `pubDate` of the entry to that time (AUD-PKG-P10-repo-ops-host).
- A Windows release (app and host) stops before signing when `windows.timestamp_url` of the
  distribution is empty, so no release is signed without a timestamp (AUD-PKG-P10-repo-ops-host).
- The summary and runbook of `BothyMalwareScanFailing` say that it counts scans that end with
  `error`, not uploads that stay pending (AUD-PKG-P10-repo-ops-host).
- The contrast gate of the design tokens compares the unrounded contrast ratio with the WCAG
  threshold, so a pair at 4.496:1 no longer passes 4.5:1; the accent step search uses the same
  rule (AUD-PKG-P10-repo-ops-host).
- `make bootstrap` asks for Node.js 24, the highest `engines.node` of the npm packages it
  installs, instead of 22.13; a test holds the check to every tracked `package.json` and to the
  `node-version` of the workflows (AUD-PKG-P10-repo-ops-host).
- The root `osv-scanner.toml` drops the license overrides of `jni` 1.1.0 and `jni_flutter`
  1.0.4+1, which the lock no longer holds, and a test fails on such an orphaned Pub override
  (AUD-PKG-P10-repo-ops-host).
- The SPDX header gate also checks OpenTofu, SQL, C and C++ and PowerShell files; the OpenTofu
  modules, one SQL query file and the Windows and iOS runner sources that lacked the header carry
  it now, and the Windows runner states the Apache-2.0 license as its legal copyright instead of
  "All rights reserved", like the macOS app (AUD-PKG-P10-repo-ops-host).
- The design-token generator ends with exit code 2 and one line naming the theme and the role
  when a role is missing in one theme, and naming the member when the rule file lacks
  `contrast_pairs` or `palette_guards`, instead of a `KeyError` traceback
  (AUD-PKG-P10-repo-ops-host).
- `make signal-check` fails when the dashboards, the rule files, the Sloth files or the panel
  queries hold no expression, and `make templates-check` fails when the template folder holds no
  template, instead of passing with less to check (AUD-PKG-P10-repo-ops-host).
- The external asset check of the docs site also fails on images, frames, media, objects,
  embeds and on `<link>` icons, manifests, preconnects and DNS prefetches of another origin, not
  only on scripts, stylesheets and fonts (AUD-PKG-P10-repo-ops-host).
- The terminal catalogue gate reports a test table of chapter 24 section 14 with a header other
  than `ID | Test | Expected` and always compares the title and expected cells, instead of
  silently comparing IDs only (AUD-PKG-P10-repo-ops-host).
- `make signal-check` ends with one line and exit code 2 on a dashboard or rule file it cannot
  read or parse, and the SonarQube sync with one line on a read timeout or a malformed answer of
  the Web API, instead of a traceback (AUD-PKG-P10-repo-ops-host).
