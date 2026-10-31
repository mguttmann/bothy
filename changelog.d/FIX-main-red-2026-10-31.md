### Fixed

- CI: the heavy Linux jobs free disk space right after checkout (`tools/ci/free_disk.sh`).
  The runner image ships .NET, GHC, Swift, Boost, CodeQL bundles and Docker images that no
  job uses; with them the disk ran full on main ("No space left on device" in `gates`,
  `test (tools)` and `console-e2e`). Jobs without a JDK, Android SDK, Flutter or emulator
  also drop the Android SDK.
- CI: the Android phone tests of the client run with every input method of the device
  disabled and enable them again afterwards. The emulator's keyboard edited the composer at
  the same time as the dictation test's text input probe, so `apps (Android API 26)` failed
  at random.
