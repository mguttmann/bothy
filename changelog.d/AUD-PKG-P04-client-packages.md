### Changed

- The forum and channel views read the tags of all threads of a channel in one query instead of
  one query per thread on every change.
- The fake server of the client tests answers as the server where the difference could hide a
  client defect: a failing handler is a `500` with a recorded violation instead of an empty
  success, `moveThread` answers `409 operator_mismatch`, removes the tags and writes
  `thread.moved_out`, a refused first message leaves no thread behind, `Idempotency-Key` acts
  only on operations that declare it and checks its UUIDv7 format, a proof over the previous
  nonce gets the current one on a success, a half-open gateway session can be resumed, and the
  terminal stream refuses an expired token and closes a revoked session with `4012`.
- `terminal_ui` pins its renderer `xterm` to exactly 4.0.0, the version its escape filter and
  painters are built against.

### Fixed

- The apps end a REST request whose server accepts the connection and then stays silent
  (`client.http_connect_timeout`, `client.http_receive_timeout`), so such a request no longer
  holds its outbox lane until the app restarts.
- A read whose server asks to wait longer than the backoff cap (`Retry-After`) fails at once
  with that answer instead of waiting for hours inside the request.
- A step-up with the device key asks the server for its challenge after the person answered
  the prompt, so a slow answer no longer sends an expired challenge, and requests that need a
  step-up at the same time share one prompt and one confirmation.
- After a nonce, proof or token repeat, a terminal request that needs a step-up goes back to
  the terminal's own step-up instead of a second prompt without a purpose.
- A request waits at most `refreshCallerDeadline` for a token refresh and then fails as a
  network error, so a hanging key store or signer call no longer blocks every request until
  the app restarts.
- Signing out ends the session and deletes the local data also when the key store fails to
  delete the stored token.
- After the server closes the connection with `4015`, `4017` or `4018`, the offline banner of
  the desktop and the phone names the reason and offers "Reconnect"; before, only an app restart
  (or, after `4017`, hiding the window) brought the connection back.
- A connection attempt that runs into `client.sync.connect_timeout` before its socket opens
  aborts that socket, so no second socket opens beside the next attempt.
- Retrying, editing, removing or discarding an outbox entry, the echo confirmation and a
  confirmed response store their changes over several rows in one database transaction, so an
  interrupted change no longer leaves a failed entry beside its replacement.
- Failing background steps of the sync (sends, the outbox start, the echo confirmation, the run
  cleanup after a resync, the device poll after `4016`) are logged instead of raising an
  unhandled error, for example after the local data was deleted.
- A gap in a thread's messages that an app start forgot is found again from the stored messages
  when the thread gets the focus and loaded, so messages behind it no longer stay hidden.
- A refused read position or draft write no longer stays in the outbox for good; it is deleted
  and logged.
- The stored sync cursor never moves back within one sync epoch while pages are applied; a resync
  still takes the position of the server's snapshot.
- The statistics views load again when the host changes the space, the period or the source, and
  the timeline shows agent names that arrive after it loaded, instead of short IDs.
- A defect inside a statistics view is reported to the app's error handler instead of showing
  only "Something went wrong", and an invalid `viz.timeline_speeds` falls back to the default
  speeds.
- Sorting the table of the live topology keeps the agent tree of each run under its run.
- In the run graph, the arrow keys scroll the selected step into view, and the selection stays
  on its step when new steps arrive above it.
- The run history filters by the wire value of the run state.
- In the terminal, Cmd+Shift+S (Apple systems) or Ctrl+Shift+S reads the visible screen to a
  screen reader, and the screen's hint names the keys; on macOS, Windows and in the browser the
  command "Read screen" was only a custom action, which their screen readers never offer.
- The web console's terminal in a browser on Windows keeps input methods and dead keys; the
  workaround for the Windows app applied there too.
- "End" of a terminal session closes the tab only when the server confirms the end; on a failure
  the tab stays detached with the problem and offers "End" again, instead of showing the session
  as ended while it kept running on the host.
- The ten run reasons that showed as a raw code (for example a runtime without a login, an
  adapter restart or a used-up usage window) show a translated text.
- The screen reader label of the live box leaves out the context use when it was not measured
  instead of reading "context 0 percent", and the personal backend badge without a known holder
  no longer ends with an empty name.
- The inputs of a form card are named by their field label for screen readers.
- The number of command suggestions and the title of a card reach screen readers.
- The primary button shows its hover and pressed colors.
- Times and days format in UTC for a time zone the app's database does not know, instead of
  failing to show the screen.
- The collapsed section "Archived (n)" of the sidebar shows the unread mark and the mentions of
  the archived channels it hides.
- A chat moved between two channels you can both see no longer disappears from your device with
  its messages; it shows up in the new channel with its history, unread count and search hits.
- The terminal's screen reader hint for "Read screen" names the keys in the app's language, for
  example "Strg+Umschalt+S" in German.
