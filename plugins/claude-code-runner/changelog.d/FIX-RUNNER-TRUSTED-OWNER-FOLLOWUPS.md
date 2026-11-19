### Fixed

- Background mode: a follow-up the person who triggered the run writes while it works now
  counts as that person's own word, including an approval for a step that needs it. Before,
  Claude Code and the runner's instructions treated every follow-up as a message of another
  session that can approve nothing, so the person had to send it again after the turn ended.
  The run's first message lists the digests of one-time owner codes that only the runner
  knows; the owner's follow-up (decided from the server's data, never from its text) names
  the next code, and the session checks it with a tool. Follow-ups of anyone else and lines
  of other programs on the host keep the old rule. When a session takes a line of another
  program that carries a code the runner wrote, the runner stops the session and the run
  ends `failed` with `background_owner_code_stolen`. Print mode is unchanged.
