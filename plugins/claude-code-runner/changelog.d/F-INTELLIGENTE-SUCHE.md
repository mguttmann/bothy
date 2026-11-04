### Added

- Search assistant (09 18.16): the runner answers `search.assist_requested` with one `claude -p`
  call without tools and without a session on disk, the fixed instruction
  `prompts/search_assist.md` and the candidates as a marked data block. The reply becomes picks
  of the sent references and further search terms, sent with the usage of the call; a failed
  call, an unreadable reply or the time limit sends nothing (`search_assist_failed`). New keys
  `runner.search_assist_model` (default `haiku`) and `runner.search_assist_timeout` (default
  `9s`).
