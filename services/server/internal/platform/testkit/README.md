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

`testkit.PatientDatabase()` gives the database deadlines of a serve process of a test that does not
check them (`database.acquire_timeout`, `database.statement_timeout`, `database.lock_timeout`,
`database.idle_in_tx_timeout` and `http.request_timeout`, the deadline of a request that bounds
its waits for the database, each `Patience`): on a loaded runner a start step that waited longer
than the production deadlines failed the start and ended the process, a request answered 503 or
500, and a process that stalled inside a transaction lost its session (PL/56). `serveEnv`
of `internal/wire` starts every serve process with them and `harness.Start` every harness server;
a test of a deadline passes its own value (docs/DECISIONS.md PL/31, PL/37, PL/52).

The Semgrep rule `bothy-go-test-wall-clock-wait` (`.semgrep/`) refuses literal seconds or minutes
as the bound of a wait in a `_test.go` file. A bounded give-up or a latency bound under test
carries `// nosemgrep: bothy-go-test-wall-clock-wait -- <reason>`. The rule does not see a wait
bounded by a constant of the test or by `time.Sleep`.

## Timing comparisons

`testkit.TimingSame(tolerance, a, b)` compares two operations that must not be told apart by
their time (AZ-P4: an invisible and an unknown ID). Per round it runs warm-up pairs, then counted
pairs in alternating order, and compares the shortest counted duration of each side within
`TimingBound(tolerance)`; it tries up to `TimingRounds` rounds and fails only when every round
differs. Load and the scheduler only add delay, so a burst of load slows a round but cannot fail
the comparison, while a real difference lengthens the shortest run in every round.
`TimingBound` alone widens a bound under the race detector; a new timing comparison uses
`TimingSame` instead of comparing medians once. Load that lasts the whole comparison lengthens a
real, allowed difference by the same factor as the operations, so `TimingSame` runs only in a
top-level test that does not call `t.Parallel` (Semgrep rule `bothy-go-test-timing-parallel`): Go
starts the parallel tests of the package only after every such test has finished. Other packages
of the same `go test` run still load the machine; that is not excluded.
