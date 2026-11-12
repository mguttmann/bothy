### Fixed

- The Mac app no longer asks for the permission to show notifications when it starts, before
  the login: it asks when its first notification is about to show.
- A notification of the desktop apps goes away once its thread is read, on this device or on
  another one (on Windows in an installed package). Until now it stayed in the notification
  list until the person removed it. The banner of a phone that is in front goes the same way.
- A failure of push, of sharing from other apps on iOS and macOS, of a local notification or of
  the sending of app metrics, which the apps pass over so that the rest keeps working, now
  leaves a line in the developer log of the app: the step and the type of the error, never a
  text, a name or a token.
