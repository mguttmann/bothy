### Fixed

- On macOS, dictation and text tools that write through the Accessibility API (the selected
  text or the value of the focused field) insert their text into the composer and every other
  text field again. Before, the tool reported success and read its text back while the field
  stayed empty.
