### Added

- The search has two more filters: "Agent" finds the conversations with one agent, and "Tag"
  finds the posts of the chosen forum that carry a tag.

### Fixed

- A file found by the search can be opened: the hit leads to the message the file is attached
  to. Before, a file hit could not be selected at all.
- A search that takes too long now asks to narrow it, and a search refused because of too many
  searches names how long to wait, instead of "The search did not work. Try again." for both;
  the hits stored on the device stay visible. The search assistant shows the same two texts.
- The search view says "Searching" while it waits for the server, shows a failure at once
  instead of repeating the request for minutes, and stops the request of an earlier search when
  a new one starts.
