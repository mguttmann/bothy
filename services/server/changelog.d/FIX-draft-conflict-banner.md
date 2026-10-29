### Fixed

- A draft write reports `overwrote_newer` only when it replaced different text or files that
  another device saved after the version the client last saw (04 14, addendum of 2026-10-29).
  A device's own earlier write, which its next keystroke replaces before the answer arrives, a
  draft with the same content, and an empty draft no longer report it, so the app stops showing
  "This draft replaced a newer version from another device" while one device types alone.
