### Changed

- The platform suite of the host service is a required job (`host platform`) of the pull request
  CI and of the build branches for changes of `services/host/` and of
  `.github/workflows/host-platform.yml`, in the public phase; `host-platform.yml` is a reusable
  workflow that also runs weekly on `main` (AUD-PKG-R2-06-host-platform-ci).

### Fixed

- `host-platform.yml` removes the default ACL the hosted Ubuntu image gives `/opt`; every folder
  below inherited it and was writable by every user, so the host service refused its start
  (AUD-PKG-R2-06/1).
