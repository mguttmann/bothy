### Added

- `RunHandle.finishPending` and `RunHandle.retryFinish`: an end the server did not store stays
  with the handle and can be sent again.
- `InboxStoreOptions.closedRetentionMs` and `CLOSED_ENTRY_RETENTION_MS` (one day): how long
  `MemoryInboxStore` and `FileInboxStore` keep a closed entry.
- FakeServer: `trigger(text, { command })` queues the run of a model command; a follow-up gets a
  row of its run (`followupState`) that a patch moves forward only; `costs: true` or
  `enableCosts()` accepts `costs` in `finishRun` (`costs(runId)`); `reportAgentSession`
  (`sessionReports`), `getInstance` (`limits`) and the upload session `createUpload`,
  `uploadPart`, `completeUpload` and `abortUpload` (`uploadPartSize`, `upload(fileId)`).
- `test/fake-contract.test.ts` holds the routes of the FakeServer and the form of every answer
  equal to `docs/api/openapi.yaml`.

### Changed

- `InboxStore.prune` also drops an entry that stayed closed for the retention; the adapter
  prunes its inbox at its start and every hour. Before, no entry and no `jti` row ever left
  the inbox.
- The FakeServer's `createMessage` refuses a content above `limits.message_max_content_bytes`
  (`/content_md`, `too_long`) and attaches completed uploads named in `attachment_file_ids`.

### Fixed

- A run that a person starts with a model command (`trigger_kind = command`) now starts. The
  adapter compared no `command_id` for such a run, refused its token and ended the run `failed`
  with `invalid_invocation_token`.
- The end of a run is no longer lost when `finishRun` gets no answer or a `5xx` after the
  retries, for example while the server restarts. The adapter used to close the run on its side,
  and the server ended it `interrupted` although the work was done. It now keeps the run in
  `active_runs` and sends the end the handler asked for again until the server stores or refuses
  it.
- `PollingSource` no longer sends heartbeats without a pause when `HeartbeatAck.interval_ms` is
  zero, negative or missing; it keeps its current interval.
- The adapter forgets the token of a run when the run is over for it, instead of keeping the
  token of every run it ever held.
