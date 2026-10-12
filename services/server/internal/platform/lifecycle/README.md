# Package `platform/lifecycle`

Status: built (M1-11). Normative are
[02-architektur.md](../../../../../docs/spec/02-architektur.md) sections "Befehle des Binarys",
"Panics", "Start" and "Shutdown" (R-AR-16, R-AR-17); the chapter wins on any difference.

## Purpose

Runs the start and the graceful shutdown of a server process. `bothy serve` builds one `Runner`
in `internal/wire` and exits with the code of `Run`.

## Code

| File | Content |
|---|---|
| `phase.go` | `Phase`, the fixed shutdown order, `Phases`, `String` |
| `runner.go` | `Component`, `StopStep`, `Options`, `Runner` (`New`, `Add`, `OnStop`, `Fail`, `FailOn`, `Run`, `Err`), exit codes |
| `doc.go` | package documentation with an example |

## Start

Components start in registration order (`Add`). A component returns when its part runs and
leaves long-running work to its own goroutines. The first error, a panic, a `Fail`, a signal or
the end of the context of `Run` ends the start; the stop steps registered so far then run
through the plan. A component that waits for a dependency must honor its context, so a signal
during the wait ends the start promptly.

## Phases

Stop steps run in this order; steps of one phase in reverse registration order.

| Phase | Step | Roles |
|---|---|---|
| `not_ready` | readiness turns false (`shutting_down`) | all |
| `drain` | wait `shutdown.drain_delay` | api, all |
| `http` | `http.Server.Shutdown` with `shutdown.http_grace` | api, all |
| `connections` | save agent cursors, close WebSockets with `4013` (M1-65) | api, all |
| `event_tail` | stop the log tail and `LISTEN` (M1-16) | api, all |
| `consumers` | worker consumers finish their round (M2) | worker, all |
| `jobs` | River soft stop with `shutdown.jobs_soft_timeout`, then the wait for the cancelled jobs (M1-14) | worker, all |
| `telemetry` | flush within `shutdown.telemetry_flush_timeout` | all |
| `database` | close the pool | all |
| `internal_listener` | close `/healthz` and `/readyz` | all |

Roles differ only in which steps they register; the order yields the sequences of 02
"Shutdown" for `api`, `worker` and `all`.

## Budgets

- The total `TotalTimeout` (`shutdown.total_timeout`) counts from the shutdown request (the
  signal, the `Fail`, the failed start or the end of the context), so a component that is slow
  to give up its start uses up the same budget.
- Each step runs with the smaller of its `Budget` and what is left of the total; `Budget` 0
  means only the rest of the total bounds it.
- A step that outlives its budget is left behind (`shutdown step timed out`); a failing or
  panicking step is logged (`shutdown step failed`); the plan moves on in both cases.
- When the total is spent, the remaining steps are skipped and `shutdown budget exceeded`
  names the phase; the exit code is 1.
- When `Run` begins, the sum of the registered budgets must not exceed the total; otherwise
  nothing starts and `Run` returns 2 (`Err` names both values).

## Exit codes

| Code | Meaning |
|---|---|
| `0` (`ExitOK`) | graceful end after a signal or the end of the context, also during the start |
| `1` (`ExitFailure`) | failed start, `Fail`, or a shutdown over the total |
| `2` (`ExitUsage`) | step budgets above the total |
| `3` (`ExitForced`) | a second signal; no further step runs |

`Err` returns the cause of `1` and `2`; `cmd/bothy` prints it on stderr, because the log
handlers keep no error text.

## Adding a component or a stop step

A later ticket adds its part in `internal/wire` (`serve.go`, function `register`): `Add` a
component at its place in the start order and `OnStop` its step with the phase from the table
and its budget key. A step whose budget comes from configuration is registered before `Run`,
so the budget check sees it. A part that fails at run time (a panic in the log tail) calls
`Fail`; a part that reports its loss on a channel (the River client of `platform/jobs`,
`Runtime.Failed`) is watched with `FailOn(name, ch)`: an error on the channel ends the process
with exit code 1, a channel closed without a value (the orderly `Stop`) does nothing.

## Dependencies

`internal/kernel/clock` and `internal/platform/otel/attr` for the log keys.

## Tests

`runner_test.go` with a fake clock and an injected signal channel; one test signals the test
process itself to cover `os/signal`.
