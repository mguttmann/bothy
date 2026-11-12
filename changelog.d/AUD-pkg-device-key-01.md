### Fixed

- A device that lost a key of its registration is no longer stuck with a login the server
  refuses. This happened without a word when the device passcode was turned off and on (iOS and
  macOS delete the key for confirmations then), after a restore, or when another key took the
  place of the registered one: the app created a new key and kept the old login, so every
  request or every confirmation failed. The app now compares its keys with the ones it
  registered at every start, shows "You are logged out" with the reason "The key of this device
  can no longer be used", deletes the data of the device, and registers as a new device at the
  next login. The old entry stays under Devices for the person to remove.
- On Android, new biometrics invalidate the key for confirmations; the next confirmation now
  ends the login on the device the same way instead of failing without a way forward.
- On Windows, a device whose key was lost (for example after the TPM was cleared) no longer
  stops on the server address screen with the keychain text at every start.
- The apps no longer create, read or delete device keys on the thread that draws the screen,
  and on iOS and macOS an open Touch ID, Face ID or password prompt no longer holds every
  request of the app until it is answered.
- On Windows, the app no longer takes over a key that another program placed under its key name
  with a policy that allows export; it deletes that key and creates its own.

### Changed

- On Windows, the app keeps the key for request signatures open instead of opening it for every
  request.
- The refresh token is no longer kept in the memory of the app next to the access token; it is
  read from the secure storage of the device when a refresh needs it.
- In `packages/device_key`, the type with the texts of the Android confirmation prompt is named
  `StepUpPromptTexts` (before: `StepUpPrompt`, the name of a port of `packages/auth`), and the
  package has the test library `package:device_key/testing.dart`.
