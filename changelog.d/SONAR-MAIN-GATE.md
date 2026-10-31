### Security

- The standalone Android build of the plugin `device_key` locks its resolved dependencies: every
  configuration and the build script classpath resolve exactly the versions in the committed
  `gradle.lockfile` and `buildscript-gradle.lockfile` files, and a check fails when a Gradle build
  of the repository has no lock file. This clears the vulnerability that had kept the SonarQube
  Cloud quality gate of `main` red since 2026-10-25.
