### Fixed

- `completeImport` no longer answers `409 conflict` with `reason` `lead_not_bothy` because of an
  imported channel that was deleted or archived here (or whose space was archived) while the
  source still led it. Its lead could no longer change (`setImportLead` answers `404`), so the
  source could never be completed. Such a channel no longer blocks the completion; a live channel
  that the source still leads keeps the `409` (22 section 13.5).
