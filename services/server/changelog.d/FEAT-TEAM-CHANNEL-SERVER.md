### Added

- Team channels (09 3.9): an owner switches a text channel to team mode (`enableChannelTeam`,
  `disableChannelTeam`), adds agents with their home channel and a handle (`addTeamMember`,
  `removeTeamMember`, `getChannelTeam`) and stops every run of the team (`stopChannelTeam`);
  actions `team.read`, `team.update` and `team.stop`, owner only. Each member gets one work
  thread in its home channel.
- An owner's message in a team channel reaches every member as a copy in its work thread, a
  follow-up of a running run or a new run with `trigger_kind = team`; a member's final reply is
  mirrored into the team channel, and every member it mentions by handle gets it the same way on
  behalf of the channel's sponsor. The job `runs.team_input` does the copying; every input and
  every delivery is checked with `agent.invoke` for the owner, and the readers of the team
  channel and the home channel must be the same people.
- The safety brake of a team channel: `team.turn_budget` member inputs since an owner's last
  message, `team.pingpong_max` alternations between two members, and the stop; every pause posts a
  note and is audited. New keys `team.turn_budget`, `team.pingpong_max`, `team.max_members` and
  `team.handle_max_chars`.
- Migrations 00106 and 00107: the tables `team_channels`, `team_members`, `team_inputs` and
  `team_mirrors`, and the trigger kind `team` of runs.

### Changed

- The command name `team` is reserved; agent messages outside team channels still never trigger.
- A member's reply hands on only when it was sent with the invocation token of the run it names
  in `run_id`; the `run_id` alone no longer links an agent's message to a run. Adapters that send
  through the SDK outbox already do this. A reply edited after it was written is not handed on.
- `updateMessage` refuses to edit a live box anchor (`kind = run`) with `403`, like a system
  line.
