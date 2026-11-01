### Fixed

- A session that works in the background stays reachable (09 18.5). A turn that starts a
  background job, subagent or workflow can end with a short answer: the runner keeps the Claude
  Code process and its stdin open while that work runs, instead of closing stdin after the
  answer, which made Claude Code stop the work. Every message of a person meanwhile becomes a
  turn of its own in the same session and gets its own answer within seconds, such as a status
  question; when the work ends, Claude Code reports it and that answer is posted in the thread.
  The run stays `running` until the background work is done and reports progress meanwhile, so
  it is not shown as stalled. The system prompt now tells Claude Code to start long work in the
  background, end its turn at once and never wait for background work inside a turn.
  Every turn now also carries `CLAUDE_PRODUCT_RUN=1`, so a user-level hook that keeps headless
  turns open while background tasks run lets the turn end.
