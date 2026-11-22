### Added

- The token check of an `agent.invocation` with `trigger_kind = team` compares the token's
  `message_id` with the payload's, as for a message run; before, such a token had no
  counterpart and was refused. The FakeServer sends team runs (`trigger(..., trigger_kind=
  "team", authority=..., team=...)`) and follow-ups of the agent Texter (FEAT-TEAM-CHANNEL-RUNNER).
