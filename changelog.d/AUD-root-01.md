### Security

- The advisory gate `make osv` and the license gate `make licenses` read the Gradle lock files
  of the Android builds, which no gate read before: a Maven package that ships in the Android
  app (the release runtime classpath and the desugaring library) now fails for a known
  vulnerability or a license outside the list. Packages that only the build and its tests
  resolve are counted and do not fail. A test holds every tracked lock file to the gate.
- The Android app takes Guava 33.7.2, which fixes GHSA-xxph-c9ww-hj94 (memory exhaustion while
  reading a serialized collection); the new gate reported it for both lock files of the app.

### Changed

- `make lint` runs ShellCheck over every tracked shell script instead of two folders, bandit
  over every tool folder, and mypy also reads `tools/testing`; a test fails for a new tool
  folder outside these lists.
- The license of `desugar_jdk_libs` in the Android app (GPL-2.0 with the Classpath Exception)
  is accepted by the license gate until a date; whether the project accepts it for good is the
  open question OQ-27.

### Fixed

- The installation guide for Docker Compose describes how to run an own instance from the source
  until the first release (both images, the two `.env` values, the update) and no longer calls
  the stack impossible to start.
- `CONTRIBUTING.md` names what each component needs, shows the first `make check` with the
  switches a fork needs and describes the short ways to run less than every gate.
