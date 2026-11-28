### Changed

- Computer use waits only as long as the screen needs (docs/spec/27-computer-use.md 6.4): `wait`
  and the settle after the last action of a `batch` end once the screen stayed unchanged for
  300 ms after a change, or when nothing changed for 1.5 s (`wait`) or 400 ms (settle); the image
  of the result is the last screenshot the host compared, so no extra capture follows. They end at
  the latest at `duration_ms` or `computer.settle`, so a video or a game waits as long as before.
  The comparison runs in the host service on the screenshots of the helper; the helpers are
  unchanged. Small changes such as a blinking caret or a spinner do not count. A `wait` before the
  last item of a batch keeps its full time. A raised `computer.settle` is now only the longest
  settle, no longer the time every settle takes.
- The tool descriptions of `bothy-computer-mcp` steer toward one `batch` per decision, keys in
  every program and a `wait` with the time the step usually needs as the last item.
