# Troubleshooting the development setup

Failures of the gates, the image build and the app tests that come from the environment, not from
the change. Check here before you look for a cause in your code. Failures of an installation are
in the guides of [docs/operations](../operations/install-compose.md) at the step where they occur.

| Symptom | Cause | What to do |
|---|---|---|
| `make lint` or `make check` fails with `Error: parallel golangci-lint is running` | golangci-lint takes a lock file in the temporary directory; a second run on the same machine (another checkout, another session) holds it | give each run its own temporary directory: `export TMPDIR=$(mktemp -d)`, then run again |
| `make image` or `make scancode` ends at once with `docker buildx is required` | the Docker client finds no buildx plugin in its configuration directory | install the buildx plugin for the Docker client in use, or point `DOCKER_CONFIG` to a configuration directory whose `cli-plugins` has it ([CONTRIBUTING.md](../../CONTRIBUTING.md), "Development setup") |
| a `docker compose build` of a Dockerfile fails with `failed to parse platform` | Compose fell back to the classic builder, which does not know `FROM --platform=$BUILDPLATFORM` | the same as above: Compose needs buildx |
| tests of `services/host` fail on an unchanged base: `TestUnixAdapters` reads mode `0600` instead of `0640`, the Unix socket tests of `internal/startup` fail with `bind: invalid argument` | the shell runs with `umask 077`, and on macOS a socket path under a long `TMPDIR` is longer than the 104 bytes the system allows | run `umask 022` and set a short `TMPDIR` (for example `export TMPDIR=$(mktemp -d /tmp/bothy.XXXX)`) before `go test` |
| golden tests of the apps fail with "icon assets still loading" when the machine is busy | `dsPumpIcons` of `packages/design_system/lib/testing.dart` waits a fixed 10 seconds by default | run the tests again on an idle machine; a test that renders many icons may pass a longer `timeout` |
| long Flutter integration tests fail with the timeout of `package:test` (30 seconds) when the machine is busy | the test needs most of its timeout alone | run again on an idle machine; do not run two copies of the suite at once |
| `flutter test` fails with a `lipo` error "can't move temporary file" or a copy error of `NativeAssetsManifest` | two `flutter test` runs in the same package directory share `build/native_assets` | run one `flutter test` per package directory at a time; for a parallel run use a second checkout |
