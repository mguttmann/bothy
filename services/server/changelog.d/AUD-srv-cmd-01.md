### Changed

- `bothy import discord`: an embed is taken over with the name of its author, its fields (one line
  `<name>: <value>` each) and its footer text, not only with title, link and description. A bot
  message whose statement lies in its fields no longer arrives with the title alone.
- `bothy import discord`: the attachments of a forwarded message are no longer dropped silently.
  They are still not loaded, but the message gets one line per attachment with the file name, the
  size and the jump link to Discord, and the report counts them (`attachments_forwarded`).
- The check report has two more counters: `attachments_forwarded` and `pins_not_taken`. The text
  report shows them in the columns "Attachments" and "Pins".
- The error of a call the server refused names the place, the rule and the item, for example
  `applyImportBatch: status 400 validation_failed, batch thread:<id>, /items/1/data/content_md
  empty of message:<id>`, instead of the bare status. It carries no content.

### Fixed

- `bothy import discord --apply` no longer stops for good at Discord data the server refuses.
  A channel topic with line breaks or above `channel.topic_max_chars`, a thread name or a forum
  tag name with control or bidi characters, a message that converts to no text (an embed that
  holds only an image) and more pins in a thread than `pin.max_per_thread` each made the server
  refuse the whole batch, and every new run ended with exit `1` at the same place. The tool now
  sends names and topics without those characters and cuts the topic, counts such a message as
  skipped (`empty`) and counts the pins above the limit as not taken over; the dry run reports
  the same, except for the pins, whose limit only the server knows.
- The report of a full run shows the notes that arise during the run (a channel that became
  unreadable, a message that is too long, the new notes above). They were lost before.
- The read mirror says why a thread stays behind: the note of the channel and a line on standard
  error name the thread and the error. Before, the reason went to a debug log that is never
  written, and the report repeated "a thread failed" in every round.
- `bothy import discord switch` no longer concludes that the Discord channel is not locked when
  messages stay behind because their thread failed in the last round; it names the thread and
  the reason instead.
- `bothy import discord switch` and the migration guide no longer advise to change
  `planned_operator` or to add a source with a corrected mapping file after the import. The
  mapping of a source cannot be changed, so both ways ended with exit `2` or with a second copy
  of everything; the texts now name `--force` and say when a new source is still a way.
