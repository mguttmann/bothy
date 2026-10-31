### Fixed

- An app with another bundle identifier or application ID no longer picks up the session of an
  earlier build from a keychain or file both can reach (the macOS login keychain of a debug or
  profile build, the Windows secret file): it shows no keychain access prompts and starts with
  enrollment, and an earlier build keeps its own session. Existing secrets move once into the
  app's own place on update, so no one is signed out.
