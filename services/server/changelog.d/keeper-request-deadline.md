### Fixed

- A request that ends at `http.request_timeout` while it waits for the database answers
  `503 unavailable` with `Retry-After` instead of `500 internal_error`, also where an adapter
  wrapped the driver's deadline error as an internal error.
- The serve processes and harness servers of the tests run requests up to the test patience
  (`http.request_timeout`), as their database deadlines do, so a loaded runner no longer turns a
  slow request into 500 or 503.
