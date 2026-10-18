# Chaos tests of the event path (`services/server/test/chaos`)

Status: built (M1-72). Component `testsupport` (M1-05), build tag `perf`: `make perf` runs the
tests, `make check` only compiles, vets and lints them.

Each test starts `bothy serve --role=all` as a child process (`harness.StartProcess`) on a durable
database (`pgtest.Durable()`), with every rate limit raised, and builds a world through the public
API (`NewWorld`): one channel, `Writers` people with write who post into its main thread and
`Receivers` people with read who hold gateway connections. A writer repeats an unacknowledged send
with the same `client_msg_id` and the same `Idempotency-Key` until the server answers `201` or
`200` (backoff `retryPause` doubling to `retryPauseMax`; after the stop at most `finishWait`). A
receiver whose connection ends connects again with the last seq it saw as `cursor`.

| Test | Fault | Parameters |
|---|---|---|
| `TestChaos_ListenConnectionLoss` | `pg_terminate_backend` of the `LISTEN` backend of the log tail (`application_name` `bothy-event-tail`), `listenKills` times, `killEvery` apart | `afterFault`; delivery bound: `sync.log_poll_interval` plus `database.listen_reconnect_backoff_cap` plus `listenSlack` |
| `TestChaos_DatabaseRestartDuringTail` | restart of the PostgreSQL container on the same host port (`pgtest.DB.Restart`) while writers and the tail run | `beforeRestart`, `afterFault`, `restartSettle` |
| `TestChaos_SigkillDuringCommit` | `SIGKILL` of the server process at random moments under write load, then a restart | `kills`, `killAfterMin`, `killAfterMax`, `killSettle` |

After each test `CheckInvariants` (`invariants.go`, read-only queries through `AdminDB()`) holds:

1. `seq` in `event_log` is gapless from 1 and the counter `event_seq` equals its maximum;
2. every message has exactly one `message.created` event and every such event its message;
3. per thread `thread_seq` is gapless from 1 and `threads.last_message_seq` equals its maximum;
4. every acknowledged send exists exactly once, and no attempted send exists twice;
5. no `idempotency_keys` row is left `in_progress`;
6. `verifyAuditChain` reports a valid chain;
7. every receiver saw every `message.created` of the channel after its first `ready`, and never a
   new seq below one it had seen (a repeated seq is a duplicate and dropped by seq).

The receivers keep their connections until every acknowledged message arrived (a receiver
reconnects only when the server ended its connection, as after `SIGKILL`), so a log tail that never
resumes cannot hide behind the catch-up of a reconnect. After the fault the load runs for the
delivery bound (`sync.log_poll_interval` plus `database.listen_reconnect_backoff_cap` plus
`listenSlack`) plus `afterFault`, and `CheckDeliveredWithin` requires every message acknowledged
after the last fault at every receiver within that bound (`TestChaos_ListenConnectionLoss`,
`TestChaos_DatabaseRestartDuringTail`); losing `LISTEN` must also leave every gateway connection
open.
