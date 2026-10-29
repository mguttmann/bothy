### Added

- FakeServer: `add_forum(..., served=False)` for a forum another agent operates,
  `set_operator` (delivers `access.changed` with the channel added or removed, or no event with
  `announce=False`) and `unbind_forum` (deletes the binding, `agent.config_changed`); a binding
  change of a forum the agent does not serve sends it no event, as on the server.
