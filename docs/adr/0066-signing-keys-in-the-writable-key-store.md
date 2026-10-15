---
status: accepted
date: 2026-10-10
decision-makers: Planning lead
consulted: Team F (operations), Team C (security), Team D (agent platform)
informed: all planning teams
---

# Signing keys in the writable key store

## Context and Problem Statement

The server signs access tokens, invocation tokens, outgoing webhooks and audit checkpoints with
its own keys per purpose. The private part is held only in the secret provider; the database holds
the public part and the reference (`server_signing_keys.private_key_ref`,
[03-data-model.md](../spec/03-data-model.md)). Keys go through the states `next`,
`active`, `retiring` and `retired`; the invocation token changes keys on schedule, and every purpose
needs an emergency rotation ([09-agents.md](../spec/09-agents.md), section 4.5;
[19-security.md](../spec/19-security.md), section 10).

The drivers `file` and `env` of the secret provider only read, and Compose mounts the server's
secrets read-only ([16-operations.md](../spec/16-operations.md), section 5). So the
server cannot create a new key. How are new private keys created without manual steps on the
host, without the server being able to write other secrets?

## Decision Drivers

- Scheduled rotation runs without intervention; an emergency rotation takes effect immediately.
- The private part never appears in the database, in logs or in responses (03, principle 9).
- The server gets as little write permission as possible; all other secrets stay
  read-only.
- Backup and restore cover the keys (escrow, 16 section 5.4).
- Writable drivers that come later (`azkv`, `awssm`) fit without a new decision.

## Considered Options

- Writable key store: a separate directory `secrets.signing_key_dir` that only the
  server writes, through `Put` of the file driver
- Pre-generated reserve keys: `bothy secrets init` creates keys in advance, the server
  only switches between them
- Private keys encrypted with a key encryption key in the database

## Decision Outcome

Chosen option: "Writable key store", because only it allows automatic and
emergency rotation without manual steps and limits the server's write permission to exactly one
directory.

1. `secrets.signing_key_dir` is a directory owned by the service user with mode `0700`.
   At start the server checks ownership, mode and writability; otherwise the start aborts.
2. Only the file driver can write (`Put`, `Delete`), and only under this directory; every
   other reference is read-only, `env:` never writes. `Put` creates a file with mode `0600`,
   writes atomically and never overwrites an existing name.
3. A key is Ed25519; `kid` is the fingerprint per RFC 7638; the file is named
   `<purpose>-<kid>.pem`. The server creates the file before the database row and deletes the
   private file as soon as the key is `retired` (the row stays as a record).
4. `secrets-init` no longer generates signing keys. If a purpose lacks an `active` key,
   the server creates it at start ([02-architecture.md](../spec/02-architecture.md), section
   15.1); `/readyz` fails with `signing_keys` as long as one is missing.
5. `bothy secrets export` includes the key store in the encrypted bundle.

### Consequences

- Good, because rotation and emergency rotation run without access to the host.
- Good, because a leak of the key store affects only signing keys, no
  database passwords or credentials of others.
- Bad, because the server needs a writable volume. A compromised server process
  gains little from it: it reads the active private key anyway; the only new thing is that it can
  create files in this one directory.
- Bad, because multi-node operation (level 2, [02-architecture.md](../spec/02-architecture.md)
  section 16) needs a shared volume or a writable driver (`azkv`, `awssm`);
  the ticket for multi-node operation decides that.
- Neutral, because after a crash between file and database row a file without a row can
  remain; it is never used.

### Confirmation

- Tests of build ticket M1-32 ([PLAN.md](../PLAN.md)): `TestSecretsPut_*` (only under the
  directory, mode `0600`, no overwriting), `TestStartup_SigningKeys_NotWritable` and
  `TestStartup_SigningKeys_Readiness` (start fails without a writable directory, `/readyz`
  reports `signing_keys`), `TestEnsureActive_*`, `TestEmergencyRotation_*`,
  `TestSecretsExport_IncludesKeyStore` and the canary test `TestSigningKeyCanary` (no private
  key in the database, a log, the audit, a response or an error).
- Compose mounts the volume `signing-keys` only in `api`, `worker` and, read-only, in `secrets-init`
  ([infra/compose/compose.yaml](../../infra/compose/compose.yaml)).

## Pros and Cons of the Options

### Writable key store

- Good, because the state machine from 19 runs through without manual steps.
- Good, because the write permission is limited to one directory and one kind of file.
- Bad, because an additional volume must be backed up.

### Pre-generated reserve keys

- Good, because the server writes nothing.
- Bad, because rotation without manual steps ends as soon as the reserve is used up.
- Bad, because whoever reads the secrets directory also knows every future key; an
  emergency rotation after such a leak then does not help.

### Encrypted in the database

- Good, because no additional volume is needed and multi-node operation works without a shared volume.
- Bad, because it contradicts 03: the private part is held only in the secret provider.
- Bad, because a database dump together with the encryption key reveals all keys,
  including the earlier ones.

## More Information

- [16-operations.md](../spec/16-operations.md), sections 5.2 to 5.4 and 14 (`secrets.signing_key_dir`);
  [19-security.md](../spec/19-security.md), sections 10 and 16 (`signing_key.*`);
  [09-agents.md](../spec/09-agents.md), section 4.5.
- Related: ADR 0011 (secret provider), ADR 0020 (device binding, access tokens), ADR 0023
  (invocation token), ADR 0021 (audit checkpoints).
