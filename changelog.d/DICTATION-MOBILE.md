### Added

- The phone integration tests check dictation and inserts of other apps in the composer on the
  iOS simulator and an Android emulator: a keyboard extension's insert, the system dictation,
  voice typing and other input methods, the paste of the system menu and of an input method,
  the app's Paste button and the paste key of a hardware keyboard. Each shows at once, enables
  send, is saved as the draft and is sent; an insert over the paste limit becomes `paste.txt`.
  Debug builds of the app register a text input probe for these tests; profile and release
  builds do not.
