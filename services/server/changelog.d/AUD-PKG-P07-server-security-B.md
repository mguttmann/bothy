### Fixed

- An enrollment code redeemed on a device that is already known and pending is refused with
  `409 conflict` and reason `device_pending` and stays usable, instead of being spent while the
  device stays pending.
- `identity.unlink` no longer counts the keyless web device of a console login as a device that
  can approve a pairing, so it cannot remove the last usable login method.
- The flush steps of the database phase in the shutdown (`audit_denials`, `stats_flush`) get
  their own budget, the new key `shutdown.database_flush_timeout`, which the wait for cancelled
  jobs no longer takes; before, a hanging job could spend the whole total and the open denial
  aggregations were neither written nor reported as dropped.
- The incoming webhook routes of the internal listener answer `503 unavailable` with
  `Retry-After` once the shutdown began, instead of writing while the jobs and the pool stop and
  failing with an internal error after the pool closed.
