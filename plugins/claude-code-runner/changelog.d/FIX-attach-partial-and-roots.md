### Changed

- Answer attachments go out best effort: every file that passes its checks and uploads is sent
  with the answer, and only the refused files are named below it, in the order of their
  `ATTACH:` lines. Before, one file outside the allowed folders stopped every other file of the
  answer (`set_not_sent`). The crash-safe record keeps each file uploaded once and the answer
  sent once.
- The notice line of a file outside the allowed folders says how to send it: copy it into
  `TMPDIR` and attach the copy.
- The runner prompt tells Claude to write files for the person into the project directory or
  `$TMPDIR`, and to copy a non-secret file from anywhere else into `$TMPDIR` before attaching
  it.
