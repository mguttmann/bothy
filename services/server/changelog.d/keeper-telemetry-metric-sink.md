### Fixed

- The telemetry sinks of the test harness give every server start a fresh metric reader: after a
  start retried on a taken public port, collecting metrics failed with `reader is shutdown`.
