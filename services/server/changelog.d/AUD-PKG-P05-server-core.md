### Fixed

- Every 401 problem carries a `WWW-Authenticate` challenge; `step_up_required` uses the RFC 9470
  error `insufficient_user_authentication`.
- Only the gateway and the three terminal upgrade routes may skip the per-route stages of the
  HTTP chain; the chain refuses the in-band marker on any other route.
- A token check waiting for the signing key cache returns when its own context ends instead of
  waiting for a hanging reload.
- Retention deletes in transactions of at most 32 rows, so a large batch no longer overflows the
  subtransaction cache of PostgreSQL.
- A COMMIT already sent is no longer aborted by a canceled request, so a durable transaction
  always runs its after-commit work.
- River's job timeout counts as outcome `failed` instead of `cancelled`.
- The loss line of the LISTEN connection names the error class instead of only the failed step.
- Cost retention deletes a correction chain newest ID first, so it also completes with
  `retention.batch_size = 1`.
- Stats minutes are flushed in key order, so concurrent flushes of several processes no longer
  deadlock.
- The head and pending count of a webhook subscription, the retention of webhook deliveries and
  push registrations and the grant and membership expiry sweep read partial indexes instead of
  the whole history (migrations 00113 to 00115, 00117, 00118).
- The import mirror ends with exit 2 on a Discord token that is refused or a revoked session
  instead of retrying it, notes Discord channels created after its start and asks a channel that
  answered 403 again in its next round.
- The import tool retries a 5xx or a broken connection of Discord and its CDN three times, and a
  full `--apply` run stops after the current batch on SIGINT or SIGTERM and frees the lease.
- `bothy import login` and `logout` read the tool configuration (`--config`, `BOTHY_IMPORT__`
  variables).
- `bothy setup` exits with 2 on a configuration that does not load.
- The route generator cuts the strict request and response objects into as many parts as they
  need below the file limit and fails itself before a generated file passes the limit.
- templatecopy replaces the embedded agent templates only when the new copy is complete.

### Changed

- A failed job attempt logs `job attempt failed` with its kind and error class (Error on the last
  attempt); River's routine log lines move to debug.
- The unused indexes on `webhook_deliveries.next_attempt_at` and `agents.last_heartbeat_at` are
  dropped (migrations 00116, 00119).
- `agent.disable` no longer reads the status components in its transaction.
- The sqlc code of every module holds only the table structs its own queries use, and the
  generated files of identity, authz, agents, chat and terminal end in `.gen.go`.
- A depguard rule keeps test helpers and container libraries out of production code.
- The help of `bothy-backupd backup` and `restore-test` no longer promises exit 1 for a run of
  the service.
