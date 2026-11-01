### Security

- A locked password sign-in no longer ends the session of a device that confirms actions with
  the password and the 6-digit code (the Windows app and the web console). Anyone who knew the
  login name could lock the password sign-in with wrong logins; the correct password was then
  counted as a failed confirmation until the session was revoked. Now the app stops asking and
  says that the password sign-in is locked, to try again later or to confirm on another device.

### Fixed

- A device that cannot confirm actions at all (for example the Windows app of a person without
  a local account) now says so instead of "The confirmation was cancelled" in the new project
  dialog, or a revoked session on the device and session screens.
