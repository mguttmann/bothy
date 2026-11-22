### Added

- API contract of the team channels (FEAT-TEAM-CHANNEL-SERVER): the operations `getChannelTeam`,
  `enableChannelTeam`, `disableChannelTeam`, `addTeamMember`, `removeTeamMember` and
  `stopChannelTeam`, the schemas `Team`, `TeamMember` and `AgentTeamFacts`, the trigger kind `team`
  of `Run` and of `AgentInvocationPayload` with its member `team`; the Dart client knows them.
