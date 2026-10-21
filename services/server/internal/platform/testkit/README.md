# Package `platform/testkit`

Status: built (PL/8). Test helper only; tests and the test helpers (`pgtest`, the SeaweedFS
helpers) import it, no production package does.

## Purpose

One way for tests to wait, and one mark for slow integration tests. A loaded host and the race
detector slow every step by an unknown factor, so a test that bounds a correct step by its own
few seconds fails changes that are fine. Every wait for something that must happen uses
`Patience` (named value `quality.test.patience`,
[23-quality.md](../../../../../docs/spec/23-quality.md)); a condition that never holds still fails
the test, only later.

## Usage

```go
testkit.Until(t, "the job finished", func() bool { return done.Load() })
msg := testkit.Receive(t, "the notification", ch)
ctx := testkit.Context(t)               // ends after Patience or with the test
_ = conn.SetDeadline(testkit.Deadline())
```

- A wait for something that must not happen keeps its short bound: a slow host can only make it
  pass.
- A check that something stays fast makes the slow alternative take longer than `Patience` (a
  drain delay of 30 minutes, a server that holds its connections until the test ends) and
  compares the elapsed time with `Patience`.
- `testkit.Integration(t)` marks a test that needs a container runtime. `go test -short` skips
  it; `make test` and CI never pass `-short`. `pgtest.New` and the SeaweedFS helpers call it, so
  every test that gets a database or an object store carries the mark.

`testkit.PatientGateway(interval)` gives the gateway windows of a test that does not check them
(heartbeat interval `interval` with `HeartbeatMissLimit(interval)`, identify timeout `Patience`):
no heartbeat or identify deadline closes a connection before `Patience`. The gateway worlds of
`internal/wire` and the test server of `gateway/adapters/ws` start with them; the Semgrep rule `bothy-go-test-gateway-window` refuses a window of its own in a `_test.go` file, and a
test of a deadline carries a `nosemgrep` with its reason (docs/DECISIONS.md PL/22).

The Semgrep rule `bothy-go-test-wall-clock-wait` (`.semgrep/`) refuses literal seconds or minutes
as the bound of a wait in a `_test.go` file. A bounded give-up or a latency bound under test
carries `// nosemgrep: bothy-go-test-wall-clock-wait -- <reason>`. The rule does not see a wait
bounded by a constant of the test or by `time.Sleep`.
