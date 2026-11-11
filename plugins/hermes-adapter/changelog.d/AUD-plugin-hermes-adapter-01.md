### Fixed

- A run in which a person answered a card of the agent (a click or a typed answer) now ends with
  its token counts, costs and thread title, and it stays open while a delegation or a background
  process the turn started is still at work. Before, such a run ended at the moment its turn
  finished, without costs, and background work went on without a run.
- A typed answer to a question or an approval card is no longer processed twice. The adapter now
  acknowledges the answer Hermes took, so the server does not start a second run with the same
  text after the run ended.
- A voice message sent while a run is open is no longer passed to the agent as an empty message
  and then marked as taken. It waits for the end of the run, and the next run takes it as its
  message and transcribes it, like a follow-up with any other file. The same now holds for audio
  sent together with a text.
- Stopping a run after a card of it was answered now always ends the run as stopped. Before, such
  a run could end as completed when stopping its work took a moment, so a message sent meanwhile
  could start a new run although the person had pressed stop.
