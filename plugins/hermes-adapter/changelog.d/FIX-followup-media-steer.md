### Fixed

- A follow-up with an image or another file sent while a run is open now reaches the agent at
  once. The adapter waits for the server's check of the files, stores them in Hermes' media cache
  as for a normal message, and passes the person's text with the path of each file into the
  running turn, so the agent can open them. A picture sent as the answer to a question of the
  agent answers it the same way. Before, such a follow-up waited for the end of the run. A voice
  message still waits for the end of the run, where it is transcribed; a text sent with it is
  passed on at once. Every follow-up the adapter does not pass into the turn now gets a log line
  with the reason. New key `hermes_adapter.followup_file_wait`.
