# Moving to another target

Status: draft M0. Runbook for [16-operations.md](../spec/16-operations.md) 11. Applies to every move:
Mac to Linux VM, VM to cloud, cloud to cloud, container database to managed service.

![Move](diagrams/cloud-migration.svg)

## 1. Principle

A move is a restore without data loss: in maintenance mode the old instance accepts nothing more,
its last state moves completely. Therefore the sync epoch stays the same, and
clients continue with their cursor. `instance_id`, token pepper and signing key move along;
so tokens, devices and invocation tokens stay valid. `bothy restore finalize --relocation` proves
on the target that nothing was lost (section 3 step 4) before it keeps the epoch.

A move to a managed PostgreSQL service with automatic failover: a failover raises the timeline,
and the start lock holds the server not ready until `bothy restore finalize` ran
([disaster-recovery.md](disaster-recovery.md), case G). Set `database.restore_guard = off` only
where the provider documents that a failover loses no committed transaction; otherwise a failover
is handled like a restore with a new epoch ([16-operations.md](../spec/16-operations.md) 8.4).

## 2. Preparation (days before)

1. Provision the target with OpenTofu ([infra/tofu/README.md](../../infra/tofu/README.md)), same
   `BOTHY_VERSION` as the old instance.
   On a VM target the module `generic-vm` sets up the host and runs the setup wizard; its input
   `bothy_version` is the product release whose bundle pins that server version
   ([infra/tofu/modules/generic-vm/README.md](../../infra/tofu/modules/generic-vm/README.md)).
   The wizard generates fresh secrets on the target; step 2 replaces them with the bundle of the
   old instance before the first start of the restored data.
   On Hetzner Cloud, AWS and Azure the modules `hetzner`, `aws` and `azure` create the network
   (only the edge port public, SSH from `ssh_allowed_cidrs`) and the host and then call
   `generic-vm` ([infra/tofu/README.md](../../infra/tofu/README.md)). Point the DNS name at the
   address in the output `ssh_endpoint` only in step 7 of section 3. The off-site bucket of the
   target (created by `aws` with `offsite_region`, by hand on Hetzner, elsewhere for Azure) gets
   its keys by hand, as the module READMEs describe; until then the target backs up locally only.
2. Load the secrets bundle. Push keys and app identity stay the same; push registrations
   of the devices therefore stay valid.
3. Trial run: restore the latest backup of the old instance on the target
   ([backup-restore.md](backup-restore.md) 5.3), restore test green, measure the duration. The
   measured duration plus a buffer is the maintenance window.
4. Lower the DNS TTL of the hostname, at least one old TTL before the move.
5. Prepare identity:
    - OIDC: register the redirect URIs of the new host with the provider (in addition, not instead).
    - Passkeys are bound to the domain (RP ID). The same domain keeps them; a new domain
      means: register passkeys again, device keys stay valid.
    - Tailscale: prepare a new device with the same `TS_HOSTNAME`, but not yet as the target of the
      name.
6. Status page: announce the maintenance (`status_incidents.kind = maintenance`).

## 3. Move (in the maintenance window)

1. Old instance: `server.maintenance_mode = read_only`, restart `api` and `worker`
   ([ADR 0065](../adr/0065-maintenance-mode-read-only.md)). Writing actions get `503` with
   code `maintenance` and `Retry-After`; apps keep their outbox. Exceptions (ADR 0065,
   item 4):
    - Responses of ongoing runs: an agent with a run in `running` may continue to send `run.update`,
      `run.finish`, `agent.heartbeat` and `ack`, plus messages and files in the thread of
      this run. `run.start` is blocked; runs in `queued` and `waiting_input` wait and
      move along.
    - Token refresh issues new access tokens without rotating the refresh token.
    - Emergency revocations stay possible: `auth.step_up`, `session.revoke`, `device.revoke`,
      `token.revoke`, `user.disable` and the revocation of a refresh family on reuse.
    - Deliveries (`push`, `webhooks`) and the job `runs.cancel_timeout` keep running in the `worker`;
      all other queues and periodic jobs pause.

   As built (M7-08): check the mode before step 3. `getStatus` shows every component and
   `overall_state` as `maintenance`, a write such as `createMessage` answers `503` with code
   `maintenance` and the `Retry-After` of `server.maintenance_retry_after`, and the metric
   `bothy.maintenance.rejected` counts the refused requests per action. The web console stays
   logged in (its idle clock still writes), but a new login is a write and is refused, so keep a
   console or app session of the owner open for emergency revocations. The target starts with
   `server.maintenance_mode = off`: a copied configuration with `read_only` would answer every write
   there with `503 maintenance` after the switch. The commands (`bothy migrate`, `bothy restore`,
   `bothy setup`) do not read the mode.
