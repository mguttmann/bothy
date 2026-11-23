### Changed

- Crash reports are off by default, also in builds that name a crash service; a person turns
  them on in the privacy settings.

### Fixed

- "Send again" after an interrupted run keeps the draft you typed in the thread, and says so
  when there is nothing it can send again instead of doing nothing.
- A note about stopped autonomous work with a reason this app does not know shows the server's
  text instead of a wrong reason.
- Closing a terminal session in the app sends an idempotency key, as the console does.
- The reaction dialog refuses more than one character before anything is sent.
- "New agent" follows a runner that starts or stops hosting agents without a restart of the
  app: when the runner's presence changes, when the app returns to the foreground or reconnects,
  and when the dialog opens.
- Quick switcher rows that cannot be opened yet say so and keep the switcher open.
- The status page in the apps refreshes on a timer, so the state of an agent component does
  not go stale.
- The search assistant keeps answering on an instance that allows fewer earlier questions.
- The close button of the `/status` dialog reads "Close".
