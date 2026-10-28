### Fixed

- The Windows CI jobs no longer fail when the Chocolatey feed is down. They read the Flutter pin
  of the root `pubspec.yaml` themselves and pass it to the Flutter setup as a version, so the
  setup no longer installs `yq` from Chocolatey (PL/60).
