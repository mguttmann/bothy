### Fixed

- CI: the status test of the real checks adds its minute rows of `sli.api_availability` with the
  same additive upsert as the flush of the own measurement. The server measures the requests of
  the test into the same minutes, and when its flush wrote such a row first, the plain insert of
  the test failed on the unique key, so the test failed once and passed on its rerun (issue #238).
