# Environments and feature flags

Status: draft M0. Guide to [16-operations.md](../spec/16-operations.md) 6.

## 1. Environments

| Environment | Who | Where | Data | Images |
|---|---|---|---|---|
| `dev` | development | workstation, Compose | synthetic (seed data of the tests) | built locally or any version |
| `staging` | acceptance before stable | own host or cloud, Compose | synthetic or imported from a test source | release candidate by digest |
| `prod` | operations | target system | real | release by digest |

### 1.1 What may differ

Only configuration: `instance.environment`, `server.public_url`, secrets, push environment
(APNs sandbox in `dev`), telemetry overlay, backup schedules. Code, images and migrations
are the same.

### 1.2 What `instance.environment` does

| Effect | `dev` | `staging` | `prod` |
|---|---|---|---|
| HTTP without TLS on loopback for clients | allowed | no | no |
| Banner in the apps | "Dev" | "Staging" | none |
| `deployment.environment.name` in telemetry | `dev` | `staging` | `prod` |
| Warning without an off-site target | notice | warning in the admin console | warning in the admin console at every login of the owner |
| Seed commands (`bothy dev seed`) | allowed | no | no |

### 1.3 Promotion

1. Beta (`server/vX.Y.Z-beta.N`) to `staging`; portability test via a product beta
   (`vA.B.C-beta.M`) whose bundle sets `BOTHY_VERSION` to this server beta
   (`make release-prep COMPONENT=bothy SERVER_VERSION=X.Y.Z-beta.N`,
   [18-release.md](../spec/18-release.md) 2.1; [16-operations.md](../spec/16-operations.md) 11.1), and
   acceptance. The numbers of the two tags are not related: every component has its own
   version ([18-release.md](../spec/18-release.md) 1).
2. Proven: stable (`X.Y.Z`) from the same commit to `staging`, a short check, then the same
   digest to `prod` ([18-release.md](../spec/18-release.md) 5). New installations only get the
   stable server version with a stable product release whose bundle pins it; `make release-build`
   rejects a server beta in the bundle of a stable product
   ([18-release.md](../spec/18-release.md) 2.1).
3. Every environment has its own secrets, its own `instance_id`, its own push keys.
4. Production data never leaves `prod`. The owner investigates errors that only occur with real
   data in `prod` with `access.view_as` and traces, not with a copy.

## 2. Feature flags

Registration and cleanup gate: [ADR 0064](../adr/0064-feature-flag-registration-with-a-cleanup-gate.md).

### 2.1 Kinds

| Kind | Purpose | Lifetime | Example |
|---|---|---|---|
| `release` | roll out a new feature step by step | until `remove_by` (a version in the code) | new live box display |
| `option` | permanent product switch of the owner | permanent, documented in [configuration.md](configuration.md) 7 | `dm.enabled` |

Flags are not configuration: no numbers, no limits, no security boundaries, no
secrets. A flag may only switch a feature on or off.

### 2.2 Registration in code

Every flag is registered in one place in the server with: key, kind, description (English,
for the console), default, for `release` the version `remove_by`, and the responsible module.
The server evaluates unknown keys in the table `feature_flags` as off and shows them in
the console as "orphaned".

### 2.3 Evaluation

```
value = flag.enabled
if principal or one of its valid groups is in flag.rollout:
    value = not flag.enabled
```

The server evaluates, never the client. Clients receive the values valid for them via REST
(`instance.read`) and reload when `instance.updated` names a changed key.
Flags act on agents and apps as on people; `rollout`, however, only knows people and groups.

### 2.4 Admin console

| View | Content |
|---|---|
| List | key, kind, description, global value, number of exceptions, last change |
| Detail | exceptions (people, groups), history from the audit log |
| Change | `feature_flag.update`: owner, step-up, reason, audit |

### 2.5 Cleanup rule

1. A `release` flag names `remove_by`. A gate check fails when the version in
   `VERSION` of the server reaches `remove_by` and the flag is still registered.
2. Whoever removes the flag removes code and registration and deletes the row in the same
   change via a migration.
3. A `release` flag is never reinterpreted as `option`; if a switch is to stay permanently,
   a new `option` flag with its own key is created.
