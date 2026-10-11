---
status: accepted
date: 2026-10-10
decision-makers: Planning lead, Team F (operations)
consulted: Team A (architecture), Team C (security)
informed: all teams
---

# 12-factor configuration and secret provider interface

## Context and Problem Statement

Bothy runs on a Mac with Compose, on Linux VMs and in clouds (B12). Every target delivers
configuration and secrets differently: files, environment variables, Azure Key Vault, AWS Secrets
Manager. The data model stores third-party credentials only as a reference `secret_ref`
([03-data-model.md](../spec/03-data-model.md) 1.9). How does the server get configuration and
secrets without secrets ending up in images, logs or telemetry and without a target
changing the code?

## Decision Drivers

* the same images on all targets, differences only in configuration (B12)
* secrets never in environment variables, logs, error messages or OpenTofu state
* rotation of secrets without reinstallation
* typos must never silently set an insecure default
* operations without an additional mandatory service on a single host

## Considered Options

* Environment plus optional file; secrets only as references through a secret provider interface
* Only environment variables, secrets directly as values
* Configuration in the database, managed only through the admin console
* Mandatory external secret store (for example a Vault service)

## Decision Outcome

Chosen option: "Environment plus optional file; secrets only as references through a
secret provider interface", because it works on every target without a further service and
still keeps secrets out of the environment and logs.

Rules (details [16-operations.md](../spec/16-operations.md) 5):

1. Process configuration from `BOTHY_*` environment variables and optionally a YAML file; precedence
   environment over file over default. Key `a.b_c` is named `BOTHY_A__B_C`.
2. Unknown keys and invalid values abort the start.
3. Secrets are references `<scheme>:<path>`. Drivers in M1: `file`, `env`; later `azkv`,
   `awssm`. Interface `Get` and optionally `Put`; read cache `secrets.cache_ttl`.
4. Instance settings and feature flags stay in the database (admin console, step-up,
   audit); they are not process configuration.
5. In the Compose stack, `secrets-init` creates missing secrets and creates a
   subdirectory per service; every service sees only its own.

### Consequences

* Good, because a target only delivers configuration and the code stays the same.
* Good, because secrets as files per service are visible neither in `docker inspect` nor in child
  processes.
* Good, because rotation takes effect through the cache without a restart.
* Bad, because `secrets-init` is a separate building block with permissions to write.
* Bad, because the configuration reference is kept in line with the chapters by hand until M1.
* Neutral, because platforms without files (some PaaS) can use `env:`, with the known
  drawbacks.

### Confirmation

* Unit test: an unknown `BOTHY_*` key and type errors abort the start.
* The canary test ([privacy.md](../observability/privacy.md) 6) contains secrets as canary words;
  they must not appear in any log or any telemetry.
* Generator of the configuration reference from the schema; a check compares schema, reference and
  chapter tables.
* `docker compose config` in the gate; a check that no service sets secrets as an environment value.

## Pros and Cons of the Options

### Environment plus file, secrets as references

* Good, because it follows 12-factor and secrets still live as files or in stores.
* Good, because the reference looks the same in the data model (`secret_ref`) and in the
  configuration.
* Bad, because two ways (value and reference) must be explained.

### Only environment variables with values

* Good, because simplest and available everywhere.
* Bad, because values get into child processes, crash reports and `docker inspect`.
* Bad, because rotation needs a restart.

### Configuration in the database

* Good, because the admin console could change everything, with audit.
* Bad, because the server needs the database credentials before the database (chicken and egg).
* Bad, because a console error can bring operations to a halt.

### Mandatory external secret store

* Good, because rotation, audit and access permissions live in the store.
* Bad, because a single host would then have to run a further highly available service.
* Neutral, because stores still remain possible as later drivers.

## More Information

* Configuration reference: [docs/operations/configuration.md](../operations/configuration.md).
* Storage of the secrets and consequences of losing them: [16-operations.md](../spec/16-operations.md) 5.4.
* Follow-up ADR proposed: drivers for Azure Key Vault and AWS Secrets Manager, as soon as a
  cloud target needs them.
