### Changed

- The specification states that each device of a person navigates on its own (01 6.7, R-PR-24;
  04 14, R-SY-29): the route, the selected sidebar row, the open columns and the scroll position
  stay on the device; only data (messages, read state, counters, drafts, settings, sidebar order,
  notification preferences) syncs. Two client tests hold the rule with two devices of one person
  against the fake server: a desktop app keeps its route while the other device opens a thread,
  marks it read and receives a new message there, and the same with a phone app next to it.
