### Added

- Moving a project folder to the trash (FEAT-delete-spaces-chats, 09 18.15):
  `build_heartbeat(project_trash_results=...)` and `RuntimeStatus.project_trash_results` send
  the outcome of each folder move (distinct channels, a reason of the form `REASON_FORM`), and
  a readiness entry may carry `trash_check`, whose reason has the same form.
- `AgentHandler.on_heartbeat_ack` receives every heartbeat the server accepted through
  `sendHeartbeat` together with its `HeartbeatAck` (from `Adapter.send_heartbeat` and the polling
  source), so a runner reads `HeartbeatAck.project_trash`; the gateway's `heartbeat_ack` carries
  no `HeartbeatAck`.
- FakeServer: `request_project_trash` hands a folder move in every heartbeat answer until a
  heartbeat reports its outcome; `project_trash_results` lists the reported outcomes.
- Generated models and operations of the deletion of spaces, channels and chats.
