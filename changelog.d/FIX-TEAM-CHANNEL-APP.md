### Changed

- Team channels in the app: an owner's message that mentions members by `@handle` now says "to
  @a, @b" instead of "to all members", since it reaches only the mentioned members; a message
  without a member mention still says "to all members".
- The add form of the team dialog lists every active agent of the server and filters it as you
  type, offers only the home channels the agent is no member with yet (and no agent whose every
  home channel is taken), and fills in a handle from the chosen home channel that stays editable.
- In the composer of a team channel `@` lists every member at once with its agent; further letters
  filter the list, and a tap, a click, Enter or Tab inserts `@handle`. On the desktop arrow up and
  down move through the list and Escape closes it. German and English.