2. Wait until no runs are active ("Running now" empty). If they run too long, the
   owner cancels them (`run.cancel`); after `agent.cancel_grace` the server sets them to
   `cancelled` itself.
3. Final backup: `pg_dump`, object backup, manifest. Note the point in time: every
   emergency revocation after it is missing on the target.
4. Target: restore of the logical dump and of the objects; compare the manifest (row counts,
   `head_seq`, audit head, file count). A difference means: abort, make the old instance writable again.
   Then save the manifest of the final backup (`backup_runs.manifest` of that run on the old
   instance) as `manifest.json` next to the Compose file and run
   `docker compose run --rm -v ./manifest.json:/tmp/manifest.json:ro migrate restore finalize --relocation /tmp/manifest.json`
   (the `migrate` container is read-only and sees only its secrets, so the file is mounted): it compares
   `event_log.head_seq`, `audit.last_seq`, `audit.last_hash` and `sync_epoch` with the restored
   database and stores the identity of the new cluster, keeping the epoch and every cursor. The
   target runs on a new cluster, so without this step it stays not ready (`restore_not_finalized`,
   [16-operations.md](../spec/16-operations.md) 8.4). Exit `1` names the differing field and changes
   nothing: abort as above, or, if the target must go live with what it has, run
   `bothy restore finalize` without the option (new epoch, clients reload).
5. Start the target, restore test by hand, smoke tests ([16-operations.md](../spec/16-operations.md) 11.1).
6. Repeat on the target the emergency revocations that happened on the old instance after the final
   backup (audit log of the old instance from the noted point in time on).
7. Switch DNS or the Tailscale name.
8. Stop the old instance (do not delete it).

## 4. After the move

1. Push notifications and webhooks that the old instance delivered after the final backup
   may be delivered a second time by the target (at least once, same `webhook-id`).
2. Clients reconnect, continue with their cursor and send their outbox.
3. Agent adapters reconnect with their consumer; their `acked_seq` has moved along.
4. Check: status page green, SLO dashboards without a drop, first backup on the target and
   first automatic restore test successful.
5. Remove the old redirect URIs at the OIDC provider.
6. Delete the old instance after a period that the owner sets (volumes, backups of the old
   target after their retention period).

## 5. New hostname

If the hostname changes, the apps do not know the new address. Until chapter 06 defines a way,
the following applies: people enter the new address; the login stays valid if
`instance_id` and signing key are the same. Planned is a hint `moved_to` in
`instance.discovery.read` of the old instance, signed with the instance key; the app accepts
it only with the same `instance_id` (question for team B).

## 6. Switching between container database and managed service

1. To the managed service: as above, restore of the logical dump into the service. The OpenTofu
   module creates the roles. `bothy-backupd` then runs without pgBackRest.
   As built (M7-16): not possible yet. Amazon RDS and Azure Database for PostgreSQL offer
   PostgreSQL 18, but `install.sh` and the Compose stack cannot use an external database, so the
   modules `aws` and `azure` refuse `database_mode = managed` (docs/DECISIONS.md D-M7-16-4).
2. Back: as above, into a container database; immediately afterwards a full backup with pgBackRest.
