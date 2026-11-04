### Fixed

- Copy as text (01 6.12 item 7, R-PR-35): a code block without a language or with the language
  `text`, `plaintext`, `txt`, `markdown` or `md` has a second button "Copy as text" on macOS,
  Windows, iOS and Android. It copies the block as running text: the line breaks inside a
  paragraph become spaces, while paragraphs, list items, headings and Markdown hard breaks keep
  their lines, so a text an agent broke by hand pastes into a mail or another chat as normal
  paragraphs. "Copy code" still copies the exact text, and blocks in a programming language do
  not offer the new button. The toggle "Wrap lines" is now called "Wrap display" and its
  tooltip says that it changes only the display, never the copy.
