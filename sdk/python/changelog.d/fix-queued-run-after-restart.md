### Fixed

- The Fake-Agent (and with it the demo agent) keeps the events of its first REST poll after a
  restart: a run queued while the adapter was down is received and started instead of staying
  `queued` until `run.queued_max_age`.
