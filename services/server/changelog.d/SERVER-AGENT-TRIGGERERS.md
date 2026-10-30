### Added

- `GET /api/v1/agents/me/triggerers` (`getAgentTriggerers`, action `agent.triggerers.read`,
  agent-self): the allow list of an agent adapter (09 17.7) with the people who may trigger the
  calling agent and the rights version of the request. Before, the route answered `404` and every
  adapter with the allow list on, the Hermes adapter among them, refused every run with
  `adapter_error`.
