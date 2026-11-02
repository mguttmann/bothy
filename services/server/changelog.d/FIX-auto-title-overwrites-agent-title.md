### Fixed

- An automatic thread title no longer replaces a title set on purpose: `setThreadTitle` takes
  `auto = true` for a title a reference adapter suggests after a run, and such a title only fills
  a thread whose title is empty or a placeholder; otherwise the thread stays unchanged (`200`).
  A status title an agent put on its forum post therefore survives the owner's answer (09 section
  12 point 5).
