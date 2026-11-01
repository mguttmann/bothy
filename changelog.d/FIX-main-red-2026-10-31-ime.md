### Fixed

- CI: the Android phone tests of the client also disable the packages of the device's input
  methods, not only the input methods. Android enabled the default keyboard again on its
  own on API 26, so `apps (Android API 26)` still failed at random in the dictation test.
