### Fixed

- An enrollment code redeemed on a device that is already known and pending is refused with
  `409 conflict` and reason `device_pending` and stays usable, instead of being spent while the
  device stays pending.
- `identity.unlink` no longer counts the keyless web device of a console login as a device that
  can approve a pairing, so it cannot remove the last usable login method.
