### Added

- `bothy version` prints the server version and the commit of the build on one line; the image
  build and `make -C services/server release-build` embed the commit through `-ldflags -X`
  (`unknown` for a plain `go build`).
- The access log line `http request` carries `bothy.request.duration` (seconds until the handler
  returned).
- The counter `bothy.job.discarded` counts jobs whose last attempt failed, per job kind; its
  series start at 0. The alert `BothyJobFinallyFailed` (ticket) fires for `privacy.erase`,
  `instance.key_rotation` and `sync.partition_maintenance`.
- `extendGroupMember` (`POST /api/v1/groups/{group_id}/members/{user_id}/extend`, new action
  `group.member.extend`, owner with step-up, audited): a time-limited membership in effect gets a
  later expiry on the same row, so access never lapses.
- `SearchAssistResult.candidates_capped` says that a search of the question reached
  `search.max_candidates`, so older sessions may be missing.
- `bothy setup bootstrap-owner` loads the embedded start prices into an empty price table right
  after the first owner (new system action `price.load_start`, audited with the number of rows).

### Changed

- `retention.content` also empties the texts of runs (status line, excerpt, summary, plan), the
  titles and details of their steps and the label, condition and result of ended wake-ups, and
  the titles of live threads whose last activity lies before the period (`thread.updated`
  without an actor). The rows stay.
- The search assistant asks only an agent with `billing_mode = organization`; any other agent
  falls back to full text with `assistant_not_configured` and a warning in the log.
- An imported message whose content breaks the content rules of a message no longer rolls its
  batch back: the item is stored as `failed` with the field code in `error`, and the batch goes on.

### Removed

- The owner switch `supports_title` of `agent_operator_config.v1`: a write that sends it gets
  `400 validation_failed`, a stored value is ignored.
- The unused port `Store.IPKeyByID` of the audit module and its query; spec 20 no longer promises
  a check of a known IP against the audit log.
