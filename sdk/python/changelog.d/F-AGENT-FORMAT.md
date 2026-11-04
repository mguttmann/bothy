### Added

- `bothy_sdk.message_format` with `MESSAGE_FORMAT_RULES` (also exported by `bothy_sdk`) and
  `MESSAGE_FORMAT_TITLE`: the format rules every agent adapter hands its runtime on every run
  (running text without hard line breaks, fenced code with a language, GFM tables, a `text`
  block for text to copy, no block quotes for it).
