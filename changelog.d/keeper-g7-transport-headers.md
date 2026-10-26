### Fixed

- Contract tests: G7 no longer sends generated `Host`, `Content-Length`, `Transfer-Encoding`
  or `Expect` headers, which the HTTP server of the standard library answers itself before any
  handler; an empty `Content-Length` made G7 fail now and then (issue #238).
