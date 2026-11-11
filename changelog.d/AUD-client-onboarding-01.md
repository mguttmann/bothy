### Changed

- The apps show the main view of a signed-in person from the local data as soon as they start,
  before the instance answers, and apply what the instance says afterwards. Until the first
  screen is known they show the mark of the product alone instead of the server address form.

### Fixed

- A signed-in app no longer waits on "Connecting" and then on the login screen at its start when
  the instance does not answer, for example while a VPN is off.
- A device whose secret store lost the key of the local database is no longer stuck on "The app
  could not open its local data": without a stored login the local data are created anew and the
  login shows; with a stored login the server address screen offers "Reset local data", which
  keeps the login and loads everything again.
- When a part of the local data cannot be deleted at a sign-out, the screen "You are logged out"
  says so instead of "The data of this device has been deleted", and the app deletes the rest
  at its next start.
- "Log out" no longer waits for an instance that does not answer: after a limit the app cuts the
  connection and deletes the local data of the device.
