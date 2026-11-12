### Added

- Configuration key `retention.import_after_completion` and the job `retention.import`: the
  items of an import source and then the source itself are deleted this long after the source
  was completed or cancelled. Until now nothing deleted them, although the privacy chapter and the
  migration guide said so. The imported spaces, channels, threads, messages, files and people
  stay; after the deletion a repeated import of the same Discord server would copy everything
  again. A running or paused source keeps every row.
- Migration `00105` builds a partial index on `messages (import_item_id)` concurrently, so the
  job does not read the table `messages` for every item it deletes.

### Fixed

- A large import no longer stops with `500 internal_error`. `import.progress` carried the whole
  source with the cursor of the import tool, which grows with every thread; once that passed
  `sync.event_payload_max_bytes`, `setImportCursor`, every batch and every state change of the
  source failed. Such an event now carries the state and the counters without `cursor` and with
  an empty `config`, and `truncated: true` in its envelope; `getImport` returns the whole source.
  A cursor above `http.max_json_body_bytes` is `413 payload_too_large`.
- A full import run after the switch-over of a channel no longer changes the threads there. The
  server applied the name, the done state and the last activity of Discord to a posted thread
  also in a channel with the lead `bothy`, so threads that Discord had archived meanwhile were
  closed and threads closed in Bothy were opened again. A posted thread item now changes its
  thread only while Discord leads the channel. New messages of such a run are still taken over.
