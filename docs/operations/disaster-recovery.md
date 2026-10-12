# Disaster recovery

Status: draft M0. Runbook for [16-operations.md](../spec/16-operations.md) 9. Targets:
`dr.rpo_target` (largest data loss) and `dr.rto_target` (time until operation resumes). The
automatic restore test measures the duration on every run.

## 1. Be prepared

| Item | Where | Check |
|---|---|---|
| Secrets bundle (age), current | offline with the owner, separate from the off-site target | test decryption after every change |
| Private age key of the owner | second place (for example a password manager with emergency access) | yearly |
| Off-site access for reading | in the secrets bundle | the restore test reads from there |
| Configuration (`.env`, overlays, OpenTofu variables) | repository of the instance operator | on every change |
| External alert path | Alertmanager `external-oncall`, watchdog | the watchdog heartbeat arrives |
| This runbook offline | copy next to the secrets bundle | yearly |

## 2. Decision tree

1. Is the host reachable and the volume `pgdata` readable? No: case B.
2. Is data wrong (deleted, damaged) and is the time of the error known? Yes: case A.
3. Have keys leaked? Yes: case D or E, then check whether A or B is needed.
4. Is the off-site target missing? Case C.

## 3. Cases

### Case A: wrong data, healthy host

1. Create an incident on the status page.
2. Determine the point in time: audit log (`audit.read`), logs, trace of the faulty request.
3. PITR according to [backup-restore.md](backup-restore.md) 5.2.
4. After the start: update the status page, inform the affected people (lost period).

### Case B: host lost

1. Provision a new host (same OpenTofu module or a new VM), same `BOTHY_VERSION`.
2. Decrypt the secrets bundle and place it in `infra/compose/secrets/`
   (`mkdir -p infra/compose/secrets && age --decrypt -i <key file> bundle.age | tar -x -C infra/compose/secrets/`;
   the directory is ignored by Git and missing in a fresh checkout); `secrets-init`
   takes over the files instead of generating new ones. If the volume `secrets` already exists
   (for example after a first start that generated new values), `secrets-init` aborts on differing
   generated values; then
   `docker compose run --rm secrets-init secrets init --source /input --target /run/bothy/secrets --owner server=65532:65532 --owner migrate=65532:65532 --adopt-source`
   takes over the values of the bundle. When `secrets-init` reports the entries as `adopted`,
   `updated` or `kept`, delete the decrypted generated files from `infra/compose/secrets/`: `_store` holds
   them, and the source needs only the supplied ones. The same token pepper and the same signing
   keys mean: tokens and devices stay valid.
3. `docker compose up -d secrets-init postgres backup`, then restore from repo2 according to
   [backup-restore.md](backup-restore.md) 5.2 from step 4 (target: latest state).
4. Restore the objects from restic.
5. `bothy restore finalize`, restore test, then start all services.
6. Point DNS or the Tailscale device to the new host. With `tailscale` the device is new; keep
   `TS_HOSTNAME` the same, remove the old device in the tailnet administration.
7. Record the duration and compare it with `dr.rto_target`.

### Case C: off-site target lost or compromised

1. Create a new target (different credentials), switch `BOTHY_BACKUP_OFFSITE_TARGET`.
2. Immediately a full backup and a logical dump, then a restore test.
3. On compromise: rotate the backup keys (new repositories, delete the old ones after a period),
   because the attacker can read old backups with the old key.

### Case D: signing key compromised

1. `signing_key.rotate` with `mode = emergency` for the affected purpose (owner, step-up): all
   keys of the purpose become `retired` immediately, a new one becomes `active`. If the key store
   (`secrets.signing_key_dir`) itself was affected, rotate all purposes.
2. Revoke all sessions (`session.revoke` for all) for `access_token`; for `invocation` the
   rotation is enough, because invocation tokens are only valid for `agent.invocation_token_ttl`;
   for `webhook_out` inform the receivers.
3. Search the audit log for use of the old key.

### Case E: token pepper compromised

1. Provide a new pepper as a file, restart the server.
2. Consequence: all API tokens, refresh tokens and incoming webhook tokens are invalid. The owner
   issues new tokens for agents and apps; people log in again.
3. A database dump without the pepper reveals no usable tokens (R-DM-06); with the pepper it does.
   Therefore: check whether the database has leaked too.

### Case F: ransomware or takeover of the host

1. Disconnect the host from the network, do not repair it. Treat it as case B, with a new host.
2. Rotate all secrets (cases D and E, database passwords, regenerate the push keys at the
   provider, new OIDC client secret).
3. Restore from the off-site target; it is append-only and therefore intact.
4. Choose a point in time before the takeover (case A) if data could have been manipulated.

## 4. After each case

1. Close the incident with cause, duration and lost period.
2. Post-mortem: what would have warned earlier? New alerts or checks as a ticket.
3. Restore test and watchdog green again.
