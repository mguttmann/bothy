### Added

- Search assistant (chapter 14 4.10, 09 10.6): `AgentHandler.on_search_assist` answers the
  ephemeral `search.assist_requested` with an `AssistReply` (picks, terms and the usage of the
  model call); the adapter runs it until the request's `expires_at` and sends the reply with
  `answerSearchAssist`. `None`, an error or the deadline send nothing. Module
  `bothy_sdk.search_assist` with `AssistQuestion`, `AssistCandidate`, `AssistChoice` and
  `AssistReply`; the FakeServer sends questions with `request_search_assist` and records the
  replies (`search_assist_answer`).
- Generated core of `assistSearch`, `answerSearchAssist` and `search.assist_requested`; both
  operations are in the retry table (`agent_runtime`, `read_only`).
