### Changed

- Computer use is faster per model round (27 6.4, 7): every `batch` item before the last goes to
  the host with the new optional `cu_action` member `quick`, so it waits 50 ms instead of
  `computer.settle` and takes no screenshot; only the last item waits `computer.settle` and returns
  the image. The host names the feature `quick` in Noise message 2, and `bothy-computer-mcp` sends
  `quick` only to a host that names it, so older hosts keep the earlier behaviour. The parts of a
  long `type` text before the last go with `quick` as well. `open_session` returns the first
  screenshot of an active session, and the tool descriptions steer toward fewer rounds (a batch
  with its `wait`, the host ID of `list_desktops`, pages opened by their address, keys instead of
  the pointer).
