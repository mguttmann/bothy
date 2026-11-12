### Added

- The agent list of the administration (desktop and web console) has "Add adapter agent": it
  creates the agent of a Claude Code runner, of Hermes or of an adapter of your own from a name,
  a handle and the adapter kind, and opens its page, where its token is created. Until now such
  an agent could only be created with a request to the API.
- The page of an adapter agent shows its tool class with what the runtime may do and changes it
  with a reason ("Change tool class"), shows which features are switched off and the default
  model and changes them ("Edit features"), and has "Disable" and "Enable" with a confirmation
  that says what ends and what does not come back. A disabled built-in agent has "Enable" too.

### Fixed

- The interview of "New agent" no longer ends in "Too many entries" when the builder keeps
  asking past the server's limit of turns: the interview then ends with the draft, and "Use what
  we have" works at any length.
- In "New agent" and on the card of a built-in agent an ended session and a refresh that could
  not reach the server are no longer called a cancelled confirmation; in the whole administration
  a refresh that could not reach the server reads as "No connection" instead of an ended session.
- The agent page reads the agent, its targets and the people side by side instead of one after
  the other, and no longer reads the people, the spaces and the channels twice or the groups and
  hosts at all; the names of a refused switch are read when the refusal arrives. An answer for
  an agent that is no longer shown cannot replace the shown one.

### Changed

- The guide "Connect your own agent" and chapter 11 section 13 name the owner steps the
  administration has a screen for (the agent, its token, the grants) instead of all of them.
