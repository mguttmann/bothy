### Fixed

- With several api nodes, a device whose session was closed on another node got 4102 without a
  reason instead of 4100 with the close reason when the host's data connection ended before the
  pairing node learned that the session had started; the relay now reads the session row first.
