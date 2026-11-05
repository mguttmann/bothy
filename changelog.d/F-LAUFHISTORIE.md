### Added

- The page "Live" has a tab "Run history" ("Laufhistorie") next to the live topology, on the desktop
  apps, the mobile apps and the web console: the finished runs (completed, failed, cancelled) newest
  first, filtered by the space of the page and by end state, each with its chat, agent, start, end,
  duration, end state, cost ("Estimate", final amount or "information only"), input, output and
  cache tokens and subagents, page by page with "Load more runs". A note says how far the history
  reaches: runs stay as long as their thread. A click opens the detail with the same figures, the
  model, the summary and the last text of the agent, the steps as the tree of the run graph
  (subagents, background jobs, tool calls with duration and state), and buttons that open the chat
  or jump to the message that started the run (25 6.7). TV mode does not rotate to it.
- The fake server of the sync engine answers `listRunHistory`.
