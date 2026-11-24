### Fixed

- A built-in agent made from a template whose MCP server names its secret as `env:NAME` (for
  example `research-summarizer`) no longer ends every run with `mcp_reference_unavailable`: the
  runner reads `env:NAME` of a definition as the file `NAME` in the agent's own folder
  `<runner.hosted_secrets_dir>/<agent_id>/`, with the same checks as a `file:` reference there. It
  never reads the runner's environment for it; a missing file still ends the run with that cause
  and logs only the error code.
