### Fixed

- GitHub app: a failed write of the state directory (a full disk) keeps the job queued and
  retries it instead of dropping it, and a delivery whose state write failed is taken back, so
  the redelivery neither runs twice nor reuses a sequence number.
- GitHub app and Jira app: `job.failed` and `http.failed` name the class of an unexpected error
  in `code`; the unused log attribute `step` is gone.
- Cost reconciliation app: passes are `reconciler.pass_interval` apart (default `1h`) instead of
  `reconciler.min_request_interval`, so the server's audit trail no longer fills with a
  submission of every day of the lookback each minute.

### Changed

- The READMEs of the GitHub app, the Jira app and the cost reconciliation app and
  docs/spec/11-extensions.md section 10 state the limits: lost `503` deliveries need a manual
  redelivery, one instance per state directory, a backup of the state directory, the token of one
  account instead of an app identity, and tests only against fakes of the providers.
