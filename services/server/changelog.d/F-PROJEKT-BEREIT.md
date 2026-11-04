### Added

- `POST /messages/{message_id}/retrigger` (`retriggerMessage`) sends a message that failed with
  `project_not_ready`, `operator_cannot_serve_project` or `operator_not_in_channel` to the agent
  again once the cause is fixed: only its author, with the rights of triggering; a thread whose
  operator never completed a run in it takes the channel's operator (09 3.8).

### Fixed

- A project forum can no longer be bound to, or given, an operator that cannot serve a project:
  `putProjectBinding` and `setChannelOperator` answer `400 validation_failed` with the field code
  `operator_cannot_serve_project` unless the operator is an agent with `adapter_kind`
  `code_runner` (09 18.16).
- A binding that is not ready under such an operator reads `ready_reason`
  `operator_cannot_serve_project`, and a message there fails with that reason instead of a bare
  `project_not_ready`.
