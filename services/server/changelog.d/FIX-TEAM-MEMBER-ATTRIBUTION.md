### Fixed

- A team copy names the one member it came from: the mirror of a member's reply in the team
  channel and a member's input in another member's work thread carry `team_handle`, the handle of
  the member of the reply's work thread, also when one agent is several members (09 3.9.2). New
  column `messages.team_handle` (migration 00110, filled for the copies written before it) and the
  optional read-only `Message.team_handle` of the API.
