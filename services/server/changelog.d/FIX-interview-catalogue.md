### Fixed

- The agent interview sends the template catalogue to the builder agent and checks a draft
  against it: a drafted MCP server that equals a server of a shipped or private template except
  its name now passes, instead of every drafted server being refused as `mcp_not_in_catalogue`.
