### Fixed

- The token check of an `agent.invocation` with `trigger_kind = team` compares the token's
  `message_id` with the payload; before, every team run was refused as `invalid` (09 3.9.9).

### Added

- The FakeServer serves team runs (`trigger(..., trigger_kind="team", team=..., authority=...)`),
  a follow-up written by an agent, and `FakeServer.token_run`, the run an accepted
  `Invocation-Token` header named.
