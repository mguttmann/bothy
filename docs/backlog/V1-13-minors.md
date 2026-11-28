# Minors of V1-13 (platform runs of the host service)

Collected, not fixed on their own; the next run in `services/host/test/platform` reads and takes
them along.

- `removeWindowsService` (`test/platform/os_test.go`) only logs when a service is still installed
  after its delete; the next `install` then fails with its own message. Failing the test there
  would name the cause one step earlier.
- golangci-lint with `--build-tags platform` reports G204 on the `exec.CommandContext` of
  `stageUnixImpostor` (moved, not new); `make lint` does not lint the platform tag.
- `TestSudoProbeMatchesSudo` cannot catch a regression on a host without passwordless sudo (both
  answers are "no" there); it does on the hosted runners and on hosts with such a rule.
