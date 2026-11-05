### Added

- Search assistant (ADR 0075, chapter 14 4.10): `POST /api/v1/search/assist` (`assistSearch`,
  action `search.assist`, people only, `rl.trigger`) answers a question in the person's own words
  with a short list of sessions. The server turns the question and the earlier questions of the
  popup into terms, runs the thread title and message search with the person's rights, fuses the
  hits per thread and, when `search.assist.agent_id` names an assistant agent with a live gateway
  connection, sends the candidates that agent may read itself as the ephemeral event
  `search.assist_requested` (no token, no ids). The agent answers with
  `POST /api/v1/search-assist-requests/{request_id}/answer` (`answerSearchAssist`, action
  `search.assist.respond`): picks with a reason, further terms and the usage of its model call.
  The person gets the picks first, then full-text hits, each with the thread, the matching message
  as jump target, channel, space and time, the terms of the question and a cost estimate for
  information. Without an agent, without a connection or after `search.assist.timeout` the answer
  is full text with `fallback_reason`. Nothing of a question is stored or logged.
- Configuration keys `search.assist.agent_id`, `search.assist.timeout` (below
  `http.request_timeout`), `search.assist.max_candidates`, `search.assist.max_hits`,
  `search.assist.max_terms`, `search.assist.max_turns`, `search.assist.excerpt_max_chars` and
  `search.assist.reason_max_chars`.
