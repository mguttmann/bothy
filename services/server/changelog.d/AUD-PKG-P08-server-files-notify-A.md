### Fixed

- The S3 driver fails a write when the source reports a read error together with the last byte,
  instead of completing the object.
- A multipart upload created for a first part is aborted at once when its id cannot be stored (a
  database error, or the file was deleted meanwhile), instead of staying open.
- An export package gets its `files` row before its object is written and becomes deliverable only
  after length and SHA-256 match; a refused package is purged through its row, and a package whose
  store ended early is removed by the cleanup after `file.failed_row_ttl`, so no object with
  personal data is left without a row that names it.
