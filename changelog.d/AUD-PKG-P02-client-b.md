### Changed

- "Log out" in the desktop sidebar asks once, as on the phone, and names the changes that still
  wait to be sent.

### Fixed

- When the keychain refuses access while connecting, at the start or while a device waits for
  confirmation, the app shows the error instead of staying on "Connecting".
- "Connect" no longer runs twice at once.
- A browser login on the desktop can be cancelled while it waits.
- The "Live" view opens through its own route.
- On macOS, "Get the update" says when the app cannot open the update itself.
- iPhones register with their device name and model instead of "localhost".
- A failed server read is no longer repeated by the app on top of its own read retry.
- Connections and the local database are released after discovery, after a failed open and at
  the end of an account.
- A forum opened from a link before it was synced shows its posts when it arrives.
- Mentions: "Load more" loads a page once, and a failed page shows its error with "Try again".
- A restored draft keeps its reply and shows who it replies to.
- An upload saved by an older app version no longer replaces the chat history with an error.
- A message that cannot be saved on the device stays in the input with a note instead of being
  lost.
- Two quick presses on the channel column or the overview pin end where they started.
- "Trigger again" stops watching once its notice is closed.
- Approval cards with a restriction from a newer server stay locked.
- Turning crash reports off while they start really stops them.
- App metrics are sent when the app goes to the background and before logging out.
- Cold start and thread open timings no longer count a login, the app lock or the wait for a
  first message.
- The offline search no longer asks again and again for a transcript the server refuses.
- The share sheet searches after a pause in typing and says when a share could not be saved.
- A notification level or quiet hours rule the server refuses no longer stays visible.
- The push and share extensions keep their session in the keychain while it is rewritten, and
  reuse one network session per process.
- The Windows app names its product and license in its file properties instead of template
  values.
