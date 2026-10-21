# Moving to another target

Status: draft M0. Runbook for [16-operations.md](../spec/16-operations.md) 11. Applies to every move:
Mac to Linux VM, VM to cloud, cloud to cloud, container database to managed service.

![Move](diagrams/cloud-migration.svg)

## 1. Principle

A move is a restore without data loss: in maintenance mode the old instance accepts nothing more,
its last state moves completely. Therefore the sync epoch stays the same, and
clients continue with their cursor. `instance_id`, token pepper and signing key move along;
so tokens, devices and invocation tokens stay valid.

## 2. Preparation (days before)

1. Provision the target with OpenTofu ([infra/tofu/README.md](../../infra/tofu/README.md)), same
   `BOTHY_VERSION` as the old instance.
   On a VM target the module `generic-vm` sets up the host and runs the setup wizard; its input
   `bothy_version` is the product release whose bundle pins that server version
   ([infra/tofu/modules/generic-vm/README.md](../../infra/tofu/modules/generic-vm/README.md)).
   The wizard generates fresh secrets on the target; step 2 replaces them with the bundle of the
   old instance before the first start of the restored data.
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
2. Wait until no runs are active ("Running now" empty). If they run too long, the
   owner cancels them (`run.cancel`); after `agent.cancel_grace` the server sets them to
   `cancelled` itself.
3. Final backup: `pg_dump`, object backup, manifest. Note the point in time: every
   emergency revocation after it is missing on the target.
4. Target: restore of the logical dump and of the objects; compare the manifest (row counts,
   `head_seq`, audit head, file count). A difference means: abort, make the old instance writable again.
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
2. Back: as above, into a container database; immediately afterwards a full backup with pgBackRest.
