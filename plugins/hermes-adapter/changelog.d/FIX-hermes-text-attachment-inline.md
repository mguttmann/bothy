### Fixed

- A text file sent to a Hermes agent (a long paste the app sends as `paste.txt`, a Markdown,
  JSON, YAML, CSV, log or diff file) reaches the turn with its text, as a data block of the
  user's data, and Hermes' note says the content is included. Before, the turn held only the
  file's cached path, so an agent without read access to the profile's media cache could not
  open it. A text over `hermes_adapter.max_message_bytes` keeps the path note plus a note that
  its content was too large; binary files and images are passed as before. An `@file:`,
  `@url:` or `@diff` reference in the file's text stays text: Hermes does not expand it.
