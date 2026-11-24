### Fixed

- The S3 driver fails a write when the source reports a read error together with the last byte,
  instead of completing the object.
- A multipart upload created for a first part is aborted at once when its id cannot be stored (a
  database error, or the file was deleted meanwhile), instead of staying open.
- An export package gets its `files` row before its object is written and becomes deliverable only
  after length and SHA-256 match; a refused package is purged through its row, and a package whose
  store ended early is removed by the cleanup after `file.failed_row_ttl`, so no object with
  personal data is left without a row that names it.
- With the S3 driver the scan pass completes the multipart upload of a confirmed file before it
  checks clamd, so a scanner outage no longer keeps the bytes in an incomplete upload that a
  storage lifecycle rule may abort.
- The media type detector keeps its own read limit of `file.sniff_bytes` instead of changing a
  process-wide limit of the library in its constructor, so a second detector can no longer make
  Office documents read as ZIP archives.
- Reopening the session of an import file sets a failed file pending again with `file.updated`
  for the uploader, and aborts after the commit the S3 multipart upload that the old session still
  named, instead of dropping its id and leaving the upload open.

### Changed

- Migration `00123` adds the partial index `files (deleted_at, id) WHERE deleted_at IS NOT NULL`,
  so the catch-up and the backlog gauge of `files.cleanup_pending` no longer read the whole
  `files` table on every run.
