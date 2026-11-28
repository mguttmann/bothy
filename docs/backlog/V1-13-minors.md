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
- The `OVERLAPPED` of `connectOnce` lies on the goroutine stack while `ConnectNamedPipe` is
  pending (`internal/localchan/pipe_windows.go`, the same in `internal/confirm/peer_windows.go`);
  a stack shrink between the wait and `CancelIoEx` could leave a closed listener's `Accept`
  waiting. Not reproduced; a heap-allocated `Overlapped` and event would remove the window.
- `ListenPipe` does not close the event `closed` when the first instance fails, and a real error
  of `WaitForMultipleObjects` is reported as `net.ErrClosed`.
- D-M2-42-29 (`docs/decisions.d/M2-42.md`) could point to V1-13/9 and AUD-misc-misc-01, which
  now carry its state.
- `TestConnectionEndsWhileIdle` (`internal/computermcp/faults_test.go:61`) failed once in the unit
  step of the macOS platform job (run 46128962469, "after the close: ... connection_lost") and
  passed in the runs before; the package is not part of V1-13. A flake to look at when that
  package is next touched.
