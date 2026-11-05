### Added

- `GET /run-history` (`listRunHistory`, `run.overview.read`) pages the finished runs across the
  threads the caller may read, newest end first, with the filters `space_id`, `agent_id` and
  `state` and a cursor; each entry carries the space, model, metrics (tokens, subagents) and the
  chat of the run, and every run passes `run.read` like in `listActiveRuns` (25 6.7). The
  additive migration `00092` builds the partial index `runs_finished_idx` concurrently.
