### Changed

- The phone app asks once before "Log out" in the Me tab and says what the logout deletes: the
  data of the device, its device keys and, when there are some, the changes that still wait to
  be sent.

### Fixed

- Phone: a terminal session no longer attaches again without its screen. When the app was in
  the background on the terminal and a tapped notification, a link or a share then opened another
  page, the session attached at every return to the app, after a while with a confirmation
  prompt over the other page. It now stays detached until the person attaches it from the hosts
  screen. An opening or an attach that is still on its way when the terminal screen is left is
  stopped, and one that completes while the app is in the background or locked detaches at once
  and attaches on the return.
- Phone: a thread or a channel opened from a notification or a link while the server cannot be
  reached shows a progress mark instead of an empty page and loads when the connection is back.
- Android: a photo taken while the system ended the app is deleted at the next start instead of
  staying in the app's cache.
