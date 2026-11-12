### Added

- `docs/quality/wcag-criteria.md` lists every success criterion of WCAG 2.2 levels A and AA with
  its state, the tests that hold it and what is open; a test holds the page to the list of the
  recommendation and to tests that exist. It is the work list for the accessibility goal of the
  specification, not a conformance report.

### Changed

- A status line of a document opens with a word of a fixed list (`normative`, `built`,
  `planned`, and `design` or `template` for these two kinds of page), defined in `docs/SPEC.md`
  section 8.1; a test holds every status line to the list and to the roadmap. The chapters and
  guides no longer say "draft M0", and the READMEs of `services`, `services/server`, `apps`,
  `packages` and `infra` describe what exists.
- The feature list of the specification has the place `API` for a function the server offers
  and no app yet: changing the tags of a post, managing the tags of a forum, the default
  operator of a space, the behavior of a channel, templates of a channel, and changing or
  deleting a template. A test holds every row of the list against the calls of the apps. The
  views are plan row V3-18.

### Fixed

- The specification and the guides no longer say that the admin console shows the backup runs
  or warns without an off-site target: no operation of the API reads `backup_runs`. They name
  where the state is (exit code, log, metrics and alerts, the table), and
  `docs/operations/backup-restore.md` 1.1 gives the command that reads the runs and saves the
  manifest of a run, which two runbooks asked for without naming one. The view is plan row
  V3-19.
