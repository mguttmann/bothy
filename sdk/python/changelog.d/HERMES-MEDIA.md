### Changed

- The FakeServer answers `createMessage` of `kind = voice` with an uploaded attachment with
  `409 file_not_ready`, as the server does for a file whose scan has not ended; the FakeServer
  never scans an upload.
